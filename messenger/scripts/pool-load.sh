#!/bin/bash
# Закон Литтла на живом стенде (урок 2.9): сколько соединений пула на самом деле занято
# под нагрузкой чтением и хватает ли пула меньше умолчания.
#
# Для каждого размера пула скрипт пересоздаёт контейнер messenger с
# MESSENGER_DB_MAX_CONNS=<размер>, запускает scripts/read-load.sh и, пока идёт чтение,
# 20 раз в секунду спрашивает базу, сколько соединений мессенджера сейчас выполняют
# запрос (pg_stat_activity, state = 'active'). Среднее по выборкам — L закона Литтла,
# запросы в секунду из read-load.sh — λ, их частное — W: сколько один запрос API держит
# соединение с базой.
#
# База видит только своё время: пока ответ идёт по сети к мессенджеру и pgx разбирает
# строки, процесс сервера уже «idle», а соединение ещё не вернулось в пул. Поэтому L по
# pg_stat_activity — нижняя оценка занятости пула. Верхнюю даёт прогон с пулом из одного
# соединения: если он держит тот же поток запросов, то по закону Литтла L ≤ 1 и
# W ≤ 1 / λ уже с учётом сети и клиента.
#
# Порядок прогонов чередуется по раундам (раунд r — размеры со сдвигом на r, нечётные
# раунды ещё и в обратном порядке): при одном прогоне каждого размера в фиксированном
# порядке эффект порядка (прогретые кэши базы и ОС, частота процессора, рост таблиц от
# подготовки read-load.sh) неотличим от эффекта пула — урок 1.5.10. Перед первым раундом —
# прогрев, его числа не печатаются.
#
# Для каждого прогона скрипт снимает счётчики cgroup v2 cpu.stat обоих контейнеров до и
# после чтения: процессор мессенджера и базы на запрос API и доля периодов квоты, в которых
# мессенджер упёрся в cpus: 1 (nr_throttled / nr_periods). У базы квоты нет.
#
# Запуск из каталога messenger (нужны секреты из README.md в корне, «В Docker»):
#   scripts/pool-load.sh [клиентов] [секунд] [раундов] [размеры пула…] > ../labs/2a/results/messenger-pool.txt
# По умолчанию — 8 клиентов, 20 с, 6 раундов, пулы 10, 2 и 1. Стек остаётся поднятым с
# пулом по умолчанию; остановить — docker compose stop.
set -euo pipefail
export LC_ALL=C.UTF-8

n=${1:-8}
secs=${2:-20}
rounds=${3:-6}
shift $(($# < 3 ? $# : 3))
sizes=("$@")
[ ${#sizes[@]} -gt 0 ] || sizes=(10 2 1)
# Период выборки: \watch в одном сеансе psql, а не новый процесс на каждую выборку —
# иначе запуск docker exec стоил бы больше, чем сам запрос, и выборок было бы мало.
interval=0.05
warmup=10
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT

fail() {
	echo "ПРОВАЛ: $*" >&2
	exit 1
}

# region sample
# Соединения мессенджера: клиентские процессы базы messenger, кроме сеанса самой
# выборки. active — выполняет запрос прямо сейчас; total — сколько соединений пул
# открыл к этому моменту (простаивающие тоже держат процесс сервера).
sample_sql="SELECT count(*) FILTER (WHERE state = 'active'), count(*)
FROM pg_stat_activity
WHERE datname = 'messenger' AND backend_type = 'client backend' AND pid <> pg_backend_pid()"
# endregion sample

# cpustat СЕРВИС — строка «usage_usec nr_periods nr_throttled» из cgroup v2 контейнера.
cpustat() {
	local id f
	id=$(docker compose ps -q "$1")
	f=/sys/fs/cgroup/system.slice/docker-$id.scope/cpu.stat
	[ -r "$f" ] || fail "нет $f (нужен cgroup v2 и драйвер systemd)"
	awk '$1 == "usage_usec" { u = $2 } $1 == "nr_periods" { p = $2 } $1 == "nr_throttled" { t = $2 }
		END { print u + 0, p + 0, t + 0 }' "$f"
}

# measure РАЗМЕР СЕКУНД — один прогон; пишет в $dir/run строку
# «λ медиана p99 L max открыто W CPU_м CPU_б троттлинг_% не_200».
measure() {
	local size=$1 dur=$2 load count rps total cpu0 cpu1 db0 db1
	MESSENGER_DB_MAX_CONNS=$size docker compose up -d --wait --force-recreate --no-deps messenger >"$dir/up.log" 2>&1 ||
		{ cat "$dir/up.log" >&2; fail "messenger с пулом $size не стал healthy"; }

	scripts/read-load.sh "$n" "$dur" >"$dir/load.txt" 2>"$dir/load.err" &
	load=$!
	# Выборки и счётчики — только пока идёт само чтение: подготовка read-load.sh
	# (регистрация, 50 сообщений) — другая нагрузка. «читаю:» печатается прямо перед стартом.
	for _ in $(seq 1 600); do
		grep -q 'читаю:' "$dir/load.txt" 2>/dev/null && break
		kill -0 "$load" 2>/dev/null || break
		sleep 0.1
	done
	grep -q 'читаю:' "$dir/load.txt" || { cat "$dir/load.err" >&2; fail "read-load.sh не начал чтение"; }
	cpu0=$(cpustat messenger)
	db0=$(cpustat db)
	# Окно выборки начинается через секунду после старта и заказано на dur − 2 с: каждый
	# такт \watch длится interval плюс время самого запроса (доли миллисекунды), так что
	# окно выходит чуть длиннее заказанного и всё равно кончается до конца чтения.
	sleep 1
	count=$(awk -v s="$dur" -v i="$interval" 'BEGIN { printf "%d", (s - 2) / i }')
	printf '%s\n\\watch i=%s c=%s\n' "$sample_sql" "$interval" "$count" |
		docker compose exec -T db psql -X -q -A -t -F ' ' >"$dir/samples.txt"
	wait "$load" || { cat "$dir/load.err" >&2; fail "read-load.sh завершился с ошибкой"; }
	cpu1=$(cpustat messenger)
	db1=$(cpustat db)

	rps=$(awk '/^запросов:/ { print $(NF - 2) }' "$dir/load.txt")
	total=$(awk '/^запросов:/ { print $2 }' "$dir/load.txt")
	[ -n "$rps" ] || fail "нет строки «запросов:» в выводе read-load.sh"
	grep -q '^200 ' "$dir/load.txt" || fail "нет ответов 200 в выводе read-load.sh"
	# region little
	awk -v rps="$rps" -v total="$total" -v size="$size" -v c0="$cpu0" -v c1="$cpu1" -v d0="$db0" -v d1="$db1" -v other="$(grep -vc '^200 \|^запросов:\|читаю:' "$dir/load.txt" || true)" '
		FNR == NR { if ($1 == "200") { med = $5; p99 = $8 } next }
		NF == 2 { k++; sum += $1; if ($1 > max) max = $1; if ($2 > open) open = $2 }
		END {
			if (k == 0) { print "ПРОВАЛ: нет выборок" > "/dev/stderr"; exit 1 }
			split(c0, a); split(c1, b); split(d0, x); split(d1, y)
			L = sum / k
			periods = b[2] - a[2]
			thr = 0
			if (periods > 0) thr = 100 * (b[3] - a[3]) / periods
			printf "%s %s %s %.3f %d %d %.3f %.0f %.0f %.0f %d\n",
				rps, med, p99, L, max, open, 1000 * L / rps,
				(b[1] - a[1]) / total, (y[1] - x[1]) / total,
				thr, other
		}' "$dir/load.txt" "$dir/samples.txt" >"$dir/run"
	# endregion little
}

echo "# Пул соединений под нагрузкой чтением: L, λ и W закона Литтла, $rounds раундов вперемешку (урок 2.9)"
echo "# $(date -u +%Y-%m-%dT%H:%MZ), $(docker compose version | head -n1), Docker Engine $(docker version -f '{{.Server.Version}}'), $(uname -sr)"
echo "# машина: $(grep -m1 'model name' /proc/cpuinfo | cut -d: -f2 | sed 's/^ //'), $(nproc) логических процессоров, частота: $(cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor 2>/dev/null || echo '?'); стенд messenger/compose.yaml (messenger: cpus: 1, mem_limit: 256m; db: без квоты CPU)"
echo "# Воспроизвести: cd messenger && scripts/pool-load.sh $n $secs $rounds ${sizes[*]} > ../labs/2a/results/messenger-pool.txt"
echo "#"
echo "# Столбцы: λ — запросов API в секунду (read-load.sh, $n клиентов, $secs с); медиана и p99 ответа, мс;"
echo "# L — среднее число соединений мессенджера в состоянии active (выборка pg_stat_activity раз в $interval с);"
echo "# max — наибольшее L в выборке; откр — сколько соединений пул открыл; W = L / λ, мс — время базы на запрос API;"
echo "# CPU м и CPU б — процессор мессенджера и базы на запрос API, мкс (cgroup cpu.stat, usage_usec);"
echo "# трот — доля периодов квоты мессенджера с троттлингом (nr_throttled / nr_periods), %."
echo

docker compose up -d --build --wait >"$dir/up.log" 2>&1 || { cat "$dir/up.log" >&2; fail "стек не поднялся"; }
measure "${sizes[0]}" "$warmup"
echo "прогрев: пул ${sizes[0]}, $warmup с — не учитывается"
echo

printf '%-5s %-4s %6s %7s %7s %5s %3s %4s %6s %6s %6s %5s\n' раунд пул λ медиана p99 L max откр W "CPU м" "CPU б" трот
m=${#sizes[@]}
for r in $(seq 0 $((rounds - 1))); do
	order=()
	for i in $(seq 0 $((m - 1))); do order+=("${sizes[$(((i + r) % m))]}"); done
	if [ $((r % 2)) -eq 1 ]; then
		rev=()
		for ((i = m - 1; i >= 0; i--)); do rev+=("${order[$i]}"); done
		order=("${rev[@]}")
	fi
	for size in "${order[@]}"; do
		measure "$size" "$secs"
		read -r rps med p99 L max open W msg_cpu db_cpu thr other <"$dir/run"
		[ "$other" -eq 0 ] || fail "раунд $((r + 1)), пул $size: ответы не 200 — $(cat "$dir/load.txt")"
		printf '%-5s %-4s %6s %7s %7s %5s %3s %4s %6s %6s %6s %5s\n' "$((r + 1))" "$size" "$rps" "$med" "$p99" "$L" "$max" "$open" "$W" "$msg_cpu" "$db_cpu" "$thr"
		echo "$size $rps $med $p99 $L $W $msg_cpu $db_cpu $thr" >>"$dir/all"
	done
done

echo
echo "Сводка по размеру пула: медиана (минимум–максимум) по $rounds раундам"
printf '%-4s %-20s %-20s %-23s %-23s %-17s %-17s %s\n' пул λ "p99, мс" L "W, мс" "CPU м, мкс" "CPU б, мкс" "трот, %"
for size in "${sizes[@]}"; do
	line="$size"
	for col in 2 4 5 6 7 8 9; do
		line+=" $(awk -v s="$size" -v c="$col" '$1 == s { print $c }' "$dir/all" | sort -g | awk '
			{ v[NR] = $1 } END { med = NR % 2 ? v[(NR + 1) / 2] : (v[NR / 2] + v[NR / 2 + 1]) / 2
				printf "%.4g(%s–%s)", med, v[1], v[NR] }')"
	done
	# shellcheck disable=SC2086 # разбиение на столбцы — намеренно
	printf '%-4s %-20s %-20s %-23s %-23s %-17s %-17s %s\n' $line
done

# Стек — обратно с умолчанием бинаря, как его поднимает docker compose up.
docker compose up -d --wait --force-recreate --no-deps messenger >"$dir/up.log" 2>&1 ||
	{ cat "$dir/up.log" >&2; fail "messenger с пулом по умолчанию не стал healthy"; }
echo
echo "стек оставлен поднятым с MESSENGER_DB_MAX_CONNS по умолчанию (10)"
