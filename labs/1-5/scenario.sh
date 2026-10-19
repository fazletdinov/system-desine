#!/bin/bash
# Сценарный прогон лабы 1.5: запускает каждую программу и проверяет ключевые строки вывода.
# Падает на первом расхождении; check:labs запускает его из каталога labs/.
# Пороги выбраны с запасом под раннер в 2–4 ядра: проверяется порядок величины, а не точное число.
# Проверки написаны как «условие || fail»: fail завершает скрипт, так что «A && B || C» тут ветвление.
# shellcheck disable=SC2015
set -euo pipefail

cd "$(dirname "$0")/.."

fail() {
	echo "ПРОВАЛ: $*" >&2
	exit 1
}

ok() {
	echo "ОК: $*"
}

bin=$(mktemp -d)
trap 'rm -rf "$bin"' EXIT

build() { # build <имя> <пакет> [флаги go build…]
	local name=$1 pkg=$2
	shift 2
	go build "$@" -o "$bin/$name" "$pkg"
}

# first_number <текст> <регулярка с одной группой (…)> — число из первого совпадения.
first_number() {
	printf '%s\n' "$1" | sed -nE "s/^.*$2.*$/\1/p" | head -n 1
}

# less <a> <b> — a < b для чисел с точкой.
less() { awk -v a="$1" -v b="$2" 'BEGIN { exit !(a < b) }'; }

# has_header <вывод> — первые три строки шапки: версия Go, GOMAXPROCS, NumCPU.
has_header() {
	printf '%s\n' "$1" | head -n 3 | grep -Eq '^go version: go1\.[0-9]+' &&
		printf '%s\n' "$1" | sed -n 2p | grep -Eq '^GOMAXPROCS: [0-9]+$' &&
		printf '%s\n' "$1" | sed -n 3p | grep -Eq '^NumCPU: [0-9]+$'
}

# --- 1. sched/cost: цена горутины и потока ---
build cost ./1-5/sched/cost
out=$("$bin/cost")
has_header "$out" || fail "sched/cost: нет шапки go version / GOMAXPROCS / NumCPU:
$out"
stack=$(first_number "$out" 'стек на горутину: ([0-9]+) байт')
spawn=$(first_number "$out" 'создание одной горутины: ([0-9]+) нс')
[ -n "$stack" ] && [ "$stack" -ge 1024 ] && [ "$stack" -le 16384 ] || fail "sched/cost: стек на горутину $stack байт, ждали 1–16 КиБ:
$out"
[ -n "$spawn" ] && [ "$spawn" -lt 100000 ] || fail "sched/cost: создание горутины $spawn нс, ждали меньше 100 мкс:
$out"
echo "$out" | grep -Eq 'рост RSS на поток: [0-9]+ КиБ' || fail "sched/cost: нет строки про RSS потока:
$out"
ok "sched/cost: горутина — $stack байт стека и $spawn нс на создание"

# --- 2. sched/preempt: асинхронное вытеснение (Go 1.14) ---
build preempt ./1-5/sched/preempt
# Одна повторная попытка: на шумном раннере main может проснуться поздно и без нашей вины.
for _ in 1 2; do
	out=$("$bin/preempt")
	with=$(first_number "$out" 'main проснулся через ([0-9]+) мс')
	if [ -n "$with" ] && [ "$with" -lt 100 ]; then break; fi
done
out_off=$(GODEBUG=asyncpreemptoff=1 "$bin/preempt")
without=$(first_number "$out_off" 'main проснулся через ([0-9]+) мс')
[ -n "$with" ] && [ "$with" -lt 100 ] || fail "sched/preempt: с вытеснением main проснулся через $with мс, ждали меньше 100:
$out"
[ -n "$without" ] && [ "$without" -ge 1000 ] || fail "sched/preempt: с asyncpreemptoff=1 main проснулся через $without мс, ждали не меньше 1000:
$out_off"
ok "sched/preempt: main просыпается через $with мс; с asyncpreemptoff=1 — через $without мс"

# --- 3. sched/syscall: блокирующий системный вызов занимает поток ---
build syscall ./1-5/sched/syscall
out=$("$bin/syscall")
sys_plus=$(first_number "$out" 'syscall\.Nanosleep x[0-9]+: .*\(\+([0-9]+)\)')
sleep_plus=$(first_number "$out" 'time\.Sleep +x[0-9]+: .*\(\+([0-9]+)\)')
[ -n "$sys_plus" ] && [ "$sys_plus" -ge 2 ] || fail "sched/syscall: syscall.Nanosleep добавил $sys_plus потоков, ждали не меньше 2:
$out"
[ -n "$sleep_plus" ] && [ "$sleep_plus" -le 1 ] || fail "sched/syscall: time.Sleep добавил $sleep_plus потоков, ждали не больше 1:
$out"
ok "sched/syscall: Nanosleep +$sys_plus потоков, time.Sleep +$sleep_plus"

# --- 4. chan/ops: таблица операций над каналами ---
build ops ./1-5/chan/ops
out=$("$bin/ops")
for want in 'panic: close of nil channel' 'panic: send on closed channel' 'panic: close of closed channel' 'ок (v=0, ok=false)' 'блокирует навсегда'; do
	echo "$out" | grep -qF "$want" || fail "chan/ops: в таблице нет «$want»:
$out"
done
ok "chan/ops: таблица содержит три паники, чтение из закрытого и вечную блокировку"

# --- 5. chan/timers: time.After в цикле и один Timer ---
build timers ./1-5/chan/timers
out=$("$bin/timers")
after_mib=$(first_number "$out" 'time\.After в цикле: .*выделено за прогон +([0-9.]+) МиБ')
reuse_mib=$(first_number "$out" 'один Timer \+ Reset: .*выделено за прогон +([0-9.]+) МиБ')
after_heap=$(first_number "$out" 'time\.After в цикле: .*после +([0-9.]+) МиБ, выделено')
[ -n "$after_mib" ] && less 100 "$after_mib" || fail "chan/timers: time.After выделил $after_mib МиБ, ждали больше 100:
$out"
[ -n "$reuse_mib" ] && less "$reuse_mib" 1 || fail "chan/timers: Timer+Reset выделил $reuse_mib МиБ, ждали меньше 1:
$out"
[ -n "$after_heap" ] && less "$after_heap" 64 || fail "chan/timers: куча после time.After $after_heap МиБ — таймеры не собираются:
$out"
ok "chan/timers: time.After — $after_mib МиБ за прогон, куча в конце $after_heap МиБ; Timer+Reset — $reuse_mib МиБ"

# --- 6. race/flagloop: гонка за флагом (labdemo) ---
build flagloop ./1-5/race/flagloop -tags labdemo
out=$("$bin/flagloop")
echo "$out" | grep -Eq '^обычный bool: +(завершилось|зависло)$' || fail "race/flagloop: нет итога для обычного bool:
$out"
echo "$out" | grep -Eq '^atomic\.Bool: +завершилось$' || fail "race/flagloop: версия с atomic.Bool не завершилась:
$out"
plain=$(first_number "$out" 'обычный bool: +([а-я]+)')
build flagloop-race ./1-5/race/flagloop -tags labdemo -race
set +e
race_out=$("$bin/flagloop-race" 2>&1)
race_code=$?
set -e
echo "$race_out" | grep -q 'WARNING: DATA RACE' || fail "race/flagloop -race: нет WARNING: DATA RACE:
$race_out"
[ "$race_code" -ne 0 ] || fail "race/flagloop -race: код выхода 0, ждали ненулевой"
ok "race/flagloop: обычный bool — $plain, atomic.Bool — завершилось, -race видит гонку"

# --- 7. race/iface: гонка за переменную-интерфейс (labdemo) ---
build iface ./1-5/race/iface -tags labdemo
# Порча интерфейса — гонка на удачу: одна попытка из трёх должна упасть (обычно падает каждая).
# Обе проверки iface идут с GOMAXPROCS=4: на одном P программа не падает, а -race не печатает
# ни одного отчёта (стек прошлого обращения успевает уйти из истории детектора, урок 1.5.3,
# «Память у него конечная»). На раннере с одним vCPU четыре P всё равно чередуются потоками ОС.
crashed=no
for _ in 1 2 3; do
	set +e
	out=$(GOMAXPROCS=4 "$bin/iface" 2>&1)
	code=$?
	set -e
	if [ "$code" -ne 0 ] && echo "$out" | grep -Eq 'SIGSEGV|fatal error|panic:'; then
		crashed=yes
		break
	fi
done
[ "$crashed" = yes ] || fail "race/iface: три запуска без падения:
$out"
build iface-race ./1-5/race/iface -tags labdemo -race
set +e
race_out=$(GOMAXPROCS=4 "$bin/iface-race" 2>&1)
set -e
echo "$race_out" | grep -q 'WARNING: DATA RACE' || fail "race/iface -race: нет WARNING: DATA RACE:
$race_out"
ok "race/iface: без -race программа падает ($(echo "$out" | grep -Eo 'SIGSEGV|fatal error|panic:' | head -n 1)), с -race — WARNING: DATA RACE"

# --- 8. sync/starve: режим голодания Mutex ---
build starve ./1-5/sync/starve
out=$("$bin/starve")
light_n=$(first_number "$out" 'быстрая горутина: +захватов ([0-9]+)')
light_max=$(first_number "$out" 'быстрая горутина: .*макс\. ожидание ([0-9.]+) мс')
[ -n "$light_n" ] && [ "$light_n" -gt 0 ] || fail "sync/starve: быстрая горутина не получила замок:
$out"
[ -n "$light_max" ] && less "$light_max" 250 || fail "sync/starve: максимальное ожидание быстрой горутины '$light_max' мс, ждали меньше 250:
$out"
ok "sync/starve: быстрая горутина получила замок $light_n раз, ждала не дольше $light_max мс"

# --- 9. patterns/leak: утечка горутин (labdemo) ---
build leak ./1-5/patterns/leak -tags labdemo
out=$("$bin/leak")
leaks=$(printf '%s\n' "$out" | sed -nE 's/^утекло горутин: ([0-9]+)$/\1/p')
first_leak=$(printf '%s\n' "$leaks" | sed -n 1p)
second_leak=$(printf '%s\n' "$leaks" | sed -n 2p)
[ "$first_leak" = 0 ] || fail "patterns/leak: исправленная версия утекла $first_leak горутин:
$out"
[ -n "$second_leak" ] && [ "$second_leak" -gt 0 ] || fail "patterns/leak: исходная версия не утекла ('$second_leak'):
$out"
echo "$out" | grep -q 'goroutineleak profile: total' || fail "patterns/leak: нет профиля goroutineleak:
$out"
ok "patterns/leak: исходная версия утекла $second_leak горутин, исправленная — $first_leak"

# --- 10. patterns/limit: горутина на задачу и семафор ---
build limit ./1-5/patterns/limit
out=$("$bin/limit")
free_peak=$(first_number "$out" 'горутина на задачу: .*пик горутин +[0-9]+ \(из них свои: +([0-9]+)\)')
sem_peak=$(first_number "$out" 'семафор на канале: .*пик горутин +[0-9]+ \(из них свои: +([0-9]+)\)')
sem_ms=$(first_number "$out" 'семафор на канале: +время +([0-9]+) мс')
[ -n "$free_peak" ] && [ "$free_peak" -ge 150 ] || fail "patterns/limit: пик без ограничения $free_peak, ждали не меньше 150:
$out"
[ -n "$sem_peak" ] && [ "$sem_peak" -le 12 ] || fail "patterns/limit: пик с семафором $sem_peak, ждали не больше 12:
$out"
[ -n "$sem_ms" ] && [ "$sem_ms" -ge 400 ] || fail "patterns/limit: семафор уложился в $sem_ms мс — 200 задач по 20 мс при лимите 8 не могут быстрее 500:
$out"
ok "patterns/limit: пик $free_peak горутин без лимита и $sem_peak с семафором на 8 ($sem_ms мс)"

# --- 11. бенчмарки: работоспособность (числа снимаются отдельно, см. README) ---
out=$(go test -run='^$' -bench=. -benchtime=100ms ./1-5/sync/ ./1-5/sched/ 2>&1) || fail "бенчмарки упали:
$out"
for name in BenchmarkCounterMutex BenchmarkCounterMutexRead BenchmarkCounterRWMutexRead BenchmarkCounterAtomic BenchmarkFalseSharingPacked BenchmarkFalseSharingPadded BenchmarkSpawnGoroutine BenchmarkSwitchGoroutines BenchmarkSwitchOSThreads; do
	echo "$out" | grep -q "^$name" || fail "в бенчмарках нет $name:
$out"
done
echo "$out" | grep -E '^Benchmark'
ok "бенчмарки sync и sched отработали"

echo "Лаба 1.5: все проверки прошли."
