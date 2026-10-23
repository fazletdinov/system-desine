#!/bin/bash
# Лаба 0.4: задержки памяти, SSD, fsync и сети на этой машине.
#   bash 0-4/scenario.sh           # полный прогон (~3 минуты, нужно ~4,5 ГБ свободного места), пишет 0-4/results/*.txt
#   bash 0-4/scenario.sh --check   # короткий прогон для check:labs (~1 минута): только порядок величины,
#                                  # results/ не трогает
# Запуск из labs/ или откуда угодно: скрипт сам переходит в labs/. Нужны Go и Docker.
# Каталог для тестовых файлов диска — LAB04_DATA (по умолчанию 0-4/data); он должен быть на настоящем
# диске, не в tmpfs: иначе fio и pg_test_fsync измерят память.
set -euo pipefail
# Вывод по-русски: без UTF-8-локали sed и awk режут кириллицу по байтам.
export LC_ALL=C.UTF-8

cd "$(dirname "$0")/.."

fail() {
	echo "ПРОВАЛ: $*" >&2
	exit 1
}

ok() {
	echo "ОК: $*"
}

check=0
case "${1:-}" in
--check) check=1 ;;
"") ;;
*) fail "неизвестный аргумент $1; есть только --check" ;;
esac

command -v go >/dev/null || fail "не найден go: нужен Go 1.27 (labs/go.mod)"
command -v docker >/dev/null || fail "не найден docker: диск и сеть меряются в контейнерах (fio, pg_test_fsync, ping) — установите Docker"
docker info >/dev/null 2>&1 || fail "Docker установлен, но демон недоступен (не запущен или нет прав на /var/run/docker.sock)"

# Полный прогон — длительности из урока: файл fio 4 ГБ (больше любого DRAM-кэша SSD на запись
# раскладки и заведомо не «горячий»), 20 с на замер, 5 с на каждый метод pg_test_fsync, 50 пингов.
if [ "$check" = 1 ]; then
	fio_size=256m fio_secs=2 fsync_secs=1 pings=10 hot_pings=200
	out=$(mktemp -d)
else
	fio_size=4g fio_secs=20 fsync_secs=5 pings=50 hot_pings=500
	out=0-4/results
	mkdir -p "$out"
fi

export LAB04_UID LAB04_GID LAB04_DATA
LAB04_UID=$(id -u)
LAB04_GID=$(id -g)
LAB04_DATA=${LAB04_DATA:-$PWD/0-4/data}
# Каталог создаём сами: иначе его создаст Docker от root, и контейнер с вашим UID не сможет писать.
mkdir -p "$LAB04_DATA"
LAB04_DATA=$(realpath "$LAB04_DATA")
fstype=$(df --output=fstype "$LAB04_DATA" | tail -n 1)
case "$fstype" in
tmpfs | ramfs | overlay | overlayfs)
	fail "каталог данных $LAB04_DATA лежит на $fstype — это память или слой контейнера, а не диск.
Укажите каталог на настоящем диске: LAB04_DATA=/путь/на/диске bash 0-4/scenario.sh"
	;;
esac

# --progress quiet: строки «Container … Created» иначе попали бы в results/ вместе с выводом замера.
compose=(docker compose --progress quiet -p lab04-scenario -f 0-4/compose.yaml)
bin=$(mktemp -d)
cleanup() {
	# Тестовый файл fio — гигабайты на диске: не оставляем его ни при успехе, ни при ошибке.
	rm -f "$LAB04_DATA"/fio-test.dat "$LAB04_DATA"/ssd_*.json "$LAB04_DATA"/pg_test_fsync.out "$LAB04_DATA"/.odirect-probe
	"${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
	rm -rf "$bin"
	if [ "$check" = 1 ]; then rm -rf "$out"; fi
}
trap cleanup EXIT

# --- шапка: без описания машины числа нельзя сравнить с вашими ---
src=$(df --output=source "$LAB04_DATA" | tail -n 1)
# lsblk -s идёт от раздела (или LVM-тома) вниз, к физическому диску.
disk=$(lsblk -s -n -r -o NAME,TYPE "$src" 2>/dev/null | awk '$2 == "disk" { print $1; exit }')
header=$(
	# shellcheck source=/dev/null
	. /etc/os-release
	cpu=$(lscpu | sed -nE 's/^Model name: +//p' | head -n 1)
	echo "машина: $cpu, $(nproc) логических процессоров, ОЗУ $(free -h | awk '/^Mem:/ { print $2 }'), $PRETTY_NAME, ядро $(uname -r)"
	echo "go: $(go version)"
	echo "docker: $(docker version --format '{{.Server.Version}}')"
	echo "каталог данных: ФС $fstype на $src, диск: $(lsblk -d -n -o NAME,MODEL "/dev/$disk" | tr -s ' '), ROTA=$(lsblk -d -n -o ROTA "/dev/$disk" | tr -d ' '), TRAN=$(lsblk -d -n -o TRAN "/dev/$disk" | tr -d ' ')"
	# Все диски, кроме loop (major 7): по ROTA видно, есть ли HDD (1 — вращающийся).
	echo "диски (lsblk -d -o NAME,MODEL,ROTA,TRAN):"
	lsblk -d -e 7 -o NAME,MODEL,ROTA,TRAN | sed 's/^/  /'
	# THP: при always куча Go может попасть на страницы 2 МиБ, и промахов TLB в замере RAM станет меньше.
	echo "transparent hugepages: $(cat /sys/kernel/mm/transparent_hugepage/enabled 2>/dev/null || echo нет данных)"
	echo "дата: $(date -u +%Y-%m-%dT%H:%MZ)"
	echo
)

# save <файл> <вывод> — шапка плюс вывод замера; строки RESULT — в конце вывода.
save() {
	printf '%s\n%s\n' "$header" "$2" >"$out/$1"
}

# result <файл> <ключ> — значение из строки «RESULT <ключ> <значение> <единица>».
result() {
	local v
	v=$(awk -v k="$2" '$1 == "RESULT" && $2 == k { print $3 }' "$out/$1")
	[ -n "$v" ] || fail "$1: нет строки RESULT $2"
	echo "$v"
}

# in_range <файл> <ключ> <от> <до> <единица> — порядок величины, а не точное число: машины разные.
in_range() {
	local v
	v=$(result "$1" "$2")
	awk -v v="$v" -v lo="$3" -v hi="$4" 'BEGIN { exit !(v >= lo && v <= hi) }' ||
		fail "$1: $2 = $v $5, ждали $3–$4 $5 — проверьте, что замер шёл по нужному уровню (см. README)"
	ok "$2 = $v $5 (ждали $3–$4)"
}

check_flag=()
[ "$check" = 1 ] && check_flag=(-check)

# less_than <файл> <ключ a> <ключ b> — a < b: уровни памяти идут по возрастанию задержки.
less_than() {
	local a b
	a=$(result "$1" "$2")
	b=$(result "$1" "$3")
	awk -v a="$a" -v b="$b" 'BEGIN { exit !(a < b) }' ||
		fail "$1: $2 = $a не меньше $3 = $b — замер попал не в тот уровень кэша (см. README)"
	ok "$2 < $3 ($a < $b)"
}

# --- 1. память: Go, один поток на одном ядре, без Docker ---
go build -o "$bin/mem" ./0-4/mem
# taskset -c 2 закрепляет замер за одним ядром: без этого планировщик может перенести поток на
# другое ядро с холодными L1 и L2 (а у гибридных CPU — на E-ядро). Ядро 2, а не 0: на ядро 0
# ОС обычно направляет больше прерываний.
pin=()
command -v taskset >/dev/null && pin=(taskset -c 2)
o=$("${pin[@]}" "$bin/mem" -mode latency "${check_flag[@]}") || fail "mem -mode latency упал:
$o"
save mem-latency.txt "$o"
less_than mem-latency.txt l1_random_ns l2_random_ns
less_than mem-latency.txt l2_random_ns ram_random_ns
result mem-latency.txt l2_512k_random_ns >/dev/null
in_range mem-latency.txt ram_random_ns 30 300 нс
o=$("${pin[@]}" "$bin/mem" -mode seq "${check_flag[@]}") || fail "mem -mode seq упал:
$o"
save mem-seq-1mb.txt "$o"
in_range mem-seq-1mb.txt ram_seq_1mb_us 10 500 мкс

# --- 2. SSD: fio в контейнере, файл на диске хоста, O_DIRECT, один запрос в полёте ---
b=$("${compose[@]}" build -q fio 2>&1) || fail "сборка образа fio упала:
$b"
rm -f "$LAB04_DATA/fio-test.dat"
fio_run() {
	local file=$1 key=$2 rw=$3 bs=$4 o code=0
	o=$("${compose[@]}" run --rm -T fio "$key" "$rw" "$bs" "$fio_size" "$fio_secs" 2>&1) || code=$?
	[ "$code" = 3 ] && fail "$o"
	[ "$code" = 0 ] || fail "fio $key упал (код $code):
$o"
	save "$file" "$o"
	# JSON fio — рядом с результатом: в нём все перцентили и гистограмма, а не только p50/p99.
	[ "$check" = 1 ] || cp "$LAB04_DATA/$key.json" "$out/${file%.txt}.fio.json"
}
fio_run ssd-randread-4k.txt ssd_rand4k randread 4k
fio_run ssd-seqread-1m.txt ssd_seq1m read 1M
rm -f "$LAB04_DATA/fio-test.dat"
in_range ssd-randread-4k.txt ssd_rand4k_lat_p50_us 5 2000 мкс
for k in p50_us p99_us mean_us lat_p50_us lat_p99_us lat_mean_us; do
	result ssd-randread-4k.txt "ssd_rand4k_$k" >/dev/null
	result ssd-seqread-1m.txt "ssd_seq1m_$k" >/dev/null
done

# --- 3. fsync: pg_test_fsync из образа PostgreSQL 18.6 на том же каталоге ---
o=$("${compose[@]}" run --rm -T fsync -f /data/pg_test_fsync.out -s "$fsync_secs" 2>&1) ||
	fail "pg_test_fsync упал:
$o"
# Строки раздела «one 8kB write». fsync_ms — fdatasync: это wal_sync_method по умолчанию в
# PostgreSQL на Linux, а одна запись 8 КБ — одна страница WAL при COMMIT. fsync_full_ms — fsync
# той же записи: он сбрасывает ещё и метаданные файла (время изменения), а на ext4 это коммит
# журнала ФС. Время — из ops/sec: в usecs/op pg_test_fsync округляет до целых микросекунд.
ops_of() {
	printf '%s\n' "$o" | awk -v m="$1" '/using one 8kB write/ { s = 1 } /using two 8kB writes/ { s = 0 } s && $1 == m { print $2; exit }'
}
ops=$(ops_of fdatasync)
ops_full=$(ops_of fsync)
if [ -z "$ops" ] || [ -z "$ops_full" ]; then
	fail "в выводе pg_test_fsync нет строк fdatasync и fsync:
$o"
fi
save fsync.txt "$o

строки замера: раздел «Compare file sync methods using one 8kB write»; fsync_ms — метод fdatasync
(wal_sync_method по умолчанию на Linux), fsync_full_ms — метод fsync
RESULT fsync_ms $(awk -v o="$ops" 'BEGIN { printf "%.3f", 1000 / o }') ms
RESULT fsync_full_ms $(awk -v o="$ops_full" 'BEGIN { printf "%.3f", 1000 / o }') ms"
in_range fsync.txt fsync_ms 0.01 50 мс
result fsync.txt fsync_full_ms >/dev/null

# --- 4. сеть: ping между двумя контейнерами одной bridge-сети и по localhost ---
# Два режима. Плотный (основные ключи): пауза 2 мс — процессор не успевает глубоко уснуть, это ближе
# к цене самого сетевого стека. Редкий (*_idle_us): пауза 0,2 с — между пакетами ядро уходит в
# глубокий C-state, и в RTT входит пробуждение; так выглядит RTT редкого запроса. Плотный режим —
# с паузой меньше 0,2 с, busybox ping разрешает его только root: в контейнере pinger он root.
# Число — медиана времени по строкам «time=… ms» каждого пакета, а не среднее итоговой строки:
# единичные выбросы на сотни микросекунд (прерывание, вытеснение) сдвигают среднее, а не медиану.
median_us() {
	printf '%s\n' "$1" | sed -nE 's/^.* time=([0-9.]+) ms$/\1/p' | sort -g |
		awk '{ v[NR] = $1 } END { if (NR) printf "%.1f", v[int((NR + 1) / 2)] * 1000 }'
}
# ping_run <адрес> <пакетов> <пауза> — вывод ping из контейнера pinger; проверяет, что ответы были.
ping_run() {
	local o
	o=$("${compose[@]}" run --rm -T pinger ping -c "$2" -i "$3" "$1" 2>&1) || fail "ping $1:
$o"
	[ -n "$(median_us "$o")" ] || fail "ping $1: нет ни одной строки time=:
$o"
	printf '%s\n' "$o"
}
ch=$(ping_run peer "$hot_pings" 0.002)
lh=$(ping_run 127.0.0.1 "$hot_pings" 0.002)
ci=$(ping_run peer "$pings" 0.2)
li=$(ping_run 127.0.0.1 "$pings" 0.2)
save net-rtt.txt "Плотный режим (пауза 2 мс) — нижняя граница сетевого стека одной машины, это не ЦОД:
пакет не выходит в провод, а проходит veth → мост docker → veth внутри одного ядра.
Число — медиана по пакетам; ниже — итоговые строки ping (все $hot_pings строк пакетов не печатаем).

\$ ping -c $hot_pings -i 0.002 peer        # из контейнера pinger в контейнер peer, одна bridge-сеть
$(printf '%s\n' "$ch" | tail -n 3)
медиана: $(median_us "$ch") мкс

\$ ping -c $hot_pings -i 0.002 127.0.0.1   # loopback внутри контейнера pinger
$(printf '%s\n' "$lh" | tail -n 3)
медиана: $(median_us "$lh") мкс

Редкий режим (пауза 0,2 с): между пакетами процессор засыпает, в RTT входит его пробуждение.

\$ ping -c $pings -i 0.2 peer
$ci
медиана: $(median_us "$ci") мкс

\$ ping -c $pings -i 0.2 127.0.0.1
$li
медиана: $(median_us "$li") мкс

RESULT rtt_container_us $(median_us "$ch") us
RESULT rtt_localhost_us $(median_us "$lh") us
RESULT rtt_container_idle_us $(median_us "$ci") us
RESULT rtt_localhost_idle_us $(median_us "$li") us"
in_range net-rtt.txt rtt_container_us 5 500 мкс
in_range net-rtt.txt rtt_localhost_us 1 500 мкс
result net-rtt.txt rtt_container_idle_us >/dev/null
result net-rtt.txt rtt_localhost_idle_us >/dev/null

[ "$check" = 1 ] || ok "результаты записаны в $out/"
echo "Лаба 0.4: все проверки прошли."
