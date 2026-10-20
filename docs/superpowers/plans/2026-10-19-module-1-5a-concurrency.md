# Модуль 1.5, часть 1.5a «Конкурентность в Go» — план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Уроки 1.5.1–1.5.6 о конкурентности в Go, лаба `labs/1-5`, две песочницы и четыре плеера. В мессенджере у всех горутин появляется один владелец, добавляются фоновая очистка ключей идемпотентности и ограничение одновременных хешей Argon2id. Весь репозиторий переходит на Go 1.27.2.

**Architecture:**
- Раскладка и правила слоёв модуля 1 не меняются.
- Новое в мессенджере — технические пакеты `platform/lifecycle` (компоненты под одним `errgroup`, порядок остановки) и `platform/periodic` (работа по `Ticker` с джиттером). В `auth/adapters/hashlimit` появляется декоратор над портом `PasswordHasher`, у самого порта — `ctx`. В `idempotency` добавляется узкий порт `Sweeper`.
- Лаба — маленькие программы в общем модуле `labs/` без Docker. Намеренные гонки и утечки собираются только с тегом `labdemo`.
- Код растёт урок за уроком. Тег `m01-5-lNN` ставится на последний коммит задачи урока. Визуализации (задачи 3–4) и лаба (задача 5) идут раньше уроков, переход на Go 1.27 — задача 1.

**Tech Stack:**
- курс: VitePress 1.6.4, Vue 3.5.43, TypeScript 5.9.3 (7.x не поддерживается `vue-tsc`), Vitest 5.0.3, mermaid 12.1.0, Node 26;
- мессенджер и лабы: **Go 1.27.2** (после задачи 1; локально у автора сейчас 1.26.5/1.26.9 — задача 1 ставит 1.27.2), `golangci-lint` **v2.14.0** (2026-09-24; поддержка Go 1.27 — с v2.13.0), `govulncheck` v1.8.0 (v1.9.0 от 2026-10-11 моложе двух недель), `golang.org/x/sync` **v0.24.0** (2026-09-23);
- `golang:1.27.2-trixie` — digest `sha256:47667934874913a449b96faa7ef4777fa285d9ec29126ee80ed61c45b7cccf0e` (проверен `docker buildx imagetools inspect` 2026-10-19; выпуск 2026-10-08 — исключение из правила двух недель как выпуск безопасности, спека §2).

**Spec:**
- `docs/superpowers/specs/2026-10-19-module-1-5a-concurrency-design.md` — главный.
- Спецификации 1a, 1b, 1c и `2026-10-09-course-platform-design.md` (§1, §2.2–2.5) — правила, которые продолжают действовать.

**Отступления от стандарта плана (как в модуле 1):**
- Go-код дан **сигнатурами и тестами**. Тело даётся только там, где тесты не определяют алгоритм (допуск в `hashlimit`).
- Тексты уроков, рендереры, сцены и ADR даны **контрактом**: поведение, обязательные свойства, имена. Ревьюер проверяет контракт.

**Факты, проверенные при подготовке (2026-10-19; не перепроверять, но не противоречить):**
- Release notes Go 1.27:
  - `goroutineleak` общедоступен в `runtime/pprof` и на `/debug/pprof/goroutineleak`, `GOEXPERIMENT=goroutineleakprofile` удалён;
  - появились `testing/synctest.Sleep` и `net/http/httptest.NewTestServer` (фейковая сеть для `synctest`);
  - `asynctimerchan` удалён, каналы `time` всегда синхронные;
  - `encoding/json` работает на реализации v2 (отказ — `GOEXPERIMENT=nojsonv2`);
  - `go test` запускает vet-проверку `stdversion`;
  - `go mod tidy` для `go 1.27` сливает блоки `require`;
  - аллокация мелких объектов до 30 % дешевле (`nosizespecializedmalloc`).
- Release notes Go 1.26:
  - Green Tea GC включён по умолчанию;
  - в `runtime/metrics` появились `/sched/goroutines…`, `/sched/threads:threads`, `/sched/goroutines-created:goroutines`.
- PR #4: `trivy` у `golang:1.27.0` — CVE-2026-78667, CVE-2026-78669 (net/http), CVE-2026-97031 (crypto/tls), исправлены в 1.27.2 и 1.26.9 (прогон `ci` 39224836632, джоба `image`).
- В мессенджере:
  - `auth/usecase.PasswordHasher` — `Hash(p domain.Password) (domain.PasswordHash, error)` и `Verify(plaintext string, hash domain.PasswordHash) (bool, error)`, без `ctx`;
  - `NewLogin` считает фиктивный хеш в конструкторе (`login.go:19`);
  - `SignUp.Execute` хеширует **до** `RegisterProfile` (имя проверяется только там);
  - `problem.Problem{Code, Detail, InvalidParams}` — заголовков нет;
  - в `openapi.yaml` есть общий ответ `ServiceUnavailable`;
  - `idempotency.Store` — `Reserve`/`Complete`/`Release`;
  - `storetest.Run(t, func(t) idempotency.Store)`;
  - `Clock` объявлен в `idempotency.go`, `clock.System` — конкретный тип;
  - строка журнала об остановке HTTP — «сервер остановлен»;
  - `loadTimeouts` проверяет `DrainDelay + ShutdownTimeout < terminationGrace`.
- `check-labs.ts`: `gofmt -l`, `go vet`, `go test -race` по всему `labs/`, `compose config` 0b и 1c, сценарий 1c в контейнере.
- Радиогруппа `.panel` повторена в `LayerCacheSandbox.vue` (4 раза), `IdempotencySandbox.vue`, `PaginationSandbox.vue`, `SupplyChainSandbox.vue`.

**Решения плана, уточняющие спеку (рулинги до исполнения):**
1. **Отказ при переполнении — `auth/usecase.ErrOverloaded`**, объявлен рядом с портом `PasswordHasher` как часть его контракта. Адаптер `httpapi` не импортирует `hashlimit`, потому что адаптеры друг друга не знают.
2. **Проверка имени до хеширования — новый узкий порт `ProfileValidator`** в `auth/usecase` с методом `ValidateProfile(ctx, username, displayName string) error`. Реализация — в `userbridge`, через `userdomain.NewUsername`/`NewDisplayName`; ошибки переводятся так же, как в `RegisterProfile`. `SignUp` получает его четвёртой зависимостью.
3. **`problem.Problem` получает поле `RetryAfter time.Duration`**: при `> 0` `Write` ставит `Retry-After` в целых секундах (округление вверх).
4. **Допуск в `hashlimit` — два буферизованных канала**: `slots` ёмкостью `N+Q` (неблокирующий захват, иначе `ErrOverloaded`) и `run` ёмкостью `N` (захват с `select` по `ctx.Done()`). `x/sync/semaphore` не используется — урок показывает его как альтернативу. `x/sync` всё равно становится прямой зависимостью ради `errgroup`.
5. **Порядок остановки в `lifecycle` — две стадии**: `lifecycle.Run(ctx, log, Stages{Front: []Component{http}, Back: []Component{…}})`. `Front` останавливается первым, `Back` получает отмену после выхода всех `Front` или после сбоя любого компонента. Срок `Back` — экспортируемая константа `lifecycle.BackStopTimeout = 2 * time.Second` (её учитывает `loadTimeouts`).
6. **HTTP-сервер как компонент** — адаптер в `cmd/messenger/main.go` над существующим `httpserver.Run`. Сам `httpserver` не меняется, кроме проверки `requestTimeout <= 0` и комментария `writeMargin`.
7. **Второй сигнал** — функция `exitOnSecondSignal(sig <-chan os.Signal, first <-chan struct{}, exit func(int), log *slog.Logger)` в `cmd/messenger/signals.go`; в тесте `exit` подменяется.
8. **Теги `m01-5-l01…l06`.** Переход на 1.27 (задача 1) и лаба (задача 5) входят в `m01-5-l01`, потому что тег ставится на последнем коммите задачи 6.
10. **`periodic` — `time.Timer` с `Reset`, а не `time.Ticker`** (спека §4.3 называет `Ticker`): джиттер нужен на каждый интервал, а `Ticker` держит фиксированный период; «следующая итерация не наслаивается» выполняется естественно — таймер взводится после окончания итерации. Урок 1.5.6 показывает оба и объясняет выбор.
9. **Числа в уроках снимает исполнитель** на своей машине. В урок вставляются вывод и строка «машина: <CPU>, <ядер>, <ОС>, go1.27.2».

## Global Constraints

- Команды npm — из `/home/idel/DevGo/system-desine/course`, Go-команды мессенджера — из `/home/idel/DevGo/system-desine/messenger`, лаб — из `/home/idel/DevGo/system-desine/labs`. `golangci-lint` — в `~/go/bin` (`PATH=$HOME/go/bin:$PATH`).
- Ветка `feat/module-1-5a` (создана, в ней коммит спеки). `main` защищён ruleset: слияние только через PR с зелёной `ci-ok`, и PR сливает пользователь. `git push` ветки — только с явного согласия пользователя.
- Язык контента — русский; английский оригинал термина даётся в скобках при первом упоминании. Каждый термин при первом появлении в уроке оформляется `<T id="…">`.
- Тексты уроков — по основной спецификации:
  - §1 «Голос автора»: упрощения помечаются `<Callout type="simplified">`;
  - §2.2 — все 10 правил, включая самодостаточность;
  - §2.3 — полноэкранный режим у каждой визуализации;
  - §2.4 — `<Part>` у каждого `##`, `<N>` для 3–8 ключевых чисел (только результат формулы или измерения), `<Deep>` на 150–500 слов.
- Структура урока — как у 1.17:
  - «Проблема» → теория (несколько `##`) → «Визуализация» → «Код»;
  - затем «Что появилось в лабе» (1.5.1–1.5.5) или «Что изменилось в мессенджере» (1.5.6);
  - затем «Типичные ошибки» → «Компромисс» → «Самопроверка» (5–8 `<SelfCheck>`) → «Источники».
- Заголовок урока — `title: "1.5.N. …"`. Строку «Модуль 1.5 · Урок N» даёт `lesson-kicker.ts` по папке `01-5-go-concurrency`.
- Чистые TS-модули не импортируют `vue` и не трогают DOM. В компонентах DOM трогают только `onMounted`, обработчики и `watch` без `immediate`; слушатели снимаются в `onBeforeUnmount`. Цвета задаются только CSS-переменными. Обязательны обе темы, контраст ≥ 4,5:1, `prefers-reduced-motion`, ширина 375 px без горизонтальной прокрутки и `VizFrame` у каждой сцены.
- Go-код — по стандарту пользователя (`~/.claude/CLAUDE.md`) и §2.5:
  - порт объявляется у потребителя, интерфейсы узкие;
  - конструкторы `NewX(deps…) (*X, error)` отвергают `nil` и неверные значения;
  - `ctx` — первый аргумент;
  - ошибки оборачиваются `%w`, сообщения — по-русски со строчной буквы;
  - у каждой горутины владелец и способ остановки;
  - `gofmt`, `go vet`, `golangci-lint run`, `go test -race` — чистые;
  - все лимиты и сроки — именованные константы с комментарием «почему».
- **Новые тесты с временем и горутинами — только через `testing/synctest`.** Ни один тест не ждёт через `time.Sleep`.
- Код в уроках вставляется только через `<<< @/../<путь>#<регион>{lang}`. Иллюстративные Go-фрагменты помечаются `<!-- go-check -->`.
- Вывод команд в уроках — реальный, с описанием машины.
- Коммиты на русском, заканчиваются строкой `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Факты сверяются с первоисточником:
  - go.dev/ref/mem;
  - исходники Go 1.27.2: `runtime/proc.go`, `runtime/chan.go`, `runtime/select.go`, `runtime/preempt.go`, `sync/mutex.go`, `internal/sync/mutex.go`;
  - release notes 1.14, 1.23, 1.25–1.27;
  - документация `golang.org/x/sync`;
  - Cox-Buday «Concurrency in Go».

## Review Focus

1. **Отмена `ctx` ожидающего вызова в `hashlimit`.** Ожидается: место в очереди освобождается, следующий вызов проходит, счётчики не «утекают». Покрывает `TestLimiterCancelReleasesSlot` (задача 11).
2. **Паника внутри `next.Hash` под ограничителем.** Ожидается: место освобождается через `defer`, хотя процесс всё равно падает. При перехвате паники выше ограничитель не должен «заклинить» навсегда. Покрывает `TestLimiterReleasesOnPanic` (задача 11).
3. **Сбой компонента `Back`, пока `Front` (HTTP) работает.** Ожидается: `Front` получает отмену и проходит полный дренаж, `Back` останавливаются, `Run` возвращает ошибку с именем упавшего компонента, а не `nil` и не ошибку отмены. Покрывает `TestRunBackFailureStopsFrontWithDrain` (задача 12).
4. **Компонент, игнорирующий отмену.** Ожидается: `Run` не висит вечно, через `backStopTimeout` возвращает ошибку «не остановился за 2s» с именем компонента. Покрывает `TestRunBackStopTimeout` (задача 12).
5. **`MESSENGER_HASH_QUEUE=0` и пустые или нечисловые значения переменных.** Ожидается: очередь 0 означает «отказ сразу, если все `N` заняты»; мусор даёт ошибку старта с именем переменной. Покрывают табличные `TestLoadConcurrencyConfig` (задача 12) и `TestLimiterZeroQueue` (задача 11).

---

## Структура файлов

```
.github/workflows/ci.yml                    # задача 1 (GO_VERSION, golangci-lint), 5 (labs/1-5 в джобе labs)
messenger/
├── go.mod, go.sum                          # задачи 1, 12 (x/sync прямой)
├── Dockerfile                              # задача 1
├── compose.yaml                            # задача 12 (новые переменные)
├── .golangci.yml                           # задача 12 (depguard)
├── cmd/messenger/{main.go,app.go,app_test.go,signals.go,signals_test.go,config.go?}  # задача 12
├── internal/platform/
│   ├── lifecycle/{doc.go,lifecycle.go,lifecycle_test.go}       # задача 12
│   ├── periodic/{doc.go,periodic.go,periodic_test.go}          # задача 12
│   ├── idempotency/{idempotency.go,memory/store.go,memory/store_test.go,storetest/storetest.go}  # задача 12
│   ├── problem/{problem.go,problem_test.go}                    # задача 11 (RetryAfter, CodeOverloaded)
│   └── httpserver/server.go                                    # задача 12 (проверка, комментарий)
├── internal/auth/
│   ├── usecase/{ports.go,signup.go,login.go,*_test.go}         # задача 11
│   └── adapters/{argon2id,hashlimit,userbridge,httpapi}/…      # задача 11
├── api/openapi.yaml (+ gen)                                    # задача 11
└── docs/adr/{0011-…,0012-…}.md                                 # задачи 1, 13
labs/
├── go.mod                                  # задача 1
├── 0b/Dockerfile, 1c/Dockerfile            # задача 1
└── 1-5/{README.md,scenario.sh,sched,chan,race,sync,patterns}   # задача 5
course/
├── glossary/terms.yaml                     # задача 2
├── scripts/check-labs.ts                   # задача 5
├── viz/components/SegmentedToggle.vue      # задача 2
├── viz/simulations/{channel-lab,worker-pool,gmp-scheduler,happens-before,mutex-modes,app-lifecycle}.ts (+ .test.ts)  # задачи 3, 4
├── viz/renderers/{ChannelLab,WorkerPool,GmpScheduler,HappensBefore,MutexModes,AppLifecycle}View.vue
├── viz/scenes/{ChannelLabSandbox,WorkerPoolSandbox,GmpScheduler,HappensBefore,MutexModes,AppLifecycle}.vue
├── modules/01-go-service/examples/dockerfiles/*.Dockerfile     # задача 1
├── modules/01-go-service/{04,07,12,13,17}-*.md                 # задачи 1, 13 (версии, ссылки, правки)
└── modules/01-5-go-concurrency/{index.md,01-…06-*.md}          # задачи 2, 6–10, 13
```

---

### Task 1: Переход на Go 1.27.2 и ADR-0011

**Files:**
- Modify: `messenger/go.mod`, `messenger/go.sum`, `labs/go.mod`, `labs/go.sum` (если есть)
- Modify: `messenger/Dockerfile`, `labs/0b/Dockerfile`, `labs/1c/Dockerfile`, `course/modules/01-go-service/examples/dockerfiles/{naive,alpine,scratch}.Dockerfile`
- Modify: `.github/workflows/ci.yml` (`GO_VERSION`, `GOLANGCI_LINT_VERSION`), скрипты `course/scripts/check-*.ts`, если в них закреплена версия линтера
- Modify: уроки модуля 1 — только места, описывающие текущее состояние (см. шаг 5)
- Create: `messenger/docs/adr/0011-линия-go.md`

**Interfaces:**
- Produces: тулчейн 1.27.2 везде; ADR-0011 (на него ссылаются урок 1.5.1 и задача 13).

- [ ] **Step 1: Поставить Go 1.27.2 локально и проверить факты release notes.** `go install golang.org/dl/go1.27.2@latest && go1.27.2 download` (или пакет ОС); `go version` → `go1.27.2`. Проверить маленькими программами в scratchpad (не в репозитории): `synctest.Sleep` компилируется; `httptest.NewTestServer` существует; `pprof.Lookup("goroutineleak") != nil` без `GOEXPERIMENT`. Если что-то не так — остановиться и сообщить контроллеру (спека и уроки опираются на эти факты).

- [ ] **Step 2: Поднять версии.**
  - `go.mod` обоих модулей: `go 1.27.0`; `go mod tidy` (ожидаемо сливает блоки `require` — это нормально).
  - Все `FROM golang:…` → `golang:1.27.2-trixie@sha256:47667934874913a449b96faa7ef4777fa285d9ec29126ee80ed61c45b7cccf0e` (в `labs/0b/Dockerfile` до этого не было digest — добавить).
  - `ci.yml`: `GO_VERSION: "1.27.x"`, `GOLANGCI_LINT_VERSION: v2.14.0`; локально `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0`.

- [ ] **Step 3: Прогнать всё и разобрать падения.**

Run (messenger): `go build ./... && go vet ./... && go test -race -count=1 ./... && golangci-lint run`
Run (labs): `go vet ./... && go test -race ./...`
Run (course): `npm test && npm run typecheck && npm run check:go && npm run check:messenger && npm run check:labs && npm run check:images && npm run build`
Expected: PASS. Если упали тесты, сравнивающие текст ошибок JSON (`httpjson.DecodeError` → `detail`), — обновить ожидания тестов на новый текст и **записать старый и новый текст** для шага 5. Новые замечания линтера v2.14.0 — исправить код, а не отключать проверку. `trivy` в `check:images` — без HIGH/CRITICAL с исправлением.

- [ ] **Step 4: ADR-0011 «Линия Go в репозитории»** (контракт, по шаблону `0000-template.md`): одна версия Go на весь репозиторий (перечислить все места из шага 2); бот приносит только патчи (`dependabot.yml`, правило `ignore`); минорная версия — отдельная задача с прогоном всех проверок; исключение из выдержки 14 дней для выпусков безопасности (на примере PR #4 и 1.27.2, с CVE и датами); отвергнутые варианты: «каждый модуль на своей версии», «сливать минорные PR бота».

- [ ] **Step 5: Тексты.** `git grep -n "1\.26" course/modules` — по каждому вхождению решить по правилу спеки §2: исторический факт — оставить; текущее состояние, которое не привязано к снятому выводу, — обновить на 1.27; реальный вывод уроков 1.15–1.19 — оставить, добавив рядом один раз на урок пометку «вывод снят на Go 1.26.9 (тег `m01-lNN`)». Если в шаге 3 изменился текст ошибки JSON, а уроки 1.9–1.10 показывают его реальным выводом — оставить вывод с той же пометкой и добавить фразу «с Go 1.27 текст `detail` другой: …». README: версия Go.

- [ ] **Step 6: Проверить** — `npm run build && npm run check:refs`; Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add -A messenger labs .github course
git commit -m "chore: переход на Go 1.27.2 и golangci-lint v2.14.0; ADR-0011

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Глоссарий, `SegmentedToggle`, страница модуля

**Files:**
- Modify: `course/glossary/terms.yaml`
- Create: `course/viz/components/SegmentedToggle.vue` (+ тест логики клавиш, если она выносится в `.ts`)
- Modify: `course/viz/scenes/{LayerCacheSandbox,IdempotencySandbox,PaginationSandbox,SupplyChainSandbox}.vue`
- Create: `course/modules/01-5-go-concurrency/index.md`

**Interfaces:**
- Produces: id терминов (ниже) — их используют задачи 3–13; компонент `SegmentedToggle` с props `{ label: string; options: readonly { value: T; text: string }[]; modelValue: T }` и событием `update:modelValue` — используют задачи 3 и 4.

- [ ] **Step 1: Термины.** Добавить (формат как у `race-detector`: `term`, `en`, `short`, `long`; ссылки в `long` — на уроки 1.5.N): `goroutine`, `os-thread`, `go-scheduler`, `gmp`, `run-queue`, `work-stealing`, `preemption`, `sysmon`, `channel`, `unbuffered-channel`, `buffered-channel`, `select`, `memory-model`, `happens-before`, `mutex`, `rwmutex`, `starvation-mode`, `atomic-operation`, `cas`, `false-sharing`, `cache-line`, `sync-pool`, `pipeline-pattern`, `fan-out`, `fan-in`, `worker-pool`, `semaphore`, `errgroup`, `backpressure`, `goroutine-leak`, `synctest`, `ticker`, `jitter`, `thundering-herd`, `component-lifecycle`, `crash-only`. Перед добавлением — `grep "^<id>:"`; существующие не дублировать. Дополнить `data-race` и `race-detector` (`long`: модель памяти, векторные часы, «подробно — уроки 1.5.3 и 1.6»); `gomaxprocs` — минимум из affinity и квоты cgroup (бэклог 1c).

- [ ] **Step 2: `SegmentedToggle`.** Радиогруппа из кнопок `role="radio"` с `aria-checked`, стрелки ←/→ двигают выбор и фокус (roving tabindex: `tabindex=0` только у выбранной), вид — как у нынешнего `.panel`. Заменить им все радиогруппы `.panel` в четырёх песочницах; поведение песочниц не меняется (их Vitest-тесты логики остаются зелёными; визуально — Playwright-скриншот одной песочницы до и после).

- [ ] **Step 3: `index.md`** — `title: Модуль 1.5. Конкурентность и рантайм Go`, `order: 1.5`; абзац «зачем модуль», список «в этом модуле вы…» (по целям спеки §1), таблица уроков 1.5.1–1.5.6 (пока без ссылок на несуществующие файлы — ссылки добавляются в задачах уроков; либо ссылки сразу, если сборка не падает на отсутствующих — проверить `npm run build`), строка «часть 1.5b — следующая».

- [ ] **Step 4: Проверить** — `npm test && npm run typecheck && npm run build`; Expected: PASS; в сайдбаре модуль 1.5 стоит между 1 и 2a (2a ещё нет — после модуля 1).

- [ ] **Step 5: Commit** — `feat(course): глоссарий модуля 1.5, SegmentedToggle, страница модуля`.

---

### Task 3: Песочницы `channel-lab` и `worker-pool`

**Files:**
- Create: `course/viz/simulations/channel-lab.ts`, `channel-lab.test.ts`, `worker-pool.ts`, `worker-pool.test.ts`
- Create: `course/viz/renderers/{ChannelLab,WorkerPool}View.vue`, `course/viz/scenes/{ChannelLabSandbox,WorkerPoolSandbox}.vue`

**Interfaces:**
- Consumes: `Simulation<S, A, F>`, `Step`, `term()`, `Player`, `VizFrame`, `SegmentedToggle` (задача 2); id глоссария (задача 2).
- Produces: сцены `ChannelLabSandbox` (урок 1.5.2), `WorkerPoolSandbox` (урок 1.5.5).

- [ ] **Step 1: `channel-lab` — модель и тесты.**

Типы (экспорт):
```ts
type GState = 'running' | 'blocked-send' | 'blocked-recv' | 'panicked' | 'done'
interface Goroutine { id: 'G1' | 'G2' | 'G3' | 'G4'; state: GState; holding?: number; got?: { value: number | null; ok: boolean } }
interface ChannelLabState { cap: 0 | 1 | 3; nil: boolean; closed: boolean; buf: number[]; sendq: { g: string; value: number }[]; recvq: string[]; gs: Goroutine[]; log: { kind: 'handoff' | 'buffered' | 'blocked' | 'woke' | 'panic' | 'zero' | 'default'; text: string }[] }
type ChannelLabAction = { type: 'send'; g: string; value: number } | { type: 'recv'; g: string } | { type: 'close'; g: string } | { type: 'setCap'; cap: 0 | 1 | 3 } | { type: 'setNil'; nil: boolean } | { type: 'selectDefault'; g: string; op: 'send' | 'recv'; value?: number } | { type: 'reset' }
export const channelLab: Simulation<ChannelLabState, ChannelLabAction>
```
Тексты паник — дословно рантайма: `send on closed channel`, `close of closed channel`, `close of nil channel`. Заблокированная горутина действий не принимает (`apply` возвращает состояние без изменений и строку журнала «G2 заблокирована»).

Тесты (`it` по-русски):
- `cap=0`, G2 ждёт получения, G1 отправляет 7 → `buf` пуст, G2 `got {7, ok:true}`, журнал `handoff`;
- `cap=1`: отправка 1 — в буфер; отправка 2 — G блокируется (`sendq` длины 1);
- получение из буфера при ждущем отправителе: получатель берёт голову буфера, значение отправителя встаёт в хвост, отправитель просыпается (порядок FIFO сохраняется);
- `close` будит всех из `recvq` с `{null, ok:false}`, ждущие отправители — `panicked` с `send on closed channel`;
- закрытый канал с данными в буфере: получения сначала отдают данные с `ok:true`, затем ноль с `ok:false`;
- повторный `close` → паника `close of closed channel`; `close` nil-канала → `close of nil channel`;
- nil-канал: и отправка, и получение блокируют навсегда, `close` — паника;
- `selectDefault` на полном/пустом канале не блокирует, журнал `default`.

- [ ] **Step 2: `worker-pool` — модель и тесты.** Дискретная модель по тикам 100 мс, детерминированная (без `Math.random`; при необходимости — свой PRNG с сидом).

```ts
type PoolMode = 'unbounded' | 'pool' | 'semaphore'
interface WorkerPoolParams { mode: PoolMode; lambda: number /* задач/с */; workers: number; queue: number; taskMs: number; burst: boolean /* ×5 на 3 с с t=5 с */ }
interface WorkerPoolTick { t: number; goroutines: number; memKiB: number /* goroutines × 8 КиБ — оценка, подпись говорит «оценка» */; queued: number; done: number; rejected: number; p50ms: number; p99ms: number }
export function simulateWorkerPool(p: WorkerPoolParams, seconds: number): WorkerPoolTick[]
```
Пропускная способность пула — `workers × 1000 / taskMs` задач/с. `unbounded`: каждая задача — горутина, задачи выполняются с замедлением при числе одновременно работающих > `workers` (CPU — `workers` ядер: время задачи × (активные / workers)). `pool`: `workers` горутин + очередь без отказов (очередь `queue` — буфер канала; при переполнении производитель блокируется, задержка копится у производителя). `semaphore`: при `workers` занятых и `queue` ждущих новая задача отвергается (`rejected++`).

Тесты:
- при `lambda` > пропускной способности `unbounded`: `goroutines` к концу ≥ 3× от значения на 2-й секунде (рост без предела);
- `pool`: `goroutines ≤ workers + 1` во всех тиках, `p99ms` растёт со временем при перегрузке, `rejected === 0`;
- `semaphore`: `p99ms` ограничена `(queue / workers + 1) × taskMs × 1.1`, `rejected > 0` при перегрузке;
- при `lambda` = 50 % пропускной способности все три режима: `p99ms` в пределах 20 % друг от друга, `rejected === 0`;
- `burst`: у `semaphore` отказы появляются только в окне всплеска (±1 с).

Песочница: `Simulation` с действиями на изменение параметров и `run` (пересчёт `seconds = 20`); кадры для `Player` — по 1 на секунду.

- [ ] **Step 3: Рендереры и сцены.**
  - `ChannelLabView`: канал как кольцевой буфер ячеек (ёмкость 0 — «рандеву», одна щель), очереди `sendq`/`recvq` слева/справа, карточки G1–G4 со статусом цветом **и** текстом, журнал последних 6 событий; кнопки действий у каждой G (недоступные — `disabled` с `title` «G заблокирована»), `SegmentedToggle` ёмкости, переключатель nil, «Сброс».
  - `WorkerPoolView`: три SVG-графика во времени с общей осью (горутины/память; очередь; p50/p99 и отказы), ползунки `lambda` 10–500, `workers` 1–16, `queue` 0–64, `taskMs` 10–500, `SegmentedToggle` режима, флажок «всплеск». `viewBox`, без фиксированной ширины.
- [ ] **Step 4: Проверить** — `npm test && npm run typecheck`; Expected: PASS.
- [ ] **Step 5: Commit** — `feat(viz): песочницы channel-lab и worker-pool`.

---

### Task 4: Плееры `gmp-scheduler`, `happens-before`, `mutex-modes`, `app-lifecycle`

**Files:**
- Create: `course/viz/simulations/{gmp-scheduler,happens-before,mutex-modes,app-lifecycle}.ts` и `.test.ts`
- Create: `course/viz/renderers/{GmpScheduler,HappensBefore,MutexModes,AppLifecycle}View.vue`, `course/viz/scenes/{GmpScheduler,HappensBefore,MutexModes,AppLifecycle}.vue`

**Interfaces:**
- Consumes: `Scenario`, `Step`, `term()`, `Player` (варианты — как в `GracefulShutdown.vue` задачи 3 модуля 1c), `VizFrame`, `SegmentedToggle`.
- Produces: сцены для уроков 1.5.1, 1.5.3, 1.5.4, 1.5.6.

Каждый модуль экспортирует `xxxScenario(variant?): readonly Step<XxxFrame>[]` — чистый и детерминированный.

- [ ] **Step 1: `gmp-scheduler` (1.5.1).** Кадр: `{ ps: { id: 'P0' | 'P1'; m: string | null; running: string | null; runnext: string | null; local: string[] }[]; global: string[]; ms: { id: string; state: 'running' | 'spinning' | 'syscall' | 'idle' }[]; note: 'spawn' | 'steal' | 'syscall-handoff' | 'preempt' | 'syscall-return' | 'idle' }`. Шаги (по одному кадру и больше): G1 на P0 делает `go G2`, `go G3`, `go G4` (G4 в `runnext`, предыдущие — в локальную очередь); P1 пуст → крадёт половину локальной очереди P0; G5 на P1 входит в блокирующий syscall → M1 уходит в `syscall`, sysmon передаёт P1 новому M2; G1 крутит цикл > 10 мс → вытеснение сигналом (`preempt`), G1 в конец очереди; G5 возвращается из syscall, свободного P нет → в глобальную очередь, M1 → `idle`.
  Тесты: на каждом кадре у каждого P не больше одной `running` G; после `steal` длины очередей P0 и P1 отличаются не больше чем на 1; во время `syscall-handoff` у P1 `m` ≠ M1; после `syscall-return` G5 в `global`; подписи содержат `gmp`, `work-stealing`, `preemption`, `sysmon`.

- [ ] **Step 2: `happens-before` (1.5.3)** — варианты `none`, `channel`, `mutex`. Кадр: `{ g1: Event[]; g2: Event[]; edges: { from: string; to: string }[]; outcome: 'any' | 'guaranteed' | null; raceReport: boolean }`, `Event = { id: string; op: string; t: number }`. `none`: G1 пишет `data = 42`, `done = true`; G2 читает `done`, `data` — рёбер нет, `outcome: 'any'`, `raceReport: true` (с текстом в стиле `WARNING: DATA RACE` в подписи). `channel`: `data = 42` → `ch <- struct{}{}`, `<-ch` → `read data`; ребро «отправка → завершение получения» (правило модели памяти: отправка синхронизирована с соответствующим получением). `mutex`: `Unlock` n → `Lock` n+1.
  Тесты: в `none` нет рёбер и `raceReport`; в `channel` и `mutex` есть путь по рёбрам и программному порядку от записи `data` к чтению `data`, `outcome: 'guaranteed'`, `raceReport: false`; подписи содержат `happens-before`, `data-race`, `memory-model`.

- [ ] **Step 3: `mutex-modes` (1.5.4).** Кадр: `{ t: number /* мс */; mode: 'normal' | 'starving'; owner: string | null; waiters: { g: string; since: number }[]; arriving: string | null; handoff: 'barging' | 'direct' | null }`. Шаги: нормальный режим — G1 держит, G2 ждёт; G1 отпускает, G2 просыпается, но новоприбывшая G3 (уже на CPU) захватывает раньше (`barging`); G2 ждёт > 1 мс → `starving`; при `Unlock` замок передаётся напрямую G2 (`direct`), новоприбывшие встают в хвост; очередь пуста или последний ожидавший ждал < 1 мс → `normal`.
  Тесты: `starving` наступает только когда у первого ожидающего `t − since > 1`; в `starving` `handoff` всегда `direct`, `arriving` не становится `owner`; последний кадр `normal`; подписи содержат `mutex`, `starvation-mode`.

- [ ] **Step 4: `app-lifecycle` (1.5.6)** — варианты `signal`, `component-failure`, `second-signal`. Кадр: `{ t: number /* с */; http: 'serving' | 'draining' | 'shutting-down' | 'stopped'; readyz: 200 | 503; back: { name: 'idempotency-sweeper'; state: 'running' | 'stopping' | 'stopped' | 'failed' }[]; hashQueue: { running: number; waiting: number; rejected: number }; exit: number | null; note: string }`. Значения как в мессенджере: `DRAIN_DELAY` 5 с, `SHUTDOWN_TIMEOUT` 10 с, `backStopTimeout` 2 с, `HASH_CONCURRENCY` 2, `HASH_QUEUE` 8.
  - `signal`: работа (очистка по тику, хеши: 2 выполняются, 3 ждут) → SIGTERM: `readyz` 503, `draining` → t=5 `shutting-down` → HTTP `stopped` → `back` `stopping` → `stopped` → `exit: 0`.
  - `component-failure`: `back` → `failed` → HTTP `draining` (тот же путь) → `exit: 1`.
  - `second-signal`: SIGTERM → `draining` → второй SIGTERM на t=2 → `exit: 1` сразу, HTTP не дошёл до `stopped`.
  Тесты: во всех вариантах `back` не переходит в `stopping`, пока `http` не `stopped` (кроме `second-signal`, где выход немедленный); `component-failure` последний кадр `exit: 1` и `readyz` стал 503 до `shutting-down`; `second-signal` — `exit: 1` при `t < 5`; подписи содержат `component-lifecycle`, `graceful-shutdown`, `errgroup`, `crash-only`.

- [ ] **Step 5: Рендереры и сцены.** `GmpSchedulerView`: P как колонки с очередями, M под ними, глобальная очередь сверху, на 375 px P одна под другой. `HappensBeforeView`: две вертикальные шкалы, рёбра — стрелки SVG, бейдж «гонка» цветом и текстом. `MutexModesView`: шкала времени в мс с порогом 1 мс, очередь ожидающих, режим крупно. `AppLifecycleView`: шкала 0–20 с, дорожки HTTP / фоновые / очередь хешей, итоговый код выхода.
- [ ] **Step 6: Проверить** — `npm test && npm run typecheck`; Expected: PASS.
- [ ] **Step 7: Commit** — `feat(viz): плееры gmp-scheduler, happens-before, mutex-modes, app-lifecycle`.

---

### Task 5: Лаба `labs/1-5`

**Files:**
- Create: `labs/1-5/README.md`, `labs/1-5/scenario.sh`
- Create: `labs/1-5/sched/{cost,preempt,syscall}/main.go`, `labs/1-5/sched/cost_test.go` (бенчмарки)
- Create: `labs/1-5/chan/{ops,timers}/main.go`
- Create: `labs/1-5/race/{flagloop,iface}/main.go` (тег `labdemo`)
- Create: `labs/1-5/sync/sync_test.go` (бенчмарки Mutex/RWMutex/atomic, false sharing), `labs/1-5/sync/starve/main.go`
- Create: `labs/1-5/patterns/{leak,limit}/main.go` (`leak` — тег `labdemo`)
- Modify: `course/scripts/check-labs.ts`, `.github/workflows/ci.yml` (джоба `labs`)

**Interfaces:**
- Produces: регионы `// #region …` для уроков (имена ниже); вывод программ — для уроков 1.5.1–1.5.5.

Правило: всё без тега `labdemo` проходит `go vet` и `go test -race` общего модуля. Программы с тегом `labdemo` намеренно гоняются/текут; `scenario.sh` собирает их с `-tags labdemo` и проверяет вывод.

- [ ] **Step 1: Программы** (каждая печатает ровно то, что урок вставит; первые строки — `go version`, `GOMAXPROCS`, `runtime.NumCPU()`):
  - `sched/cost` — создаёт 100 000 горутин, ждущих на канале; печатает прирост `runtime/metrics` `/memory/classes/heap/stacks:bytes` на горутину и время создания на горутину; то же для 1 000 потоков ОС через `runtime.LockOSThread` в горутинах (печатает рост RSS из `/proc/self/status` `VmRSS` и время). Регион `cost`.
  - `sched/preempt` — `GOMAXPROCS(1)`, горутина в тесном цикле без вызовов функций, главная горутина ждёт `time.Sleep(10ms)` и печатает «main проснулся через …»; запускается дважды: обычно и с `GODEBUG=asyncpreemptoff=1` (во втором случае main ждёт, пока цикл не кончится — цикл ограничен 2 с, программа печатает время). Регион `preempt`.
  - `sched/syscall` — 8 горутин в блокирующем `syscall.Nanosleep` по 200 мс (не `time.Sleep`!), печатает `/sched/threads:threads` до и во время; рядом 8 горутин в `time.Sleep` — потоков не прибавляется. Регион `syscall`.
  - `chan/ops` — таблица операций (send/recv/close/len/cap) над nil, открытым, закрытым каналом: каждая операция в горутине с `recover` и тайм-аутом 50 мс для «блокирует»; печатает таблицу Markdown. Регион `ops`.
  - `chan/timers` — цикл `select` с `time.After(1s)` и быстрым каналом 1e6 итераций: печатает `HeapAlloc` до/после и число живых таймеров нельзя — печатает память; комментарий в коде о поведении до 1.23 и о 1.27. Регион `timers`.
  - `race/flagloop` (`labdemo`) — `done bool` без синхронизации и цикл `for !done {}`; запуск с тайм-аутом 2 с через `context` в main-обёртке: печатает «зависло» или «завершилось»; рядом — версия с `atomic.Bool`. Регион `flagloop`.
  - `race/iface` (`labdemo`) — две горутины пишут в переменную-интерфейс значения разных типов, третья вызывает метод: ожидается паника/`SIGSEGV` (без `-race`) или `WARNING: DATA RACE` (с `-race`). Регион `iface`.
  - `sync/sync_test.go` — `BenchmarkCounter{Mutex,RWMutexRead,Atomic}` с `b.RunParallel`; `BenchmarkFalseSharing{Packed,Padded}` (два `atomic.Int64` рядом и через `[56]byte` / `cpu.CacheLinePad` из `golang.org/x/sys/cpu`, если он уже в `labs/go.mod`, иначе ручной паддинг 64 байта — **без новых зависимостей**). Регионы `counter`, `padding`.
  - `sync/starve` — две горутины берут замок в цикле, одна держит его долго: печатает распределение захватов и максимальное ожидание (показ режима голодания). Регион `starve`.
  - `patterns/leak` (`labdemo`) — функция из release notes 1.26 (`processWorkItems` с небуферизованным каналом и ранним возвратом по ошибке); после вызова — `runtime.GC()` и `pprof.Lookup("goroutineleak").WriteTo(os.Stdout, 1)`; печатает число утёкших. Регион `leak`, регион `leak-fix` (буфер = числу задач или `errgroup`).
  - `patterns/limit` — 200 задач по 20 мс тремя способами: горутина на задачу, семафор на канале на 8, `errgroup.SetLimit(8)` — если `golang.org/x/sync` уже есть в `labs/go.mod`, иначе только первые два и в README строка «errgroup — в мессенджере, урок 1.5.6» (**без новых зависимостей в labs**); печатает время и пик горутин (`runtime.NumGoroutine` в отдельной горутине-наблюдателе с остановкой). Регион `limit`.

- [ ] **Step 2: `scenario.sh`** (`#!/bin/bash`, `set -euo pipefail`, на каждую проверку — строка `ОК: …` или `ПРОВАЛ: …` и `exit 1`): запускает каждую программу (`go run`, для `labdemo` — `go run -tags labdemo`, для `iface` — ожидается ненулевой код), проверяет ключевые строки (например, `preempt` с `asyncpreemptoff=1` печатает время ≥ 1 с, а без — < 100 мс; `leak` печатает ненулевое число; `race/iface -race` содержит `WARNING: DATA RACE`), бенчмарки — `go test -run=^$ -bench=. -benchtime=100ms ./1-5/sync/` на работоспособность. Последняя строка — «Лаба 1.5: все проверки прошли.». Время — ≤ 2 мин.

- [ ] **Step 3: `check-labs.ts` и CI.** Запуск `bash 1-5/scenario.sh` из `labs/` (таймаут 180 с), проверка последней строки; в `ci.yml` джоба `labs` — тот же скрипт. `go vet ./...` и `go test -race ./...` всего модуля остаются чистыми (файлы `labdemo` в них не попадают).

- [ ] **Step 4: README** — к какому уроку какая программа, как запускать, ожидаемый вывод (сокращённо), что значит каждое число.

- [ ] **Step 5: Проверить** — `npm run check:labs`; Expected: PASS, последняя строка сценария на месте.

- [ ] **Step 6: Снять полные числа для уроков** — каждую программу запустить по 3 раза, бенчмарки — `-benchtime=1s -count=5 -cpu=1,4,8`, сохранить вывод в scratchpad (не в репозиторий) для задач 6–10 вместе с описанием машины (`lscpu | head`, `uname -r`, `go version`).

- [ ] **Step 7: Commit** — `feat(labs): лаба 1.5 — горутины, каналы, гонки, sync, паттерны`.

---

### Task 6: Урок 1.5.1 «Горутины и планировщик» (тег `m01-5-l01`)

**Files:** Create: `course/modules/01-5-go-concurrency/01-goroutines-scheduler.md`; Modify: `index.md` (ссылка).

**Interfaces:** Consumes: сцена `GmpScheduler` (задача 4); регионы `cost`, `preempt`, `syscall` (задача 5); вывод шага 6 задачи 5; ADR-0011.

- [ ] **Step 1: Написать урок (контракт).** Содержание — строка 1.5.1 таблицы спеки §3, все пункты. Обязательно:
  - числа «цена потока против горутины» — из вывода `sched/cost` с описанием машины;
  - `schedtrace`: реальный вывод `GODEBUG=schedtrace=1000` на `sched/cost` с разбором полей строки (`gomaxprocs`, `idleprocs`, `threads`, `spinningthreads`, `runqueue`, `[…]` — локальные очереди);
  - откуда 256 и 61 — ссылки на строки `runtime/proc.go` 1.27.2 (имена функций `runqput`, `findRunnable`, `runqsteal`);
  - `<Deep>`: «Почему в Go нет пула потоков в стиле Java и откуда взялся P» (планировщик Вьюкова 2012, M:N, роль P);
  - раздел «Что изменилось в репозитории»: переход на 1.27.2 (ссылка на ADR-0011, что из 1.27 понадобится модулю) и `labs/1-5`;
  - типичные ошибки (5–7): «горутины бесплатны», `runtime.Gosched()` как синхронизация, `GOMAXPROCS` больше ядер «для скорости», `LockOSThread` без нужды, ожидание порядка запуска горутин.
- [ ] **Step 2: Проверить** — `npm run build && npm run check:refs && npm run check:go`; Expected: PASS.
- [ ] **Step 3: Commit и тег** — `docs(course): урок 1.5.1 — горутины и планировщик GMP`; `git tag m01-5-l01`.

### Task 7: Урок 1.5.2 «Каналы» (тег `m01-5-l02`)

**Files:** Create: `02-channels.md`. **Consumes:** `ChannelLabSandbox` (задача 3); регионы `ops`, `timers`.

- [ ] **Step 1: Урок (контракт)** — строка 1.5.2 спеки. Таблица операций — **вывод программы `chan/ops`**, а не набранная руками. Внутренности `hchan` — по `runtime/chan.go` 1.27.2 (поля `qcount`, `dataqsiz`, `buf`, `sendx`, `recvx`, `recvq`, `sendq`, `lock`; функции `chansend`, `chanrecv`, `send` с прямым копированием в стек получателя). `select` — по `runtime/select.go` (случайный порядок опроса `pollorder`, порядок блокировок `lockorder`). `<Deep>`: «CSP Хоара и почему Go выбрал каналы, но оставил мьютексы». Типичные ошибки: закрытие со стороны получателя, закрытие из нескольких писателей, `time.After` в горячем цикле (с оговоркой версий), буфер «чтобы не блокировалось», `len(ch)` для принятия решений.
- [ ] **Step 2: Проверить** (как в задаче 6). **Step 3: Commit и тег** `m01-5-l02`.

### Task 8: Урок 1.5.3 «Модель памяти и гонки» (тег `m01-5-l03`)

**Files:** Create: `03-memory-model.md`. **Consumes:** `HappensBefore`; регионы `flagloop`, `iface`; вывод `-race`.

- [ ] **Step 1: Урок (контракт)** — строка 1.5.3 спеки. Определения — по go.dev/ref/mem (цитаты в переводе с пометкой); вывод `go run -race` на `race/iface` — реальный, с разбором блоков отчёта (`Write at … by goroutine N`, `Previous read …`, `Goroutine N created at`). Пример «безобидной гонки» из практики — с источником (например, разбор гонок на интерфейсах Дмитрия Вьюкова) или помечен как иллюстрация. `<Deep>`: «Почему Go не дал гарантий как Java для гонок и что такое DRF-SC». Ссылка из урока 1.4 на этот урок (правка 1.4 — в задаче 13).
- [ ] **Step 2–3:** проверка, commit, тег `m01-5-l03`.

### Task 9: Урок 1.5.4 «`sync` и атомики» (тег `m01-5-l04`)

**Files:** Create: `04-sync.md`. **Consumes:** `MutexModes`; регионы `counter`, `padding`, `starve`; числа бенчмарков `-cpu=1,4,8`.

- [ ] **Step 1: Урок (контракт)** — строка 1.5.4 спеки. Устройство `Mutex` — по `internal/sync/mutex.go` 1.27.2 (константы `mutexLocked`, `mutexWoken`, `mutexStarving`, `starvationThresholdNs = 1e6`). Таблица бенчмарков из лабы с выводом «при каком `-cpu` `RWMutex` проигрывает/выигрывает» — по реальным числам, без заранее заданного вывода. Разбор мьютексов мессенджера (`chat/adapters/memory`, `idempotency/memory`) — регионы существующих файлов. `<Deep>`: «Lock-free — не значит быстрее». Типичные ошибки: копирование структуры с мьютексом (`go vet copylocks`), `RLock` → `Lock` апгрейд, `sync.Pool` для соединений, атомики для двух связанных полей, `WaitGroup.Add` внутри горутины.
- [ ] **Step 2–3:** проверка, commit, тег `m01-5-l04`.

### Task 10: Урок 1.5.5 «Паттерны конкурентности» (тег `m01-5-l05`)

**Files:** Create: `05-patterns.md`. **Consumes:** `WorkerPoolSandbox`; регионы `leak`, `leak-fix`, `limit`; вывод `goroutineleak`.

- [ ] **Step 1: Урок (контракт)** — строка 1.5.5 спеки. `errgroup` — по документации `golang.org/x/sync/errgroup` v0.24.0 (`WithContext`, `Go`, `Wait`, `SetLimit`, `TryGo`; что `Wait` возвращает первую ошибку). Утечка — вывод `goroutineleak` из лабы и объяснение механизма через GC (цитата release notes 1.26 в переводе). `synctest` — пример теста с `synctest.Test`, `time.Sleep` внутри пузыря (виртуальное время), `synctest.Wait`; иллюстративный фрагмент с `<!-- go-check -->`. Сравнение пул/семафор/`errgroup.SetLimit` — таблица «когда что» и числа из `patterns/limit`. Обратное давление — три политики и что выбрал мессенджер (ссылка вперёд на 1.5.6). `<Deep>`: «Закон Литтла и почему очередь без предела — это отложенный отказ». Типичные ошибки: `wg.Add` в горутине, игнор ошибки из `errgroup.Wait`, захват переменной цикла (что изменилось в 1.22), неограниченный `go` на запрос, `time.Sleep` в тестах.
- [ ] **Step 2–3:** проверка, commit, тег `m01-5-l05`.

---

### Task 11: Мессенджер — `ctx` у хешера, проверка имени до хеша, `hashlimit`, `503 overloaded`

**Files:**
- Modify: `messenger/internal/auth/usecase/{ports.go,signup.go,login.go,signup_test.go,login_test.go}` (и фейки)
- Modify: `messenger/internal/auth/adapters/argon2id/{argon2id.go,argon2id_test.go}`
- Modify: `messenger/internal/auth/adapters/userbridge/{bridge.go,bridge_test.go}`
- Create: `messenger/internal/auth/adapters/hashlimit/{doc.go,hashlimit.go,hashlimit_test.go}`
- Modify: `messenger/internal/auth/adapters/httpapi/{errors.go,handler_test.go}`
- Modify: `messenger/internal/platform/problem/{problem.go,problem_test.go}`
- Modify: `messenger/api/openapi.yaml` и сгенерированный код (`go generate ./...`)
- Modify: `messenger/cmd/messenger/app.go` (сборка `hashlimit` с параметрами по умолчанию; конфиг — задача 12)

**Interfaces:**
- Produces:
```go
// auth/usecase/ports.go
var ErrOverloaded = errors.New("auth: слишком много одновременных вычислений хеша")
type PasswordHasher interface {
	// Hash и Verify могут вернуть ErrOverloaded (очередь полна) или ошибку ctx (истёк срок ожидания).
	Hash(ctx context.Context, p domain.Password) (domain.PasswordHash, error)
	Verify(ctx context.Context, plaintext string, hash domain.PasswordHash) (bool, error)
}
type ProfileValidator interface {
	// ValidateProfile — *domain.ValidationError с полем username или display_name, как RegisterProfile.
	ValidateProfile(ctx context.Context, username, displayName string) error
}
func NewSignUp(hasher PasswordHasher, profiles ProfileRegistrar, validator ProfileValidator, creds CredentialsSaver) (*SignUp, error)

// auth/adapters/hashlimit
type Limiter struct{ /* next, slots, run */ }
func New(next usecase.PasswordHasher, concurrency, queue int) (*Limiter, error) // concurrency > 0, queue >= 0
func (l *Limiter) Hash(ctx context.Context, p domain.Password) (domain.PasswordHash, error)
func (l *Limiter) Verify(ctx context.Context, plaintext string, hash domain.PasswordHash) (bool, error)

// platform/problem
const CodeOverloaded Code = "overloaded" // registry: {http.StatusServiceUnavailable, "Сервер перегружен"}
type Problem struct { Code Code; Detail string; InvalidParams []InvalidParam; RetryAfter time.Duration }
```
- `userbridge.Bridge` реализует `ProfileValidator`.

- [ ] **Step 1: Тесты `problem`** — `TestWriteRetryAfter`: `RetryAfter: 1500*time.Millisecond` → заголовок `Retry-After: 2`; `0` → заголовка нет; `CodeOverloaded` → 503 и `title` «Сервер перегружен»; существующий тест «код есть в `openapi.yaml`» падает, пока не обновлён контракт.
- [ ] **Step 2: Тесты `hashlimit`** (все через `synctest.Test`; фейковый `next` блокируется на канале, который тест отпускает):
  - `TestLimiterRunsAtMostN`: `New(fake, 2, 4)`, 6 одновременных `Hash` → внутри `next` одновременно не больше 2 (счётчик-максимум в фейке);
  - `TestLimiterRejectsOverQueue`: `New(fake, 2, 4)`, 7-й одновременный вызов → `errors.Is(err, usecase.ErrOverloaded)` сразу (без ожидания: `synctest.Wait()` до отпускания фейка, ошибка уже получена);
  - `TestLimiterZeroQueue`: `New(fake, 1, 0)`, второй одновременный → `ErrOverloaded`;
  - `TestLimiterCancelReleasesSlot`: `New(fake, 1, 1)`, первый занят, второй ждёт с `ctx`, который отменяется → второй получает `context.Canceled`; затем новый вызов становится в очередь (а не `ErrOverloaded`);
  - `TestLimiterReleasesOnPanic`: `next` паникует; вызов в горутине с `recover` → после него `New(…,1,0)`-ограничитель принимает следующий вызов;
  - `TestLimiterVerifyLimitedToo`: `Verify` делит те же места с `Hash`;
  - `TestNewRejectsBadArgs`: `nil`, `concurrency ≤ 0`, `queue < 0`.
- [ ] **Step 3: Реализация `hashlimit`** — рулинг 4: `slots chan struct{}` (cap `N+Q`): `select { case slots <- struct{}{}: default: return ErrOverloaded }`, `defer <-slots`; затем `select { case run <- struct{}{}: case <-ctx.Done(): return ctx.Err() }`, `defer <-run`; затем `next`. Комментарии «почему два канала».
- [ ] **Step 4: Порт с `ctx`.** Обновить порт, `argon2id` (`ctx.Err()` до вычисления; комментарий: вычисление Argon2 не прерывается — нет точки проверки внутри `argon2.IDKey`), use case'ы, фейки, тесты. `NewLogin`: `hasher.Hash(context.Background(), pw)` с комментарием (сборка до приёма запросов, семафор свободен).
- [ ] **Step 5: Проверка имени до хеша.** Тест `TestSignUpInvalidUsernameDoesNotHash`: фейковый валидатор возвращает `&domain.ValidationError{Field: "username"}` → `Execute` возвращает её, фейковый хешер **не вызывался**. `SignUp.Execute`: `NewPassword` → `ValidateProfile` → `Hash` → `RegisterProfile` → … (комментарий: дорогой хеш — после всех дешёвых проверок). `userbridge.ValidateProfile` — через `userdomain.NewUsername`/`NewDisplayName`, перевод ошибок как в `RegisterProfile`; тест в `bridge_test.go`.
- [ ] **Step 6: HTTP.** `httpapi/errors.go`: `errors.Is(err, usecase.ErrOverloaded)` → `problem.Problem{Code: CodeOverloaded, Detail: "повторите через секунду", RetryAfter: overloadedRetryAfter}` (константа 1 с в `httpapi`). Тест обработчика: хешер-фейк возвращает `ErrOverloaded` на `POST /v1/users` и `POST /v1/tokens` → 503, `code: overloaded`, `Retry-After: 1`, тело проходит `openapitest`.
- [ ] **Step 7: Контракт.** `openapi.yaml`: `overloaded` в enum `ProblemCode`; у ответа `ServiceUnavailable` — заголовок `Retry-After` (integer, секунды, необязательный, описание «есть у `overloaded`»). `go generate ./...`, `git diff` сгенерированного кода — ожидаемый.
- [ ] **Step 8: Сборка.** `app.go`: `hasher` → `hashlimit.New(argon, runtime.GOMAXPROCS(0), 4*runtime.GOMAXPROCS(0))` (значения из конфига — задача 12; пока константы по умолчанию с именами `defaultHashQueueFactor = 4`), передаётся в `NewSignUp` и `NewLogin`; `bridge` — третьим аргументом `NewSignUp`.
- [ ] **Step 9: Проверить** — `go test -race -count=1 ./... && go test -race -count=20 ./internal/auth/... && golangci-lint run`; Expected: PASS.
- [ ] **Step 10: Commit** — `feat(messenger): ограничение одновременных хешей Argon2id, 503 overloaded, проверка имени до хеша`.

---

### Task 12: Мессенджер — `lifecycle`, `periodic`, очистка идемпотентности, второй сигнал, конфиг

**Files:**
- Create: `messenger/internal/platform/lifecycle/{doc.go,lifecycle.go,lifecycle_test.go}`
- Create: `messenger/internal/platform/periodic/{doc.go,periodic.go,periodic_test.go}`
- Modify: `messenger/internal/platform/idempotency/{idempotency.go,memory/store.go,memory/store_test.go,storetest/storetest.go}`
- Create: `messenger/cmd/messenger/{signals.go,signals_test.go}`; Modify: `main.go`, `app.go`, `app_test.go`
- Modify: `messenger/internal/platform/httpserver/{server.go,server_test.go}`
- Modify: `messenger/go.mod` (`golang.org/x/sync v0.24.0` прямой), `messenger/.golangci.yml`, `messenger/compose.yaml`, `messenger/README.md` (если переменные описаны там)

**Interfaces:**
- Consumes: `hashlimit.New` (задача 11).
- Produces:
```go
// platform/lifecycle
type Component interface {
	Name() string
	// Run работает до отмены ctx и тогда возвращает nil; ошибка до отмены — сбой.
	Run(ctx context.Context) error
}
type Stages struct{ Front, Back []Component }
const BackStopTimeout = 2 * time.Second // срок выхода Back после их отмены; учитывает loadTimeouts
func Run(ctx context.Context, log *slog.Logger, s Stages) error

// platform/periodic
type Config struct {
	Name   string
	Every  time.Duration // > 0
	Jitter float64       // [0, 1)
	Fn     func(ctx context.Context) error
	Log    *slog.Logger
}
type Task struct{ /* cfg, rand */ }
func New(c Config) (*Task, error)          // проверка полей: пустое имя, Every <= 0, Jitter вне [0,1), nil Fn/Log
func (t *Task) Name() string
func (t *Task) Run(ctx context.Context) error // реализует lifecycle.Component

// platform/idempotency
type Sweeper interface {
	// Sweep удаляет записи с ExpiresAt <= now; возвращает число удалённых.
	Sweep(ctx context.Context, now time.Time) (removed int, err error)
}
// storetest
func RunSweeper(t *testing.T, newStore func(t *testing.T) interface{ idempotency.Store; idempotency.Sweeper })

// cmd/messenger/signals.go
func exitOnSecondSignal(sig <-chan os.Signal, first <-chan struct{}, exit func(int), log *slog.Logger)
```

- [ ] **Step 1: Тесты `lifecycle`** (все в `synctest.Test`, компоненты — фейки с управляемым поведением и журналом событий):
  - `TestRunStopsFrontBeforeBack`: отмена внешнего `ctx` → журнал: все `Front` вернулись раньше, чем любой `Back` увидел отмену; `Run` → `nil`;
  - `TestRunBackFailureStopsFrontWithDrain`: `Back` возвращает ошибку `boom` до отмены → `Front` получает отмену (и фейк-HTTP успевает «дренаж» — проверяется, что его `Run` завершился штатно), `Run` → ошибка, `errors.Is(err, boom)` и текст содержит имя компонента;
  - `TestRunFrontFailure`: падает `Front` → `Back` отменяются, ошибка с именем;
  - `TestRunBackStopTimeout`: `Back` игнорирует отмену → через `BackStopTimeout` `Run` возвращает ошибку, содержащую имя и `2s`; (проверка виртуального времени: `time.Since(start) == BackStopTimeout` после возврата, в пределах пузыря);
  - `TestRunNilAfterCleanStop`: все компоненты выходят с `nil` после отмены → `nil`, даже если `ctx.Err() != nil` (ошибка отмены не считается сбоем).
- [ ] **Step 2: Реализация `lifecycle.Run`.** Два `errgroup`: `front` с `ctx`-ом `frontCtx`, `back` с `backCtx` (оба производные от общего `failCtx`, который отменяется при сбое любого компонента или внешнем `ctx`); `backCtx` отменяется после `front.Wait()`; ожидание `back.Wait()` ограничено `BackStopTimeout` от момента отмены `backCtx`. Ошибки оборачиваются `fmt.Errorf("компонент %s: %w", name, err)`; первая — та, что вернулась раньше. Комментарий «почему Front раньше Back».
- [ ] **Step 3: Тесты и реализация `periodic`** (`synctest`): `TestTaskFirstRunAfterEvery` (за `Every − ε` не вызвана, после — 1 раз); `TestTaskJitterWithinBounds` (при `Jitter 0.1` интервалы в `[0.9·Every, 1.1·Every]` на 50 итерациях, `math/rand/v2` с сидом из теста через неэкспортируемое поле или опцию); `TestTaskSlowIterationDoesNotOverlap` (`Fn` длится `2·Every` → одновременно не больше одного вызова); `TestTaskErrorLoggedAndContinues`; `TestTaskStopsOnCancel` (после отмены `Run` → `nil`, пузырь завершается — горутин не осталось). Реализация — `time.Timer` с `Reset` на каждый интервал с джиттером (а не `Ticker`: джиттер на каждый тик) — комментарий «почему Timer».
- [ ] **Step 4: `Sweeper`.** `storetest.RunSweeper`: удаляет только `ExpiresAt <= now`; не трогает живые и «в работе» до TTL; повторный `Sweep` → 0; отменённый `ctx` → ошибка. `memory.Store.Sweep`: порции по `sweepBatch = 1000` ключей под замком, между порциями замок отпускается (комментарий о компромиссе); `store_test.go` вызывает `storetest.RunSweeper`. Комментарий пакета `memory` и ADR-0005 — «просроченные записи удаляет фоновая очистка (урок 1.5.6); до неё запись перезаписывал `Reserve` после TTL».
- [ ] **Step 5: Второй сигнал.** `TestExitOnSecondSignal`: `first` закрыт, в `sig` пришёл сигнал → `exit(1)` вызван один раз, в журнале «второй сигнал»; `TestNoExitBeforeFirst`: сигнал до закрытия `first` → `exit` не вызван (сигнал уже обработал `NotifyContext`). В `main.go`: `signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)` рядом с `NotifyContext`; горутина `exitOnSecondSignal` с владельцем `run` (останавливается по `defer`-закрытию канала остановки). Комментарий про PID 1 и `os.Exit` без `defer`.
- [ ] **Step 6: Сборка и конфиг.**
  - `config`: `IdempotencySweepEvery`, `HashConcurrency`, `HashQueue`; переменные — константы `envIdempotencySweepEvery = "MESSENGER_IDEMPOTENCY_SWEEP_EVERY"` и т. д.; **все** имена `MESSENGER_*` в `app.go`/`cli.go`/`healthcheck.go` — константами (бэклог 1c).
  - Значения по умолчанию: `1m`, `runtime.GOMAXPROCS(0)`, `4 × HashConcurrency`. Проверки: `> 0`, `> 0`, `≥ 0`; ошибка называет переменную.
  - `loadTimeouts`: `DrainDelay + ShutdownTimeout + lifecycle.BackStopTimeout < terminationGrace`.
  - `newApp` возвращает также `back []lifecycle.Component` (очистка идемпотентности: `periodic.New(periodic.Config{Name: "idempotency-sweeper", Every: cfg.IdempotencySweepEvery, Jitter: 0.1, Fn: …})`, `Fn` вызывает `idemStore.Sweep(ctx, clk.Now())` и пишет `removed` на Debug).
  - `main.run`: компонент `httpComponent` (адаптер над `httpserver.Run` с `Stop` из 1c) — `Front`; `lifecycle.Run(ctx, log, Stages{Front: …, Back: a.back})`; после — «сервер остановлен» как раньше и новая строка «фоновые компоненты остановлены» на Info в `lifecycle`.
  - Тесты: `TestLoadConcurrencyConfig` (табличный: умолчания, `HASH_QUEUE=0`, `HASH_CONCURRENCY=0` → ошибка, `abc` → ошибка с именем, `SWEEP_EVERY=-1s` → ошибка); `TestLoadTimeoutsIncludesBackStop`; `app_test`: сборка возвращает компонент очистки, остановка через `lifecycle` проходит.
- [ ] **Step 7: `httpserver`** — `New` отвергает `requestTimeout <= 0` (сигнатура меняется на `(*http.Server, error)` — обновить вызовы и тест); комментарий `writeMargin` уточнить: «запас, чтобы ответ 503 timeout успел записаться до дедлайна записи соединения».
- [ ] **Step 8: Переход тестов остановки на `synctest`.** `TestRunServesDuringDrainDelay` и соседние в `httpserver`/`app_test` — переписать на `synctest` + `httptest.NewTestServer`, если `httpserver.Run` можно вызвать с `net.Listener` из фейковой сети (если нужен параметр `Listener` — добавить в `Stop` или новую функцию `Serve(ctx, srv, ln, stop)`; решение и причина — в сообщении коммита). Если не выходит без переделки API — оставить и записать в бэклог с причиной.
- [ ] **Step 9: `.golangci.yml`, `compose.yaml`.** depguard: `platform/lifecycle` и `platform/periodic` не импортируют `messenger/internal/{user,chat,auth}`; контексты не импортируют `platform/lifecycle`; `auth/adapters/hashlimit` — только `$gostd`, `auth/usecase`, `auth/domain`. `compose.yaml`: три новые переменные с умолчаниями и комментарием про память хешей (`N × 19 МиБ` против `GOMEMLIMIT`).
- [ ] **Step 10: Проверить** — `go test -race -count=1 ./... && go test -race -count=20 ./internal/platform/lifecycle/... ./internal/platform/periodic/... ./internal/auth/adapters/hashlimit/... ./cmd/... && golangci-lint run && (cd ../course && npm run check:messenger && npm run check:images)`; Expected: PASS; `git grep -n "time.Sleep" -- '*_test.go'` в новых файлах — пусто.
- [ ] **Step 11: Commit** — `feat(messenger): жизненный цикл компонентов на errgroup, фоновая очистка идемпотентности, второй сигнал`.

---

### Task 13: Урок 1.5.6, ADR-0012, правки уроков модуля 1 (тег `m01-5-l06`)

**Files:**
- Create: `course/modules/01-5-go-concurrency/06-background-work.md`, `messenger/docs/adr/0012-жизненный-цикл-и-фоновая-работа.md`
- Create: `messenger/scripts/overload.sh` (50 одновременных `POST /v1/users` через `curl` + `xargs -P`, сводка по кодам ответа)
- Modify: `course/modules/01-go-service/{04-usecases,07-http-adapter,12-*,13-passwords,17-service-in-container}.md`, `messenger/docs/adr/0005-идемпотентность.md`

**Interfaces:** Consumes: сцена `AppLifecycle`; регионы (поставить в коде задач 11–12): `lifecycle-run`, `periodic-run`, `sweep`, `limiter`, `signup-order`, `second-signal`, `stages-wiring`.

- [ ] **Step 1: Реальный вывод.**
  - `docker compose up` (из `messenger/`), в журнале — строки очистки (временно `MESSENGER_IDEMPOTENCY_SWEEP_EVERY=5s` и уровень Debug, если он настраивается; если нет — показать строку Info «фоновые компоненты остановлены» и описать очистку тестом).
  - `scripts/overload.sh` при `MESSENGER_HASH_CONCURRENCY=2`, `MESSENGER_HASH_QUEUE=4`: распределение кодов (201 / 503 `overloaded` с `Retry-After`), `docker stats --no-stream` (RSS).
  - То же при `MESSENGER_HASH_CONCURRENCY=1000`, `MESSENGER_HASH_QUEUE=0`: RSS и, если случится, OOM-kill (`docker inspect -f '{{.State.OOMKilled}}'`). Если OOM не случается на этой машине — вывод как есть и расчёт, при скольких одновременных он случится.
  - `docker compose stop`: порядок строк журнала, код выхода; `docker kill -s TERM` дважды подряд: код 1 и строка «второй сигнал».
- [ ] **Step 2: Урок (контракт)** — строка 1.5.6 спеки, все пункты. Обязательно: расчёт памяти хешей (`N × 19 МиБ` против 230 МиБ) с `<N>`; таблица «три политики обратного давления» и почему для хешей — «отказ сразу»; почему паника в фоновой горутине роняет процесс (рантайм не даёт перехватить панику чужой горутины) и почему мы этого не скрываем; ограничения «у каждого экземпляра своя очистка» (вне рамок — модули 7, 10); `<Deep>`: «Crash-only software (Candea, Fox, 2003) и почему перезапуск — штатный путь»; раздел «Что изменилось в мессенджере» с деревом файлов; типичные ошибки (5–7): горутина в `init`/конструкторе без владельца, `ticker` без `Stop` (до 1.23 — утечка), `recover` в фоновой горутине «чтобы не падало», остановка фоновых раньше HTTP, бесконечная очередь к дорогой операции, ожидание в `select` без `ctx`.
- [ ] **Step 3: ADR-0012** «Жизненный цикл и фоновая работа» (контракт): две стадии и порядок; сбой компонента → остановка процесса с дренажом; без `recover` в фоновых; второй сигнал → `os.Exit(1)`; ограничение хешей и `503 overloaded`; отвергнутые варианты (`recover` + перезапуск компонента внутри процесса — супервизор в стиле Erlang; общий пул воркеров для всех дорогих операций; `x/sync/semaphore`).
- [ ] **Step 4: Правки модуля 1.** Уроки 1.4, 1.7: «модуль 1.5» → ссылки на 1.5.3/1.5.4 и 1.5.6; 1.12 и ADR-0005: формулировка об удалении просроченных ключей; 1.13: пометка «с 1.5.6 у `PasswordHasher` есть `ctx`, а регистрация проверяет имя до хеша» и правка абзаца о DoS (теперь и регистрация); 1.17: фраза о втором сигнале со ссылкой на 1.5.6; урок 1.20: «Дальше — модуль 1.5» → ссылка на страницу модуля.
- [ ] **Step 5: Проверить** — `npm run build && npm run check:refs && npm run check:go && npm test`; Expected: PASS.
- [ ] **Step 6: Commit и тег** — `docs(course): урок 1.5.6 — жизненный цикл и фоновая работа; ADR-0012`; `git tag m01-5-l06`.

---

### Task 14: Итоговая проверка, README, бэклог

**Files:** Modify: `README.md`, `course/modules/01-5-go-concurrency/index.md`, `docs/superpowers/backlog-after-module-0.md`

- [ ] **Step 1: Все проверки.** `npm test && npm run typecheck && npm run check:go && npm run check:labs && npm run check:messenger && npm run check:refs && npm run check:workflows && npm run check:images && npm run build && npm run check:base`; Expected: PASS.
- [ ] **Step 2: Теги.** Для каждого `m01-5-l01…l06`: `git worktree add` во временный каталог scratchpad, `go build ./... && go test -race ./...` в `messenger/` и `labs/`, `golangci-lint run` в `messenger/`; Expected: PASS на всех шести.
- [ ] **Step 3: Playwright** на 375 и 1280 px, тёмная и светлая тема, уроки 1.5.1–1.5.6 (предпросмотр `npx vitepress preview . --port 4196`): нет горизонтальной прокрутки (`document.documentElement.scrollWidth <= innerWidth`); каждая песочница реагирует (одно действие меняет вывод); полноэкранный режим открывается и закрывается Esc; mermaid — без обрезки подписей (проверка из исправления `MermaidDiagram.vue`: высота `div` в `foreignObject` ≤ высоты `foreignObject`).
- [ ] **Step 4: Страница модуля, README, бэклог.** `index.md` — ссылки на все уроки; README — модуль 1.5a в списке, новые переменные мессенджера, лаба 1.5; бэклог — раздел «Выполнено в подпроекте „Модуль 1.5a“» (закрытые пункты из спеки §8) и «Остатки модуля 1.5a».
- [ ] **Step 5: Commit** — `docs: итог части 1.5a — страница модуля, README, бэклог`. Затем — ревью всей ветки (навык subagent-driven-development) и PR в `main` только с согласия пользователя.
