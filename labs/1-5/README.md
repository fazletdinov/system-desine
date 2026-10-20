# Лабораторная 1.5 — горутины, каналы, гонки, sync, паттерны

Уроки модуля 1.5a «Конкурентность в Go». Лаба — маленькие Go-программы в общем модуле `labs/`, Docker
не нужен. Каждая печатает первыми тремя строками версию Go, `GOMAXPROCS` и `NumCPU`: без них числа
нельзя сравнить с вашими. Числа зависят от машины; в уроках они даны вместе с описанием машины, а
`scenario.sh` проверяет порядок величины, а не точное значение.

```
1-5/
├── scenario.sh          # запускает всё и проверяет ключевые строки (его запускает check:labs)
├── labenv/              # шапка вывода: go version, GOMAXPROCS, NumCPU
├── sched/               # урок 1.5.1: цена горутины, вытеснение, системные вызовы
│   ├── cost/  preempt/  syscall/
│   └── cost_test.go     # бенчмарки: создание горутины, переключение горутин и потоков ОС
├── chan/                # урок 1.5.2: каналы и таймеры
│   └── ops/  timers/
├── race/                # урок 1.5.3: модель памяти и гонки (тег labdemo)
│   └── flagloop/  iface/  history/
├── sync/                # урок 1.5.4: Mutex, RWMutex, atomic, false sharing, голодание
│   ├── sync_test.go
│   └── starve/
└── patterns/            # урок 1.5.5: конвейер, утечки, ограничение параллелизма
    └── pipeline/  leak/  limit/
```

## Запуск

Нужен Go 1.27 (`labs/go.mod`). Из каталога `labs/`:

```bash
bash 1-5/scenario.sh                      # все проверки, около 15 секунд
go run ./1-5/sched/cost                   # одна программа
go run -tags labdemo ./1-5/race/iface     # программы с намеренной гонкой или утечкой — только с тегом
go run -race -tags labdemo ./1-5/race/iface
go test -run='^$' -bench=. -benchtime=1s -count=5 -cpu=1,4,8 ./1-5/sync/
```

Тег `labdemo` держит программы с намеренными гонками и утечками вне `go vet ./...` и
`go test -race ./...` общего модуля: те остаются чистыми. Единственная внешняя зависимость лабы —
`golang.org/x/sync` v0.24.0 (`errgroup` и `semaphore` в `patterns`, урок 1.5.5); паддинг в `sync` считается вручную.

## Что показывает каждая программа

В выводе ниже — прогон на Intel Core i5-12400 (12 логических процессоров), Linux 7.0, go1.27.2. Шапка
(`go version`, `GOMAXPROCS`, `NumCPU`) опущена.

### sched/cost — урок 1.5.1, регион `cost`

100 000 горутин ждут на канале; затем 1 000 горутин, закреплённых за собственными потоками ОС
(`runtime.LockOSThread`).

```
горутин: 100000
стек на горутину: 2052 байт
создание одной горутины: 1052 нс
потоков ОС: 1000
рост RSS на поток: 15 КиБ
создание одного потока: 7 мкс
```

- «стек на горутину» — прирост метрики `/memory/classes/heap/stacks:bytes` на горутину: стартовый
  стек 2 КиБ плюс служебное;
- «рост RSS на поток» — сколько физической памяти прибавил процесс на поток (`VmRSS` из
  `/proc/self/status`). Виртуально у потока 8 МиБ стека, но страницы выделяются по мере касания, поэтому
  RSS мал; главная цена потока — время и работа ядра, а не память;
- время создания считается как общее время запуска, делённое на число горутин (потоков).

### sched/preempt — урок 1.5.1, регион `preempt`

`GOMAXPROCS(1)`, горутина крутит цикл без вызовов функций (около 2 с), main спит 10 мс. Запускается дважды.

```
$ go run ./1-5/sched/preempt
main проснулся через 18 мс (просил 10 мс)
цикл работал 1810 мс
$ GODEBUG=asyncpreemptoff=1 go run ./1-5/sched/preempt
main проснулся через 1750 мс (просил 10 мс)
цикл работал 1750 мс
```

С асинхронным вытеснением (Go 1.14, сигнал `SIGURG`) `sysmon` снимает цикл с процессора через 10 мс; без
него main ждёт, пока цикл не кончится сам. Цикл крутится без вызовов функций, а время в нём не
читается: `time.Now` — вызов функции, то есть точка вытеснения. Число итераций программа подбирает
замером, чтобы цикл занимал около 2 с на любой машине.

### sched/syscall — урок 1.5.1, регион `syscall`

8 горутин в блокирующем `syscall.Nanosleep` по 200 мс и 8 горутин в `time.Sleep`; потоки ОС считаются по
`/sched/threads/total:threads`.

```
time.Sleep        x8: потоков до 5, во время 5 (+0)
syscall.Nanosleep x8: потоков до 5, во время 11 (+6)
```

Горутина в блокирующем системном вызове держит свой поток (M) до возврата из ядра, поэтому потоков
становится примерно столько, сколько таких горутин (плюс-минус уже существовавшие и свободные потоки);
`sysmon` заодно отнимает у них процессор (P), чтобы остальной Go-код продолжал работать. `time.Sleep`
рантайм знает: горутина паркуется, поток свободен. `time.Sleep` идёт первым, чтобы «+0» не было следствием
уже раздутого пула потоков.

### chan/ops — урок 1.5.2, регион `ops`

Таблица Markdown: send, recv, close, len, cap над nil, небуферизованным, пустым буферизованным, полным
буферизованным и закрытым каналом. Каждая операция идёт в горутине с `recover`; не вернувшаяся за 50 мс
считается блокирующей навсегда.

```
| операция | nil | открытый, cap 0 | открытый, пустой, cap 1 | открытый, полный, cap 1 | закрытый, cap 1 |
| --- | --- | --- | --- | --- | --- |
| `ch <- 1` | блокирует навсегда | блокирует навсегда | ок | блокирует навсегда | panic: send on closed channel |
| `<-ch` | блокирует навсегда | блокирует навсегда | блокирует навсегда | ок (v=1, ok=true) | ок (v=0, ok=false) |
| `close(ch)` | panic: close of nil channel | ок | ок | ок | panic: close of closed channel |
| `len(ch)` | ок (0) | ок (0) | ок (0) | ок (1) | ок (0) |
| `cap(ch)` | ок (0) | ок (0) | ок (1) | ок (1) | ок (1) |
```

### chan/timers — урок 1.5.2, регион `timers`

Миллион итераций `select` с быстрым каналом: в первом варианте на каждой итерации новый `time.After(1s)`,
во втором один `time.Timer` с `Reset`. Печатается `HeapAlloc` до и после, всего выделено за прогон и число
циклов GC (`runtime.ReadMemStats`).

```
time.After в цикле:    HeapAlloc до   0.25 МиБ, после   3.40 МиБ, выделено за прогон   236.5 МиБ, циклов GC 69
один Timer + Reset:    HeapAlloc до   0.27 МиБ, после   0.28 МиБ, выделено за прогон     0.0 МиБ, циклов GC 0
```

До Go 1.23 несработавшие таймеры держались в куче до срабатывания (миллион таймеров на секунду ждали бы
сотни МиБ); с 1.23 они собираются, поэтому «после» невелико. Но каждый `time.After` всё равно аллоцирует
таймер и канал — отсюда 236 МиБ за прогон и десятки GC, тогда как один `Timer` не аллоцирует ничего.
В Go 1.27 убран `asynctimerchan`: канал таймера всегда синхронный.

### race/flagloop — урок 1.5.3, регион `flagloop` (тег `labdemo`)

Цикл ждёт обычный `bool`, который выставляет другая горутина; рядом — версия с `atomic.Bool`. Main ждёт
итог 2 с через `context`.

```
обычный bool:  завершилось
atomic.Bool:   завершилось
```

Честный результат: gc сегодня не выносит чтение флага из цикла, поэтому обычный `bool` обычно
«завершается». Это свойство компилятора, а не гарантия языка: по спецификации запись может так и не стать
видимой. С `-race` обе гонки видны сразу:

```
$ go run -race -tags labdemo ./1-5/race/flagloop
WARNING: DATA RACE
Write at 0x… by main goroutine:
  main.main() …/flagloop/main.go:60
Previous read at 0x… by goroutine 8:
  main.waitPlainFlag() …/flagloop/main.go:30
```

### race/iface — урок 1.5.3, регион `iface` (тег `labdemo`)

Два писателя чередуют в переменной-интерфейсе значения двух типов (`small` и функциональный `callback`), третья
горутина вызывает метод. Значение интерфейса — два слова (тип и данные), запись пары не атомарна; читатель
видит тип `callback` с указателем на число 1 и «вызывает» адрес 1.

```
$ go run -tags labdemo ./1-5/race/iface
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x1 pc=0x1]
$ go run -race -tags labdemo ./1-5/race/iface
WARNING: DATA RACE
```

Без `-race` код выхода ненулевой; `-race` печатает отчёты и (из-за числа повторов гонки) весь вывод
длинный.

### race/history — урок 1.5.3, регион `history` (тег `labdemo`, только с `-race`)

Две горутины по `-n` раз пишут в одну переменную без синхронизации. Гонка есть при любом `n`, но найдёт
ли её детектор, зависит от того, сохранился ли в его журнале стек прошлого обращения. На одном P порог
зависит от `GORACE=history_size`:

```
$ go build -race -tags labdemo -o history ./1-5/race/history
$ GOMAXPROCS=1 GORACE=atexit_sleep_ms=0 ./history -n 20000 2>&1 | grep -c 'WARNING: DATA RACE'
1
$ GOMAXPROCS=1 GORACE=atexit_sleep_ms=0 ./history -n 30000 2>&1 | grep -c 'WARNING: DATA RACE'
0
$ GOMAXPROCS=1 GORACE='atexit_sleep_ms=0 history_size=1' ./history -n 30000 2>&1 | grep -c 'WARNING: DATA RACE'
1
```

Пороги — порядок величины на машине автора, а не константы. В `scenario.sh` программа не входит: результат
зависит от планировщика и не детерминирован.

### sync/sync_test.go — урок 1.5.4, регионы `counter` и `padding`

```
BenchmarkCounterMutex-12          	 1947332	        61.56 ns/op
BenchmarkCounterMutexRead-12      	 4322102	        84.60 ns/op
BenchmarkCounterRWMutexRead-12    	 3169353	        37.84 ns/op
BenchmarkCounterAtomic-12         	 6838264	        17.52 ns/op
BenchmarkFalseSharingPacked-12    	 3792921	        32.85 ns/op
BenchmarkFalseSharingPadded-12    	19039011	         6.338 ns/op
```

Сравнимые пары: `CounterMutex` и `CounterAtomic` — запись; `CounterMutexRead` и `CounterRWMutexRead` —
чтение (одинаковая работа, разный замок). `RWMutexRead` с `CounterMutex` сравнивать нельзя: у того
критическая секция пишет. Первые четыре — `b.RunParallel` на всех ядрах: чем больше ядер борется за одну переменную, тем дороже.
`FalseSharing*` — две горутины, каждая бьёт по своему счётчику; в `Packed` счётчики лежат в одной
строке кэша (64 байта) и ядра отнимают её друг у друга, в `Padded` между ними 56 байт паддинга.
Данные не общие, общая только строка кэша.

Регионы `longread`, `casloop` и `sharded` — дополнительные пары к уроку 1.5.4:

```
BenchmarkLongReadMutex/4096-8         	 1186012	      1005 ns/op
BenchmarkLongReadRWMutex/4096-8       	12797736	        93.99 ns/op
BenchmarkCounterCASLoop-8             	32529403	        36.30 ns/op	         0.5514 retries/op
BenchmarkShardedCounter-8             	1000000000	         1.096 ns/op
```

`LongReadMutex` и `LongReadRWMutex` — одинаковое чтение длинной секции (обход среза из 256 или 4096
чисел) под разными замками: на длинной секции `RWMutex` выигрывает в разы, на короткой (`CounterRWMutexRead`) —
нет. `CounterCASLoop` — тот же счётчик, что `CounterAtomic`, но CAS-циклом; `retries/op` — средняя доля
неудачных CAS. `ShardedCounter` — у каждой горутины своя ячейка в своей строке кэша; сравнивать с
`CounterAtomic`. Числа снимаются так:

```bash
go test -run='^$' -bench='LongRead|Counter(Mutex|Atomic|CASLoop)$|ShardedCounter' -benchtime=1s -count=5 -cpu=1,4,8 ./1-5/sync/
```

### sync/starve — урок 1.5.4, регион `starve`

Секунду две горутины берут один `sync.Mutex`: «держатель» держит замок 2 мс, «быстрая» берёт на миг.

```
держатель (держит 2ms): захватов 461, макс. ожидание 0.11 мс
быстрая горутина:       захватов 30941, макс. ожидание 5.51 мс, ожиданий дольше 1ms: 231
ожидания быстрой горутины: <0,1 мс: 30710, 0,1–1 мс: 0, 1–3 мс: 1, 3–5 мс: 229, ≥5 мс: 1
```

Последняя строка — гистограмма ожиданий (регион `histogram`): долгие ожидания лежат в корзине 3–5 мс,
то есть быстрая горутина ждёт два удержания по 2 мс, а не одно.

Ожидание дольше 1 мс переводит `Mutex` в режим голодания: замок передаётся ждущему напрямую, и никто не
ждёт неограниченно. Поэтому максимум ожидания — единицы миллисекунд, а не секунды.

### patterns/pipeline — урок 1.5.5, регионы `generator`, `stage`, `fanout`, `ordered`; тесты — `synctest`, `first-error`

Пакет, а не программа: генератор, стадия, раздача воркерам со слиянием и параллельная обработка с
сохранением порядка на `errgroup.SetLimit`. Тесты идут в «пузыре» `testing/synctest` с виртуальным временем:

```
$ go test -count=1 -v ./1-5/patterns/pipeline/
=== RUN   TestPipelineStopsWhenConsumerLeaves
--- PASS: TestPipelineStopsWhenConsumerLeaves (0.00s)
=== RUN   TestParallelMapVirtualTime
--- PASS: TestParallelMapVirtualTime (0.00s)
=== RUN   TestOrderedMapKeepsOrder
--- PASS: TestOrderedMapKeepsOrder (0.00s)
=== RUN   TestOrderedMapFirstErrorCancelsOthers
--- PASS: TestOrderedMapFirstErrorCancelsOthers (0.00s)
PASS
```

`TestParallelMapVirtualTime` проверяет, что восемь задач по секунде на четырёх воркерах заняли ровно 2 с
виртуального времени, а `TestOrderedMapFirstErrorCancelsOthers` — что первая ошибка отменила остальных за 1 с,
хотя каждая ждала бы час. Реального ожидания нет ни в одном тесте.

### patterns/leak — урок 1.5.5, регионы `leak`, `leak-fix`, `leak-group`, `leak-test` (тег `labdemo`)

Функция `processWorkItems` из release notes Go 1.26: небуферизованный канал и ранний возврат по ошибке. Рядом
два исправления: буфер на число задач (`processWorkItemsFixed`) и `errgroup` (`processWorkItemsGroup`). После
каждого вызова программа читает профиль `goroutineleak` (в 1.27 он общедоступен; запись профиля сама
запускает цикл сборки мусора с поиском утечек).

```
processWorkItemsFixed: ошибка "задача не удалась"
утекло горутин: 0
processWorkItemsGroup: ошибка "задача не удалась"
утекло горутин: 0
processWorkItems: ошибка "задача не удалась"
утекло горутин: 9
goroutineleak profile: total 9
9 @ 0x… 0x… 0x… 0x… 0x…
#	0x…	main.processWorkItems.func1+0x44	…/leak/main.go:69
```

Профиль накопительный, поэтому исправленные версии идут первыми. `leak_test.go` прогоняет все три функции в
`synctest.Test`: тест с `processWorkItems` и тест с буферным исправлением падают («main bubble goroutine has
exited but blocked goroutines remain»), потому что горутины пережили вызов; `errgroup`-версия проходит.

```bash
go test -tags labdemo -run '^TestProcessWorkItemsGroup$' ./1-5/patterns/leak/   # ok
go test -tags labdemo -run '^TestProcessWorkItemsLeaks$' ./1-5/patterns/leak/   # FAIL — намеренно
```

### patterns/limit — урок 1.5.5, регионы `limit`, `pool`, `errgroup`, `weighted`, `first-error`, `bench`

200 задач по 20 мс: горутина на задачу и четыре ограничителя на 8 — семафор на буферизованном канале, пул
воркеров, `errgroup.SetLimit`, `semaphore.Weighted`. «Пик горутин» — максимум `runtime.NumGoroutine`, который
опрашивает отдельная горутина-наблюдатель. Последний прогон — `errgroup.WithContext`, где задача 50 падает.

```
горутина на задачу:  время   20 мс, пик горутин 202 (из них свои: 200)
семафор на канале:   время  514 мс, пик горутин  10 (из них свои:   8)
пул воркеров:        время  514 мс, пик горутин  10 (из них свои:   8)
errgroup.SetLimit:   время  518 мс, пик горутин  10 (из них свои:   8)
semaphore.Weighted:  время  517 мс, пик горутин  10 (из них свои:   8)
errgroup, отказ:     время  124 мс, пик горутин  10 (из них свои:   8)
errgroup, отказ:     выполнено задач 48 из 200, Wait вернул "задача 50: отказ"
```

Лимит 8 даёт 200 / 8 × 20 мс = 500 мс у любого ограничителя. Место в семафоре занимается до запуска горутины,
иначе все 200 горутин уже созданы. Бенчмарк `LimiterOverhead` меряет цену самого ограничителя на почти пустой
задаче:

```bash
go test -run='^$' -bench=LimiterOverhead -benchtime=1s -count=5 ./1-5/patterns/limit/
```

```
BenchmarkLimiterOverhead/channel-semaphore-12         	 2495641	       548.3 ns/op
BenchmarkLimiterOverhead/pool-12                      	 3977941	       297.9 ns/op
BenchmarkLimiterOverhead/errgroup-12                  	 2166590	       585.7 ns/op
BenchmarkLimiterOverhead/weighted-12                  	 2139986	       625.3 ns/op
```

(медианы пяти прогонов). Пул дешевле, потому что не создаёт горутину на задачу; на задаче длиннее
нескольких микросекунд разница исчезает.

## Как читать `scenario.sh`

На каждую проверку — строка `ОК: …` или `ПРОВАЛ: …` (в stderr, затем выход с кодом 1). Последняя строка —
`Лаба 1.5: все проверки прошли.` Пороги с запасом под раннер в 2–4 ядра: например, `main` с вытеснением должен
проснуться за 100 мс (обычно 20), а с `asyncpreemptoff=1` — не раньше чем через 1 с (обычно 1,7–1,9 с).
`check:labs` запускает сценарий из `labs/` с таймаутом 180 с, CI — тем же скриптом (джоба `labs`).
