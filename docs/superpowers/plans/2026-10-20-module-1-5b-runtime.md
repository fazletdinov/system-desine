# Модуль 1.5, часть 1.5b «Рантайм Go изнутри» — план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Уроки 1.5.7–1.5.11 (netpoller и цена соединения, стек и аллокатор, сборщик мусора, профилирование мессенджера, мини-интервью модуля 1.5), лаба `labs/1-5` дополняется группами `netpoll`, `alloc`, `gc`, `pgo`, `fold`; три песочницы и два плеера; в мессенджере — служебный порт с pprof и flight recorder по медленным запросам (ADR-0013).

**Architecture:**
- Раскладка и правила слоёв модулей 1 и 1.5a не меняются.
- Новое в мессенджере — два технических пакета: `platform/debugserver` (служебный HTTP-слушатель с явно зарегистрированными обработчиками pprof, компонент `lifecycle` стадии `Back`) и `platform/flightrec` (один `trace.FlightRecorder` на процесс и компонент `Dumper`, который пишет снимок по сигналу middleware о медленном запросе).
- Лаба — маленькие программы в общем модуле `labs/` без Docker и без новых зависимостей. Данные для песочниц `escape-lab` и `flamegraph` снимаются программами лабы и коммитятся в `course/viz/data/`.
- Тег `m01-5-lNN` ставится на последний коммит задачи урока. Глоссарий, лаба и визуализации (задачи 1–4) идут раньше уроков и входят в `m01-5-l07`.

**Tech Stack:** как в 1.5a: VitePress 1.6.4, Vue 3.5.43, TypeScript 5.9.3, Vitest 5.0.3, mermaid 12.1.0, Node 26; Go **1.27.2** (`toolchain go1.27.2`), `golangci-lint` v2.14.0, `golang.org/x/sync` v0.24.0. Новых Go-зависимостей нет (решение плана 3).

**Spec:**
- `docs/superpowers/specs/2026-10-20-module-1-5b-runtime-design.md` — главный.
- `docs/superpowers/specs/2026-10-19-module-1-5a-concurrency-design.md`, спецификации 1a–1c и `2026-10-09-course-platform-design.md` (§1, §2.2–2.5) — действующие правила.

**Отступления от стандарта плана (как в 1.5a):** Go-код дан сигнатурами и тестами; уроки, рендереры, сцены и ADR — контрактом.

**Факты, проверенные при подготовке (2026-10-20, по исходникам Go 1.27.2; не перепроверять, но не противоречить):**
- `runtime/trace`: `NewFlightRecorder(cfg FlightRecorderConfig) *FlightRecorder`; `FlightRecorderConfig{MinAge time.Duration; MaxBytes uint64}` (`MaxBytes` важнее `MinAge`, оба — подсказки); методы `Start() error`, `Stop()`, `WriteTo(io.Writer) (int64, error)`, `Enabled() bool`. **В процессе активен не больше одного рекордера.**
- Green Tea включён по умолчанию (`internal/buildcfg/exp.go: GreenTeaGC: true`); отключение — флаг сборки `GOEXPERIMENT=nogreenteagc`.
- `runtime/metrics` в 1.27: `/memory/classes/heap/stacks:bytes`, `/memory/classes/os-stacks:bytes`, `/gc/scan/stack:bytes`, `/gc/stack/starting-size:bytes`, `/sched/goroutines:goroutines`, `/gc/heap/goal:bytes`, `/gc/heap/live:bytes`, `/gc/pauses:seconds`, `/gc/gogc:percent`, `/gc/gomemlimit:bytes`, `/gc/limiter/last-enabled:gc-cycle`, `/cpu/classes/gc/{mark/assist,mark/dedicated,mark/idle,pause,total}:cpu-seconds`.
- `internal/runtime/gc/sizeclasses.go`: `NumSizeClasses = 68`; tiny-аллокатор — `_TinySize = 16`; `mcache` — на P.
- `gcBackgroundUtilization = 0.25`; ограничитель CPU GC (`mgclimit.go`) — «очень консервативный предел 50 %», дырявое ведро, включён всегда (лимит памяти в `mgclimit.go` не проверяется; рулинг R12), главный сценарий — живая куча выше `GOMEMLIMIT`.
- `stackMin = 2048`; начальный размер стека пересчитывается каждой сборкой по среднему (`stack.go`).
- `MemProfileRate = 512 * 1024`; CPU-профилировщик по умолчанию 100 Гц.
- `net/http/pprof`: `Index`, `Cmdline`, `Profile`, `Symbol`, `Trace`, `Handler(name string) http.Handler`. `pprof.Profile` сам отвечает ошибкой, если `seconds` больше `WriteTimeout` сервера.
- Netpoller: `netpoll_epoll.go` регистрирует сокет с `EPOLLIN|EPOLLOUT|EPOLLRDHUP|EPOLLET` (edge-triggered); `netpoll` вызывают `findRunnable` (неблокирующе и блокирующе), `startTheWorldWithSema` и `sysmon` (если сеть не опрашивали > 10 мс).
- `goroutineleak` — обычный профиль в 1.27 (без флага).
- Не проверено по исходникам — сверить по release notes в задаче урока: версии появления `GOMEMLIMIT` (1.19), гибридного барьера (1.8), учёта стеков и глобальных в цели кучи (1.18), `testing.B.Loop`/`runtime.AddCleanup`/`weak` (1.24), `FlightRecorder` (1.25), цена трассировщика после 1.21.
- В мессенджере: `lifecycle.Run(ctx, log, Stages{Front, Back})`, `lifecycle.BackStopTimeout = 2 * time.Second`; `httpserver.New(addr, h, log, requestTimeout) (*http.Server, error)`, `httpserver.Run(ctx, srv, Stop)`, `httpserver.Serve(ctx, srv, ln, Stop)`, `Stop{Drain func(); DrainDelay, Timeout time.Duration}` (`Drain` может быть `nil`); `newApp` возвращает `app{handler, probes, back}`; цепочка middleware — регион `chain` в `app.go`; имена переменных — константы `env*` в `app.go`; `durationEnv`, `intEnv` уже есть. В `compose.yaml` — `read_only: true`, `tmpfs: /tmp`, `mem_limit: 256m`, `cpus: 1`.
- Тренажёр интервью: банк `course/interview/module-01.yaml` (`module: "01"`, 29 вопросов, поля `id`, `lesson`, `topic`, `level`, `question`, `answer`, `checklist`, `red_flags`, `follow_ups`), загрузчик `interview.data.ts` читает все `*.yaml`, урок 1.20 подключает `<InterviewSim :bank="data['01']" />`.
- `labs/1-5/scenario.sh` фиксирует `LC_ALL=C.UTF-8`; пороги в нём — относительные (уроки CI 1.5a: `sed` раннера не принимает `[а-я]`, длительность циклов на раннере короче).

**Решения плана, уточняющие спеку (рулинги до исполнения):**
1. **`Dumper` — компонент `Back`, middleware только сигналит.** Middleware после медленного запроса делает неблокирующую отправку в канал ёмкостью 1; файл пишет горутина `Dumper.Run` (владелец — `lifecycle`). Так запись нескольких мегабайт не задерживает ответ и не плодит горутин без владельца.
2. **Пакеты:** `platform/flightrec` (`Recorder`, `Dumper`), `platform/debugserver` (`NewHandler`, `Server`). Middleware живёт в `flightrec` (`Dumper.Middleware`) и не импортирует `platform/middleware` — сигнатура `func(http.Handler) http.Handler` совместима с `middleware.Chain`.
3. **Свёртка профилей — без новых зависимостей**: программа `labs/1-5/fold` разбирает текст `go tool pprof -traces` и пишет JSON свёрнутых стеков. `github.com/google/pprof/profile` не подключается: у него нет тегов, только псевдоверсии.
4. **Лимит длительности профиля:** `debugserver.MaxProfileDuration = 60 * time.Second`; `?seconds` больше лимита или не число — `400` до вызова `pprof.Profile`/`Trace`; `WriteTimeout` служебного сервера = `MaxProfileDuration + 10 * time.Second`.
5. **Остановка служебного сервера:** `httpserver.Run` с `Stop{Timeout: debugserver.StopTimeout}`, `StopTimeout = time.Second` (< `BackStopTimeout`); незавершённый профиль обрывается `Close` — так и задумано (спека §3.1).
6. **Профили конкуренции:** при заданном `MESSENGER_DEBUG_ADDR` корень композиции вызывает `debugserver.EnableContentionProfiles()` — `runtime.SetMutexProfileFraction(MutexProfileFraction)` с `MutexProfileFraction = 100` и `runtime.SetBlockProfileRate(BlockProfileRate)` с `BlockProfileRate = 10_000` (нс: в среднем одно событие на 10 мкс блокировки). Урок 1.5.10 замеряет цену.
7. **Окно рекордера:** `flightrec.Window = 10 * time.Second` (`MinAge`), `flightrec.MaxBytes = 16 << 20`, `flightrec.MinInterval = time.Minute` между файлами. `MESSENGER_FLIGHT_THRESHOLD` обязан быть `< Window`.
8. **Данные песочниц** — `course/viz/data/escape-lab.json` и `course/viz/data/flame-messenger.json`; общий размер ≤ 200 КБ; стеки с долей < 0,1 % сливаются в узел «прочее» конвертером.
9. **Числа в уроках снимает исполнитель** на своей машине (строка «машина: <CPU>, <ядер>, <ОС>, go1.27.2»). Сырые выводы — в рабочей папке плана (`.superpowers/sdd/…/lab-numbers/`).

## Global Constraints

- Команды npm — из `/home/idel/DevGo/system-desine/course`, Go мессенджера — из `messenger/`, лаб — из `labs/`. Go 1.27.2: `export PATH=/home/idel/sdk/go1.27.2/bin:$PATH` (локальный `go` по умолчанию — 1.26.5). `golangci-lint` — в `~/go/bin`.
- Ветка `feat/module-1-5b` (в ней коммит спеки). `main` защищён ruleset: слияние только через PR с зелёной `ci-ok`. `git push` — только с явного согласия пользователя.
- Язык контента — русский; английский оригинал термина в скобках при первом упоминании; термин при первом появлении — `<T id="…">`.
- Тексты уроков — по основной спецификации §1, §2.2 (10 правил), §2.3, §2.4 (`<Part>` у каждого `##`, `<N>` для 3–8 ключевых чисел, `<Deep>` на 150–500 слов, `<Callout type="simplified">` для упрощений).
- Структура урока — как в 1.5a: «Проблема» → теория → «Визуализация» → «Код» → «Что появилось в лабе» (1.5.7–1.5.9) / «Что изменилось в мессенджере» (1.5.10) → «Типичные ошибки» → «Компромисс» → «Самопроверка» (5–8 `<SelfCheck>`) → «Источники». Урок 1.5.11 — по структуре 1.20.
- Заголовок — `title: "1.5.N. …"`; строку «Модуль 1.5 · Урок N» даёт `lesson-kicker.ts`.
- Визуализации: чистые TS-модули без `vue` и DOM; DOM — только в `onMounted`, обработчиках и `watch` без `immediate`; слушатели снимаются в `onBeforeUnmount`; цвета — CSS-переменные `--viz-*`; обе темы, контраст ≥ 4,5:1, `prefers-reduced-motion`, 375 px без горизонтальной прокрутки, `VizFrame` у каждой сцены; радиогруппы — `SegmentedToggle`.
- Go-код — по `~/.claude/CLAUDE.md` и §2.5: порт у потребителя, узкие интерфейсы; `NewX(deps…) (*X, error)` отвергает `nil` и неверные значения; `ctx` первым; `%w`; сообщения по-русски со строчной; у каждой горутины владелец; лимиты и сроки — именованные константы с «почему»; `gofmt`, `go vet`, `golangci-lint run`, `go test -race` чистые.
- Новые тесты с временем и горутинами — только через `testing/synctest`; ни один тест не ждёт через `time.Sleep`.
- Код в уроках — только `<<< @/../<путь>#<регион>{lang}`; иллюстративные фрагменты — `<!-- go-check -->`.
- Вывод команд в уроках — реальный, с описанием машины; каждое число воспроизводимо программой из `labs/` или командой из урока; гипотезы помечены.
- `labs/1-5/scenario.sh`: только относительные пороги и пороги порядка величины; никаких диапазонов кириллицы в регулярных выражениях; общее время на раннере ≤ 2 мин.
- Коммиты на русском, последняя строка — `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Факты — по исходникам Go 1.27.2 (`runtime/netpoll.go`, `netpoll_epoll.go`, `proc.go`, `stack.go`, `malloc.go`, `internal/runtime/gc/sizeclasses.go`, `mgc.go`, `mgcpacer.go`, `mgclimit.go`, `mbarrier.go`, `runtime/pprof`, `runtime/trace`, `net/http/pprof`), Go GC Guide (go.dev/doc/gc-guide), release notes 1.8–1.27, документации `cmd/compile` и `cmd/pprof`.

## Review Focus

1. **`?seconds=` больше лимита, отрицательный или не число.** Ожидается `400` с понятным текстом, а не соединение, которое держится минуты. Покрывает `TestProfileSecondsLimit` (задача 8).
2. **Серия медленных запросов.** Ожидается: не больше одного файла за `MinInterval`, запрос никогда не ждёт записи снимка (канал полон — сигнал отбрасывается). Покрывают `TestDumperRateLimit` и `TestMiddlewareNeverBlocks` (задача 8).
3. **Неверная комбинация переменных:** `FLIGHT_DIR` без `DEBUG_ADDR`, `FLIGHT_THRESHOLD ≥ Window`, несуществующий каталог, `DEBUG_ADDR` без порта. Ожидается ошибка старта с именем переменной. Покрывает табличный `TestLoadDebugConfig` (задача 8).
4. **`DEBUG_ADDR` пуст.** Ожидается: нет слушателя, нет рекордера, профили конкуренции не включены, `/debug/pprof/` на основном порту — `404`. Покрывают `TestNewAppDebugDisabled` и `TestPprofNotOnMainPort` (задача 8).
5. **Остановка во время снятия профиля.** Ожидается: служебный сервер закрывается за `StopTimeout`, `lifecycle.Run` не упирается в `BackStopTimeout`. Покрывает `TestServerStopsDuringLongProfile` (задача 8).

---

## Структура файлов

```
messenger/
├── internal/platform/debugserver/{doc.go,debugserver.go,debugserver_test.go}   # задача 8
├── internal/platform/flightrec/{doc.go,recorder.go,dumper.go,*_test.go}        # задача 8
├── cmd/messenger/{app.go,app_test.go,main.go}                                  # задача 8
├── .golangci.yml, compose.yaml, README.md                                       # задача 8
├── scripts/read-load.sh                                                         # задача 10
└── docs/adr/{0008-…(строка),0013-служебный-порт-и-профилирование.md}           # задача 10
labs/1-5/
├── netpoll/            # задача 2: serve и dial, замер на N соединений
├── alloc/              # задача 2: escape/cases, escape/dump, бенчмарки аллокаций, рост стека
├── gc/                 # задача 2: gctrace, limiter, greentea
├── pgo/                # задача 2: программа с горячим путём, бенчмарк без и с профилем
├── fold/               # задача 9: текст pprof -traces → JSON свёрнутых стеков
├── scenario.sh, README.md
course/
├── glossary/terms.yaml                                                          # задача 1
├── interview/module-01-5.yaml                                                   # задача 11
├── viz/data/{escape-lab.json,flame-messenger.json}                              # задачи 4, 9
├── viz/simulations/{gc-pacer,tricolor,netpoller,escape-lab,flamegraph}.ts (+ .test.ts)
├── viz/renderers/{GcPacer,Tricolor,Netpoller,EscapeLab,Flamegraph}View.vue
├── viz/scenes/{GcPacerSandbox,Tricolor,Netpoller,EscapeLabSandbox,FlamegraphSandbox}.vue
├── modules/00-foundation/06-messenger-requirements.md                           # задача 12 (ссылка на замер)
└── modules/01-5-go-concurrency/{index.md,01-…,06-…,07-netpoller,08-stack-heap,09-gc,10-profiling,11-interview}.md
```

---

### Task 1: Глоссарий и страница модуля

**Files:** Modify: `course/glossary/terms.yaml`, `course/modules/01-5-go-concurrency/index.md`

**Interfaces:** Produces: id терминов для уроков 1.5.7–1.5.11 (список — спека §6; существующие id переиспользовать, совпадения проверить `grep -n "^- id:"`).

- [ ] **Step 1:** Добавить термины спеки §6, которых нет: определение 1–3 предложения с примером, `related` на существующие (`gomemlimit`, `gomaxprocs`, `goroutine`, `sysmon`, `sync-pool`, `goroutine-leak`). Определения сверить с фактами плана (`epoll` — edge-triggered в Go; ограничитель — около 50 %, только с `GOMEMLIMIT`; Green Tea — по умолчанию с 1.26).
- [ ] **Step 2:** `index.md`: вместо анонса части 1.5b — список уроков 1.5.7–1.5.11 с одной строкой о каждом (ссылки появятся по мере уроков; до урока — текст без ссылки, чтобы `check:refs` проходил).
- [ ] **Step 3: Проверить** — `npm test -- glossary && npm run check:refs && npm run build`; Expected: PASS.
- [ ] **Step 4: Commit** — `docs(course): глоссарий и страница модуля для части 1.5b`.

---

### Task 2: Лаба — `netpoll`, `alloc`, `gc`, `pgo`

**Files:**
- Create: `labs/1-5/netpoll/{main.go,serve.go,dial.go,stats.go,stats_test.go}`, `labs/1-5/alloc/{escape/cases/*.go,escape/dump/main.go,bench_test.go,stack/main.go}`, `labs/1-5/gc/{gctrace/main.go,limiter/main.go,greentea/bench_test.go}`, `labs/1-5/pgo/{main.go,hot.go,hot_test.go}`
- Modify: `labs/1-5/scenario.sh`, `labs/1-5/README.md`

**Interfaces:**
- Produces: `labs/1-5/alloc/escape/dump` пишет JSON `[{ "case": string, "variant": string, "file": string, "line": number, "where": "stack"|"heap", "compiler": string }]` (строки `-gcflags=-m=1`, путь — относительно `alloc/escape/cases`); его потребляет задача 4.
- Produces: шапка вывода всех программ — как в 1.5a (`go version`, `GOMAXPROCS`, `NumCPU`) для `has_header`.

- [ ] **Step 1: `netpoll`.** Один бинарь, подкоманды:
  - `netpoll serve -addr 127.0.0.1:9000 -mode bufio|lazy -n N` — принимает N соединений; `bufio`: горутина на соединение с `bufio.NewReaderSize(conn, 4096)` и `ReadString('\n')`; `lazy`: горутина на соединение без собственного буфера, буфер 4 КиБ берётся из `sync.Pool` только на время чтения (контраст для урока; не настоящий gobwas). Когда принято N — пауза `settle = 2 * time.Second`, `runtime.GC()`, печать: RSS (`VmRSS` из `/proc/self/status`), горутины, `/memory/classes/heap/stacks:bytes`, `/memory/classes/heap/objects:bytes`, строка `TCP:` из `/proc/net/sockstat` до и после, и **итог** `на соединение: RSS X КиБ, стеки Y КиБ, куча Z КиБ`. Затем ждёт SIGTERM.
  - `netpoll dial -addr 127.0.0.1:9000 -n N -src 8` — открывает N соединений с локальных адресов `127.0.0.2…127.0.0.(1+src)` (`net.Dialer{LocalAddr}`), держит их до SIGTERM.
  - Обе подкоманды до работы сравнивают `N + 64` с мягким `RLIMIT_NOFILE` (Go поднимает его до жёсткого) и при нехватке пишут `нужно N+64 дескрипторов, лимит L: поднимите ulimit -n` с кодом 2; `dial` проверяет `N/src ≤ 28000` (эфемерные порты) с аналогичной ошибкой.
  - Тест `stats_test.go`: разбор `VmRSS` и строки `TCP: inuse … mem …` на фиксированных текстах.
- [ ] **Step 2: `alloc`.**
  - `escape/cases`: 8–10 функций по спеке (возврат значения/указателя; срез константной и переменной длины; большой массив > 64 КиБ — `ir.MaxStackVarSize`, сверить в `cmd/compile`; сохранение в `any`/`fmt.Println`; замыкание, переживающее функцию; отправка указателя в канал; `//go:noinline` против встраивания); каждый вариант — отдельная функция `Case<Имя><Вариант>` с комментарием-ключом `// escape:<case>/<variant>` над ней.
  - `escape/dump`: `go build -gcflags=-m=1` пакета `cases`, сопоставление строк вывода с функциями по номеру строки, JSON в stdout; сортировка стабильная.
  - `bench_test.go`: `BenchmarkAppendGrow`/`BenchmarkAppendPrealloc`, `BenchmarkBufferNew`/`BenchmarkBufferPool`, `BenchmarkSlicePointers`/`BenchmarkSliceValues` — через `b.Loop()` и `b.ReportAllocs()`.
  - `stack/main.go`: рекурсия глубины 10/1000/100000, печать `/memory/classes/heap/stacks:bytes` и `/gc/stack/starting-size:bytes` до и после — рост стека копированием.
- [ ] **Step 3: `gc`.**
  - `gctrace`: держит живую кучу `-live 64MiB`, аллоцирует мусор со скоростью `-rate` в течение `-dur 3s`, печатает число циклов (`/gc/cycles/total:gc-cycles`), долю CPU на GC (`/cpu/classes/gc/total` к `/cpu/classes/total`), пик `/gc/heap/goal:bytes`. Запускается с `GODEBUG=gctrace=1` и разными `GOGC`.
  - `limiter`: живая куча растёт выше `GOMEMLIMIT` (задаётся снаружи); печатает долю CPU GC за окна по 500 мс и `/gc/limiter/last-enabled:gc-cycle` — показ, что ограничитель включился и доля не уходит к 100 %.
  - `greentea/bench_test.go`: бенчмарк разметки кучи из множества мелких объектов с указателями; урок запускает его дважды — обычной сборкой и `GOEXPERIMENT=nogreenteagc`.
- [ ] **Step 4: `pgo`.** `hot.go` — горячий путь с вызовами через интерфейс и небольшими функциями, которые PGO встраивает/девиртуализирует (например, разбор и маршрутизация сообщений); `hot_test.go` — бенчмарк; урок: `go test -bench . -cpuprofile cpu.pprof`, `cp cpu.pprof default.pgo`, повтор с `-pgo=auto` против `-pgo=off`, `benchstat`. `default.pgo` **не** коммитится (`.gitignore` в `labs/1-5/pgo/`).
- [ ] **Step 5: `scenario.sh`** (короткие режимы, проверки порядка величины):
  - `netpoll`: `serve -n 1000` и `dial -n 1000 -src 2` в фоне; ждать строку итога (цикл `until grep -q 'на соединение:'` с потолком 30 с); проверить, что RSS на соединение в `[1, 200]` КиБ и горутин ≥ 1000; остановить оба `kill -TERM`, `wait`.
  - `alloc`: `dump` даёт JSON, где каждый `// escape:` ключ присутствует и хотя бы один вариант — `heap`, хотя бы один — `stack`; `stack` — стеки после рекурсии больше, чем до; бенчмарки `-benchtime=100ms` отработали.
  - `gc/gctrace`: при `GOGC=50` циклов больше, чем при `GOGC=400` (относительная проверка); `gc/limiter` с `GOMEMLIMIT` ниже живой кучи завершился и `last-enabled > 0`.
  - `pgo`: сборка с `-pgo=off` и с профилем из короткого бенчмарка прошла.
  - Время — замерить `time bash 1-5/scenario.sh`; если > 90 с локально — урезать параметры.
- [ ] **Step 6: Полные прогоны для уроков:** `netpoll` на 10⁴ и 10⁵ (оба режима), `gctrace` при `GOGC=50/100/400` и `GOGC=off GOMEMLIMIT=…`, `limiter`, `greentea` в обоих вариантах (`-count=10`, `benchstat`), бенчмарки `alloc` (`-count=10`, `benchstat`), `pgo`. Сохранить выводы с описанием машины в `.superpowers/sdd/<план>/lab-numbers/`.
- [ ] **Step 7: README** — что запускать к какому уроку, требования к `ulimit -n` для 10⁵.
- [ ] **Step 8: Проверить** — из `labs/`: `gofmt -l . && go vet ./... && go test -race ./... && LC_ALL=C bash 1-5/scenario.sh`; затем `cd ../course && npm run check:labs`; Expected: PASS, сценарий ≤ 90 с локально.
- [ ] **Step 9: Commit** — `feat(labs): лаба 1.5b — соединения, аллокации, сборщик мусора, PGO`.

---

### Task 3: Песочница `gc-pacer`, плееры `tricolor` и `netpoller`

**Files:** Create: `course/viz/simulations/{gc-pacer,tricolor,netpoller}.ts` (+ `.test.ts`), `course/viz/renderers/{GcPacer,Tricolor,Netpoller}View.vue`, `course/viz/scenes/{GcPacerSandbox,Tricolor,Netpoller}.vue`

**Interfaces:**
- Consumes: формулы `course/viz/simulations/gomemlimit.ts` (`MEM_LIMIT_MIB`, модель цели кучи) — переиспользовать экспортом, не копировать.
- Produces: сцены `GcPacerSandbox`, `Tricolor`, `Netpoller` для уроков 1.5.7 и 1.5.9.

- [ ] **Step 1: Тесты `gc-pacer`** (`Simulation<S, A, F>`, шаг модели 100 мс):
  - `goal is 2x live at GOGC=100 without limit`: живая 100 МиБ → цель 200 МиБ;
  - `limit below goal raises GC frequency`: при лимите 150 МиБ циклов за 10 с больше, чем без лимита;
  - `limiter caps GC CPU near 50%`: живая куча выше лимита → доля CPU GC в каждом кадре ≤ 0,55 и средняя ≥ 0,4; без ограничителя (переключатель «как до 1.19») — растёт к 1,0 (спираль смерти);
  - `GOGC=off without limit never collects`: циклов 0, куча растёт до OOM контейнера (`MEM_LIMIT_MIB`);
  - `GOGC=off with limit collects only near limit`;
  - `leak action grows live heap`, `burst action raises alloc rate for 3 s`.
- [ ] **Step 2: Реализация `gc-pacer`** + рендерер (график кучи/цели/лимита во времени, полоса CPU GC, счётчики циклов и OOM) + сцена (ползунки `GOGC` 25–800 и «off», `GOMEMLIMIT` выкл/64–256 МиБ, живая куча, скорость аллокаций; кнопки «утечка», «всплеск», «старт/пауза»; переключатель «ограничитель 1.19+ / без ограничителя»). Подписи с `termId`.
- [ ] **Step 3: Плеер `tricolor`** — сценарий по спеке §5.2: граф 7–9 объектов, шаги разметки; вариант «без барьера» (мутатор кладёт указатель на белый объект в чёрный и стирает его из серого → белый освобождён, подсветка «освобождён живой») и «с барьером» (объект затеняется). Тесты: в конце варианта без барьера есть достижимый белый объект; с барьером — нет; инвариант «чёрный не указывает на белый» нарушается только в варианте без барьера.
- [ ] **Step 4: Плеер `netpoller`** — сценарий спеки §5.2 (`conn.Read` → `EAGAIN` → `gopark` на `pollDesc`, M свободен и выполняет другую G → данные → `epoll_wait` в `findRunnable` → G готова → читает) и вариант «дедлайн истёк» (таймер рантайма будит G с `os.ErrDeadlineExceeded`). Тесты: в каждом шаге у G ровно одно состояние; M не простаивает, пока есть готовые G; в варианте с дедлайном чтение заканчивается ошибкой.
- [ ] **Step 5: Проверить** — `npm test && npm run typecheck`; Expected: PASS.
- [ ] **Step 6: Commit** — `feat(viz): песочница gc-pacer, плееры tricolor и netpoller`.

---

### Task 4: Песочница `escape-lab`

**Files:** Create: `course/viz/data/escape-lab.json`, `course/viz/simulations/escape-lab.{ts,test.ts}`, `course/viz/renderers/EscapeLabView.vue`, `course/viz/scenes/EscapeLabSandbox.vue`

**Interfaces:** Consumes: JSON `labs/1-5/alloc/escape/dump` (задача 2). Produces: сцена `EscapeLabSandbox` для урока 1.5.8.

- [ ] **Step 1:** `cd labs && go run ./1-5/alloc/escape/dump > ../course/viz/data/escape-lab.json`.
- [ ] **Step 2: Тесты:** каталог песочницы (случаи, переключатели, комбинация → вариант) для каждой комбинации находит запись в JSON; у каждого случая есть хотя бы один вариант `heap`; у варианта `heap` строка компилятора содержит `escapes to heap` или `moved to heap`; исходник варианта для показа берётся из каталога (текст функции хранится в TS рядом с ключом, тест сверяет, что ключи совпадают с JSON).
- [ ] **Step 3: Реализация** — выбор случая (список), переключатели одной детали, вывод: код функции, плашка «стек/куча» (цвет со смыслом: куча — `--viz-warn`), реальная строка компилятора моноширинным шрифтом, пояснение правила одной фразой.
- [ ] **Step 4: Проверить** — `npm test && npm run typecheck`; Expected: PASS.
- [ ] **Step 5: Commit** — `feat(viz): песочница escape-lab на реальном выводе компилятора`.

---

### Task 5: Урок 1.5.7 «Netpoller и цена соединения» (тег `m01-5-l07`)

**Files:** Create: `course/modules/01-5-go-concurrency/07-netpoller.md`

**Interfaces:** Consumes: сцена `Netpoller`, лаба `netpoll`, выводы шага 6 задачи 2.

- [ ] **Step 1: Урок (контракт)** — строка 1.5.7 спеки, все пункты. Обязательно: `strace -f -e trace=epoll_ctl,epoll_pwait,epoll_wait` на `netpoll serve` (реальный вывод; связь с уроком 0b.1); таблица «замер против оценки 0.6 против Камардина» с `<N>`; разбор, куда ушли килобайты (стек, буфер `bufio`, `netFD`/`pollDesc`, ядро по `sockstat`), и разница `bufio` против `lazy`; почему `os.File` на диске держит M (связь с 1.5.1); `<Deep>` о `io_uring` и почему сеть Go на `epoll`; контраст «горутина на соединение против цикла событий» с выводом для модуля 3. Типичные ошибки (5–7): `ulimit -n` 1024 в контейнере, `SetReadDeadline` один раз вместо каждого чтения, буфер 64 КиБ на соединение «на всякий случай», забытый `Close` → утечка дескрипторов и горутин, один исходный адрес и исчерпание эфемерных портов в нагрузочном тесте.
- [ ] **Step 2: Проверить** — `npm run build && npm run check:refs && npm run check:go`; Expected: PASS.
- [ ] **Step 3: Commit и тег** — `docs(course): урок 1.5.7 — netpoller и цена соединения`; `git tag m01-5-l07`.

---

### Task 6: Урок 1.5.8 «Стек, куча и аллокатор» (тег `m01-5-l08`)

**Files:** Create: `course/modules/01-5-go-concurrency/08-stack-heap.md`

- [ ] **Step 1: Урок (контракт)** — строка 1.5.8 спеки. Обязательно: реальный вывод `-gcflags=-m=1` и `-m=2` на одном случае; схема `mcache` → `mcentral` → `mheap` (mermaid, классы палитры); таблица бенчмарков `alloc` с `benchstat`; вывод `stack` (рост стека); прямая связь с GC: «каждая аллокация — будущая работа сборщика» (мостик к 1.5.9); где в мессенджере аллокации — гипотеза, проверка — 1.5.10. Типичные ошибки: указатели «для скорости» на мелких структурах, `[]*T` против `[]T`, `fmt.Sprintf` в горячем пути, `sync.Pool` для объектов разного размера, оптимизация без профиля.
- [ ] **Step 2: Проверить** — как в задаче 5.
- [ ] **Step 3: Commit и тег** — `docs(course): урок 1.5.8 — стек, куча и аллокатор`; `git tag m01-5-l08`.

---

### Task 7: Урок 1.5.9 «Сборщик мусора» (тег `m01-5-l09`)

**Files:** Create: `course/modules/01-5-go-concurrency/09-gc.md`; Modify: `course/modules/01-go-service/17-service-in-container.md` (одна фраза-ссылка: «что модель упростила — урок 1.5.9»)

- [ ] **Step 1: Урок (контракт)** — строка 1.5.9 спеки. Обязательно: разбор одной строки `gctrace` по полям (реальная строка из лабы); `<N>` для цели кучи, 25 %, около 50 %; таблица «`GOGC` × `GOMEMLIMIT`: четыре сочетания и когда какое»; вывод `limiter` и `greentea` с `benchstat`; ограничитель описан как «около 50 %, дырявое ведро»; упрощения модели `gc-pacer` в `<Callout type="simplified">`. Типичные ошибки: `GOMEMLIMIT` равен лимиту контейнера (нет запаса под не-кучу), `GOGC=off` без лимита, `runtime.GC()` «для порядка», финализатор для освобождения ресурса, вера в «паузы GC — главная проблема Go» без замера.
- [ ] **Step 2: Проверить** — как в задаче 5.
- [ ] **Step 3: Commit и тег** — `docs(course): урок 1.5.9 — сборщик мусора`; `git tag m01-5-l09`.

---

### Task 8: Мессенджер — служебный порт, pprof, flight recorder, конфиг

**Files:**
- Create: `messenger/internal/platform/debugserver/{doc.go,debugserver.go,debugserver_test.go}`
- Create: `messenger/internal/platform/flightrec/{doc.go,recorder.go,recorder_test.go,dumper.go,dumper_test.go}`
- Modify: `messenger/cmd/messenger/{app.go,app_test.go,main.go}`, `messenger/.golangci.yml`, `messenger/compose.yaml`, `messenger/README.md`

**Interfaces:**
- Consumes: `lifecycle.Component`, `lifecycle.BackStopTimeout`, `httpserver.Run`/`Serve`, `httpserver.Stop`.
- Produces:
```go
// platform/flightrec
const (
	Window      = 10 * time.Second // MinAge рекордера; порог медленного запроса обязан быть меньше
	MaxBytes    = 16 << 20         // потолок памяти буфера трассы
	MinInterval = time.Minute      // не чаще одного файла: медленный период не завалит диск
)
type Recorder struct{ /* *trace.FlightRecorder, mu */ }
func Start() (*Recorder, error)                         // NewFlightRecorder{MinAge: Window, MaxBytes}; Start
func (r *Recorder) WriteTo(w io.Writer) (int64, error)  // под мьютексом: снимки не пересекаются
func (r *Recorder) Stop()

type Snapshotter interface{ WriteTo(w io.Writer) (int64, error) } // порт у потребителя (Dumper)
type Clock interface{ Now() time.Time }
type Dumper struct{ /* src, dir, threshold, clk, log, trigger chan struct{} (cap 1), last time.Time */ }
func NewDumper(src Snapshotter, dir string, threshold time.Duration, clk Clock, log *slog.Logger) (*Dumper, error) // threshold в (0, Window)
func (d *Dumper) Middleware(next http.Handler) http.Handler // замер длительности; медленный → неблокирующий сигнал
func (d *Dumper) Name() string                              // "flight-dumper"
func (d *Dumper) Run(ctx context.Context) error            // lifecycle.Component: пишет <dir>/flight-<UTC 20061020T150405Z>.trace

// platform/debugserver
const (
	MaxProfileDuration   = 60 * time.Second
	StopTimeout          = time.Second // < lifecycle.BackStopTimeout
	MutexProfileFraction = 100
	BlockProfileRate     = 10_000 // нс
)
type Snapshotter interface{ WriteTo(w io.Writer) (int64, error) }
func NewHandler(flight Snapshotter) http.Handler // flight == nil → /debug/flight отвечает 404
func EnableContentionProfiles()
type Server struct{ /* srv *http.Server, log */ }
func New(addr string, h http.Handler, log *slog.Logger) (*Server, error) // WriteTimeout = MaxProfileDuration + 10s
func (s *Server) Name() string                  // "debug"
func (s *Server) Run(ctx context.Context) error // httpserver.Run(ctx, srv, Stop{Timeout: StopTimeout})
func (s *Server) Serve(ctx context.Context, ln net.Listener) error // для тестов
```

- [ ] **Step 1: Тесты `flightrec`** (`synctest`, фейковый `Snapshotter` пишет `"trace"` и считает вызовы, `t.TempDir()`):
  - `TestNewDumperValidates`: `nil` src/clk/log, пустой `dir`, `threshold` `0` и `Window` → ошибки;
  - `TestDumperWritesOnSlowRequest`: обработчик «спит» `threshold + 1ms` виртуального времени → после `synctest.Wait` в каталоге один файл `flight-*.trace` с содержимым `trace`;
  - `TestDumperSkipsFastRequest`: `threshold − 1ms` → файлов нет;
  - `TestDumperRateLimit`: 5 медленных подряд → 1 файл; после `MinInterval` ещё медленный → 2 файла;
  - `TestMiddlewareNeverBlocks`: `Run` не запущен (никто не читает канал), 3 медленных запроса завершаются (ответ 200 у каждого), горутин после теста не осталось;
  - `TestDumperStopsOnCancel`: отмена → `Run` возвращает `nil`;
  - `TestRecorderWriteTo` (без `synctest`): `Start`, немного работы, `WriteTo` в буфер → `n > 0`; `Stop`; второй `Start` после `Stop` проходит.
- [ ] **Step 2: Реализация `flightrec`.** Ошибка записи файла — Warn в журнал, `Run` продолжает (снимок — диагностика, не повод ронять процесс). Комментарии: почему один рекордер на процесс, почему `MinAge > threshold`, почему канал ёмкостью 1 и отбрасывание.
- [ ] **Step 3: Тесты `debugserver`** (`synctest` + `net.Pipe`-слушатель как в тестах `httpserver` 1.5a там, где нужен сервер; остальное — `httptest.NewRecorder`):
  - `TestHandlerServesPprofIndex`: `GET /debug/pprof/` → 200, тело содержит `goroutine`;
  - `TestHandlerNamedProfiles`: `heap`, `goroutine`, `goroutineleak`, `mutex`, `block`, `allocs` → 200;
  - `TestProfileSecondsLimit`: `/debug/pprof/profile?seconds=61`, `?seconds=-1`, `?seconds=abc`, то же для `/debug/pprof/trace` → 400; `?seconds=1` не отвергается проверкой (дальше — `pprof.Profile`);
  - `TestFlightEndpoint`: с фейком → 200 и тело фейка, `Content-Type: application/octet-stream`, `Content-Disposition` с именем файла; с `nil` → 404;
  - `TestServerStopsDuringLongProfile`: идёт `?seconds=30`; отмена `ctx` → `Serve` возвращается не позже `StopTimeout` виртуального времени.
- [ ] **Step 4: Реализация `debugserver`.** Свой `http.ServeMux`; обработчики `pprof.Index` (на `/debug/pprof/`), `Cmdline`, `Profile`, `Symbol`, `Trace` и `/debug/flight`; именованные профили отдаёт `pprof.Index` по пути. Проверка `seconds` — обёртка до `Profile`/`Trace`. `ReadHeaderTimeout` — как у основного сервера (константа `httpserver` не экспортирована — своя константа с тем же значением и комментарием).
- [ ] **Step 5: Конфиг и сборка.**
  - Константы `envDebugAddr = "MESSENGER_DEBUG_ADDR"`, `envFlightDir = "MESSENGER_FLIGHT_DIR"`, `envFlightThreshold = "MESSENGER_FLIGHT_THRESHOLD"`; `config{DebugAddr, FlightDir string; FlightThreshold time.Duration}`; функция `loadDebug(cfg *config) error`: адрес — пусто или `net.SplitHostPort` без ошибки; каталог — пусто или существующий каталог (`os.Stat` + `IsDir`), непустой каталог при пустом адресе — ошибка; порог — `durationEnv(…, time.Second, false)` и `< flightrec.Window`.
  - `newApp`: при `DebugAddr != ""` — `debugserver.EnableContentionProfiles()`, `flightrec.Start()`, `debugserver.New(cfg.DebugAddr, debugserver.NewHandler(rec), log)` в `back`; при `FlightDir != ""` — `flightrec.NewDumper(rec, …)` в `back` и `dumper.Middleware` первым в цепочке `chain` (снаружи `RequestID`: замер всего запроса). `app` получает поле `recorder *flightrec.Recorder` (может быть `nil`); `run` останавливает его `defer` после `lifecycle.Run`.
  - Тесты `app_test.go`: `TestLoadDebugConfig` (табличный: всё пусто — выкл; `:6060` — вкл; `6060` без двоеточия — ошибка с именем; каталог без адреса — ошибка; несуществующий каталог — ошибка; порог `10s` — ошибка «меньше 10s»; порог `abc` — ошибка с именем), `TestNewAppDebugDisabled` (`back` содержит только очистку, `recorder == nil`), `TestNewAppDebugEnabled` (в `back` есть `debug`, а с каталогом — и `flight-dumper`), `TestPprofNotOnMainPort` (`GET /debug/pprof/` через `a.handler` → 404 problem+json).
- [ ] **Step 6: `.golangci.yml`** — depguard: `net/http/pprof` разрешён только в `internal/platform/debugserver`; `runtime/trace` — только в `internal/platform/flightrec`; оба пакета не импортируют `messenger/internal/{user,chat,auth}`.
- [ ] **Step 7: `compose.yaml` и README.** `MESSENGER_DEBUG_ADDR: ${MESSENGER_DEBUG_ADDR:-:6060}`, `MESSENGER_FLIGHT_DIR: ${MESSENGER_FLIGHT_DIR:-/tmp}`, `MESSENGER_FLIGHT_THRESHOLD: ${MESSENGER_FLIGHT_THRESHOLD:-1s}`; порт `"127.0.0.1:6060:6060"` с комментарием «только loopback: pprof раскрывает стеки и командную строку» и ссылкой на ADR-0013. Бинарь по умолчанию служебный порт не открывает (комментарий: compose — учебный стенд).
- [ ] **Step 8: Проверить** — `go test -race -count=1 ./... && go test -race -count=20 ./internal/platform/debugserver/... ./internal/platform/flightrec/... && golangci-lint run && (cd ../course && npm run check:messenger)`; Expected: PASS; `git grep -n "time.Sleep" -- 'internal/platform/debugserver/*_test.go' 'internal/platform/flightrec/*_test.go'` — пусто. Затем `docker compose up -d --build`: `curl -s 127.0.0.1:6060/debug/pprof/ | head`, `curl -s -o /dev/null -w '%{http_code}' 127.0.0.1:8080/debug/pprof/` → `404`; `docker compose stop` → код 0 и строка остановки служебного сервера после «сервер остановлен».
- [ ] **Step 9: Commit** — `feat(messenger): служебный порт с pprof и flight recorder по медленным запросам`.

---

### Task 9: Песочница `flamegraph` на профиле мессенджера

**Files:** Create: `labs/1-5/fold/{main.go,fold.go,fold_test.go}`, `messenger/scripts/read-load.sh`, `course/viz/data/flame-messenger.json`, `course/viz/simulations/flamegraph.{ts,test.ts}`, `course/viz/renderers/FlamegraphView.vue`, `course/viz/scenes/FlamegraphSandbox.vue`

**Interfaces:**
- Consumes: служебный порт (задача 8), `scripts/overload.sh`.
- Produces: `fold` читает текст `go tool pprof -traces` из stdin, пишет JSON `{ "sampleType": string, "unit": string, "total": number, "stacks": [{ "frames": string[] /* от корня к листу */, "value": number }] }`; `-min 0.001` сливает стеки с долей ниже порога в `["прочее"]`. Файл данных — объект `{ "cpu": …, "alloc_space": …, "inuse_space": … }`.

- [ ] **Step 1: Тесты `fold`** на фиксированном тексте `-traces` (два образца, общий префикс): кадры в порядке корень → лист, значения суммируются у одинаковых стеков, единицы (`ms`, `MB`, `kB`) приводятся к базовой (нс, байты), порог сливает мелкие в «прочее», `total` равен сумме.
- [ ] **Step 2: Реализация `fold`** и `scripts/read-load.sh` (N параллельных `GET /v1/chats/{id}/messages` с токеном; токен и чат создаются скриптом через API).
- [ ] **Step 3: Снять данные.** `docker compose up -d --build`; нагрузка `overload.sh` + `read-load.sh` 30 с; одновременно `curl -o cpu.pprof '127.0.0.1:6060/debug/pprof/profile?seconds=20'`, затем `heap` (`alloc_space`, `inuse_space` через `-sample_index`); `go tool pprof -traces -sample_index=… <бинарь> <профиль> | go run ./1-5/fold -min 0.001`; собрать `course/viz/data/flame-messenger.json` (≤ 150 КБ). Сырые профили — в рабочую папку плана (для урока).
- [ ] **Step 4: Тесты `flamegraph.ts`:** свёртка стеков в дерево; сумма долей детей ≤ доли родителя, корень = 100 %; зум на узел делает его корнем отображения с долей 100 % и сохраняет относительные доли поддерева; поиск подсвечивает все узлы с подстрокой и считает их суммарную долю (без двойного счёта вложенных); переключение типа выборки меняет дерево; данные из `flame-messenger.json` разбираются без ошибок.
- [ ] **Step 5: Реализация** — рендер прямоугольников (SVG, ширина — доля), подпись обрезается по ширине, подсказка с именем и долей (клавиатура: Tab по узлам, Enter — зум, Esc — к корню), поиск, `SegmentedToggle` для типа выборки, кнопка «к корню». Узлы мессенджера (`messenger/…`) и рантайма/стандартной библиотеки различаются цветом со смыслом (легенда).
- [ ] **Step 6: Проверить** — `npm test && npm run typecheck`; `cd ../labs && go test ./1-5/fold/...`; Expected: PASS.
- [ ] **Step 7: Commit** — `feat(viz): песочница flamegraph на профиле мессенджера`.

---

### Task 10: Урок 1.5.10 «Профилирование мессенджера», ADR-0013 (тег `m01-5-l10`)

**Files:** Create: `course/modules/01-5-go-concurrency/10-profiling.md`, `messenger/docs/adr/0013-служебный-порт-и-профилирование.md`; Modify: `messenger/docs/adr/0008-пробы-здоровья-и-остановка.md` (строка «pprof — служебный порт с 1.5.10, ADR-0013»), регионы в коде задачи 8 (`debug-handler`, `profile-limit`, `flight-middleware`, `flight-dumper`, `debug-wiring`)

- [ ] **Step 1: Реальный вывод** (машина в строке): `go tool pprof -top` CPU под нагрузкой (Argon2 сверху); `-list` одной функции мессенджера; `heap` — проверка гипотезы урока 1.5.6 о пике памяти при параллельных хешах (подтверждена/опровергнута — честно); `goroutine?debug=1` и `goroutineleak` (0 утечек); `mutex`/`block` (кто держит замки хранилищ, ожидание семафора `hashlimit`); `go tool trace` по `/debug/pprof/trace?seconds=5` — что видно на шкале; снимок flight recorder по медленному запросу (`MESSENGER_FLIGHT_THRESHOLD=100ms` + `overload.sh`, файл в `/tmp` контейнера, `docker compose cp`); цена профилей конкуренции и рекордера — `overload.sh`/`read-load.sh` с ними и без (p50/p99, RPS); PGO — числа из лабы.
- [ ] **Step 2: Урок (контракт)** — строка 1.5.10 спеки. Обязательно: как работает выборка (SIGPROF, 100 Гц; 512 КиБ) и что из этого следует для коротких функций; почему профиль снимают с продакшена; угрозы открытого pprof и как закрыто у нас; таблица «какой профиль на какой вопрос»; `<Deep>` о непрерывном профилировании (Google-Wide Profiling, Pyroscope/Parca); раздел «Что изменилось в мессенджере» с деревом файлов. Типичные ошибки: pprof на основном порту через `_ "net/http/pprof"`, профиль с ноутбука вместо продакшена, `-top` по `flat` без `cum`, CPU-профиль для поиска ожидания (нужны block/trace), бенчмарк без `-count` и `benchstat`, вечный `SetBlockProfileRate(1)` в продакшене.
- [ ] **Step 3: ADR-0013** (контракт): контекст (ADR-0008, вариант 5); решение (служебный порт, выключен по умолчанию, явная регистрация, лимит `seconds`, профили конкуренции, один flight recorder и снимки по медленным запросам с ограничением частоты); последствия (стоимость, обрыв профиля при остановке, пробы остаются на основном порту до модуля 12); отвергнутые варианты (pprof на основном порту за аутентификацией; всегда включённый служебный порт; только непрерывное профилирование; PGO в мессенджере сейчас).
- [ ] **Step 4: Проверить** — `npm run build && npm run check:refs && npm run check:go && npm test`; Expected: PASS.
- [ ] **Step 5: Commit и тег** — `docs(course): урок 1.5.10 — профилирование мессенджера; ADR-0013`; `git tag m01-5-l10`.

---

### Task 11: Банк вопросов и урок 1.5.11 «Мини-интервью модуля 1.5» (тег `m01-5-l11`)

**Files:** Create: `course/interview/module-01-5.yaml`, `course/modules/01-5-go-concurrency/11-interview.md`

**Interfaces:** Consumes: `InterviewSim`, `interview.data.ts` (читает все `*.yaml`), схема `interview/schema.ts`.

- [ ] **Step 1: Банк** — `module: "01-5"`, `title: "Модуль 1.5. Конкурентность и рантайм Go"`, 24–30 вопросов: по 2–4 на каждый урок 1.5.1–1.5.10, уровни `M+`/`S`, формат и правила — как в `module-01.yaml` (тезис → почему → компромисс → пример из мессенджера; чеклист 4–6 пунктов; 1–3 красных флага; 1–2 фоллоу-апа). `npm test -- interview` проверяет схему; если `bank.test.ts` перечисляет банки явно — добавить новый.
- [ ] **Step 2: Урок (контракт)** — структура 1.20: «Проблема»; как отвечать на вопросы о рантайме (меряю, а не верю: какое число и каким инструментом); симулятор `<InterviewSim :bank="data['01-5']" />`; сквозная задача «Спроектируй сервис, держащий 10⁵ соединений» по шагам (горутины и память на соединение с числами 1.5.7; лимиты ОС; обратное давление и отказ; остановка; GC и `GOMEMLIMIT`; что и как профилировать); итог модуля 1.5 (что умеет мессенджер на теге `m01-5-l11`, что вы теперь можете объяснить); «Что дальше» — модуль 2 (Postgres); типичные ошибки на собеседовании; самопроверка; источники.
- [ ] **Step 3: Проверить** — `npm test && npm run build && npm run check:refs`; Expected: PASS.
- [ ] **Step 4: Commit и тег** — `docs(course): урок 1.5.11 — мини-интервью модуля 1.5`; `git tag m01-5-l11`.

---

### Task 12: Итоговая проверка, связи с ранними уроками, README, бэклог

**Files:** Modify: `README.md`, `course/modules/01-5-go-concurrency/{index.md,01-goroutines-scheduler.md,04-sync.md,05-patterns.md,06-background-work.md}`, `course/modules/00-foundation/06-messenger-requirements.md`, `course/modules/00b-network/01-layers.md`, `docs/superpowers/backlog-after-module-0.md`

- [ ] **Step 1: Связи.** Все «подробно — 1.5b», «тема части 1.5b» в уроках 1.5.1, 1.5.4, 1.5.5, 1.5.6 → ссылки на 1.5.7–1.5.10 (`grep -rn "1\.5b" course/modules`). Урок 0.6: после «мерить будем в модуле 1.5» — фраза с числом замера и ссылкой на 1.5.7 (оценка остаётся). Урок 0b.1: ссылка на 1.5.7 у `epoll_ctl`. `index.md` — ссылки на все пять уроков.
- [ ] **Step 2: Бэклог.** Из «Остатков модуля 1.5a» закрыть то, что затронуто задачами 8–12 (по месту), остальное оставить; добавить «Выполнено в подпроекте „Модуль 1.5b“» и «Остатки модуля 1.5b».
- [ ] **Step 3: Все проверки.** `npm test && npm run typecheck && npm run check:go && npm run check:labs && npm run check:messenger && npm run check:refs && npm run check:workflows && npm run check:images && npm run build && npm run check:base`; Expected: PASS.
- [ ] **Step 4: Теги.** Для каждого `m01-5-l07…l11`: `git worktree add` в scratchpad, `go build ./... && go test -race ./...` в `messenger/` и `labs/`, `golangci-lint run` в `messenger/`; Expected: PASS на всех пяти.
- [ ] **Step 5: Playwright** на 375 и 1280 px, тёмная и светлая тема, уроки 1.5.7–1.5.11 (`npx vitepress preview . --port 4196`): нет горизонтальной прокрутки; каждая песочница реагирует на действие; полноэкранный режим открывается и закрывается Esc; mermaid без обрезки подписей; flame graph — зум и поиск работают с клавиатуры.
- [ ] **Step 6: README** — модуль 1.5 целиком, новые переменные мессенджера, служебный порт.
- [ ] **Step 7: Commit** — `docs: итог части 1.5b — страница модуля, связи уроков, README, бэклог`. Затем — ревью всей ветки и PR в `main` с согласия пользователя.
