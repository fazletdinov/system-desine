#!/bin/bash
# Сценарный прогон лабы 1.5: запускает каждую программу и проверяет ключевые строки вывода.
# Падает на первом расхождении; check:labs запускает его из каталога labs/.
# Пороги выбраны с запасом под раннер в 2–4 ядра: проверяется порядок величины, а не точное число.
# Проверки написаны как «условие || fail»: fail завершает скрипт, так что «A && B || C» тут ветвление.
# shellcheck disable=SC2015
set -euo pipefail
# Вывод программ и шаблоны — по-русски: без UTF-8-локали sed и grep режут кириллицу по байтам.
export LC_ALL=C.UTF-8

cd "$(dirname "$0")/.."

fail() {
	echo "ПРОВАЛ: $*" >&2
	exit 1
}

ok() {
	echo "ОК: $*"
}

bin=$(mktemp -d)
# Фоновые процессы (сервер и клиент netpoll) не должны пережить сценарий, даже если проверка упала.
cleanup() {
	local pids
	pids=$(jobs -p)
	[ -z "$pids" ] || kill $pids 2>/dev/null || true
	rm -rf "$bin"
}
trap cleanup EXIT

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
loop_off=$(first_number "$out_off" 'цикл работал ([0-9]+) мс')
[ -n "$with" ] && [ "$with" -lt 100 ] || fail "sched/preempt: с вытеснением main проснулся через $with мс, ждали меньше 100:
$out"
# Цикл откалиброван по числу итераций, а не по часам: на раннере он короче секунды.
# Без вытеснения main ждёт конца цикла, поэтому сравниваем с длительностью цикла (допуск 50 мс).
[ -n "$without" ] && [ -n "$loop_off" ] && [ "$without" -ge 500 ] && [ $((without + 50)) -ge "$loop_off" ] || fail "sched/preempt: с asyncpreemptoff=1 main проснулся через $without мс при цикле $loop_off мс, ждали конца цикла (не меньше 500 мс):
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
plain=$(first_number "$out" 'обычный bool: +(завершилось|зависло)')
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
echo "$out" | grep -q '^ожидания быстрой горутины: <0,1 мс: [0-9]' || fail "sync/starve: нет строки с гистограммой ожиданий:
$out"
ok "sync/starve: быстрая горутина получила замок $light_n раз, ждала не дольше $light_max мс"

# --- 9. patterns/leak: утечка горутин (labdemo) ---
build leak ./1-5/patterns/leak -tags labdemo
out=$("$bin/leak")
leaks=$(printf '%s\n' "$out" | sed -nE 's/^утекло горутин: ([0-9]+)$/\1/p')
fixed_leak=$(printf '%s\n' "$leaks" | sed -n 1p)
group_leak=$(printf '%s\n' "$leaks" | sed -n 2p)
orig_leak=$(printf '%s\n' "$leaks" | sed -n 3p)
[ "$fixed_leak" = 0 ] && [ "$group_leak" = 0 ] || fail "patterns/leak: исправленные версии утекли ($fixed_leak, $group_leak):
$out"
[ -n "$orig_leak" ] && [ "$orig_leak" -gt 0 ] || fail "patterns/leak: исходная версия не утекла ('$orig_leak'):
$out"
echo "$out" | grep -q 'goroutineleak profile: total' || fail "patterns/leak: нет профиля goroutineleak:
$out"
ok "patterns/leak: исходная версия утекла $orig_leak горутин, исправленные — $fixed_leak и $group_leak"

# Тесты на synctest: errgroup-версия проходит, исходная падает, потому что горутины пережили вызов.
go test -count=1 -tags labdemo -run '^TestProcessWorkItemsGroup$' ./1-5/patterns/leak/ >/dev/null ||
	fail "patterns/leak: TestProcessWorkItemsGroup не прошёл"
if out=$(go test -count=1 -tags labdemo -run '^TestProcessWorkItemsLeaks$' ./1-5/patterns/leak/ 2>&1); then
	fail "patterns/leak: synctest не поймал утечку в TestProcessWorkItemsLeaks"
fi
echo "$out" | grep -q 'blocked goroutines remain' || fail "patterns/leak: нет сообщения synctest об оставшихся горутинах:
$out"
ok "patterns/leak: synctest.Test ловит горутины, пережившие вызов"

# --- 10. patterns/limit: горутина на задачу против четырёх ограничителей ---
build limit ./1-5/patterns/limit
out=$("$bin/limit")
free_peak=$(first_number "$out" 'горутина на задачу: .*пик горутин +[0-9]+ \(из них свои: +([0-9]+)\)')
[ -n "$free_peak" ] && [ "$free_peak" -ge 150 ] || fail "patterns/limit: пик без ограничения $free_peak, ждали не меньше 150:
$out"
for name in 'семафор на канале' 'пул воркеров' 'errgroup.SetLimit' 'semaphore.Weighted'; do
	peak=$(first_number "$out" "$name: .*пик горутин +[0-9]+ \\(из них свои: +([0-9]+)\\)")
	ms=$(first_number "$out" "$name: +время +([0-9]+) мс")
	[ -n "$peak" ] && [ "$peak" -le 12 ] || fail "patterns/limit: $name — пик $peak, ждали не больше 12:
$out"
	[ -n "$ms" ] && [ "$ms" -ge 400 ] || fail "patterns/limit: $name уложился в $ms мс — 200 задач по 20 мс при лимите 8 не могут быстрее 500:
$out"
done
done_n=$(first_number "$out" 'errgroup, отказ: +выполнено задач ([0-9]+) из')
[ -n "$done_n" ] && [ "$done_n" -lt 200 ] || fail "patterns/limit: после отказа выполнено $done_n задач, отмена не сработала:
$out"
echo "$out" | grep -q 'Wait вернул "задача 50: отказ"' || fail "patterns/limit: Wait вернул не первую ошибку:
$out"
ok "patterns/limit: пик $free_peak горутин без лимита, не больше 12 у ограничителей; после отказа выполнено $done_n из 200"

# --- 11. бенчмарки: работоспособность (числа снимаются отдельно, см. README) ---
out=$(go test -run='^$' -bench=. -benchtime=100ms ./1-5/sync/ ./1-5/sched/ ./1-5/patterns/limit/ ./1-5/alloc/ 2>&1) || fail "бенчмарки упали:
$out"
for name in BenchmarkCounterMutex BenchmarkCounterMutexRead BenchmarkCounterRWMutexRead BenchmarkCounterAtomic BenchmarkFalseSharingPacked BenchmarkFalseSharingPadded BenchmarkLongReadMutex BenchmarkLongReadRWMutex BenchmarkCounterCASLoop BenchmarkShardedCounter BenchmarkSpawnGoroutine BenchmarkSwitchGoroutines BenchmarkSwitchOSThreads BenchmarkLimiterOverhead BenchmarkAppendGrow BenchmarkAppendPrealloc BenchmarkBufferNew BenchmarkBufferPool BenchmarkSlicePointers BenchmarkSliceValues BenchmarkKeySprintf BenchmarkKeyAppend; do
	echo "$out" | grep -q "^$name" || fail "в бенчмарках нет $name:
$out"
done
echo "$out" | grep -E '^Benchmark'
ok "бенчмарки sync, sched, patterns/limit и alloc отработали"

# --- 12. netpoll: цена простаивающего соединения (урок 1.5.7) ---
build netpoll ./1-5/netpoll
# wait_for <файл> <шаблон> <секунды> — ждать строку в выводе фонового процесса.
wait_for() {
	local deadline=$((SECONDS + $3))
	until grep -q "$2" "$1"; do
		[ "$SECONDS" -lt "$deadline" ] || return 1
		sleep 0.1
	done
}
# Тысяча соединений с двух исходных адресов: дескрипторов и портов с запасом и на раннере
# (ulimit -n 65536). Порт 0 — свободный порт выбирает ядро, адрес сервер печатает сам.
serve_log="$bin/serve.txt"
dial_log="$bin/dial.txt"
"$bin/netpoll" serve -addr 127.0.0.1:0 -mode bufio -n 1000 >"$serve_log" 2>&1 &
serve_pid=$!
wait_for "$serve_log" '^слушаю ' 10 || fail "netpoll serve не начал слушать:
$(cat "$serve_log")"
addr=$(sed -nE 's/^слушаю (.*)$/\1/p' "$serve_log")
"$bin/netpoll" dial -addr "$addr" -n 1000 -src 2 >"$dial_log" 2>&1 &
dial_pid=$!
wait_for "$serve_log" 'на соединение:' 30 || fail "netpoll: за 30 с нет итога:
$(cat "$serve_log")
$(cat "$dial_log")"
kill -TERM "$dial_pid" "$serve_pid"
wait "$dial_pid" || fail "netpoll dial завершился с ошибкой:
$(cat "$dial_log")"
wait "$serve_pid" || fail "netpoll serve завершился с ошибкой:
$(cat "$serve_log")"
out=$(cat "$serve_log")
has_header "$out" || fail "netpoll serve: нет шапки:
$out"
rss=$(first_number "$out" 'на соединение: RSS ([0-9.]+) КиБ')
goroutines=$(first_number "$out" 'горутин: +до +[0-9]+, после +([0-9]+)')
# Порядок величины: горутина со стеком и буфер bufio — единицы КиБ; 200 КиБ значило бы утечку.
[ -n "$rss" ] && ! less "$rss" 1 && ! less 200 "$rss" || fail "netpoll: RSS на соединение '$rss' КиБ, ждали 1–200:
$out"
[ -n "$goroutines" ] && [ "$goroutines" -ge 1000 ] || fail "netpoll: горутин '$goroutines', ждали не меньше 1000:
$out"
ok "netpoll: 1000 соединений, RSS на соединение $rss КиБ, горутин $goroutines"

# --- 13. alloc/escape: решения escape-анализа для песочницы escape-lab (урок 1.5.8) ---
build dump ./1-5/alloc/escape/dump
out=$("$bin/dump") || fail "alloc/escape/dump упал"
keys=$(grep -rhoE 'escape:[a-z0-9-]+/[a-z0-9-]+' 1-5/alloc/escape/cases | sort -u)
records=$(printf '%s\n' "$out" | grep -c '^{"case"' || true)
[ "$records" -eq "$(printf '%s\n' "$keys" | wc -l)" ] || fail "alloc/escape: записей $records, ключей $(printf '%s\n' "$keys" | wc -l):
$out"
for key in $keys; do
	cv=${key#escape:}
	printf '%s\n' "$out" | grep -qF "{\"case\":\"${cv%/*}\",\"variant\":\"${cv#*/}\"," || fail "alloc/escape: нет записи для $cv:
$out"
done
heap=$(printf '%s\n' "$out" | grep -c '"where":"heap"' || true)
stack=$(printf '%s\n' "$out" | grep -c '"where":"stack"' || true)
[ "$heap" -ge 1 ] && [ "$stack" -ge 1 ] || fail "alloc/escape: heap $heap, stack $stack — ждали оба:
$out"
# Песочница escape-lab читает снимок вывода dump из курса: снимок обязан совпадать с выводом,
# иначе после правки примеров песочница покажет старые решения компилятора.
lab_json=../course/viz/data/escape-lab.json
snapshot_diff=$(printf '%s\n' "$out" | diff "$lab_json" - 2>&1) || fail "alloc/escape: $lab_json расходится с выводом dump; обновите его: go run ./1-5/alloc/escape/dump > $lab_json
$snapshot_diff"
ok "alloc/escape: $records вариантов, в куче $heap, на стеке $stack; $lab_json совпадает с выводом dump"

# --- 14. alloc/stack: рост стека копированием ---
build stack ./1-5/alloc/stack
out=$("$bin/stack") || fail "alloc/stack упал:
$out"
has_header "$out" || fail "alloc/stack: нет шапки:
$out"
st_before=$(first_number "$out" 'глубина +100000: стеки до +([0-9]+) КиБ')
st_bottom=$(first_number "$out" 'глубина +100000: .*на дне +([0-9]+) КиБ')
# Сто тысяч кадров по десятку слов — мегабайты стека; 1 МиБ — нижняя граница с запасом.
[ -n "$st_before" ] && [ -n "$st_bottom" ] && [ $((st_bottom - st_before)) -ge 1024 ] || fail "alloc/stack: стеки до $st_before КиБ, на дне $st_bottom КиБ — рост меньше 1 МиБ:
$out"
ok "alloc/stack: на глубине 100000 стеки выросли с $st_before до $st_bottom КиБ"

# --- 15. gc/gctrace: GOGC меняет число циклов (урок 1.5.9) ---
build gctrace ./1-5/gc/gctrace
out50=$(GOGC=50 GODEBUG=gctrace=1 "$bin/gctrace" -dur 1s 2>"$bin/gctrace.err") || fail "gc/gctrace с GOGC=50 упал:
$out50
$(tail -n 20 "$bin/gctrace.err")"
out400=$(GOGC=400 "$bin/gctrace" -dur 1s) || fail "gc/gctrace с GOGC=400 упал:
$out400"
has_header "$out50" || fail "gc/gctrace: нет шапки:
$out50"
grep -Eq '^gc [0-9]+ @' "$bin/gctrace.err" || fail "gc/gctrace: GODEBUG=gctrace=1 не напечатал строк циклов:
$(head -n 5 "$bin/gctrace.err")"
c50=$(first_number "$out50" 'циклов GC: ([0-9]+)')
c400=$(first_number "$out400" 'циклов GC: ([0-9]+)')
# Относительная проверка: запас по куче при GOGC=400 в восемь раз больше, циклов должно быть меньше.
[ -n "$c50" ] && [ -n "$c400" ] && [ "$c50" -gt "$c400" ] || fail "gc/gctrace: циклов при GOGC=50 — $c50, при GOGC=400 — $c400; ждали первое больше:
$out50
$out400"
ok "gc/gctrace: циклов GC при GOGC=50 — $c50, при GOGC=400 — $c400"

# --- 16. gc/limiter: ограничитель CPU сборщика при GOMEMLIMIT ниже живой кучи ---
build limiter ./1-5/gc/limiter
out=$(GOMEMLIMIT=64MiB "$bin/limiter") || fail "gc/limiter упал:
$out"
last=$(first_number "$out" 'ограничитель включался последний раз на цикле: ([0-9]+)')
[ -n "$last" ] && [ "$last" -gt 0 ] || fail "gc/limiter: ограничитель не включился (last-enabled '$last'):
$out"
ok "gc/limiter: ограничитель включался, последний раз на цикле $last"

# --- 17. pgo: профиль бенчмарка меняет решения компилятора (урок 1.5.10) ---
go test -run='^$' -bench=Route -benchtime=500ms -cpuprofile "$bin/pgo.pprof" -o "$bin/pgo.test" ./1-5/pgo/ >/dev/null ||
	fail "pgo: бенчмарк с -cpuprofile упал"
build pgo-off ./1-5/pgo -pgo=off
"$bin/pgo-off" -dur 100ms | grep -q '^сообщений: ' || fail "pgo: программа без профиля не напечатала итог"
# Профиль снят с тестового бинаря, поэтому и применяется к нему. parse дороже обычного бюджета
# встраивания (80): без профиля она остаётся вызовом, с профилем горячий вызов встраивается.
# Девиртуализацию h.Handle не проверяем: на коротком профиле в Handle может не попасть ни одной выборки.
off=$(go test -c -o "$bin/pgo-off.test" -pgo=off -gcflags=-m=1 ./1-5/pgo/ 2>&1) || fail "pgo: сборка с -pgo=off упала:
$off"
on=$(go test -c -o "$bin/pgo-on.test" -pgo="$bin/pgo.pprof" -gcflags=-m=1 ./1-5/pgo/ 2>&1) || fail "pgo: сборка с профилем упала:
$on"
! echo "$off" | grep -q 'inlining call to parse$' || fail "pgo: parse встроена и без профиля — пример больше не показывает PGO"
echo "$on" | grep -q 'inlining call to parse$' || fail "pgo: с профилем parse не встроена:
$on"
ok "pgo: без профиля parse — вызов, с профилем бенчмарка — встроена"

echo "Лаба 1.5: все проверки прошли."
