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
# Запуск из каталога messenger (нужны секреты из README.md в корне, «В Docker»):
#   scripts/pool-load.sh [клиентов] [секунд] [размеры пула…] > ../labs/2a/results/messenger-pool.txt
# По умолчанию — 8 клиентов, 20 с, пулы 10, 2 и 1. Стек остаётся поднятым с пулом по
# умолчанию; остановить — docker compose stop.
set -euo pipefail
export LC_ALL=C.UTF-8

n=${1:-8}
secs=${2:-20}
shift $(($# < 2 ? $# : 2))
sizes=("$@")
[ ${#sizes[@]} -gt 0 ] || sizes=(10 2 1)
# Период выборки: \watch в одном сеансе psql, а не новый процесс на каждую выборку —
# иначе запуск docker exec стоил бы больше, чем сам запрос, и выборок было бы мало.
interval=0.05
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

echo "# Пул соединений под нагрузкой чтением: L, λ и W закона Литтла (урок 2.9)"
echo "# $(date -u +%Y-%m-%dT%H:%MZ), $(docker compose version | head -n1), Docker Engine $(docker version -f '{{.Server.Version}}'), $(uname -sr)"
echo "# машина: $(grep -m1 'model name' /proc/cpuinfo | cut -d: -f2 | sed 's/^ //'), $(nproc) логических процессоров; стенд messenger/compose.yaml (cpus: 1, mem_limit: 256m)"
echo "# Воспроизвести: cd messenger && scripts/pool-load.sh $n $secs ${sizes[*]} > ../labs/2a/results/messenger-pool.txt"
echo

docker compose up -d --build --wait >"$dir/up.log" 2>&1 || { cat "$dir/up.log" >&2; fail "стек не поднялся"; }

for size in "${sizes[@]}"; do
	echo "=== пул MESSENGER_DB_MAX_CONNS=$size: $n клиентов читают историю чата $secs с ==="
	MESSENGER_DB_MAX_CONNS=$size docker compose up -d --wait --force-recreate --no-deps messenger >"$dir/up.log" 2>&1 ||
		{ cat "$dir/up.log" >&2; fail "messenger с пулом $size не стал healthy"; }

	scripts/read-load.sh "$n" "$secs" >"$dir/load.txt" 2>"$dir/load.err" &
	load=$!
	# Выборки — только пока идёт само чтение: подготовка read-load.sh (регистрация,
	# 50 сообщений) — другая нагрузка. Строка «читаю:» печатается прямо перед стартом.
	for _ in $(seq 1 600); do
		grep -q 'читаю:' "$dir/load.txt" 2>/dev/null && break
		kill -0 "$load" 2>/dev/null || break
		sleep 0.1
	done
	grep -q 'читаю:' "$dir/load.txt" || { cat "$dir/load.err" >&2; fail "read-load.sh не начал чтение"; }
	# Окно выборки — на секунду короче чтения с каждой стороны: без разгона и хвоста.
	sleep 1
	count=$(awk -v s="$secs" -v i="$interval" 'BEGIN { printf "%d", (s - 2) / i }')
	printf '%s\n\\watch i=%s c=%s\n' "$sample_sql" "$interval" "$count" |
		docker compose exec -T db psql -X -q -A -t -F ' ' >"$dir/samples.txt"
	wait "$load" || { cat "$dir/load.err" >&2; fail "read-load.sh завершился с ошибкой"; }

	sed 1d "$dir/load.txt" # первая строка — id пользователя и чата, они от прогона к прогону разные
	rps=$(awk '/^запросов:/ { print $(NF - 2) }' "$dir/load.txt")
	[ -n "$rps" ] || fail "нет строки «запросов:» в выводе read-load.sh"
	# region little
	awk -v rps="$rps" -v size="$size" '
		NF == 2 { k++; sum += $1; if ($1 > max) max = $1; if ($2 > open) open = $2; c[$1]++ }
		END {
			if (k == 0) { print "ПРОВАЛ: нет выборок" > "/dev/stderr"; exit 1 }
			L = sum / k
			printf "выборок pg_stat_activity: %d (раз в %s с)\n", k, "'"$interval"'"
			printf "соединений с запросом (active): в среднем L = %.2f, максимум %d из %d\n", L, max, size
			printf "соединений открыто пулом: до %d из %d\n", open, size
			printf "доля выборок по числу active:"
			for (a = 0; a <= max; a++) printf " %d — %.0f %%;", a, 100 * c[a] / k
			printf "\n"
			printf "W = L / λ = %.2f / %s = %.3f мс на запрос API (два запроса к базе: чат и страница истории)\n", L, rps, 1000 * L / rps
		}' "$dir/samples.txt"
	# endregion little
	echo
done

# Стек — обратно с умолчанием бинаря, как его поднимает docker compose up.
docker compose up -d --wait --force-recreate --no-deps messenger >"$dir/up.log" 2>&1 ||
	{ cat "$dir/up.log" >&2; fail "messenger с пулом по умолчанию не стал healthy"; }
echo "стек оставлен поднятым с MESSENGER_DB_MAX_CONNS по умолчанию (10)"
