# Модуль 1, часть 1b «Проектирование REST API и аутентификация» — план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Уроки 1.8–1.14 и код мессенджера в том виде, в каком его строят большие команды:
- API под `/v1` по контракту OpenAPI (oapi-codegen + проверка ответов kin-openapi);
- ошибки по RFC 9457;
- курсорная пагинация;
- идемпотентность (естественный ключ сообщений + `Idempotency-Key`);
- контекст `auth` с Argon2id;
- JWT на Ed25519 вместо заглушки `X-User-ID`;
- три песочницы и четыре плеера.

**Architecture:**
- Раскладка и правила слоёв из 1a не меняются, добавляются контекст `auth` (domain ← usecase ← adapters), маленькое общее ядро `internal/kernel/ident` и технические пакеты `platform/*` (principal, middleware, httpauth, problem, cursor, idempotency, openapitest).
- `api/openapi.yaml` — источник правды; каждый HTTP-адаптер генерирует свои DTO и `ServerInterface` (`include-tags`) в подпакет `gen/`, хендлеры реализуют интерфейс, мапперы и политика ошибок — наши.
- Код растёт урок за уроком: задачи 4–10 — по одному уроку, тег `m01-lNN` на завершающем коммите. Визуализации — задачи 2–3, до уроков.

**Tech Stack:**
- курс: VitePress 1.6.4, Vue 3.5.43, TypeScript 5.9.3, Vitest 3.2.7, Node 26;
- мессенджер: Go 1.26 (у автора 1.26.5), `golangci-lint` 2.12.2, зависимости (версии не моложе двух недель на 2026-10-14, проверены спайком):

| Модуль | Версия | Роль |
|---|---|---|
| `github.com/oapi-codegen/oapi-codegen/v2` | v2.8.0 | генератор, через директиву `tool` в `go.mod` |
| `github.com/oapi-codegen/runtime` | v1.7.0 | рантайм сгенерированного кода |
| `github.com/getkin/kin-openapi` | v0.149.0 | проверка ответов по контракту (только тесты; совпадает с версией, которую тянет oapi-codegen) |
| `github.com/golang-jwt/jwt/v5` | v5.3.1 | JWT |
| `golang.org/x/crypto` | v0.57.0 | Argon2id |
| `github.com/google/uuid` | v1.6.0 | уже есть |

**Spec:**
- `docs/superpowers/specs/2026-10-14-module-1b-rest-api-design.md` — главный.
- `docs/superpowers/specs/2026-10-13-module-1a-service-architecture-design.md` — правила 1a, которые продолжают действовать.
- `docs/superpowers/specs/2026-10-09-course-platform-design.md` — §1, §2.2–2.5.

**Отступления от стандарта плана (как в 1a):**
- **Домен, use case'ы, хранилища, контрактные тесты хранилищ, пакеты `platform/*`, адаптеры `argon2id`/`jwt`/`userbridge`, симуляции песочниц и `api/openapi.yaml` (состояние урока 1.9)** даны полным кодом.
- **HTTP-хендлеры, `main.go`, изменения контракта в уроках 1.10–1.14, рендереры, сцены, плееры и тексты уроков** даны **контрактом**: поведение, обязательные свойства, имена. Ревьюер проверяет контракт.

**Проверено спайком 2026-10-14 (не перепроверять, но не противоречить):**
- `go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0` работает; kin-openapi остаётся v0.149.0.
- `std-http-server` генерирует `ServerInterface` с параметрами пути типа `openapi_types.UUID` (для `format: uuid`), структуру `…Params` с указателями для необязательных query-параметров, `HandlerWithOptions(si, StdHTTPServerOptions{BaseRouter, Middlewares, ErrorHandlerFunc})`, регистрирует маршруты как `"GET /v1/…"` на переданном `ServeMux`. Ошибки разбора параметров — `*InvalidParamFormatError{ParamName, Err}`, `*RequiredParamError{ParamName}` и др. — **свои типы в каждом сгенерированном пакете**. Границы `minimum`/`maximum` сгенерированный код **не** проверяет. `nullable: true` строка → `*string`.
- kin-openapi `openapi3filter.ValidateResponse` с роутером `routers/gorillamux` ловит, например, отсутствие обязательного `next_cursor`.
- `jwt.ParseWithClaims` с `WithValidMethods([]string{"EdDSA"})`, `WithIssuer`, `WithAudience`, `WithLeeway`, `WithExpirationRequired`, `WithTimeFunc` проверяет токен, подписанный `ed25519.PrivateKey`; `aud` сериализуется массивом.
- `http.ServeMux.Handler(r)` для неизвестного пути и для неверного метода возвращает пустой `pattern` (405 — с `Allow: GET, HEAD, POST`); при вызове через `mux.ServeHTTP` в хендлере заполнен `r.Pattern` (например, `"POST /v1/chats"`); при вызове хендлера, полученного из `mux.Handler`, — **не** заполнен.
- VitePress находит регионы в YAML по маркерам `# region имя` / `# endregion имя` (и `#region имя`); маркер `# #region` **не** работает.

## Global Constraints

- Команды npm — из `/home/idel/DevGo/system-desine/course`; Go-команды — из `/home/idel/DevGo/system-desine/messenger`. `golangci-lint` лежит в `~/go/bin` — добавляйте в `PATH` (`PATH=$HOME/go/bin:$PATH`). Версии — точные.
- Работа идёт в ветке `feat/module-1b` (создаёт задача 1). Слияние в `main` — только с явного согласия пользователя.
- Язык контента — русский; английский оригинал термина — в скобках при первом упоминании. Каждый термин при первом появлении в уроке — `<T id="…">` (id в двойных кавычках; неизвестный id — ошибка сборки).
- Тексты уроков — по основной спецификации:
  - §1 «Голос автора»: доктор наук, объясняющий просто; упрощение без искажения, упрощения помечены `<Callout type="simplified">`;
  - §2.2 — все 10 правил, включая самодостаточность;
  - §2.3 — у каждой визуализации полноэкранный режим;
  - §2.4 — `<Part>` у каждого `##`; `<N>` для 3–8 ключевых чисел (только результат формулы или измерения, не константы и не операнды); `<Deep>` 150–500 слов (прозой, без кода).
- Чистые TS-модули не импортируют `vue` и не трогают DOM. В компонентах DOM — только в `onMounted`, обработчиках и `watch` без `immediate`; слушатели снимаются в `onBeforeUnmount`.
- Цвета только через CSS-переменные (`--viz-*`, `--vp-c-*`, `--course-*`, `--part-*`), обе темы, контраст текста ≥ 4,5:1, `prefers-reduced-motion`, 375 px без горизонтальной прокрутки, каждая сцена в `VizFrame`.
- Go-код мессенджера — стандарт пользователя (`~/.claude/CLAUDE.md`) и §2.5:
  - зависимости только внутрь: `domain` ← `usecase` ← `adapters` ← `cmd`; порт объявляется в пакете потребителя; узкие интерфейсы (1–3 метода);
  - доменные структуры без тегов `json:`/`db:`; DTO — сгенерированные (`gen/`) или в адаптере; мапперы — в адаптере;
  - один сценарий — один use case с методом `Execute`; конструкторы `NewX(deps…) (*X, error)` принимают интерфейсы и проверяют `nil`;
  - `context.Context` — первый аргумент всего, что ходит вовне; не хранится в структурах;
  - ошибки: доменные — `var ErrX = errors.New(…)` и типизированные с `Is`; обёртка `fmt.Errorf("…: %w", err)` там, где добавляется контекст; на границах — `errors.Is/As`; паника только на старте; сбой генератора id — обычная ошибка (500), не `ValidationError`;
  - сообщения ошибок — по-русски, со строчной буквы, без точки в конце;
  - у каждой горутины владелец и способ остановки; мьютекс рядом с полями, которые защищает;
  - `gofmt`, `go vet`, `golangci-lint run`, `go test -race` — чистые; сгенерированный код актуален (`check:messenger`);
  - комментарии объясняют «почему»; все лимиты — именованные константы (в `domain`, если это правило предметной области, иначе — в пакете, который им владеет).
- Код мессенджера в уроках — только `<<< @/../messenger/<путь>#<регион>{go}` (для YAML — `{yaml}`), регионы в Go `// #region имя` … `// #endregion имя`, в YAML `# region имя` … `# endregion имя`. Иллюстративные Go-фрагменты (антипримеры) — с `<!-- go-check -->` и проходят `npm run check:go`.
- Тег `m01-lNN` ставится исполнителем задачи урока на её последний коммит; в раундах исправлений — `git tag -f m01-lNN` на новый последний коммит задачи. На каждом теге `go build ./...` и `go test -race ./...` проходят.
- Вывод команд в уроках — реальный, полученный прогоном исполнителя.
- Коммиты на русском, заканчиваются строкой `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Факты из RFC, NIST, OWASP и документации библиотек — сверять с первоисточником при написании урока; спорное — помечать в тексте со ссылкой.

## Review Focus

1. **Повтор отправки сообщения после потери ответа, в том числе несколько одновременных повторов с одним `client_message_id`.** Ожидается: в истории ровно одно сообщение; все повторы получают его же (200), первый — 201; тот же `client_message_id` с другим текстом — 409. Покрывают `testDuplicateClientMessageID` и `testConcurrentDuplicateClientMessageID` в `storetest` (задача 8) и `TestSendMessageRetryReturnsExisting` (задача 8).
2. **Одновременные запросы с одним `Idempotency-Key`; паника или 5xx в обработчике.** Ожидается: создан ровно один чат, остальные получают 409 `idempotency_in_progress` или повтор ответа; после паники и после 5xx ключ свободен. Покрывают `TestIdempotencyConcurrent`, `TestIdempotencyReleasesAfterPanic`, `TestIdempotencyReleasesAfter5xx` (задача 8) и `storetest`-гонка `Reserve` (задача 8).
3. **Подделанный, чужой или просроченный токен.** Ожидается: `alg: none`, `HS256` с открытым ключом как секретом, неизвестный `kid`, изменённый байт payload, обрезанная подпись, `exp` в прошлом сверх допуска, `iat`/`nbf` в будущем сверх допуска, чужой `aud` или `iss` — всё отклонено, ответ 401 с `WWW-Authenticate: Bearer … error="invalid_token"`. Покрывает `TestVerifierRejects` (задача 10).
4. **Листание истории, пока в чат пишут.** Ожидается: никаких дублей и пропусков среди сообщений, существовавших до начала листания. Покрывает `TestListMessagesStableUnderInserts` (задача 7) и песочница `pagination` (задача 2).
5. **Вход: неизвестное имя, неверный пароль, профиль без пароля (ADR-0004).** Ожидается: везде одна ошибка `ErrInvalidCredentials` → 401 `invalid_credentials`, фиктивный хеш проверяется (фейк считает вызовы); не 500. Покрывает `TestLoginDoesNotRevealUsers` (задача 9).

---

## Структура файлов

```
messenger/
├── go.mod                                   # + tool oapi-codegen, runtime, jwt, x/crypto, kin-openapi
├── .golangci.yml                            # задача 1: правила auth, kernel, gen
├── api/openapi.yaml                         # задача 5, растёт в 6–10
├── cmd/
│   ├── messenger/{main.go, app.go, app_test.go}   # 4 (main), 10 (app.go + сквозной тест)
│   └── keygen/main.go                       # 10
├── docs/adr/ 0003-jwt-вместо-x-user-id.md (10), 0004-регистрация-через-два-контекста.md (9),
│            0005-идемпотентность.md (8), 0006-contract-first.md (5)
└── internal/
    ├── kernel/ident/{ident.go, ident_test.go}             # 9 (правило трёх)
    ├── auth/                                              # 9, 10
    │   ├── domain/   doc.go, password.go, credentials.go, errors.go (+ _test)
    │   ├── usecase/  doc.go, ports.go, signup.go, login.go (+ _test), storetest/storetest.go
    │   └── adapters/ argon2id/, jwt/ (10), memory/, userbridge/, httpapi/ (+ gen/)
    ├── user/  usecase/find_by_username.go (9); adapters/memory (9); adapters/httpapi (+ gen/) (4, 5, 9)
    ├── chat/  domain (7, 8); usecase (7, 8); adapters/memory (7, 8); adapters/httpapi (+ gen/) (4–8)
    └── platform/
        ├── principal/   (4)          ├── middleware/ (4)       ├── httpauth/ (4, 10)
        ├── problem/     (6; httperror удаляется)               ├── cursor/  (7)
        ├── idempotency/ (+ memory/) (8)                         ├── openapitest/ (5)
        └── httpjson/ (6: WriteError удаляется), httpserver/, clock/, idgen/ — как в 1a
course/
├── scripts/check-messenger.ts               # 1: проверка актуальности генерации
├── glossary/terms.yaml                      # 1: термины 1b
├── viz/simulations/ pagination.ts, idempotency.ts, jwt.ts (2);
│                    request-pipeline.ts, contract-pipeline.ts, problem-mapping.ts, login-timing.ts (3) (+ tests)
├── viz/renderers/  PaginationView, IdempotencyView, JwtView (2); RequestPipelineView, ContractPipelineView,
│                   ProblemMappingView, LoginTimingView (3) .vue
├── viz/scenes/     PaginationSandbox, IdempotencySandbox, JwtSandbox (2); RequestPipeline, ContractPipeline,
│                   ProblemMapping, LoginTiming (3) .vue
└── modules/01-go-service/ 08-resources.md … 14-jwt.md, index.md (11)
```

Порядок задач и теги:

| Задача | Содержание | Тег |
|---|---|---|
| 1 | Ветка, `.golangci.yml`, проверка генерации в `check:messenger`, глоссарий | — |
| 2 | Песочницы `pagination`, `idempotency`, `jwt` | — |
| 3 | Плееры `request-pipeline`, `contract-pipeline`, `problem-mapping`, `login-timing` | — |
| 4 | Урок 1.8 + `/v1`, middleware, `principal`, `httpauth` (заглушка) | `m01-l08` |
| 5 | Урок 1.9 + `openapi.yaml`, генерация, `openapitest`, ADR-0006 | `m01-l09` |
| 6 | Урок 1.10 + `platform/problem` (RFC 9457) | `m01-l10` |
| 7 | Урок 1.11 + курсоры, `GET /v1/chats` | `m01-l11` |
| 8 | Урок 1.12 + `client_message_id`, `Idempotency-Key`, ADR-0005 | `m01-l12` |
| 9 | Урок 1.13 + `kernel/ident`, контекст `auth`, Argon2id, ADR-0004 | `m01-l13` |
| 10 | Урок 1.14 + JWT, `httpauth.Bearer`, `keygen`, сквозной тест, ADR-0003 | `m01-l14` |
| 11 | Итоговая проверка, ссылки, README, бэклог | — |

---

### Task 1: Ветка, правила слоёв для 1b, проверка генерации, глоссарий

**Files:**
- Modify: `messenger/.golangci.yml`
- Modify: `course/scripts/check-messenger.ts`
- Modify: `course/glossary/terms.yaml`

**Interfaces:**
- Consumes: —
- Produces: правила `depguard` для `auth`, `kernel`, `gen`; `npm run check:messenger` проверяет, что `go generate ./...` ничего не меняет; термины 1b в глоссарии (их id используют задачи 2–10).

- [ ] **Step 1: Ветка**

```bash
cd /home/idel/DevGo/system-desine && git checkout main && git checkout -b feat/module-1b
```

- [ ] **Step 2: Правила `depguard`**

Изменения в `messenger/.golangci.yml` (остальное не трогать, комментарии-регионы `domain-rule`, `chat-context-rule` сохранить — на них ссылается урок 1.1):

1. Правило `domain`: в `allow` добавить `messenger/internal/kernel/ident`; запрет `pkg: messenger/internal` заменить четырьмя: `messenger/internal/user`, `messenger/internal/chat`, `messenger/internal/auth`, `messenger/internal/platform` — у всех тот же `desc`, что был. Иначе общий префикс `messenger/internal` запретил бы и `kernel/ident`.
2. Правило `usecase`: в `allow` добавить `messenger/internal/auth/domain`, `messenger/internal/auth/usecase`, `messenger/internal/kernel/ident`; в `deny` — `messenger/internal/auth/adapters` с тем же `desc`, что у соседей.
3. Правило `adapters`: в `deny` добавить `messenger/internal/auth/adapters` («адаптеры не знают друг о друге: связь — через use case»).
4. Новое правило `adapter-gen` — **исключение** для сгенерированного кода: адаптер может импортировать подпакет `gen` **своего** пакета. Механизм выбрать экспериментом (шаг 3): предпочтительно `allow` в правиле `adapters` с путями `messenger/internal/user/adapters/httpapi/gen`, `messenger/internal/chat/adapters/httpapi/gen`, `messenger/internal/auth/adapters/httpapi/gen` (в depguard v2 при совпадении с `allow` и `deny` побеждает более длинное совпадение — **проверить**); если не работает — заменить общие запреты `…/adapters` списком конкретных пакетов-адаптеров (`memory`, `httpapi`, `userchecker`, `userbridge`, `argon2id`, `jwt`).
5. `chat-context`: в `deny` добавить `messenger/internal/auth` («контекст chat не знает про auth: кто вызывает — из platform/principal»).
6. `user-context`: в `deny` добавить `messenger/internal/auth` (тот же смысл).
7. Новое `auth-context`: `files: ["**/internal/auth/**", "!**/internal/auth/adapters/userbridge/**"]`, `deny`: `messenger/internal/user` («auth не зависит от user: порты ProfileRegistrar и UserLookup, адаптер userbridge»), `messenger/internal/chat` («auth не знает про чаты»).
8. `platform`: в `deny` добавить `messenger/internal/auth`.
9. Новое `kernel`: `list-mode: strict`, `files: ["**/internal/kernel/**", "!$test"]`, `allow: [$gostd]` — общее ядро ни от чего проектного не зависит.

Сгенерированные файлы содержат строку `// Code generated … DO NOT EDIT.` — golangci-lint v2 по умолчанию (`linters.exclusions.generated: lax`) их не проверяет; явных исключений по пути не добавлять, пока шаг 3 не покажет, что это нужно.

- [ ] **Step 3: Проверить правила экспериментом**

Временно (не коммитить) создать:
- `internal/chat/adapters/httpapi/gen/x.go` (`package gen; const X = 1`) и импорт `_ "messenger/internal/chat/adapters/httpapi/gen"` в любом файле `chat/adapters/httpapi` → `golangci-lint run ./...` **без** замечаний;
- импорт `_ "messenger/internal/chat/adapters/httpapi"` в `chat/adapters/memory/store.go` → замечание `depguard` с текстом про адаптеры;
- `internal/kernel/ident/x.go` (`package ident; func IsUUID(string) bool { return false }`) и импорт в `chat/domain/ids.go` → без замечаний;
- импорт `_ "messenger/internal/platform/clock"` в `chat/domain/ids.go` → замечание правила `domain`.

Всё вернуть (`git checkout -- . && git clean -fd internal`), убедиться `golangci-lint run ./...` чист. Результаты четырёх проверок и выбранный механизм для `gen` — в отчёт.

- [ ] **Step 4: Проверка актуальности генерации в `check:messenger`**

В `course/scripts/check-messenger.ts` между `gofmt` и `go vet` добавить функцию и вызов. Код:

```ts
// Сгенерированный код (каталоги gen/) коммитится. Если контракт api/openapi.yaml поменяли,
// а go generate не запустили, код и контракт разойдутся молча — ловим это здесь.
function generatedFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = join(dir, e.name)
    if (e.isDirectory()) return generatedFiles(p)
    return p.split('/').includes('gen') && e.name.endsWith('.go') ? [p] : []
  })
}

function checkGenerated(): number {
  const snapshot = (): Map<string, string> => new Map(generatedFiles(join(MESSENGER, 'internal')).map((f) => [f, readFileSync(f, 'utf8')]))
  const before = snapshot()
  run('go', ['generate', './...'], 300_000)
  const after = snapshot()
  const changed = [...new Set([...before.keys(), ...after.keys()])].filter((f) => before.get(f) !== after.get(f))
  if (changed.length > 0) {
    console.error(`Сгенерированный код устарел — go generate ./... изменил файлы (закоммитьте их):\n${changed.map((f) => relative(MESSENGER, f)).join('\n')}`)
    process.exit(1)
  }
  return after.size
}
```

Вызов: `const generated = checkGenerated()` после проверки `gofmt`; итоговая строка — `Мессенджер: gofmt, go generate (файлов: ${generated}), go vet, golangci-lint, go test -race и регионы <<< — без ошибок (ссылок: ${refs}).` Пока каталогов `gen/` нет, `generated` равно 0, проверка проходит.

Run: `npm run check:messenger` → PASS, строка содержит `go generate (файлов: 0)`.

- [ ] **Step 5: Термины 1b в глоссарии**

В `course/glossary/terms.yaml` добавить термины (формат существующих: `term`, `en`, `short`, `long` 2–5 предложений с примером из мессенджера, `links` где есть первоисточник). Id и смысл:

| id | Термин | Суть |
|---|---|---|
| `rest` | REST | архитектурный стиль Филдинга: ресурсы, единый интерфейс методов, отсутствие состояния сессии на сервере |
| `resource` | Ресурс | то, на что указывает URI; коллекция (`/v1/chats`) и элемент (`/v1/chats/{id}`) |
| `safe-method` | Безопасный метод | по RFC 9110 не меняет состояние сервера: GET, HEAD, OPTIONS |
| `idempotent-method` | Идемпотентный метод | повтор даёт тот же эффект, что один вызов: безопасные + PUT, DELETE; POST — нет |
| `status-code` | Код статуса | трёхзначный код ответа HTTP; классы 2xx–5xx; часть контракта |
| `api-versioning` | Версионирование API | способ выпустить несовместимые изменения, не сломав старых клиентов; у нас `/v1` |
| `breaking-change` | Ломающее изменение | изменение контракта, после которого старый клиент перестаёт работать |
| `content-type` | Content-Type | заголовок с медиатипом тела: `application/json`, `application/problem+json` |
| `openapi` | OpenAPI | машиночитаемое описание HTTP API в YAML/JSON |
| `contract-first` | Contract-first | сначала пишется контракт, код подгоняется под него (против code-first) |
| `code-generation` | Генерация кода | код, полученный программой из описания; не правится руками |
| `json-schema` | JSON Schema | язык описания формы JSON-документа; на нём в OpenAPI описаны тела |
| `backward-compatibility` | Обратная совместимость | новый сервер понимает старых клиентов (Клеппман, гл. 4) |
| `problem-details` | Problem Details (RFC 9457) | стандартный JSON-формат ошибки HTTP API |
| `offset-pagination` | Offset-пагинация | «пропусти N, дай M»; дрейф страниц и стоимость больших OFFSET |
| `keyset-pagination` | Keyset-пагинация | «дай M после последнего увиденного ключа»; стабильна при вставках |
| `cursor` | Курсор пагинации | непрозрачная строка, в которой сервер закодировал, откуда продолжать |
| `page-drift` | Дрейф страниц | дубли и пропуски при листании offset-ом меняющегося списка |
| `idempotency` | Идемпотентность | повтор операции не меняет результат сверх первого раза |
| `idempotency-key` | Ключ идемпотентности | заголовок `Idempotency-Key`: клиент помечает попытку, сервер узнаёт повтор |
| `natural-key` | Естественный ключ | уникальный признак самой сущности; у нас `client_message_id` |
| `at-least-once` | Как минимум один раз | гарантия доставки, при которой возможны повторы, но не потери |
| `request-fingerprint` | Отпечаток запроса | хеш метода, пути и тела: отличает повтор от другого запроса с тем же ключом |
| `authentication` | Аутентификация | проверка, кто вы |
| `authorization` | Авторизация | проверка, что вам можно |
| `password-hashing` | Хеширование паролей | хранение не пароля, а результата медленной функции с солью |
| `salt` | Соль | случайные байты, уникальные для каждого хеша; ломают радужные таблицы |
| `argon2id` | Argon2id | функция хеширования паролей, требующая много памяти; первый выбор OWASP |
| `timing-attack` | Атака по времени | вывод секрета из времени ответа |
| `user-enumeration` | Перебор пользователей | выяснение, какие имена существуют, по различиям в ответах |
| `jwt` | JWT | JSON Web Token (RFC 7519): заголовок, claims и подпись в base64url |
| `claim` | Claim | утверждение в токене: `sub`, `exp`, `aud` и др. |
| `digital-signature` | Цифровая подпись | доказательство, что данные подписал владелец закрытого ключа и они не менялись |
| `public-key-cryptography` | Криптография с открытым ключом | пара ключей: закрытым подписывают, открытым проверяют |
| `bearer-token` | Bearer-токен | токен «предъявителя»: кто его держит, тот и проходит (RFC 6750) |
| `token-expiry` | Срок жизни токена | `exp`; короткий срок — замена мгновенному отзыву |
| `request-id` | Идентификатор запроса | метка, по которой ответ клиента находят в логах |
| `shared-kernel` | Общее ядро | маленький общий код двух контекстов, меняемый только по соглашению (Эванс) |
| `anti-corruption-layer` | Антикоррупционный слой | переводчик между моделями двух контекстов (Эванс); у нас `userchecker`, `userbridge` |
| `unit-of-work` | Unit of Work | объект, который собирает изменения и фиксирует их одной транзакцией (Фаулер) |

`middleware`, `dto`, `adr`, `contract-test`, `bounded-context` уже есть — дополнить `long`, если в них нет нужного для 1b (`middleware` — про цепочку и порядок).

Run: `npm test && npm run typecheck && npm run build` → PASS (сборка проверяет глоссарий).

- [ ] **Step 6: Коммит**

```bash
git add messenger/.golangci.yml course/scripts/check-messenger.ts course/glossary/terms.yaml
git commit -m "chore: правила слоёв для auth и gen, проверка генерации, термины 1b

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Песочницы `pagination`, `idempotency`, `jwt`

**Files:**
- Create: `course/viz/simulations/{pagination,idempotency,jwt}.ts` и `*.test.ts`
- Create: `course/viz/renderers/{PaginationView,IdempotencyView,JwtView}.vue`
- Create: `course/viz/scenes/{PaginationSandbox,IdempotencySandbox,JwtSandbox}.vue`
- Create (временный, не коммитить): программа генерации токенов для `jwt.ts` в scratchpad

**Interfaces:**
- Consumes: `Simulation`, `Step` из `viz/core/types`; `term` из `viz/core/caption`; `VizFrame`, `Player`; термины задачи 1.
- Produces: сцены `<PaginationSandbox />` (урок 1.11), `<IdempotencySandbox />` (1.12), `<JwtSandbox />` (1.14).

- [ ] **Step 1: Тесты `pagination`**

`course/viz/simulations/pagination.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { duplicates, paginationSimulation as sim, PAGE_SIZE, skipped, type PagingAction, type PagingMode } from './pagination'

const run = (mode: PagingMode, ...actions: PagingAction[]) => actions.reduce(sim.apply, sim.apply(sim.initial(), { type: 'mode', mode }))
const next: PagingAction = { type: 'next' }

describe('pagination', () => {
  it('без изменений оба режима дают одну и ту же ленту', () => {
    const a = run('offset', next, next, next)
    const b = run('cursor', next, next, next)
    expect(a.pages).toEqual(b.pages)
    expect(a.pages.flat()).toEqual([30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17, 16])
  })
  it('вставка между страницами: offset дублирует, курсор — нет', () => {
    const off = run('offset', next, { type: 'insert' }, next)
    expect(duplicates(off)).toEqual([26])
    const cur = run('cursor', next, { type: 'insert' }, next)
    expect(duplicates(cur)).toEqual([])
    expect(cur.pages[1]).toEqual([25, 24, 23, 22, 21])
  })
  it('удаление между страницами: offset пропускает, курсор — нет', () => {
    const off = run('offset', next, { type: 'delete' }, next)
    expect(skipped(off)).toEqual([25])
    const cur = run('cursor', next, { type: 'delete' }, next)
    expect(skipped(cur)).toEqual([])
  })
  it('новые сообщения не считаются пропущенными', () => {
    const s = run('cursor', next, { type: 'insert' }, { type: 'insert' }, next)
    expect(skipped(s)).toEqual([])
  })
  it('лента кончилась — пустая страница, курсор не двигается', () => {
    const actions = Array.from({ length: 30 / PAGE_SIZE + 1 }, () => next)
    const s = run('cursor', ...actions)
    expect(s.pages.at(-1)).toEqual([])
    expect(s.cursor).toBe(1)
  })
  it('смена режима и сброс начинают чтение заново', () => {
    const s = run('offset', next, { type: 'insert' }, { type: 'mode', mode: 'cursor' })
    expect(s.pages).toEqual([])
    expect(s.feed).toHaveLength(30)
    expect(sim.apply(s, { type: 'reset' }).mode).toBe('cursor')
  })
  it('у каждого шага есть подпись с термином', () => {
    const steps = sim.steps(run('offset', next, { type: 'insert' }, next))
    expect(steps).toHaveLength(1)
    expect(JSON.stringify(steps[0]?.caption)).toContain('page-drift')
  })
})
```

Run: `npx vitest run viz/simulations/pagination.test.ts` → FAIL (модуля нет).

- [ ] **Step 2: Реализация `pagination`**

`course/viz/simulations/pagination.ts`:

```ts
import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Simulation, Step } from '../core/types'

export const PAGE_SIZE = 5
export const INITIAL_COUNT = 30

export type PagingMode = 'offset' | 'cursor'

// Лента — номера сообщений (seq) от новых к старым, как история чата в мессенджере.
export interface PagingState {
  mode: PagingMode
  feed: readonly number[]
  initial: readonly number[] // лента в момент, когда начали листать
  nextSeq: number
  pages: readonly (readonly number[])[]
  offset: number // offset: сколько строк пропустить
  cursor: number | null // курсор: seq последнего показанного
  last: PagingAction['type'] | 'start'
}

export type PagingAction =
  | { type: 'next' }
  | { type: 'insert' }
  | { type: 'delete' }
  | { type: 'mode'; mode: PagingMode }
  | { type: 'reset' }

function start(mode: PagingMode): PagingState {
  const feed = Array.from({ length: INITIAL_COUNT }, (_, i) => INITIAL_COUNT - i)
  return { mode, feed, initial: feed, nextSeq: INITIAL_COUNT + 1, pages: [], offset: 0, cursor: null, last: 'start' }
}

// nextPage — то, что вернул бы сервер: offset считает строки, курсор — сравнивает ключи.
export function nextPage(s: PagingState): number[] {
  if (s.mode === 'offset') return s.feed.slice(s.offset, s.offset + PAGE_SIZE)
  const cursor = s.cursor
  const from = cursor === null ? 0 : s.feed.findIndex((seq) => seq < cursor)
  return from < 0 ? [] : s.feed.slice(from, from + PAGE_SIZE)
}

export function duplicates(s: PagingState): number[] {
  const seen = new Set<number>()
  const dup = new Set<number>()
  for (const seq of s.pages.flat()) (seen.has(seq) ? dup : seen).add(seq)
  return [...dup].sort((a, b) => b - a)
}

// Пропуск — сообщение, которое было в ленте с начала, всё ещё есть, лежит не ниже
// самого старого показанного, но не попало ни на одну страницу.
export function skipped(s: PagingState): number[] {
  const shown = new Set(s.pages.flat())
  if (shown.size === 0) return []
  const lowest = Math.min(...shown)
  const alive = new Set(s.feed)
  return s.initial.filter((seq) => seq >= lowest && !shown.has(seq) && alive.has(seq))
}

export const paginationSimulation: Simulation<PagingState, PagingAction> = {
  initial: () => start('offset'),
  apply: (s, a) => {
    switch (a.type) {
      case 'reset':
        return start(s.mode)
      case 'mode':
        return start(a.mode)
      case 'insert':
        return { ...s, feed: [s.nextSeq, ...s.feed], nextSeq: s.nextSeq + 1, last: 'insert' }
      case 'delete':
        return s.feed.length === 0 ? s : { ...s, feed: s.feed.slice(1), last: 'delete' }
      case 'next': {
        const page = nextPage(s)
        return {
          ...s,
          pages: [...s.pages, page],
          offset: s.offset + PAGE_SIZE,
          cursor: page.length > 0 ? (page.at(-1) as number) : s.cursor,
          last: 'next',
        }
      }
    }
  },
  steps: (s) => [{ state: s, caption: captionFor(s) } satisfies Step<PagingState>],
}

function captionFor(s: PagingState): Caption {
  const dup = duplicates(s)
  const skip = skipped(s)
  const how: Caption =
    s.mode === 'offset'
      ? [term('Offset-пагинация', 'offset-pagination'), { text: ': «пропусти N строк, дай 5». N считается от верха ленты, а верх двигается.' }]
      : [term('Курсор', 'cursor'), { text: ': «дай 5 старше seq последнего показанного». Ключ не зависит от того, что случилось выше.' }]
  if (s.last === 'start') return [...how, { text: ' Листайте, а между страницами добавляйте и удаляйте сообщения.' }]
  const verdict =
    dup.length + skip.length === 0
      ? 'Дублей и пропусков нет.'
      : `Дубли: ${dup.join(', ') || 'нет'}. Пропуски: ${skip.join(', ') || 'нет'}.`
  return [...how, { text: ` ${verdict} Это и есть ` }, term('дрейф страниц', 'page-drift'), { text: s.mode === 'offset' ? ' — болезнь offset.' : ' — у курсора его нет.' }]
}
```

Run: `npx vitest run viz/simulations/pagination.test.ts` → PASS.

- [ ] **Step 3: Тесты и реализация `idempotency`**

`course/viz/simulations/idempotency.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { idempotencySimulation as sim, type IdemAction, type IdemMode } from './idempotency'

const run = (mode: IdemMode, ...actions: IdemAction[]) => actions.reduce(sim.apply, sim.apply(sim.initial(), { type: 'mode', mode }))
const sendA: IdemAction = { type: 'send', body: 'A' }

describe('idempotency', () => {
  it('без ключа: ответ потерян, повтор — второй чат', () => {
    const s = run('no-key', { type: 'lose' }, sendA, sendA)
    expect(s.chats).toBe(2)
    expect(s.attempts.map((a) => a.delivered)).toEqual([false, true])
  })
  it('с ключом: повтор получает тот же ответ, чат один', () => {
    const s = run('key', { type: 'lose' }, sendA, sendA)
    expect(s.chats).toBe(1)
    expect(s.attempts.map((a) => a.outcome)).toEqual(['created', 'replayed'])
    expect(s.attempts[1]?.status).toBe(201)
  })
  it('с ключом: параллельный повтор — 409, чат один', () => {
    const s = run('key', { type: 'parallel' })
    expect(s.chats).toBe(1)
    expect(s.attempts.map((a) => a.status).sort()).toEqual([201, 409])
  })
  it('без ключа: параллельные запросы — два чата', () => {
    expect(run('no-key', { type: 'parallel' }).chats).toBe(2)
  })
  it('тот же ключ, другое тело — 422', () => {
    const s = run('key', sendA, { type: 'send', body: 'B' })
    expect(s.attempts.at(-1)?.status).toBe(422)
    expect(s.chats).toBe(1)
  })
  it('после 5xx ключ свободен, повтор создаёт чат', () => {
    const s = run('key', { type: 'fail' }, sendA, sendA)
    expect(s.attempts.map((a) => a.status)).toEqual([500, 201])
    expect(s.chats).toBe(1)
    expect(s.record?.done).toBe(true)
  })
  it('сброс сохраняет режим', () => {
    expect(sim.apply(run('key', sendA), { type: 'reset' })).toMatchObject({ mode: 'key', chats: 0, attempts: [] })
  })
})
```

`course/viz/simulations/idempotency.ts`:

```ts
import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Simulation, Step } from '../core/types'

export type IdemMode = 'no-key' | 'key'
export type Body = 'A' | 'B'
export type Outcome = 'created' | 'replayed' | 'in-progress' | 'key-reused' | 'server-error'

export interface Attempt {
  n: number
  body: Body
  status: number // что ответил сервер
  delivered: boolean // дошёл ли ответ до клиента
  outcome: Outcome
}

// Запись ключа на сервере: тело — вместо отпечатка (SHA-256 метода, пути и тела).
export interface KeyRecord {
  body: Body
  done: boolean
  status: number
}

export interface IdemState {
  mode: IdemMode
  chats: number
  record: KeyRecord | null
  attempts: readonly Attempt[]
  loseNext: boolean
  failNext: boolean
  last: IdemAction['type'] | 'start'
}

export type IdemAction =
  | { type: 'send'; body: Body }
  | { type: 'parallel' }
  | { type: 'lose' }
  | { type: 'fail' }
  | { type: 'mode'; mode: IdemMode }
  | { type: 'reset' }

const start = (mode: IdemMode): IdemState => ({ mode, chats: 0, record: null, attempts: [], loseNext: false, failNext: false, last: 'start' })

// process — сервер обрабатывает один запрос до конца. Возвращает новое состояние и ответ.
function process(s: IdemState, body: Body): { s: IdemState; status: number; outcome: Outcome } {
  if (s.mode === 'key' && s.record !== null) {
    if (s.record.body !== body) return { s, status: 422, outcome: 'key-reused' }
    if (!s.record.done) return { s, status: 409, outcome: 'in-progress' }
    return { s, status: s.record.status, outcome: 'replayed' }
  }
  if (s.failNext) return { s: { ...s, failNext: false, record: null }, status: 500, outcome: 'server-error' }
  const record = s.mode === 'key' ? { body, done: true, status: 201 } : null
  return { s: { ...s, chats: s.chats + 1, record }, status: 201, outcome: 'created' }
}

function attempt(s: IdemState, body: Body, status: number, outcome: Outcome): IdemState {
  const a: Attempt = { n: s.attempts.length + 1, body, status, delivered: !s.loseNext, outcome }
  return { ...s, attempts: [...s.attempts, a], loseNext: false }
}

export const idempotencySimulation: Simulation<IdemState, IdemAction> = {
  initial: () => start('no-key'),
  apply: (s, a) => {
    switch (a.type) {
      case 'reset':
        return start(s.mode)
      case 'mode':
        return start(a.mode)
      case 'lose':
        return { ...s, loseNext: true, last: 'lose' }
      case 'fail':
        return { ...s, failNext: true, last: 'fail' }
      case 'send': {
        const r = process(s, a.body)
        return { ...attempt(r.s, a.body, r.status, r.outcome), last: 'send' }
      }
      case 'parallel': {
        // Два одинаковых запроса пришли одновременно. С ключом первый успел занять запись
        // «в работе» — второй видит её незавершённой. Без ключа сервер их не различает.
        if (s.mode === 'key' && s.record === null && !s.failNext) {
          const busy = { ...s, record: { body: 'A' as Body, done: false, status: 0 } }
          const second = attempt(busy, 'A', 409, 'in-progress')
          const first = process({ ...second, record: null }, 'A')
          return { ...attempt(first.s, 'A', first.status, first.outcome), last: 'parallel' }
        }
        const one = process(s, 'A')
        const s1 = attempt(one.s, 'A', one.status, one.outcome)
        const two = process(s1, 'A')
        return { ...attempt(two.s, 'A', two.status, two.outcome), last: 'parallel' }
      }
    }
  },
  steps: (s) => [{ state: s, caption: captionFor(s) } satisfies Step<IdemState>],
}

function captionFor(s: IdemState): Caption {
  const mode: Caption =
    s.mode === 'key'
      ? [{ text: 'Клиент шлёт ' }, term('ключ идемпотентности', 'idempotency-key'), { text: ': сервер запоминает ответ и на повтор отдаёт его же.' }]
      : [{ text: 'Без ключа сервер не отличает повтор от нового запроса.' }]
  if (s.attempts.length === 0)
    return [...mode, { text: ' Отправьте запрос, потеряйте ответ и повторите — так ведёт себя любой клиент с ' }, term('доставкой хотя бы раз', 'at-least-once'), { text: '.' }]
  const lastA = s.attempts.at(-1) as Attempt
  const delivered = lastA.delivered ? `клиент получил ${lastA.status}` : `сервер ответил ${lastA.status}, но ответ потерялся`
  return [...mode, { text: ` Попытка ${lastA.n}: ${delivered}. Создано чатов: ${s.chats}.` }]
}
```

Run: `npx vitest run viz/simulations/idempotency.test.ts` → PASS.

- [ ] **Step 4: Настоящие токены для `jwt`**

Песочница показывает **настоящий** JWT, подписанный Ed25519 на опубликованном учебном ключе (seed — 32 байта `0x01…0x20`), чтобы любопытный ученик мог проверить его на jwt.io. Сгенерировать в scratchpad (не коммитить) программой на Go с `github.com/golang-jwt/jwt/v5@v5.3.1`:

```go
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	// kid — отпечаток RFC 7638: SHA-256 от канонического JWK с ключами по алфавиту.
	jwk := fmt.Sprintf(`{"crv":"Ed25519","kty":"OKP","x":"%s"}`, base64.RawURLEncoding.EncodeToString(pub))
	sum := sha256.Sum256([]byte(jwk))
	kid := base64.RawURLEncoding.EncodeToString(sum[:])
	iat := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	for _, aud := range []string{"messenger-api", "billing-api"} {
		claims := jwt.RegisteredClaims{
			Issuer: "messenger", Subject: "01929f6e-7a1c-7c3e-9a51-6f2d1b8e4a01",
			Audience: jwt.ClaimStrings{aud}, ExpiresAt: jwt.NewNumericDate(iat.Add(15 * time.Minute)),
			NotBefore: jwt.NewNumericDate(iat), IssuedAt: jwt.NewNumericDate(iat),
			ID: "01929f6e-7a1c-7c3e-9a51-6f2d1b8e4aff",
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
		tok.Header["kid"] = kid
		s, err := tok.SignedString(priv)
		if err != nil {
			panic(err)
		}
		fmt.Println(aud, s)
	}
	fmt.Println("x", base64.RawURLEncoding.EncodeToString(pub), "kid", kid)
}
```

Вывод (два токена, `x`, `kid`) вставить константами в `jwt.ts`. Тест шага 5 проверяет, что заголовок и payload, которые песочница кодирует сама, побайтно совпадают с подписанными, — значит, константы и модель согласованы.

- [ ] **Step 5: Тесты и реализация `jwt`**

`course/viz/simulations/jwt.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { encodeToken, jwtSimulation as sim, MAIN_TOKEN, verify, type JwtAction } from './jwt'

const run = (...actions: JwtAction[]) => actions.reduce(sim.apply, sim.initial())

describe('jwt', () => {
  it('песочница кодирует ровно те байты, что подписаны', () => {
    expect(encodeToken(sim.initial())).toBe(MAIN_TOKEN)
  })
  it('исходный токен проходит', () => {
    expect(verify(sim.initial())).toEqual({ ok: true, rule: 'ok' })
  })
  it.each<[string, JwtAction, string]>([
    ['подмена sub ломает подпись', { type: 'sub', value: 'eve' }, 'signature'],
    ['байт подписи изменён', { type: 'tamper-signature' }, 'signature'],
    ['alg: none отклоняется до проверки подписи', { type: 'alg', value: 'none' }, 'alg'],
    ['HS256 с открытым ключом как секретом', { type: 'alg', value: 'HS256' }, 'alg'],
    ['неизвестный kid', { type: 'kid', value: 'chuzhoi-kluch' }, 'kid'],
    ['часы ушли на 16 минут вперёд', { type: 'clock', seconds: 16 * 60 }, 'exp'],
    ['часы отстали на 5 минут', { type: 'clock', seconds: -5 * 60 }, 'iat'],
    ['токен выпущен для другого сервиса', { type: 'token', which: 'foreign-aud' }, 'aud'],
  ])('%s', (_, action, rule) => {
    expect(verify(run(action))).toEqual({ ok: false, rule })
  })
  it('допуск 30 секунд: 15 минут 20 секунд ещё проходят', () => {
    expect(verify(run({ type: 'clock', seconds: 15 * 60 + 20 })).ok).toBe(true)
  })
  it('сброс возвращает исходный токен', () => {
    expect(encodeToken(run({ type: 'sub', value: 'eve' }, { type: 'reset' }))).toBe(MAIN_TOKEN)
  })
})
```

`course/viz/simulations/jwt.ts` — полный код, кроме значений четырёх констант из шага 4 (`MAIN_TOKEN`, `FOREIGN_AUD_TOKEN`, `KID`, `PUBLIC_X`):

```ts
import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Simulation, Step } from '../core/types'

// Настоящие токены, подписанные Ed25519 на учебном ключе (seed 0x01…0x20, задача 2, шаг 4).
// Проверить подпись можно на jwt.io открытым ключом PUBLIC_X.
export const MAIN_TOKEN = '…'
export const FOREIGN_AUD_TOKEN = '…'
export const KID = '…'
export const PUBLIC_X = '…'

export const ISSUER = 'messenger'
export const AUDIENCE = 'messenger-api'
export const LEEWAY_SEC = 30
const IAT = Date.UTC(2026, 9, 14, 12, 0, 0) / 1000

export interface JwtHeader { alg: string; kid: string; typ: string }
export interface JwtPayload { iss: string; sub: string; aud: string[]; exp: number; nbf: number; iat: number; jti: string }

export interface JwtState {
  token: 'main' | 'foreign-aud'
  header: JwtHeader
  payload: JwtPayload
  signature: string
  clockOffsetSec: number
  last: JwtAction['type'] | 'start'
}

export type JwtAction =
  | { type: 'sub'; value: string }
  | { type: 'alg'; value: 'none' | 'HS256' }
  | { type: 'kid'; value: string }
  | { type: 'tamper-signature' }
  | { type: 'clock'; seconds: number }
  | { type: 'token'; which: 'main' | 'foreign-aud' }
  | { type: 'reset' }

export type Rule = 'ok' | 'alg' | 'kid' | 'signature' | 'exp' | 'iat' | 'nbf' | 'aud' | 'iss'

const parts = (t: string): [string, string, string] => t.split('.') as [string, string, string]

function b64urlDecode(s: string): string {
  const bin = atob(s.replace(/-/g, '+').replace(/_/g, '/'))
  return new TextDecoder().decode(Uint8Array.from(bin, (c) => c.charCodeAt(0)))
}

function b64urlEncode(s: string): string {
  const bytes = new TextEncoder().encode(s)
  return btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

function fromToken(t: string, which: JwtState['token']): JwtState {
  const [h, p, sig] = parts(t)
  return { token: which, header: JSON.parse(b64urlDecode(h)), payload: JSON.parse(b64urlDecode(p)), signature: sig, clockOffsetSec: 0, last: 'start' }
}

const original = (s: JwtState): string => (s.token === 'main' ? MAIN_TOKEN : FOREIGN_AUD_TOKEN)

// Порядок полей — как у Go (заголовок: alg, kid, typ; claims — порядок RegisteredClaims),
// иначе байты разойдутся с подписанными даже без правок.
export function encodeToken(s: JwtState): string {
  const h = { alg: s.header.alg, kid: s.header.kid, typ: s.header.typ }
  const p = { iss: s.payload.iss, sub: s.payload.sub, aud: s.payload.aud, exp: s.payload.exp, nbf: s.payload.nbf, iat: s.payload.iat, jti: s.payload.jti }
  return `${b64urlEncode(JSON.stringify(h))}.${b64urlEncode(JSON.stringify(p))}.${s.signature}`
}

// verify повторяет порядок golang-jwt v5 с нашими опциями: сначала алгоритм из белого списка,
// потом ключ по kid, потом подпись и только потом claims. Подпись в модели верна, если
// подписанные байты и сама подпись не менялись — так её и проверяет Ed25519.
export function verify(s: JwtState): { ok: boolean; rule: Rule } {
  const fail = (rule: Rule) => ({ ok: false, rule })
  if (s.header.alg !== 'EdDSA') return fail('alg')
  if (s.header.kid !== KID) return fail('kid')
  const [h, p] = parts(encodeToken(s))
  const [oh, op, osig] = parts(original(s))
  if (h !== oh || p !== op || s.signature !== osig) return fail('signature')
  const now = IAT + s.clockOffsetSec
  if (now > s.payload.exp + LEEWAY_SEC) return fail('exp')
  if (now + LEEWAY_SEC < s.payload.iat) return fail('iat')
  if (now + LEEWAY_SEC < s.payload.nbf) return fail('nbf')
  if (!s.payload.aud.includes(AUDIENCE)) return fail('aud')
  if (s.payload.iss !== ISSUER) return fail('iss')
  return { ok: true, rule: 'ok' }
}

function flipLastChar(sig: string): string {
  const c = sig.at(-1) === 'A' ? 'B' : 'A'
  return sig.slice(0, -1) + c
}

export const jwtSimulation: Simulation<JwtState, JwtAction> = {
  initial: () => fromToken(MAIN_TOKEN, 'main'),
  apply: (s, a) => {
    switch (a.type) {
      case 'reset':
        return fromToken(MAIN_TOKEN, 'main')
      case 'token':
        return { ...fromToken(a.which === 'main' ? MAIN_TOKEN : FOREIGN_AUD_TOKEN, a.which), last: 'token' }
      case 'sub':
        return { ...s, payload: { ...s.payload, sub: a.value }, last: 'sub' }
      case 'alg':
        return { ...s, header: { ...s.header, alg: a.value }, last: 'alg' }
      case 'kid':
        return { ...s, header: { ...s.header, kid: a.value }, last: 'kid' }
      case 'tamper-signature':
        return { ...s, signature: flipLastChar(s.signature), last: 'tamper-signature' }
      case 'clock':
        return { ...s, clockOffsetSec: s.clockOffsetSec + a.seconds, last: 'clock' }
    }
  },
  steps: (s) => [{ state: s, caption: captionFor(s) } satisfies Step<JwtState>],
}

const WHY: Record<Rule, string> = {
  ok: 'Токен прошёл все проверки: алгоритм из белого списка, известный ключ, подпись сходится, срок не истёк, аудитория наша.',
  alg: 'Отклонён до проверки подписи: сервер принимает только EdDSA. Если бы он верил полю alg из самого токена, атакующий выбрал бы none или HS256 с открытым ключом в роли секрета.',
  kid: 'Ключа с таким kid сервер не знает. Ключ выбирают по kid только из своего списка, а не из адреса, присланного в токене.',
  signature: 'Подпись не сходится: изменён хотя бы один байт заголовка, данных или подписи. Без закрытого ключа новую подпись не получить.',
  exp: 'Срок жизни истёк (exp плюс допуск 30 секунд на расхождение часов). Короткий срок — наша замена мгновенному отзыву токена.',
  iat: 'Токен выпущен «в будущем» по часам сервера дальше допуска — часы рассинхронизированы или токен подделан.',
  nbf: 'Токен ещё не действует (nbf в будущем дальше допуска).',
  aud: 'Подпись верна, но токен выпущен для другого сервиса (aud). Чужой токен нельзя предъявлять нам, даже если ключ тот же.',
  iss: 'Токен выпустил не наш сервис (iss).',
}

function captionFor(s: JwtState): Caption {
  const v = verify(s)
  return [
    term('JWT', 'jwt'),
    { text: ' — три части в base64url: заголовок, ' },
    term('claims', 'claim'),
    { text: ' и ' },
    term('подпись', 'digital-signature'),
    { text: `. Это не шифрование: прочитать может любой. ${WHY[v.rule]}` },
  ]
}
```

Run: `npx vitest run viz/simulations/jwt.test.ts` → PASS. Если первый тест падает — порядок полей или константы не совпадают с подписанными (исправлять модель, а не тест).

- [ ] **Step 6: Рендереры и сцены (контракт)**

Общее для трёх: `VizFrame` с полноэкранным режимом, `Player` с одним шагом-состоянием (как `CtxTreeSandbox`), панель кнопок (`min-height: 36px`, `:focus-visible`), подпись шага через `Player`, узкая раскладка ≤ 375 px через `@container`, `prefers-reduced-motion` (без анимаций переходов).

- **`PaginationView.vue` / `PaginationSandbox.vue`.** Кнопки: «Следующая страница», «Новое сообщение сверху», «Удалить верхнее», переключатель «offset / курсор» (`role="radiogroup"`), «Сброс». Вид: лента ячеек `seq` (новые — сверху/слева), прочитанные страницы цветными рамками по номеру страницы; дубли — `--viz-bad` с подписью «дубль», пропуски — штриховкой с подписью «пропуск»; новые после начала чтения — отметка «новое». Под лентой — состояние: для offset `offset=N`, для курсора — раскрытое значение `{"v":1,"seq":N}` и его base64url (как в мессенджере). Текст-введение: история чата от новых к старым, страница — 5 сообщений.
- **`IdempotencyView.vue` / `IdempotencySandbox.vue`.** Три колонки: клиент, сеть, сервер (+ хранилище ключей под сервером). Кнопки: «Отправить (тело A)», «Повторить», «Повторить с телом B», «Два запроса одновременно», «Потерять следующий ответ», «Сбой сервера на следующем запросе», переключатель «без ключа / с ключом», «Сброс». «Отправить» и «Повторить» — одно и то же действие `send A` (подписи кнопок разные для понятности). Список попыток: номер, тело, статус, доставлен ли ответ (потерянный — перечёркнутая стрелка). Счётчик «создано чатов» крупно; при `chats > 1` в режиме без ключа — предупреждение «дубль».
- **`JwtView.vue` / `JwtSandbox.vue`.** Токен строкой в три цвета (заголовок, payload, подпись — разные `--viz-*`), с переносом по символам (`overflow-wrap: anywhere`); под ним декодированные JSON заголовка и payload; вердикт: ✓/✗, правило (`alg`, `kid`, `signature`, `exp`, `iat`, `nbf`, `aud`, `iss`) и пояснение. Кнопки: «Сменить sub на Еву», «alg: none», «alg: HS256», «Чужой kid», «Изменить байт подписи», «Часы +16 мин», «Часы −5 мин», «Токен для другого сервиса», «Сброс». Подпись под сценой: учебный ключ (seed `0x01…0x20`), открытый ключ `PUBLIC_X`, «проверьте на jwt.io».

- [ ] **Step 7: Проверка и коммит**

Run: `npm test && npm run typecheck && npm run build` → PASS. Временно подключить три сцены на любую страницу `npm run dev` и посмотреть в Playwright на 375 и 1280 px в обеих темах (кнопки работают, нет горизонтальной прокрутки, ⛶ работает); страницу вернуть.

```bash
git add course/viz
git commit -m "feat(viz): песочницы pagination, idempotency, jwt

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Плееры `request-pipeline`, `contract-pipeline`, `problem-mapping`, `login-timing`

**Files:**
- Create: `course/viz/simulations/{request-pipeline,contract-pipeline,problem-mapping,login-timing}.ts` и `*.test.ts`
- Create: `course/viz/renderers/{RequestPipelineView,ContractPipelineView,ProblemMappingView,LoginTimingView}.vue`
- Create: `course/viz/scenes/{RequestPipeline,ContractPipeline,ProblemMapping,LoginTiming}.vue`

**Interfaces:**
- Consumes: `Step`, `Scenario`, `term`; термины задачи 1.
- Produces: сцены для уроков 1.8, 1.9, 1.10, 1.13.

Каждый плеер — `Scenario<S>`: функция без аргументов, возвращающая массив кадров (как `http-mapping.ts`). Состояние кадра — то, что рисует рендерер; подпись — `Caption` с `term(…)` для терминов из задачи 1; `highlight` — id подсвеченного элемента. Тесты (`*.test.ts`) для каждого: число кадров в заявленных границах; каждый `highlight` — существующий id; у каждой подписи есть текст; перечисленные ниже обязательные факты присутствуют в состоянии соответствующих кадров (проверять по полям состояния, а не по тексту подписи).

- [ ] **Step 1: `request-pipeline` (урок 1.8), 10–14 кадров**

Состояние: `{ scenario: 'ok' | 'panic' | 'no-identity' | 'unknown-route' | 'wrong-method'; at: 'request-id' | 'access-log' | 'recover' | 'auth' | 'router' | 'handler' | 'response'; status: number | null; requestId: string; log: string[] }`. Пять сценариев подряд, каждый 2–3 кадра:
1. **Успех** `POST /v1/chats/{id}/messages`: `RequestID` присвоил id (или принял входящий `X-Request-ID`) → `AccessLog` запомнил время → `Recover` → заглушка аутентификации положила пользователя в `context` → `ServeMux` выбрал шаблон → хендлер ответил 201 → `AccessLog` записал строку со статусом и длительностью.
2. **Паника в хендлере**: `Recover` поймал, ответил 500, строка лога со статусом 500 — потому что `Recover` **внутри** `AccessLog`.
3. **Нет заголовка личности**: цепочка остановилась на `auth` → 401; хендлер не вызывался.
4. **Неизвестный путь**: 404 в общем формате ошибок.
5. **Неверный метод** `DELETE /v1/chats`: 405 с `Allow`.

Обязательные факты в тесте: сценарий `panic` содержит кадр `at: 'response'` со `status: 500` и строкой лога со `500`; `no-identity` не содержит кадра `at: 'handler'`; `wrong-method` содержит `status: 405`. Рендерер: вертикальная цепочка «матрёшкой» (внешние middleware снаружи), текущий слой подсвечен, справа — лог.

- [ ] **Step 2: `contract-pipeline` (урок 1.9), 8–10 кадров**

Состояние: `{ stage: 'contract' | 'generate' | 'interface' | 'handler' | 'test'; files: { path: string; snippet: string }[]; caught: null | 'compiler' | 'contract-test' }`. Кадры: фрагмент `openapi.yaml` (операция `sendMessage`) → `go generate` → сгенерированные `SendMessageRequest` и метод `ServerInterface` → наш хендлер, реализующий метод → контрактный тест проверяет ответ. Затем два расхождения:
1. в контракте поле запроса переименовали, хендлер обращается к старому полю → **компилятор** (`caught: 'compiler'`), сообщение `go build`;
2. хендлер начал отдавать в ответе поле `seq` строкой → компилятор молчит (ответ пишем мы), **контрактный тест** падает (`caught: 'contract-test'`) с сообщением kin-openapi.

Сниппеты — правдоподобные и согласованные с задачей 5 (имена типов как у oapi-codegen: `SendMessageRequest`, `ClientMessageId`, `ServerInterface`). Тест: ровно один кадр с `caught: 'compiler'` и один с `caught: 'contract-test'`, и первый идёт раньше второго.

- [ ] **Step 3: `problem-mapping` (урок 1.10), 8–10 кадров**

Состояние: `{ source: 'domain' | 'param' | 'router' | 'panic' | 'decode'; error: string; problem: Record<string, unknown> | null; highlightFields: string[] }`. Пять источников ошибки и их путь в `application/problem+json`:
- доменная `ValidationError{Field:"text"}` → 400 `invalid_input` с `invalid_params:[{"name":"text",…}]`;
- ошибка параметра из сгенерированной обёртки (`limit=abc`) → 400 `invalid_input`, `invalid_params:[{"name":"limit",…}]`;
- маршрутизатор: `DELETE /v1/chats` → 405 `method_not_allowed` + `Allow`;
- паника → 500 `internal` без деталей, детали — только в лог;
- тело больше 1 МиБ → 413 `body_too_large`.
Каждый `problem` содержит `type` (`https://messenger.example/problems/<code>`), `title`, `status`, `detail`, `instance`, `code`, `request_id` — тест проверяет эти поля у всех непустых `problem` и что у 500 `detail` не содержит текста исходной ошибки.

- [ ] **Step 4: `login-timing` (урок 1.13), 8–10 кадров**

Состояние: `{ impl: 'naive' | 'safe'; attempt: 'unknown-user' | 'wrong-password'; response: { status: number; code: string; text: string }; elapsedMs: number; steps: string[] }`. Сначала наивная реализация: «нет такого пользователя» → 404/«пользователь не найден» за ~1 мс; «неверный пароль» → 401 за ~50 мс (Argon2id). Две утечки: текст и время. Затем правильная: оба случая — 401 `invalid_credentials`, одинаковый текст, ~50 мс, потому что для несуществующего пользователя проверяется фиктивный хеш. Числа времени — порядок величины, в подписи помечено «на машине автора; меряется в уроке». Тест: в `naive` ответы двух попыток различаются по `status` или `text` и по `elapsedMs` больше чем в 10 раз; в `safe` — совпадают по `status`, `code`, `text`, а `elapsedMs` различаются не больше чем на 20%.

- [ ] **Step 5: Рендереры, сцены, проверка, коммит**

Рендереры — по описанию в шагах, в общем стиле плееров 1a (`HttpMappingView`, `LayerFlowView`); сцены — `VizFrame` + `Player` с `label`. 

Run: `npm test && npm run typecheck && npm run build` → PASS; Playwright, как в задаче 2, шаг 7.

```bash
git add course/viz
git commit -m "feat(viz): плееры request-pipeline, contract-pipeline, problem-mapping, login-timing

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Урок 1.8 «Ресурсы, семантика HTTP и конвейер запроса»

**Files:**
- Create: `messenger/internal/platform/principal/{principal.go,principal_test.go}`
- Create: `messenger/internal/platform/middleware/{doc.go,chain.go,requestid.go,recorder.go,accesslog.go,recover.go,routes.go,middleware_test.go}`
- Create: `messenger/internal/platform/httpauth/{doc.go,httpauth.go,httpauth_test.go}`
- Delete: `messenger/internal/chat/adapters/httpapi/auth.go`; Create: `messenger/internal/chat/adapters/httpapi/caller.go`
- Modify: оба `httpapi/handler.go` (`/v1`, `Location`, личность из `principal`), оба `handler_test.go`, `messenger/cmd/messenger/main.go`
- Create: `course/modules/01-go-service/08-resources.md`

**Interfaces:**
- Consumes: код 1a; сцена `<RequestPipeline />` (задача 3).
- Produces (используют задачи 5–10):
  - `principal.WithUserID(ctx, id string) context.Context`, `principal.UserIDFrom(ctx) (string, bool)`;
  - `middleware.Middleware` (`func(http.Handler) http.Handler`), `middleware.Chain(h, mws...)` (первый — внешний), `middleware.RequestID(ids IDGenerator)`, `middleware.RequestIDFrom(ctx) string`, `middleware.RequestIDHeader`, `middleware.AccessLog(log)`, `middleware.Recover(log, respond func(w, r))`, `middleware.RouteErrors(mux, respond func(w, r, status int)) http.Handler`;
  - `httpauth.Authenticator` (`Authenticate(r *http.Request) (string, error)`), `httpauth.Middleware(a, public []string, unauthorized func(w, r, err error))`, `httpauth.ErrNoCredentials`, временный `httpauth.HeaderStub` (удаляет задача 10).

**Почему личность — в `context`, а не аргументом.** Middleware стоит до маршрутизатора и не знает сигнатур хендлеров; `context` — канонический в Go способ передать данные запроса через границы API (go.dev/blog/context). Но дальше хендлера она идёт **явным аргументом** команды use case'а — use case'ы `context` не читают.

- [ ] **Step 1: `principal`**

`internal/platform/principal/principal.go`:

```go
// Package principal переносит «кто вызывает» от middleware аутентификации к хендлерам.
// Это маленькое общее ядро (shared kernel): его знают все контексты, поэтому в нём —
// только строка id, без типов какого-либо контекста.
package principal

import "context"

// ctxKey — неэкспортируемый тип ключа: никакой другой пакет не сможет ни прочитать,
// ни подменить значение по совпавшему ключу (в отличие от ключа-строки).
type ctxKey struct{}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, ctxKey{}, userID)
}

// UserIDFrom — id того, кого аутентифицировал middleware. ok == false — личности нет:
// маршрут публичный или middleware не подключили.
func UserIDFrom(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(ctxKey{}).(string)
	return id, ok && id != ""
}
```

`principal_test.go`: пустой контекст → `ok == false`; после `WithUserID(ctx, "u")` → `"u", true`; `WithUserID(ctx, "")` → `ok == false`.

- [ ] **Step 2: Тесты `middleware`**

`internal/platform/middleware/middleware_test.go` (внешний пакет `middleware_test`). Полный код ключевых тестов:

```go
package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"messenger/internal/platform/middleware"
)

type fixedIDs string

func (f fixedIDs) NewID() string { return string(f) }

func logger(buf *bytes.Buffer) *slog.Logger { return slog.New(slog.NewJSONHandler(buf, nil)) }

func respond500(w http.ResponseWriter, _ *http.Request) { http.Error(w, "внутренняя ошибка", http.StatusInternalServerError) }

// Порядок цепочки из урока: паника должна попасть в access-log как 500 — значит,
// Recover стоит ВНУТРИ AccessLog.
func TestPanicIsLoggedAs500(t *testing.T) {
	var logs bytes.Buffer
	log := logger(&logs)
	boom := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("бум") })
	h := middleware.Chain(boom, middleware.RequestID(fixedIDs("req-1")), middleware.AccessLog(log), middleware.Recover(log, respond500))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/chats", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("статус %d, ждали 500", rec.Code)
	}
	var access map[string]any
	for line := range strings.Lines(logs.String()) {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err == nil && m["msg"] == "запрос" {
			access = m
		}
	}
	if access == nil || access["status"] != float64(500) || access["request_id"] != "req-1" {
		t.Fatalf("строка access-log: %v", access)
	}
}

func TestRecoverRepanicsAbortHandler(t *testing.T) {
	defer func() {
		if v := recover(); v != http.ErrAbortHandler { //nolint:errorlint // сравниваем значение паники, а не ошибку из цепочки
			t.Fatalf("ждали повторную панику ErrAbortHandler, получили %v", v)
		}
	}()
	abort := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })
	middleware.Recover(slog.New(slog.DiscardHandler), respond500)(abort).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestRequestID(t *testing.T) {
	tests := []struct {
		name, incoming, want string
	}{
		{"нет заголовка — свой", "", "gen-1"},
		{"допустимый — принимаем", "abc-123_X.y", "abc-123_X.y"},
		{"перевод строки — свой", "a\nb", "gen-1"},
		{"длиннее 64 — свой", strings.Repeat("a", 65), "gen-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seen string
			h := middleware.RequestID(fixedIDs("gen-1"))(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				seen = middleware.RequestIDFrom(r.Context())
			}))
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.incoming != "" {
				r.Header.Set(middleware.RequestIDHeader, tt.incoming)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, r)
			if seen != tt.want || rec.Header().Get(middleware.RequestIDHeader) != tt.want {
				t.Fatalf("в context %q, в ответе %q, ждали %q", seen, rec.Header().Get(middleware.RequestIDHeader), tt.want)
			}
		})
	}
}

func TestRouteErrors(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/chats", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	mux.HandleFunc("POST /v1/chats", func(w http.ResponseWriter, _ *http.Request) {})
	var got []int
	h := middleware.RouteErrors(mux, func(w http.ResponseWriter, _ *http.Request, status int) {
		got = append(got, status)
		w.WriteHeader(status)
	})
	tests := []struct {
		method, path string
		want         int
		allow        string
	}{
		{http.MethodGet, "/v1/chats", http.StatusTeapot, ""}, // найденный маршрут — как есть
		{http.MethodGet, "/v1/nope", http.StatusNotFound, ""},
		{http.MethodDelete, "/v1/chats", http.StatusMethodNotAllowed, "GET, HEAD, POST"},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
		if rec.Code != tt.want || rec.Header().Get("Allow") != tt.allow {
			t.Fatalf("%s %s: %d Allow=%q, ждали %d %q", tt.method, tt.path, rec.Code, rec.Header().Get("Allow"), tt.want, tt.allow)
		}
	}
	if len(got) != 2 {
		t.Fatalf("respond вызван %d раз, ждали 2 (404 и 405)", len(got))
	}
}
```

Дополнительно (код — по образцу выше): `TestChainOrder` — три middleware пишут в срез «a-in, b-in, c-in, h, c-out, b-out, a-out»; `TestAccessLogFields` — строка лога содержит `method`, `path` (без query: в query бывает курсор), `status`, `bytes`, `duration`, `request_id`; `TestAccessLogNoQuery` — запрос `/v1/chats?cursor=секрет` не оставляет `секрет` в логе.

Run: `go test ./internal/platform/middleware/` → FAIL (пакета нет).

- [ ] **Step 3: Реализация `middleware`**

`doc.go`:

```go
// Package middleware — обёртки над http.Handler, общие для всех маршрутов: идентификатор
// запроса, журнал доступа, перехват паники и ответы маршрутизатора. Формат тела ошибки
// они не знают — его передаёт composition root.
package middleware
```

`chain.go`:

```go
package middleware

import "net/http"

type Middleware func(http.Handler) http.Handler

// Chain оборачивает h так, что первый middleware в списке — самый внешний:
// Chain(h, a, b) == a(b(h)). Запрос проходит a → b → h, ответ — в обратном порядке.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
```

`requestid.go`:

```go
package middleware

import (
	"context"
	"net/http"
)

const (
	RequestIDHeader = "X-Request-ID"
	// requestIDMaxLen и набор символов ниже защищают журнал: чужое значение попадает
	// в каждую строку лога, и перевод строки в нём позволил бы подделать запись.
	requestIDMaxLen = 64
)

type IDGenerator interface {
	NewID() string
}

type requestIDKey struct{}

// RequestID принимает входящий X-Request-ID (его ставит балансировщик или клиент — так
// один запрос виден насквозь в логах нескольких сервисов), если он безопасен, иначе
// выдаёт свой. Значение возвращается в ответе: клиент сообщит его в поддержку.
func RequestID(ids IDGenerator) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(RequestIDHeader)
			if !validRequestID(id) {
				id = ids.NewID()
			}
			w.Header().Set(RequestIDHeader, id)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
		})
	}
}

// RequestIDFrom — id текущего запроса; пустая строка, если RequestID не подключён.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

func validRequestID(s string) bool {
	if s == "" || len(s) > requestIDMaxLen {
		return false
	}
	for i := range len(s) {
		c := s[i]
		ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.'
		if !ok {
			return false
		}
	}
	return true
}
```

`recorder.go`:

```go
package middleware

import "net/http"

// statusRecorder запоминает статус и размер ответа для журнала. Unwrap нужен
// http.ResponseController: через него обработчик доберётся до Flush и таймаутов
// настоящего ResponseWriter, даже если тот обёрнут.
type statusRecorder struct {
	http.ResponseWriter
	status int // 0 — обработчик ничего не записал
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }
```

`accesslog.go`:

```go
package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// AccessLog пишет одну строку на запрос после ответа. Путь — без строки запроса:
// в ней бывают курсоры и прочее, чему не место в журнале. Статус 0 — обработчик ничего
// не записал (так бывает, когда клиент ушёл раньше ответа).
func AccessLog(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			log.LogAttrs(r.Context(), slog.LevelInfo, "запрос",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Int("bytes", rec.bytes),
				slog.Duration("duration", time.Since(start)),
				slog.String("request_id", RequestIDFrom(r.Context())),
			)
		})
	}
}
```

`recover.go`:

```go
package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover превращает панику обработчика в 500 и запись в журнале со стеком: без него
// net/http закрыл бы соединение, а клиент не получил бы ответа. Тело ответа пишет respond —
// формат ошибок задаёт composition root. Если обработчик успел отправить заголовки,
// поменять статус уже нельзя: net/http только предупредит о повторном WriteHeader.
func Recover(log *slog.Logger, respond func(w http.ResponseWriter, r *http.Request)) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				v := recover()
				if v == nil {
					return
				}
				// ErrAbortHandler — договорённость net/http «прервать ответ молча»; глотать её нельзя.
				if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(v)
				}
				log.Error("паника в обработчике", "panic", v, "stack", string(debug.Stack()), "request_id", RequestIDFrom(r.Context()))
				respond(w, r)
			}()
			next.ServeHTTP(w, r)
		})
	}
}
```

`routes.go`:

```go
package middleware

import "net/http"

// RouteErrors отдаёт ошибки маршрутизации через respond, в формате остального API:
// сам ServeMux пишет 404 и 405 простым текстом. Пустой шаблон из mux.Handler значит
// «маршрута нет» (404) или «путь есть, метода нет» (405 — тогда mux кладёт методы в Allow).
// Найденный маршрут обслуживает mux.ServeHTTP: только так в хендлере заполнен r.Pattern.
func RouteErrors(mux *http.ServeMux, respond func(w http.ResponseWriter, r *http.Request, status int)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, pattern := mux.Handler(r)
		if pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}
		probe := &headerProbe{header: http.Header{}}
		h.ServeHTTP(probe, r)
		if allow := probe.header.Get("Allow"); allow != "" {
			w.Header().Set("Allow", allow)
		}
		respond(w, r, probe.status)
	})
}

// headerProbe — ResponseWriter-заглушка: забирает у ответа mux статус и Allow, тело выбрасывает.
type headerProbe struct {
	header http.Header
	status int
}

func (p *headerProbe) Header() http.Header { return p.header }

func (p *headerProbe) WriteHeader(code int) {
	if p.status == 0 {
		p.status = code
	}
}

func (p *headerProbe) Write(b []byte) (int, error) {
	if p.status == 0 {
		p.status = http.StatusOK
	}
	return len(b), nil
}
```

Run: `go test -race ./internal/platform/middleware/` → PASS.

- [ ] **Step 4: `httpauth` с заглушкой**

`internal/platform/httpauth/httpauth.go`:

```go
// Package httpauth — аутентификация на входе: middleware спрашивает Authenticator, кто
// вызывает, и кладёт ответ в context через principal. Как именно узнаётся личность —
// заголовком-заглушкой (до урока 1.14) или токеном — решает реализация Authenticator.
package httpauth

import (
	"errors"
	"net/http"
	"strings"

	"messenger/internal/platform/principal"
)

// ErrNoCredentials — запрос не предъявил ничего, по чему можно узнать вызывающего.
var ErrNoCredentials = errors.New("нет данных для аутентификации")

type Authenticator interface {
	Authenticate(r *http.Request) (userID string, err error)
}

// Middleware закрыт по умолчанию: открыты только маршруты из public ("POST /v1/users" —
// метод и путь без параметров). Забытый в списке маршрут получит 401, а не откроется всем.
// Стоит до маршрутизатора, поэтому неизвестный путь без личности тоже получает 401:
// не говорим анониму, какие пути существуют.
func Middleware(a Authenticator, public []string, unauthorized func(w http.ResponseWriter, r *http.Request, err error)) func(http.Handler) http.Handler {
	open := make(map[string]bool, len(public))
	for _, p := range public {
		open[p] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if open[r.Method+" "+r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			id, err := a.Authenticate(r)
			if err != nil {
				unauthorized(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(principal.WithUserID(r.Context(), id)))
		})
	}
}

// #region header-stub
// HeaderStub — заглушка до JWT (ADR-0002): верит заголовку X-User-ID, который присылает
// клиент, то есть любой может назваться кем угодно. Годится только для локального стенда.
// Урок 1.14 заменяет её проверкой токена; хендлеры при этом не меняются.
type HeaderStub struct{}

const UserIDHeader = "X-User-ID"

func (HeaderStub) Authenticate(r *http.Request) (string, error) {
	id := strings.TrimSpace(r.Header.Get(UserIDHeader))
	if id == "" {
		return "", ErrNoCredentials
	}
	return id, nil
}

// #endregion header-stub
```

`httpauth_test.go`: публичный маршрут пропускается без заголовка и без `principal`; закрытый без заголовка → `unauthorized` с `errors.Is(err, ErrNoCredentials)`, хендлер не вызван; с заголовком → хендлер видит `principal.UserIDFrom` = значение без пробелов по краям; `GET /v1/users` при публичном `POST /v1/users` — закрыт (метод входит в ключ).

- [ ] **Step 5: Хендлеры: `/v1`, `Location`, личность из `principal`**

- **`chat/adapters/httpapi/auth.go` удалить.** Новый `caller.go` (регион `caller`):

```go
package httpapi

import (
	"net/http"

	"messenger/internal/chat/domain"
	"messenger/internal/platform/httpjson"
	"messenger/internal/platform/principal"
)

// #region caller
// caller — пользователь, которого аутентифицировал middleware. Хендлер не знает, как
// именно: заголовком-заглушкой или токеном. Нет личности или она не UUID — 401:
// «не знаю, кто вы» — не ошибка данных запроса, поэтому до use case дело не доходит.
func (h *Handler) caller(w http.ResponseWriter, r *http.Request) (domain.UserID, bool) {
	raw, ok := principal.UserIDFrom(r.Context())
	if !ok {
		h.unauthenticated(w, r)
		return domain.UserID{}, false
	}
	id, err := domain.ParseUserID(raw)
	if err != nil {
		h.unauthenticated(w, r)
		return domain.UserID{}, false
	}
	return id, true
}

// #endregion caller

func (h *Handler) unauthenticated(w http.ResponseWriter, r *http.Request) {
	if err := httpjson.WriteError(w, http.StatusUnauthorized, "unauthenticated", "не удалось определить пользователя", ""); err != nil {
		h.log.Error("запись ответа", "path", r.URL.Path, "err", err)
	}
}
```

  Все вызовы `h.authenticate(w, r)` заменить на `h.caller(w, r)`.
- **Маршруты под `/v1`:** `POST /v1/users`, `GET /v1/users/{id}`, `POST /v1/chats`, `POST /v1/chats/{id}/messages`, `GET /v1/chats/{id}/messages`.
- **`Location`:** `POST /v1/users` → `Location: /v1/users/{id}`; `POST /v1/chats` при `Created` → `Location: /v1/chats/{id}` (при 200 для существующего личного чата — тоже `Location` на него, RFC 9110 §10.2.2 этого не запрещает, но **не** обязательно: выбрать «ставить» и объяснить в уроке). У сообщений `Location` нет: отдельного ресурса «сообщение» с `GET` в API нет — урок объясняет почему.
- **Тесты хендлеров:** вместо заголовка `X-User-ID` кладут личность через `principal.WithUserID` в контекст запроса (`r = r.WithContext(principal.WithUserID(r.Context(), aliceID))`); новые проверки — `Location` у 201; 401 при отсутствии личности и при не-UUID личности; пути `/v1/...`; `PUT /v1/chats` больше **не** тестируется на уровне адаптера (405 теперь дело `RouteErrors`).

- [ ] **Step 6: `main.go`**

Регион `wiring` (контракт):
- сборка как в 1a; `mux := http.NewServeMux()`, `Register` обоих адаптеров;
- обработчик целиком:

```go
handler := middleware.Chain(
	middleware.RouteErrors(mux, routeError),
	middleware.RequestID(ids),
	middleware.AccessLog(log),
	middleware.Recover(log, internalError),
	httpauth.Middleware(httpauth.HeaderStub{}, publicRoutes, unauthenticated),
)
```

  где `publicRoutes = []string{"POST /v1/users"}` (константа-список рядом с `run`, с комментарием «всё остальное закрыто»), а `routeError`, `internalError`, `unauthenticated` — функции в `main.go`, отвечающие в формате 1a через `httpjson.WriteError`: 404 `not_found` «маршрут не найден», 405 `method_not_allowed` «метод не поддерживается для этого пути», 500 `internal`, 401 `unauthenticated` «нужен заголовок X-User-ID» (в 1.14 текст меняется). Ошибки записи — в `log.Error`.
- регион `chain` вокруг `middleware.Chain(...)` — урок показывает его отдельно.

Run: `go build ./... && go test -race ./... && golangci-lint run ./...` → PASS.

- [ ] **Step 7: Прогон через `curl`**

Сервер запустить; сценарий 1.7 с путями `/v1/...` (регистрация двух пользователей, личный чат, сообщение, история) плюс:
- `curl -si -X POST localhost:8080/v1/users …` — показать `Location` и `X-Request-ID` в ответе;
- `curl -si -H 'X-Request-ID: demo-42' …` — тот же id в ответе и в строке access-log;
- `curl -si -X DELETE localhost:8080/v1/chats -H "X-User-ID: $ALICE"` — 405 с `Allow` в JSON-формате;
- `curl -si localhost:8080/v1/nope -H "X-User-ID: $ALICE"` — 404 в JSON-формате;
- `curl -si localhost:8080/v1/nope` — 401 (аутентификация раньше маршрутизации);
- старый путь `/chats` — 404 (версия обязательна).

Вывод и строки access-log — в отчёт; урок цитирует.

- [ ] **Step 8: Урок `08-resources.md`**

Frontmatter: `title: 1.8. Ресурсы, семантика HTTP и конвейер запроса`.

**Разделы:**
- **Проблема.** API 1a «работает», но: `POST /chats` без `Location` — клиент не знает адреса созданного; путь без версии — первое несовместимое изменение сломает всех клиентов сразу; проверка заголовка размазана по хендлерам; паника роняет соединение без следа в логе; 404 и 405 — простым текстом, не как остальные ошибки.
- **Теория:**
  - `rest` по Филдингу кратко и что из него реально используют индустриальные API (ресурсы, единый интерфейс методов, коды статуса; HATEOAS — почти никто, и почему);
  - `resource`: коллекции и элементы, вложенность `/chats/{id}/messages` и когда её не делать (больше двух уровней); существительные, а не глаголы; почему у сообщения нет своего URI в нашем API;
  - методы по RFC 9110 §9.2: `safe-method`, `idempotent-method`, кешируемость; таблица GET/HEAD/POST/PUT/PATCH/DELETE; почему POST не идемпотентен и что из этого следует (мостик к 1.12);
  - `status-code` как контракт: 200 против 201 (+`Location`) против 204; 400 против 422 (позиция RFC 9110 и почему мы используем 400 для ошибок данных, а 422 оставляем для «ключ использован с другим телом» в 1.12); 401 против 403; 404 против 405 (+`Allow`); 409; 413; 415; 5xx — сервер виноват;
  - `api-versioning`: путь `/v1`, заголовок, медиатип, датированные версии Stripe; что такое `breaking-change` (удалить поле, сменить тип, сделать необязательное обязательным) и что изменением не является (новое необязательное поле — если клиенты игнорируют незнакомые поля); почему мы выбрали путь;
  - конвейер `middleware`: «матрёшка», порядок важен — разобрать на нашей цепочке, почему `Recover` внутри `AccessLog`, почему аутентификация до маршрутизатора (и что из этого 401 на неизвестный путь); `request-id` и сквозная трассировка (мостик к модулю наблюдаемости);
  - личность в `context`: неэкспортируемый тип ключа, `principal` как `shared-kernel`; правило — `context` несёт данные запроса до хендлера, дальше — явные аргументы.
- **Визуализация:** `<RequestPipeline />` и 3 задания («найдите кадр, где паника превращается в строку лога», …).
- **Код:** регионы `chain` (main), `caller`, `header-stub`, `RouteErrors` целиком (регион `route-errors` — добавить в `routes.go`), `Recover` (регион `recover`). Сценарий `curl` с реальным выводом.
- **Под капотом:** «Как `ServeMux` отвечает 405» — по `$(go env GOROOT)/src/net/http/server.go` (`findHandler`, `matchingMethods`) и `routing_tree.go`: почему 405 возникает только когда путь совпал хотя бы с одним шаблоном, откуда берётся `HEAD` в `Allow` (GET регистрирует и HEAD), почему пустой шаблон у ответа «не найдено».
- **Ошибки:** глаголы в путях (`/sendMessage`); 200 с `{"error":…}` в теле; `Recover` снаружи журнала; ключ `context` строкой; читать личность в use case из `context`; версия «когда понадобится».
- **Компромисс:** версия в пути — проще кешировать и видно в логах, но «двигается» весь API разом; заголовок — гибче, но невидим в URL.
- **Самопроверка (3–5), источники:** RFC 9110 (§9.2 методы, §15 статусы, §10.2.2 `Location`); Филдинг, диссертация, гл. 5; Google API Design Guide (ресурсы, методы); Microsoft REST API Guidelines (версии); Stripe API versioning (блог); Эдвардс, «Let's Go Further», гл. 6 (middleware), гл. 3 (маршрутизация).
- **Что изменилось в мессенджере** (`git -c core.quotePath=false diff --stat m01-l07 m01-l08 -- messenger`).

**Термины:** `rest`, `resource`, `safe-method`, `idempotent-method`, `status-code`, `api-versioning`, `breaking-change`, `middleware`, `request-id`, `shared-kernel`.

**Факты проверить:** формулировки RFC 9110 о 201/`Location`, 405/`Allow`, 422; что `ServeMux` добавляет `HEAD` к GET-шаблону; `strings.Lines` (Go 1.24) — если используется в тестах; `slog.DiscardHandler` (Go 1.24).

- [ ] **Step 9: Проверка, коммит, тег**

Run: `npm test && npm run typecheck && npm run check:messenger && npm run build` → PASS.

```bash
git add messenger course
git commit -m "feat(messenger): /v1, конвейер middleware, личность через principal; урок 1.8

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git tag m01-l08
```

---

### Task 5: Урок 1.9 «Contract-first: OpenAPI и генерация кода» + ADR-0006

**Files:**
- Modify: `messenger/go.mod`, `messenger/go.sum`
- Create: `messenger/api/openapi.yaml`
- Create: `messenger/internal/{user,chat}/adapters/httpapi/gen/{config.yaml,generate.go,api.gen.go}`
- Create: `messenger/internal/platform/openapitest/{doc.go,openapitest.go}`
- Modify: оба `httpapi/handler.go` (реализация `gen.ServerInterface`), оба `httpapi/dto.go` (мапперы на сгенерированные типы; ручные DTO удаляются), оба `handler_test.go` (обёртка `openapitest`)
- Create: `messenger/docs/adr/0006-contract-first.md`
- Create: `course/modules/01-go-service/09-contract-first.md`

**Interfaces:**
- Consumes: задача 4.
- Produces:
  - `api/openapi.yaml` — контракт; задачи 6–10 его расширяют;
  - пакеты `…/httpapi/gen` с `ServerInterface`, моделями, `HandlerWithOptions`, `StdHTTPServerOptions`, `MiddlewareFunc`;
  - `Register(mux *http.ServeMux, mws ...gen.MiddlewareFunc)` у каждого адаптера (вариадический `mws` использует задача 8);
  - `openapitest.New(t testing.TB) *Validator`, `(*Validator).Wrap(h http.Handler) http.Handler`, `(*Validator).Uncovered() []string`.

- [ ] **Step 1: Зависимости**

```bash
go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0
go get github.com/oapi-codegen/runtime@v1.7.0 github.com/getkin/kin-openapi@v0.149.0
```

Если `sum.golang.org` отвечает `TLS handshake timeout` — повторить (в спайке помогла повторная попытка). После шага 4 — `go mod tidy`; в `go.mod` строка `tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen`.

- [ ] **Step 2: Контракт `api/openapi.yaml` (состояние урока 1.9)**

Контракт описывает API **как он есть после урока 1.8** (формат ошибок 1a, `X-User-ID`, `after`/`limit`); следующие уроки меняют его вместе с кодом. Полный текст:

```yaml
# region info
openapi: 3.0.3
info:
  title: Messenger API
  version: 1.0.0
  description: |
    HTTP API учебного мессенджера (курс system design, модуль 1).
    Контракт — источник правды: Go-типы и интерфейсы сервера генерируются из него
    (oapi-codegen), а тесты проверяют каждый ответ сервера по нему (kin-openapi).
    Ограничения полей здесь — документация для клиентов; решает домен.
# endregion info
servers:
  - url: http://127.0.0.1:8080
tags:
  - name: users
    description: Пользователи
  - name: chats
    description: Чаты и сообщения
security:
  - userIdHeader: []
paths:
  /v1/users:
    post:
      tags: [users]
      operationId: registerUser
      summary: Зарегистрировать пользователя
      security: []
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/RegisterUserRequest' }
      responses:
        '201':
          description: Пользователь создан
          headers:
            Location: { $ref: '#/components/headers/Location' }
          content:
            application/json:
              schema: { $ref: '#/components/schemas/User' }
        '400': { $ref: '#/components/responses/BadRequest' }
        '409': { $ref: '#/components/responses/Conflict' }
        '413': { $ref: '#/components/responses/PayloadTooLarge' }
        '415': { $ref: '#/components/responses/UnsupportedMediaType' }
        '500': { $ref: '#/components/responses/InternalError' }
        '503': { $ref: '#/components/responses/ServiceUnavailable' }
  /v1/users/{id}:
    get:
      tags: [users]
      operationId: getUser
      summary: Профиль пользователя
      parameters:
        - $ref: '#/components/parameters/ID'
      responses:
        '200':
          description: Профиль
          content:
            application/json:
              schema: { $ref: '#/components/schemas/User' }
        '400': { $ref: '#/components/responses/BadRequest' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '404': { $ref: '#/components/responses/NotFound' }
        '500': { $ref: '#/components/responses/InternalError' }
        '503': { $ref: '#/components/responses/ServiceUnavailable' }
  /v1/chats:
    post:
      tags: [chats]
      operationId: createChat
      summary: Создать чат
      description: Для пары, у которой личный чат уже есть, возвращает его со статусом 200.
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/CreateChatRequest' }
      responses:
        '201':
          description: Чат создан
          headers:
            Location: { $ref: '#/components/headers/Location' }
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Chat' }
        '200':
          description: Личный чат для этой пары уже был
          headers:
            Location: { $ref: '#/components/headers/Location' }
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Chat' }
        '400': { $ref: '#/components/responses/BadRequest' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '413': { $ref: '#/components/responses/PayloadTooLarge' }
        '415': { $ref: '#/components/responses/UnsupportedMediaType' }
        '500': { $ref: '#/components/responses/InternalError' }
        '503': { $ref: '#/components/responses/ServiceUnavailable' }
  # region messages-paths
  /v1/chats/{id}/messages:
    parameters:
      - $ref: '#/components/parameters/ID'
    post:
      tags: [chats]
      operationId: sendMessage
      summary: Отправить сообщение
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/SendMessageRequest' }
      responses:
        '201':
          description: Сообщение сохранено
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Message' }
        '400': { $ref: '#/components/responses/BadRequest' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '404': { $ref: '#/components/responses/NotFound' }
        '409': { $ref: '#/components/responses/Conflict' }
        '413': { $ref: '#/components/responses/PayloadTooLarge' }
        '415': { $ref: '#/components/responses/UnsupportedMediaType' }
        '500': { $ref: '#/components/responses/InternalError' }
        '503': { $ref: '#/components/responses/ServiceUnavailable' }
    get:
      tags: [chats]
      operationId: listMessages
      summary: История сообщений
      parameters:
        - name: after
          in: query
          description: Вернуть сообщения с seq больше этого.
          schema: { type: integer, format: int64, minimum: 0, default: 0 }
        - $ref: '#/components/parameters/Limit'
      responses:
        '200':
          description: Страница истории
          content:
            application/json:
              schema: { $ref: '#/components/schemas/MessageList' }
        '400': { $ref: '#/components/responses/BadRequest' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '404': { $ref: '#/components/responses/NotFound' }
        '500': { $ref: '#/components/responses/InternalError' }
        '503': { $ref: '#/components/responses/ServiceUnavailable' }
  # endregion messages-paths
components:
  securitySchemes:
    userIdHeader:
      type: apiKey
      in: header
      name: X-User-ID
      description: Временная заглушка до JWT (ADR-0002) — любой может представиться кем угодно.
  parameters:
    ID:
      name: id
      in: path
      required: true
      schema: { type: string, format: uuid }
    Limit:
      name: limit
      in: query
      description: Размер страницы, от 1 до 100.
      schema: { type: integer, minimum: 1, maximum: 100, default: 50 }
  headers:
    Location:
      description: Адрес созданного (или найденного) ресурса.
      schema: { type: string }
  responses:
    BadRequest:
      description: Некорректные данные запроса
      content:
        application/json:
          schema: { $ref: '#/components/schemas/ErrorResponse' }
    Unauthorized:
      description: Не удалось определить пользователя
      content:
        application/json:
          schema: { $ref: '#/components/schemas/ErrorResponse' }
    Forbidden:
      description: Нет доступа
      content:
        application/json:
          schema: { $ref: '#/components/schemas/ErrorResponse' }
    NotFound:
      description: Не найдено
      content:
        application/json:
          schema: { $ref: '#/components/schemas/ErrorResponse' }
    Conflict:
      description: Конфликт с текущим состоянием
      content:
        application/json:
          schema: { $ref: '#/components/schemas/ErrorResponse' }
    PayloadTooLarge:
      description: Тело больше 1 МиБ
      content:
        application/json:
          schema: { $ref: '#/components/schemas/ErrorResponse' }
    UnsupportedMediaType:
      description: Нужен Content-Type application/json
      content:
        application/json:
          schema: { $ref: '#/components/schemas/ErrorResponse' }
    InternalError:
      description: Сбой сервера
      content:
        application/json:
          schema: { $ref: '#/components/schemas/ErrorResponse' }
    ServiceUnavailable:
      description: Сервер не успел ответить
      content:
        application/json:
          schema: { $ref: '#/components/schemas/ErrorResponse' }
  schemas:
    # region schemas-request
    RegisterUserRequest:
      type: object
      additionalProperties: false
      required: [username, display_name]
      properties:
        username: { type: string, minLength: 3, maxLength: 32, pattern: '^[a-z0-9_]+$' }
        display_name: { type: string, minLength: 1, maxLength: 64 }
    CreateChatRequest:
      type: object
      additionalProperties: false
      required: [kind, member_ids]
      properties:
        kind: { type: string, enum: [direct, group] }
        title: { type: string, maxLength: 128, description: Только для группы. }
        member_ids:
          type: array
          description: Участники без создателя; для личного чата — ровно один.
          items: { type: string, format: uuid, x-go-type: string }
    SendMessageRequest:
      type: object
      additionalProperties: false
      required: [text]
      properties:
        text: { type: string, minLength: 1, maxLength: 4096 }
    # endregion schemas-request
    User:
      type: object
      additionalProperties: false
      required: [id, username, display_name, created_at]
      properties:
        id: { type: string, format: uuid, x-go-type: string }
        username: { type: string }
        display_name: { type: string }
        created_at: { type: string, format: date-time }
    Chat:
      type: object
      additionalProperties: false
      required: [id, kind, member_ids, created_at]
      properties:
        id: { type: string, format: uuid, x-go-type: string }
        kind: { type: string, enum: [direct, group] }
        title: { type: string, description: Только у группы. }
        member_ids:
          type: array
          items: { type: string, format: uuid, x-go-type: string }
        created_at: { type: string, format: date-time }
    Message:
      type: object
      additionalProperties: false
      required: [id, chat_id, author_id, text, seq, sent_at]
      properties:
        id: { type: string, format: uuid, x-go-type: string }
        chat_id: { type: string, format: uuid, x-go-type: string }
        author_id: { type: string, format: uuid, x-go-type: string }
        text: { type: string }
        seq: { type: integer, format: int64, minimum: 1 }
        sent_at: { type: string, format: date-time }
    MessageList:
      type: object
      additionalProperties: false
      required: [messages]
      properties:
        messages:
          type: array
          items: { $ref: '#/components/schemas/Message' }
    ErrorResponse:
      type: object
      additionalProperties: false
      required: [error]
      properties:
        error:
          type: object
          additionalProperties: false
          required: [code, message]
          properties:
            code: { type: string }
            message: { type: string }
            field: { type: string }
```

**Почему `x-go-type: string` у id в телах, но не у параметра пути.** Формат id проверяет домен (`Parse…ID`) и называет поле в ошибке так, как оно названо в контракте (`member_ids`). Если бы тип был `openapi_types.UUID`, кривой id в JSON упал бы в `encoding/json` с неинформативной ошибкой. Параметр пути `{id}` проверяет сгенерированная обёртка — ошибка придёт с `ParamName: "id"`, тоже как в контракте (закрывает пункт бэклога «имена полей против `{id}`»). Если `x-go-type` на свойствах не сработает в v2.8.0 — сообщить в отчёте и использовать `x-go-type-skip-optional-pointer`/`x-oapi-codegen-extra-tags` не нужно: достаточно убрать `format: uuid` у свойств тел запросов, оставив `description: UUID` (решение — в отчёт).

- [ ] **Step 3: Генерация**

Для `user` и `chat` (пример — `chat`; у `user` теги `[users]`):

`internal/chat/adapters/httpapi/gen/config.yaml`:

```yaml
# Генерируем только операции своего контекста: контекст chat не должен видеть DTO других.
package: gen
output: api.gen.go
generate:
  std-http-server: true
  models: true
output-options:
  include-tags: [chats]
```

`internal/chat/adapters/httpapi/gen/generate.go`:

```go
// Package gen — код, сгенерированный из api/openapi.yaml. Руками не править:
// поменяйте контракт и выполните go generate ./... в каталоге messenger.
package gen

//go:generate go tool oapi-codegen -config config.yaml ../../../../../api/openapi.yaml
```

Run: `go generate ./... && go build ./...` → сгенерированы `api.gen.go`; посмотреть `ServerInterface` и модели (`CreateChatRequest.MemberIds []string`, `Chat.Title *string`, `ListMessagesParams{After *int64; Limit *int}`), записать сигнатуры в отчёт.

- [ ] **Step 4: Хендлеры на сгенерированном интерфейсе (контракт)**

- `var _ gen.ServerInterface = (*Handler)(nil)` в каждом `handler.go` — компилятор сверяет хендлер с контрактом.
- `func (h *Handler) Register(mux *http.ServeMux, mws ...gen.MiddlewareFunc)` вызывает `gen.HandlerWithOptions(h, gen.StdHTTPServerOptions{BaseRouter: mux, Middlewares: mws, ErrorHandlerFunc: h.paramError})`. Ручные `mux.HandleFunc` удаляются.
- Методы — по `ServerInterface`: `RegisterUser(w, r)`, `GetUser(w, r, id openapi_types.UUID)`, `CreateChat(w, r)`, `SendMessage(w, r, id openapi_types.UUID)`, `ListMessages(w, r, id openapi_types.UUID, params gen.ListMessagesParams)`. Id из пути — `id.String()` в команду use case'а; `params.After`/`params.Limit` — указатели (`nil` → 0, как раньше). Функции `queryInt`/`queryInt64` удаляются (пункт бэклога).
- Тело — по-прежнему `httpjson.Decode(w, r, &req)` в сгенерированный тип (`gen.CreateChatRequest` и т. п.): лимит 1 МиБ, неизвестные поля, один объект, 415.
- `paramError(w, r, err error)`: `errors.As` на `*gen.InvalidParamFormatError`, `*gen.RequiredParamError`, `*gen.TooManyValuesForParamError` → `*domain.ValidationError{Field: e.ParamName, Reason: "неверный формат"}` (для `Required…` — «обязательный параметр») → `writeError`. Прочее → `ValidationError{Field: "", Reason: "некорректные параметры запроса"}`.
- Мапперы в `dto.go`: `toUser(domain.User) gen.User`, `toChat(*domain.Chat) gen.Chat` (`Title` — `nil` у личного), `toMessage(domain.Message) gen.Message`, `toMessageList([]domain.Message) gen.MessageList` (срез через `make`, не `nil`). Время — `t.UTC()` (поле `time.Time`, кодируется в RFC 3339). Регион `dto` сохранить — его цитирует урок 1.7 (обновить текст урока 1.7 одной строкой: «в уроке 1.9 эти DTO заменит генерация»).

- [ ] **Step 5: `openapitest`**

`internal/platform/openapitest/openapitest.go`:

```go
// Package openapitest проверяет ответы обработчиков по контракту api/openapi.yaml.
// Только для тестов: компилятор сверяет с контрактом запросы и сигнатуры, а ответы
// пишем мы сами — их форму ловит только проверка по схеме.
package openapitest

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

type Validator struct {
	t      testing.TB
	doc    *openapi3.T
	router routers.Router

	mu   sync.Mutex // защищает seen: обработчики в тестах бывают параллельными
	seen map[string]bool
}

// New загружает контракт. Путь — от этого файла, а не от рабочего каталога теста:
// тесты разных пакетов запускаются из разных каталогов.
func New(t testing.TB) *Validator {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	spec := filepath.Join(filepath.Dir(file), "..", "..", "..", "api", "openapi.yaml")
	doc, err := openapi3.NewLoader().LoadFromFile(spec)
	if err != nil {
		t.Fatalf("openapitest: загрузка контракта: %v", err)
	}
	if err := doc.Validate(t.Context()); err != nil {
		t.Fatalf("openapitest: контракт некорректен: %v", err)
	}
	// Роутер kin-openapi сверяет и хост с servers; в тестах хост другой.
	doc.Servers = nil
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("openapitest: роутер: %v", err)
	}
	return &Validator{t: t, doc: doc, router: router, seen: map[string]bool{}}
}

// Wrap пропускает запрос через h, проверяет ответ по контракту и отдаёт его дальше как есть.
func (v *Validator) Wrap(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		v.check(r, rec)
		for k, vals := range rec.Header() {
			w.Header()[k] = vals
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
	})
}

func (v *Validator) check(r *http.Request, rec *httptest.ResponseRecorder) {
	v.t.Helper()
	route, params, err := v.router.FindRoute(r)
	if err != nil {
		// Пути нет в контракте: так и должно быть только у ответов маршрутизатора
		// и у отказа в аутентификации, который приходит раньше маршрутизации.
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed && rec.Code != http.StatusUnauthorized {
			v.t.Errorf("openapitest: %s %s нет в контракте, а ответ %d", r.Method, r.URL.Path, rec.Code)
		}
		return
	}
	opts := &openapi3filter.Options{IncludeResponseStatus: true}
	in := &openapi3filter.RequestValidationInput{Request: r, PathParams: params, Route: route, Options: opts}
	out := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: in,
		Status:                 rec.Code,
		Header:                 rec.Header(),
		Body:                   io.NopCloser(bytes.NewReader(rec.Body.Bytes())),
		Options:                opts,
	}
	if err := openapi3filter.ValidateResponse(r.Context(), out); err != nil {
		v.t.Errorf("openapitest: %s %s → %d не по контракту: %v\nтело: %s", r.Method, r.URL.Path, rec.Code, err, rec.Body.String())
	}
	v.mu.Lock()
	v.seen[route.Method+" "+route.Path] = true
	v.mu.Unlock()
}

// Uncovered — операции контракта, ни один ответ которых не прошёл через Wrap.
func (v *Validator) Uncovered() []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	var out []string
	for path, item := range v.doc.Paths.Map() {
		for method := range item.Operations() {
			if key := method + " " + path; !v.seen[key] {
				out = append(out, key)
			}
		}
	}
	slices.Sort(out)
	return out
}
```

Проверить по исходникам kin-openapi v0.149.0 имена (`doc.Paths.Map()`, `item.Operations()`, `IncludeResponseStatus`) — при расхождении поправить и записать в отчёт. В тестах обоих адаптеров сервер — `openapitest.New(t).Wrap(mux)`; тест с намеренно сломанным ответом (временно) должен падать — проверить и вернуть.

- [ ] **Step 6: ADR-0006 и регионы**

`docs/adr/0006-contract-first.md` по шаблону: контекст (ручные DTO 1.7 и описание API расходятся молча; клиенты в модулях 3 и 6), варианты (code-first с аннотациями; контракт + только тест-сверка; контракт + генерация + тест-сверка; контракт как документ), решение (третий; oapi-codegen `std-http-server` без фреймворка, по тегу на контекст; kin-openapi в тестах), последствия (зависимость генерации; сгенерированный код в репозитории и проверка актуальности; ограничения полей в контракте — документация, правила — домен; ответы проверяет только тест).

Регионы в YAML для урока: `info`, `messages-paths`, `schemas-request` (уже в тексте выше). Проверить, что `npm run check:messenger` их находит.

- [ ] **Step 7: Урок `09-contract-first.md`**

Frontmatter: `title: 1.9. Contract-first: OpenAPI и генерация кода`.

**Разделы:**
- **Проблема.** В 1.7 форма API описана трижды: в DTO, в тексте урока, в головах клиентов. Переименовали поле в DTO — клиенты узнали об этом в проде. Мобильная команда в модуле 3 и шлюз в модуле 6 будут писать клиентов по описанию, которому нельзя доверять.
- **Теория:**
  - `code-first` против `contract-first`; что значит «контракт — источник правды»; кто в индустрии как делает (Google — protobuf, Stripe — OpenAPI из своего DSL, Zalando — API-first guidelines);
  - `openapi` 3.0.3: `paths`, операции, `operationId`, `parameters`, `requestBody`, `responses`, `components` (`schemas`, `parameters`, `responses`, `headers`), `$ref`, `tags`, `security`; `json-schema`-подмножество: `type`, `format`, `required`, `additionalProperties`, `enum`, `minimum`/`maximum`, `nullable`;
  - почему 3.0.3, а не 3.1 (поддержка генератора; 3.1 = полная JSON Schema 2020-12);
  - `code-generation` с oapi-codegen: режимы (`std-http-server`, `chi`, `strict-server`, клиент), почему `std-http-server` (тот же `ServeMux`, без фреймворка, хендлеры наши), `include-tags` — граница контекстов в генерации, `go tool` и директива `tool` в `go.mod` (Go 1.24+), `go generate`;
  - что ловит компилятор (имена и типы запроса, параметры, набор методов `ServerInterface`), что — только контрактный тест (форма ответа, статусы, заголовки), что — никто (семантика) — с таблицей;
  - `backward-compatibility` и эволюция схемы (Клеппман, гл. 4): добавление необязательного поля, «читатель игнорирует незнакомое» (закон Постела) и почему `additionalProperties: false` в **наших ответах** — инструмент теста, а не требование к клиентам;
  - ограничения полей в контракте и в домене: дублирование знания? Ответ: контракт документирует, домен решает; при расхождении прав домен, а контракт — ошибка документации (ADR-0006).
- **Визуализация:** `<ContractPipeline />` и задания.
- **Код:** регионы YAML `info`, `messages-paths`, `schemas-request`; `config.yaml` и `generate.go` целиком; фрагмент сгенерированного `ServerInterface` (через `<<<` из `api.gen.go` нельзя — регионов там нет: показать вывод `go doc ./internal/chat/adapters/httpapi/gen ServerInterface`, полученный прогоном); `openapitest.check`; демонстрация: временно поменять тип `seq` в ответе → вывод упавшего теста (реальный); временно переименовать поле в контракте → вывод `go build` (реальный).
- **Под капотом:** «Как kin-openapi проверяет ответ» — по исходникам v0.149.0 (`openapi3filter/validate_response.go`): поиск ответа по статусу (и `default`), выбор медиатипа, декодер тела, `VisitJSON` схемы; почему `IncludeResponseStatus` важен (иначе неописанный статус проходит молча).
- **Ошибки:** править `api.gen.go` руками; один `gen` на все контексты; `format: uuid` везде без мысли о том, кто проверяет; контракт как документ без проверки; генерация в CI без проверки актуальности.
- **Компромисс:** генерация даёт гарантии запроса и скорость клиентам, но добавляет зависимость, сгенерированный код и правила генератора; ручные DTO гибче, но правду о форме API не гарантируют.
- **Самопроверка, источники:** спецификация OpenAPI 3.0.3; README oapi-codegen v2; Zalando RESTful API Guidelines («API First»); Клеппман, гл. 4; Go 1.24 release notes (`tool`).
- **Что изменилось в мессенджере** (`… diff --stat m01-l08 m01-l09 -- messenger`).

**Термины:** `openapi`, `contract-first`, `code-generation`, `json-schema`, `backward-compatibility`, `contract-test`.

- [ ] **Step 8: Проверка, коммит, тег**

Run: `go mod tidy && npm test && npm run typecheck && npm run check:messenger && npm run build` → PASS (`go generate (файлов: 2)`).

```bash
git add messenger course
git commit -m "feat(messenger): контракт OpenAPI, генерация oapi-codegen, проверка ответов, ADR-0006; урок 1.9

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git tag m01-l09
```

---

### Task 6: Урок 1.10 «Ошибки по RFC 9457»

**Files:**
- Create: `messenger/internal/platform/problem/{doc.go,problem.go,respond.go,problem_test.go}`
- Delete: `messenger/internal/platform/httperror/` целиком
- Modify: `messenger/internal/platform/httpjson/httpjson.go` (удалить `WriteError` и `errorBody`/`errorDetail`), его тест
- Modify: оба `httpapi/errors.go`, `caller.go`, `paramError`, оба `handler_test.go`; `cmd/messenger/main.go`
- Modify: `messenger/api/openapi.yaml`; перегенерировать `gen/`
- Create: `course/modules/01-go-service/10-problem-details.md`

**Interfaces:**
- Consumes: задачи 4–5.
- Produces (используют 7–10):
  - `problem.ContentType`, `problem.TypeBase`;
  - `type Code string` и константы `CodeInvalidInput`, `CodeBadJSON`, `CodeUnsupportedMediaType`, `CodeBodyTooLarge`, `CodeUnauthenticated`, `CodeNotMember`, `CodeNotFound`, `CodeMethodNotAllowed`, `CodeUsernameTaken`, `CodeConflict`, `CodeTimeout`, `CodeInternal`; `problem.Codes() []Code`;
  - `type InvalidParam struct{ Name, Reason string }`, `type Problem struct{ Code Code; Detail string; InvalidParams []InvalidParam }`, `problem.Invalid(name, reason string) Problem`;
  - `problem.Write(w, r, p Problem) error`;
  - `type Classifier func(err error) (Problem, bool)`, `problem.Respond(w, r, log *slog.Logger, err error, classify Classifier)`.

Задачи 8–10 добавляют коды в реестр (одна строка в `registry` + константа + значение в `ProblemCode` контракта).

- [ ] **Step 1: Тесты `problem`**

`internal/platform/problem/problem_test.go` (внешний пакет) — полный код главных проверок:

```go
package problem_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"messenger/internal/platform/httpjson"
	"messenger/internal/platform/middleware"
	"messenger/internal/platform/problem"
)

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != problem.ContentType {
		t.Fatalf("Content-Type %q, ждали %q", ct, problem.ContentType)
	}
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestWriteFields(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/chats?x=1", nil)
	var got map[string]any
	h := middleware.RequestID(idFunc("req-7"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := problem.Write(w, r, problem.Invalid("text", "от 1 до 4096 символов")); err != nil {
			t.Fatal(err)
		}
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	got = decode(t, rec)
	want := map[string]any{
		"type": problem.TypeBase + "invalid_input", "title": "Некорректные данные", "status": float64(400),
		"detail": "от 1 до 4096 символов", "instance": "/v1/chats", "code": "invalid_input", "request_id": "req-7",
		"invalid_params": []any{map[string]any{"name": "text", "reason": "от 1 до 4096 символов"}},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("тело\n%v\nждали\n%v", got, want)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("статус %d", rec.Code)
	}
}

type idFunc string

func (f idFunc) NewID() string { return string(f) }

func TestRespondPolicy(t *testing.T) {
	never := func(error) (problem.Problem, bool) { return problem.Problem{}, false }
	secret := errors.New("pq: password authentication failed for host db-1")
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"разбор тела", &httpjson.DecodeError{Status: 413, Code: "body_too_large", Msg: "тело больше 1048576 байт"}, 413, "body_too_large"},
		{"дедлайн", fmt.Errorf("x: %w", context.DeadlineExceeded), 503, "timeout"},
		{"чужой Canceled при живом клиенте", fmt.Errorf("x: %w", context.Canceled), 500, "internal"},
		{"всё прочее", secret, 500, "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			problem.Respond(rec, httptest.NewRequest(http.MethodGet, "/v1/chats", nil), slog.New(slog.DiscardHandler), tt.err, never)
			m := decode(t, rec)
			if rec.Code != tt.status || m["code"] != tt.code {
				t.Fatalf("%d %v, ждали %d %s", rec.Code, m["code"], tt.status, tt.code)
			}
			if strings.Contains(rec.Body.String(), "db-1") {
				t.Fatal("внутренняя ошибка утекла в ответ")
			}
		})
	}
}

func TestRespondSilentWhenClientGone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/chats", nil).WithContext(ctx)
	problem.Respond(rec, r, slog.New(slog.DiscardHandler), fmt.Errorf("x: %w", context.Canceled), func(error) (problem.Problem, bool) { return problem.Problem{}, false })
	if rec.Body.Len() != 0 || rec.Header().Get("Content-Type") != "" {
		t.Fatalf("клиент ушёл — отвечать некому, а записано: %q", rec.Body.String())
	}
}

// Реестр кодов в Go и перечисление ProblemCode в контракте — одно знание в двух местах;
// тест не даёт им разойтись.
func TestCodesMatchContract(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var inContract []string
	for _, v := range doc.Components.Schemas["ProblemCode"].Value.Enum {
		inContract = append(inContract, v.(string))
	}
	var inGo []string
	for _, c := range problem.Codes() {
		inGo = append(inGo, string(c))
	}
	slices.Sort(inContract)
	if !slices.Equal(inGo, inContract) {
		t.Fatalf("коды в Go %v, в контракте %v", inGo, inContract)
	}
}
```

Плюс: `TestWriteUnknownCode` — незарегистрированный код даёт 500 `internal` и ошибку из `Write`; `TestEveryCodeHasStatusAndTitle` — у каждого из `Codes()` статус 400–599 и непустой `title`.

Run: `go test ./internal/platform/problem/` → FAIL.

- [ ] **Step 2: Реализация `problem`**

`doc.go`:

```go
// Package problem — ошибки HTTP API в формате RFC 9457 (application/problem+json):
// единственное место, где ошибка становится ответом. Контексты передают сюда свою
// таблицу «доменная ошибка → проблема» (Classifier); общее — разбор тела, отмена,
// дедлайн и 500 — решается здесь одинаково для всех.
package problem
```

`problem.go`:

```go
package problem

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"

	"messenger/internal/platform/middleware"
)

const (
	ContentType = "application/problem+json"
	// TypeBase — начало URI типа проблемы. Домен .example зарезервирован под примеры
	// (RFC 2606); в продакшене здесь адрес страницы документации об этой ошибке.
	TypeBase = "https://messenger.example/problems/"
)

// Code — стабильный машинный код проблемы: клиент ветвится по нему, а не по тексту.
// Новый код — это изменение контракта: константа, строка в registry и значение в
// ProblemCode в api/openapi.yaml (тест сверяет).
type Code string

const (
	CodeInvalidInput         Code = "invalid_input"
	CodeBadJSON              Code = "bad_json"
	CodeUnsupportedMediaType Code = "unsupported_media_type"
	CodeBodyTooLarge         Code = "body_too_large"
	CodeUnauthenticated      Code = "unauthenticated"
	CodeNotMember            Code = "not_member"
	CodeNotFound             Code = "not_found"
	CodeMethodNotAllowed     Code = "method_not_allowed"
	CodeUsernameTaken        Code = "username_taken"
	CodeConflict             Code = "conflict"
	CodeTimeout              Code = "timeout"
	CodeInternal             Code = "internal"
)

// title по RFC 9457 не меняется от случая к случаю — поэтому он в реестре, а подробности
// конкретного случая идут в detail.
type spec struct {
	status int
	title  string
}

// #region registry
var registry = map[Code]spec{
	CodeInvalidInput:         {http.StatusBadRequest, "Некорректные данные"},
	CodeBadJSON:              {http.StatusBadRequest, "Некорректный JSON"},
	CodeUnsupportedMediaType: {http.StatusUnsupportedMediaType, "Неподдерживаемый формат тела"},
	CodeBodyTooLarge:         {http.StatusRequestEntityTooLarge, "Слишком большое тело запроса"},
	CodeUnauthenticated:      {http.StatusUnauthorized, "Не удалось определить пользователя"},
	CodeNotMember:            {http.StatusForbidden, "Нет доступа к чату"},
	CodeNotFound:             {http.StatusNotFound, "Не найдено"},
	CodeMethodNotAllowed:     {http.StatusMethodNotAllowed, "Метод не поддерживается"},
	CodeUsernameTaken:        {http.StatusConflict, "Имя пользователя занято"},
	CodeConflict:             {http.StatusConflict, "Конфликт изменений"},
	CodeTimeout:              {http.StatusServiceUnavailable, "Сервер не успел ответить"},
	CodeInternal:             {http.StatusInternalServerError, "Внутренняя ошибка сервера"},
}

// #endregion registry

func Codes() []Code { return slices.Sorted(maps.Keys(registry)) }

type InvalidParam struct {
	Name   string
	Reason string
}

type Problem struct {
	Code          Code
	Detail        string
	InvalidParams []InvalidParam
}

// Invalid — проблема с одним неверным полем или параметром; name — имя из контракта.
func Invalid(name, reason string) Problem {
	return Problem{Code: CodeInvalidInput, Detail: reason, InvalidParams: []InvalidParam{{Name: name, Reason: reason}}}
}

// #region wire
// wire — форма на проводе: поля RFC 9457 и наши расширения (code, request_id,
// invalid_params). Расширения RFC разрешает; клиенты обязаны игнорировать незнакомые.
type wire struct {
	Type          string      `json:"type"`
	Title         string      `json:"title"`
	Status        int         `json:"status"`
	Detail        string      `json:"detail,omitempty"`
	Instance      string      `json:"instance"`
	Code          Code        `json:"code"`
	RequestID     string      `json:"request_id,omitempty"`
	InvalidParams []wireParam `json:"invalid_params,omitempty"`
}

type wireParam struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// #endregion wire

// Write пишет проблему. Незарегистрированный код — ошибка программиста: клиент получит
// честный 500, а вызывающий — ошибку для журнала.
func Write(w http.ResponseWriter, r *http.Request, p Problem) error {
	s, ok := registry[p.Code]
	var unknown error
	if !ok {
		unknown = fmt.Errorf("problem: незарегистрированный код %q", p.Code)
		p, s = Problem{Code: CodeInternal}, registry[CodeInternal]
	}
	body := wire{
		Type: TypeBase + string(p.Code), Title: s.title, Status: s.status, Detail: p.Detail,
		Instance: r.URL.Path, Code: p.Code, RequestID: middleware.RequestIDFrom(r.Context()),
	}
	for _, ip := range p.InvalidParams {
		body.InvalidParams = append(body.InvalidParams, wireParam(ip))
	}
	w.Header().Set("Content-Type", ContentType)
	w.WriteHeader(s.status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		return fmt.Errorf("problem: запись ответа: %w", err)
	}
	return unknown
}
```

`respond.go`:

```go
package problem

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"messenger/internal/platform/httpjson"
	"messenger/internal/platform/middleware"
)

// Classifier — таблица контекста: узнаёт его ошибки и говорит, какой проблемой они
// становятся. ok == false — «это не моя ошибка».
type Classifier func(err error) (p Problem, ok bool)

// #region respond
// Respond — единая политика: разбор тела, таблица контекста, контекст запроса,
// и всё неузнанное — 500 без деталей наружу (детали — только в журнал).
func Respond(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error, classify Classifier) {
	var decodeErr *httpjson.DecodeError
	if errors.As(err, &decodeErr) {
		p := Problem{Code: Code(decodeErr.Code), Detail: decodeErr.Msg}
		if decodeErr.Field != "" {
			p.InvalidParams = []InvalidParam{{Name: decodeErr.Field, Reason: decodeErr.Msg}}
		}
		write(w, r, log, p)
		return
	}
	if p, ok := classify(err); ok {
		write(w, r, log, p)
		return
	}
	switch {
	case errors.Is(err, context.Canceled) && r.Context().Err() != nil:
		// Клиент ушёл: отвечать некому. Проверяем именно контекст запроса — Canceled
		// от внутреннего контекста при живом клиенте это сбой сервера (иначе пустой 200).
		log.Info("клиент ушёл, не дождавшись ответа", "method", r.Method, "path", r.URL.Path, "err", err)
	case errors.Is(err, context.DeadlineExceeded):
		write(w, r, log, Problem{Code: CodeTimeout, Detail: "повторите запрос позже"})
	default:
		log.Error("внутренняя ошибка", "method", r.Method, "path", r.URL.Path, "err", err, "request_id", middleware.RequestIDFrom(r.Context()))
		write(w, r, log, Problem{Code: CodeInternal})
	}
}

// #endregion respond

func write(w http.ResponseWriter, r *http.Request, log *slog.Logger, p Problem) {
	if err := Write(w, r, p); err != nil {
		log.Error("запись ответа", "path", r.URL.Path, "err", err)
	}
}
```

Run: `go test -race ./internal/platform/problem/` → FAIL только на `TestCodesMatchContract` (контракта с `ProblemCode` ещё нет) — это ожидаемо до шага 3.

- [ ] **Step 3: Контракт (изменения)**

- `components.schemas`: удалить `ErrorResponse`; добавить:

```yaml
    # region problem-schema
    Problem:
      type: object
      description: Ошибка в формате RFC 9457 с расширениями code, request_id, invalid_params.
      required: [type, title, status, instance, code]
      additionalProperties: false
      properties:
        type: { type: string, format: uri }
        title: { type: string }
        status: { type: integer, minimum: 400, maximum: 599 }
        detail: { type: string }
        instance: { type: string }
        code: { $ref: '#/components/schemas/ProblemCode' }
        request_id: { type: string }
        invalid_params:
          type: array
          items: { $ref: '#/components/schemas/InvalidParam' }
    InvalidParam:
      type: object
      additionalProperties: false
      required: [name, reason]
      properties:
        name: { type: string }
        reason: { type: string }
    ProblemCode:
      type: string
      enum: [bad_json, body_too_large, conflict, internal, invalid_input, method_not_allowed,
             not_found, not_member, timeout, unauthenticated, unsupported_media_type, username_taken]
    # endregion problem-schema
```

- все `components.responses.*`: `content` → `application/problem+json`, схема `Problem`.
- Если kin-openapi v0.149.0 не декодирует тело `application/problem+json` (ошибка «unsupported content type») — в `openapitest.New` зарегистрировать: `openapi3filter.RegisterBodyDecoder(problem.ContentType, openapi3filter.JSONBodyDecoder)` (без импорта `problem` — строкой, через `sync.Once`); записать в отчёт, понадобилось ли.

`go generate ./...` (модели изменятся: `ErrorResponse` уйдёт). Run: `go test ./internal/platform/problem/` → PASS.

- [ ] **Step 4: Перевести всех на `problem` (контракт)**

- Оба `httpapi/errors.go`: `writeError` → `problem.Respond(w, r, log, err, problemFor)`; `problemFor` возвращает `problem.Problem`: `*domain.ValidationError` → `problem.Invalid(v.Field, v.Reason)`; `ErrChatNotFound`/`ErrUserNotFound` → `CodeNotFound`; `ErrNotMember` → `CodeNotMember`; `ErrConcurrentUpdate` → `CodeConflict`; `ErrUsernameTaken` → `CodeUsernameTaken`; `Detail` — текст сигнальной ошибки (он для людей и безопасен). Регион `error-map` сохранить.
- `caller.go` → `problem.Write(w, r, problem.Problem{Code: problem.CodeUnauthenticated, Detail: "не удалось определить пользователя"})`.
- `paramError` → `problem.Invalid(e.ParamName, …)` через `writeError` (поведение то же).
- `main.go`: `routeError` → `CodeNotFound` «маршрут не найден» / `CodeMethodNotAllowed`; `internalError` → `CodeInternal`; `unauthenticated` → `CodeUnauthenticated`.
- `platform/httperror` удалить; в `httpjson` удалить `WriteError` и типы тела ошибки (+ их тесты); `go build ./...` не должен найти ни одного использования.
- Тесты хендлеров: разбор ответа как problem (`code`, `invalid_params[0].name`, `Content-Type`); отдельный тест «живой клиент, внутренний `Canceled`» — 500 (закрывает пункт бэклога про `Canceled`); контрактная обёртка `openapitest` по-прежнему вокруг mux.

Run: `go build ./... && go test -race ./... && golangci-lint run ./...` → PASS. `grep -rn "httperror\|WriteError" internal cmd` → пусто.

- [ ] **Step 5: Прогон через `curl`**

Показать (реальный вывод в отчёт): 400 с `invalid_params` (пустой текст), 400 для `limit=abc` (параметр из обёртки), 401, 403 (`not_member`), 404 (неизвестный чат и неизвестный маршрут), 405 с `Allow`, 413 (тело 2 МиБ через `head -c`), 415 (`curl -d` без JSON-заголовка); у всех `Content-Type: application/problem+json`, `type`, `request_id` — тот же, что в заголовке `X-Request-ID` и в строке access-log.

- [ ] **Step 6: Урок `10-problem-details.md`**

Frontmatter: `title: 1.10. Ошибки по RFC 9457`.

**Разделы:**
- **Проблема.** Каждый API изобретает свой формат ошибок: у нас `{"error":{…}}`, у соседей `{"message"}`, у третьих — строка. Клиент пишет парсер под каждый; поддержка не может найти запрос по жалобе; маршрутизатор отвечал текстом. Плюс опасность: хочется «помочь» клиенту текстом внутренней ошибки.
- **Теория:**
  - RFC 9457 (заменил RFC 7807): поля `type` (URI — идентификатор, не обязан открываться; почему абсолютный; `about:blank`), `title` (не меняется от случая к случаю), `status` (дублирует статус ответа — зачем), `detail` (для человека, про этот случай), `instance`; расширения и правило «клиент игнорирует незнакомое»;
  - наши расширения: `code` (почему нужен, хотя есть `type`: удобство `switch` у клиента), `request_id` (связка с журналом — урок 1.8), `invalid_params` (пример из самого RFC);
  - реестр проблем как часть контракта; почему коды — `enum` в контракте и почему тест сверяет реестр Go с контрактом (DRY знания между кодом и описанием);
  - что нельзя в `detail`: внутренности, SQL, пути файлов, стек, существование чужих ресурсов (связь с 6.7 и OWASP API3/API8);
  - единая политика ошибок: ошибки разбора тела, параметров (из сгенерированной обёртки), маршрутизатора, паники, домена, отмены — все в одном формате; таблица с 6 строками;
  - 503 против 504 для нашего дедлайна; почему у ушедшего клиента нет ответа.
- **Визуализация:** `<ProblemMapping />` и задания.
- **Код:** регионы `registry`, `wire`, `respond`, `error-map` (chat), YAML `problem-schema`; сценарий `curl` с реальным выводом.
- **Под капотом:** «Медиатипы с суффиксом `+json`» — RFC 6839 (structured syntax suffix), как `mime.ParseMediaType` и клиенты узнают JSON по суффиксу; почему `application/problem+json`, а не `application/json` (клиент может отличить ошибку, не читая статус; контент-негоциация через `Accept`).
- **Ошибки:** 200 с ошибкой в теле; `detail` с текстом `err.Error()` из БД; `title`, меняющийся от случая к случаю; ветвление клиента по тексту; разные форматы для 404 маршрутизатора и 404 домена.
- **Компромисс:** стандартный формат против привычного своего — небольшой труд миграции против бесплатной поддержки в библиотеках и шлюзах (Spring 6, ASP.NET Core, многие API-шлюзы).
- **Самопроверка, источники:** RFC 9457; RFC 6839; Zalando API Guidelines (Problem JSON); OWASP API Security Top 10 2023 (API8 — Security Misconfiguration, подробные ошибки); Microsoft REST API Guidelines (errors).
- **Что изменилось в мессенджере** (`… m01-l09 m01-l10`).

**Термины:** `problem-details`, `content-type`.

- [ ] **Step 7: Проверка, коммит, тег**

Run: `npm test && npm run typecheck && npm run check:messenger && npm run build` → PASS.

```bash
git add -A messenger course
git commit -m "feat(messenger): ошибки по RFC 9457 (platform/problem) вместо httperror; урок 1.10

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git tag m01-l10
```

---

### Task 7: Урок 1.11 «Пагинация: offset, keyset, курсор»

**Files:**
- Modify: `messenger/internal/chat/domain/{chat.go,chat_test.go}` (`LastActivityAt`)
- Modify: `messenger/internal/chat/usecase/{ports.go,list_messages.go,usecase_test.go}`; Create: `messenger/internal/chat/usecase/{pagination.go,list_chats.go}`
- Modify: `messenger/internal/chat/adapters/memory/store.go`, `messenger/internal/chat/usecase/storetest/storetest.go`
- Create: `messenger/internal/platform/cursor/{cursor.go,cursor_test.go}`
- Modify: `messenger/api/openapi.yaml`, `chat/adapters/httpapi/{handler.go,dto.go,handler_test.go}`, `gen/`; Create: `chat/adapters/httpapi/cursors.go`
- Modify: `messenger/cmd/messenger/main.go`
- Create: `course/modules/01-go-service/11-pagination.md`

**Interfaces:**
- Consumes: задачи 4–6.
- Produces:
  - `(*domain.Chat).LastActivityAt() time.Time`, поле `ChatSnapshot.LastActivityAt`;
  - порт `MessageLister.ListMessagesBefore(ctx, chatID, beforeSeq int64, limit int) ([]domain.Message, error)` (вместо `ListMessages`);
  - `usecase.ChatPosition{LastActivityAt time.Time; ID domain.ChatID}`, порт `ChatLister.ListChatsByMember(ctx, member domain.UserID, after *ChatPosition, limit int) ([]*domain.Chat, error)`;
  - `ListMessagesQuery{ChatID, ReaderID string; BeforeSeq int64; Limit int}` → `MessagePage{Messages []domain.Message; NextBeforeSeq int64}`;
  - `NewListChats(chats ChatLister) (*ListChats, error)`, `ListChatsQuery{MemberID string; After *ChatPosition; Limit int}` → `ChatPage{Chats []*domain.Chat; Next *ChatPosition}`;
  - `cursor.Encode(kind string, payload any) (string, error)`, `cursor.Decode(s, kind string, payload any) error`, `cursor.ErrInvalid`, `cursor.MaxLen`.

- [ ] **Step 1: Домен — `LastActivityAt`**

Тест в `chat_test.go`:

```go
func TestPostMovesLastActivityForwardOnly(t *testing.T) {
	c := newDirect(t) // существующий помощник теста: личный чат, созданный в t0
	if !c.LastActivityAt().Equal(t0) {
		t.Fatalf("у нового чата активность %v, ждали время создания %v", c.LastActivityAt(), t0)
	}
	post := func(at time.Time) {
		t.Helper()
		if _, err := c.Post(mustMessageID(t, 1), c.Members()[0], mustText(t, "привет"), at); err != nil {
			t.Fatal(err)
		}
	}
	post(t0.Add(time.Minute))
	post(t0.Add(30 * time.Second)) // часы другого сервера отстают — назад не идём
	if want := t0.Add(time.Minute); !c.LastActivityAt().Equal(want) {
		t.Fatalf("активность %v, ждали %v", c.LastActivityAt(), want)
	}
	if got := domain.RestoreChat(c.Snapshot()).LastActivityAt(); !got.Equal(c.LastActivityAt()) {
		t.Fatalf("снимок потерял активность: %v", got)
	}
}
```

(Имена помощников — те, что уже есть в `chat_test.go`; если их нет — добавить рядом, по образцу существующих.)

Изменения в `chat.go`:
- поле `lastActivityAt time.Time // время последнего сообщения, до первого — время создания; ключ сортировки списка чатов` в регионе `chat`;
- `NewDirectChat` и `NewGroupChat` ставят `lastActivityAt: createdAt.UTC()`;
- в `Post` после проверок:

```go
	sentAt = sentAt.UTC()
	c.lastSeq++
	c.version++
	// Активность не идёт назад: часы разных экземпляров сервиса расходятся, и сообщение
	// с чуть меньшим временем не должно опускать чат в списке.
	if sentAt.After(c.lastActivityAt) {
		c.lastActivityAt = sentAt
	}
	return Message{id: id, chatID: c.id, authorID: author, text: text, seq: c.lastSeq, sentAt: sentAt}, nil
```

- `func (c *Chat) LastActivityAt() time.Time { return c.lastActivityAt }`;
- `ChatSnapshot.LastActivityAt`, копирование в `Snapshot` и `RestoreChat`.

Run: `go test ./internal/chat/domain/` → PASS.

- [ ] **Step 2: Порты и use case'ы**

`ports.go` — заменить `MessageLister`, добавить `ChatPosition` и `ChatLister` (в регионе `ports`):

```go
type MessageLister interface {
	// ListMessagesBefore возвращает до limit сообщений с seq < beforeSeq по убыванию seq;
	// beforeSeq == 0 — начиная с самого нового. На limit < 1 — пустая страница.
	ListMessagesBefore(ctx context.Context, chatID domain.ChatID, beforeSeq int64, limit int) ([]domain.Message, error)
}

// ChatPosition — место чата в списке «по последней активности», ключ keyset-пагинации.
// Время одно на многих — поэтому в ключе ещё и id: порядок должен быть строгим, иначе
// чаты с одинаковым временем на границе страницы потеряются или повторятся.
type ChatPosition struct {
	LastActivityAt time.Time
	ID             domain.ChatID
}

type ChatLister interface {
	// ListChatsByMember — чаты, где member участник, по убыванию (LastActivityAt, ID),
	// строго после after (nil — с начала), не больше limit. На limit < 1 — пустая страница.
	ListChatsByMember(ctx context.Context, member domain.UserID, after *ChatPosition, limit int) ([]*domain.Chat, error)
}
```

`pagination.go`:

```go
package usecase

import (
	"fmt"

	"messenger/internal/chat/domain"
)

// pageLimit — одно правило размера страницы для всех списков: 0 — значение по умолчанию.
func pageLimit(n int) (int, error) {
	if n == 0 {
		return domain.ListLimitDefault, nil
	}
	if n < 1 || n > domain.ListLimitMax {
		return 0, &domain.ValidationError{Field: "limit", Reason: fmt.Sprintf("от 1 до %d", domain.ListLimitMax)}
	}
	return n, nil
}
```

`list_messages.go` — полностью:

```go
package usecase

import (
	"context"
	"errors"
	"fmt"

	"messenger/internal/chat/domain"
)

type ListMessagesQuery struct {
	ChatID    string
	ReaderID  string
	BeforeSeq int64 // 0 — с самого нового
	Limit     int   // 0 — domain.ListLimitDefault
}

// MessagePage — страница истории от новых к старым. NextBeforeSeq == 0 — дальше ничего нет;
// иначе это ключ следующей страницы (адаптер завернёт его в курсор).
type MessagePage struct {
	Messages      []domain.Message
	NextBeforeSeq int64
}

type ListMessages struct {
	chats    ChatFinder
	messages MessageLister
}

func NewListMessages(chats ChatFinder, messages MessageLister) (*ListMessages, error) {
	if chats == nil || messages == nil {
		return nil, errors.New("list messages: nil-зависимость")
	}
	return &ListMessages{chats: chats, messages: messages}, nil
}

// #region list-messages
func (uc *ListMessages) Execute(ctx context.Context, q ListMessagesQuery) (MessagePage, error) {
	chatID, err := domain.ParseChatID(q.ChatID)
	if err != nil {
		return MessagePage{}, err
	}
	reader, err := domain.ParseUserID(q.ReaderID)
	if err != nil {
		return MessagePage{}, err
	}
	if q.BeforeSeq < 0 {
		// Отрицательный ключ мог прийти только из испорченного курсора.
		return MessagePage{}, &domain.ValidationError{Field: "cursor", Reason: "некорректная позиция"}
	}
	limit, err := pageLimit(q.Limit)
	if err != nil {
		return MessagePage{}, err
	}
	chat, err := uc.chats.FindChat(ctx, chatID)
	if err != nil {
		return MessagePage{}, fmt.Errorf("list messages: %w", err)
	}
	if !chat.IsMember(reader) {
		return MessagePage{}, domain.ErrNotMember
	}
	// На одно больше, чем просили: так узнаём, есть ли следующая страница, без отдельного
	// запроса COUNT и без лишнего пустого запроса клиента в конце ленты.
	msgs, err := uc.messages.ListMessagesBefore(ctx, chatID, q.BeforeSeq, limit+1)
	if err != nil {
		return MessagePage{}, fmt.Errorf("list messages: %w", err)
	}
	if len(msgs) <= limit {
		return MessagePage{Messages: msgs}, nil
	}
	msgs = msgs[:limit]
	return MessagePage{Messages: msgs, NextBeforeSeq: msgs[limit-1].Seq()}, nil
}

// #endregion list-messages
```

`list_chats.go` — полностью:

```go
package usecase

import (
	"context"
	"errors"
	"fmt"

	"messenger/internal/chat/domain"
)

type ListChatsQuery struct {
	MemberID string
	After    *ChatPosition // nil — с начала списка
	Limit    int
}

type ChatPage struct {
	Chats []*domain.Chat
	Next  *ChatPosition // nil — дальше ничего нет
}

type ListChats struct {
	chats ChatLister
}

func NewListChats(chats ChatLister) (*ListChats, error) {
	if chats == nil {
		return nil, errors.New("list chats: nil-зависимость")
	}
	return &ListChats{chats: chats}, nil
}

func (uc *ListChats) Execute(ctx context.Context, q ListChatsQuery) (ChatPage, error) {
	member, err := domain.ParseUserID(q.MemberID)
	if err != nil {
		return ChatPage{}, err
	}
	limit, err := pageLimit(q.Limit)
	if err != nil {
		return ChatPage{}, err
	}
	chats, err := uc.chats.ListChatsByMember(ctx, member, q.After, limit+1)
	if err != nil {
		return ChatPage{}, fmt.Errorf("list chats: %w", err)
	}
	if len(chats) <= limit {
		return ChatPage{Chats: chats}, nil
	}
	chats = chats[:limit]
	last := chats[limit-1]
	return ChatPage{Chats: chats, Next: &ChatPosition{LastActivityAt: last.LastActivityAt(), ID: last.ID()}}, nil
}
```

Тесты в `usecase_test.go`: `TestSendAndList` и `TestListLimits` переписать под `BeforeSeq`/`MessagePage` (поле ошибки для отрицательной позиции — `cursor`); в `env` добавить `chats *usecase.ListChats`; новые — полный код главного:

```go
// Листаем историю, пока в чат пишут: курсор не даёт ни дублей, ни пропусков среди тех
// сообщений, что были до начала листания.
func TestListMessagesStableUnderInserts(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	c := directChat(t, e)
	ctx := context.Background()
	send := func(n int) {
		t.Helper()
		if _, err := e.send.Execute(ctx, usecase.SendMessageCommand{ChatID: c.ID().String(), AuthorID: alice, Text: fmt.Sprintf("сообщение %d", n)}); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= 12; i++ {
		send(i)
	}
	var seen []int64
	before := int64(0)
	for page := 0; ; page++ {
		res, err := e.list.Execute(ctx, usecase.ListMessagesQuery{ChatID: c.ID().String(), ReaderID: bob, BeforeSeq: before, Limit: 5})
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range res.Messages {
			seen = append(seen, m.Seq())
		}
		if page == 0 {
			for i := 13; i <= 15; i++ { // новые сообщения между первой и второй страницей
				send(i)
			}
		}
		if res.NextBeforeSeq == 0 {
			break
		}
		before = res.NextBeforeSeq
	}
	want := []int64{12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	if fmt.Sprint(seen) != fmt.Sprint(want) {
		t.Fatalf("прочитали %v, ждали %v", seen, want)
	}
}
```

Плюс `TestListMessagesLastPageHasNoCursor` (ровно `limit` сообщений → `NextBeforeSeq == 0`; `limit+1` → не 0), `TestListChatsPaging` (три чата Алисы с разной активностью — порядок по убыванию, курсор следующей страницы, чат Евы и Боба не виден Алисе), `TestListChatsInvalid` (кривой `MemberID` → `user_id`; `Limit` за границей → `limit`), `TestListChatsRejectsNil`.

`fixedClock` в тестах возвращает одно время на всё — для порядка по активности в `TestListChatsPaging` нужен фейк часов, который можно двигать (`steppingClock` с `atomic.Int64` секунд): добавить в `fakes_test.go`.

- [ ] **Step 3: Хранилище и контракт хранилища**

`memory/store.go` — заменить `ListMessages` и добавить `ListChatsByMember`:

```go
// #region list-before
func (s *Store) ListMessagesBefore(ctx context.Context, chatID domain.ChatID, beforeSeq int64, limit int) ([]domain.Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("memory: %w", err)
	}
	if limit < 1 {
		return []domain.Message{}, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	all := s.messages[chatID] // по возрастанию seq
	to := len(all)
	if beforeSeq > 0 {
		to = sort.Search(len(all), func(i int) bool { return all[i].Seq() >= beforeSeq })
	}
	from := max(0, to-limit)
	page := make([]domain.Message, 0, to-from)
	for i := to - 1; i >= from; i-- {
		page = append(page, all[i])
	}
	return page, nil
}

// #endregion list-before

// ListChatsByMember перебирает все чаты: в памяти их немного. В Postgres (модуль 2a) это
// индекс по участнику и (last_activity_at, id) и запрос «WHERE (at, id) < ($1, $2)».
func (s *Store) ListChatsByMember(ctx context.Context, member domain.UserID, after *usecase.ChatPosition, limit int) ([]*domain.Chat, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("memory: %w", err)
	}
	if limit < 1 {
		return []*domain.Chat{}, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var found []*domain.Chat
	for _, snap := range s.chats {
		c := domain.RestoreChat(snap)
		if !c.IsMember(member) {
			continue
		}
		if after != nil && comparePosition(position(c), *after) >= 0 {
			continue
		}
		found = append(found, c)
	}
	slices.SortFunc(found, func(a, b *domain.Chat) int { return comparePosition(position(b), position(a)) })
	if len(found) > limit {
		found = found[:limit]
	}
	return found, nil
}

func position(c *domain.Chat) usecase.ChatPosition {
	return usecase.ChatPosition{LastActivityAt: c.LastActivityAt(), ID: c.ID()}
}

// comparePosition — порядок «по возрастанию»: сначала время, при равном — id.
// UUIDv7 растёт со временем, поэтому сравнение строк id тоже осмысленно.
func comparePosition(a, b usecase.ChatPosition) int {
	if c := a.LastActivityAt.Compare(b.LastActivityAt); c != 0 {
		return c
	}
	return strings.Compare(a.ID.String(), b.ID.String())
}
```

Импорт `usecase` в адаптере хранилища — нормально (адаптер реализует порт и знает его типы).

`storetest/storetest.go`:
- в `Store` добавить `usecase.ChatLister`;
- `seqs` — через `ListMessagesBefore(ctx, id, 0, domain.ListLimitMax)` и разворот в возрастающий порядок;
- `testListPaging` заменить на `testListMessagesBefore` (таблица: `before=0, limit=2` → `[5 4]`; `before=4, limit=2` → `[3 2]`; `before=2, limit=10` → `[1]`; `before=1` → пусто; `before=100, limit=2` → `[5 4]` (ключ больше любого — с самого нового); `limit=0` и `limit=-1` → пусто);
- `testListReturnsCopy` — под новый метод;
- новые: `testListChatsByMember` (только чаты участника; порядок по убыванию активности — активность двигать сообщениями с разным временем; `after` отсекает строго), `testListChatsTieBreak` (три чата, созданные в один момент `t0` без сообщений, → порядок по убыванию id; страница по 2 + курсор на последнем → третий чат, без повторов), `testListChatsLimit` (`limit < 1` → пусто, не ошибка), и `ctx`-проверки для обоих новых методов в `testCanceledContext`.

Run: `go test -race ./internal/chat/...` → PASS.

- [ ] **Step 4: `platform/cursor`**

`cursor.go`:

```go
// Package cursor кодирует курсоры пагинации: непрозрачная для клиента строка, в которой
// сервер хранит, откуда продолжить. Непрозрачность — это обещание: формат внутри можно
// менять, и клиент, который его не разбирает, не сломается.
package cursor

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	// version — версия формата: курсор живёт у клиента дольше, чем версия сервера,
	// и старый курсор после смены формата должен дать понятную ошибку, а не мусор.
	version = 1
	// MaxLen ограничивает разбор: курсор приходит от клиента, и строка в мегабайт
	// не должна доходить до base64 и JSON.
	MaxLen = 512
)

var ErrInvalid = errors.New("некорректный курсор")

type envelope struct {
	V int             `json:"v"`
	K string          `json:"k"`
	P json.RawMessage `json:"p"`
}

// Encode упаковывает payload вида kind ("messages", "chats"): курсор одного списка
// не примется другим.
func Encode(kind string, payload any) (string, error) {
	p, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("cursor: %w", err)
	}
	b, err := json.Marshal(envelope{V: version, K: kind, P: p})
	if err != nil {
		return "", fmt.Errorf("cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Decode — обратное Encode. Любая неудача — ErrInvalid (с причиной для журнала):
// клиенту хватает «курсор не годится, начните сначала».
func Decode(s, kind string, payload any) error {
	if s == "" || len(s) > MaxLen {
		return fmt.Errorf("%w: длина %d", ErrInvalid, len(s))
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return fmt.Errorf("%w: base64: %w", ErrInvalid, err)
	}
	var env envelope
	if err := strictUnmarshal(raw, &env); err != nil {
		return fmt.Errorf("%w: конверт: %w", ErrInvalid, err)
	}
	if env.V != version || env.K != kind {
		return fmt.Errorf("%w: версия %d, вид %q", ErrInvalid, env.V, env.K)
	}
	if err := strictUnmarshal(env.P, payload); err != nil {
		return fmt.Errorf("%w: данные: %w", ErrInvalid, err)
	}
	return nil
}

func strictUnmarshal(b []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
```

`cursor_test.go` (таблица): туда-обратно; чужой `kind`; версия 2 (собрать конверт руками); не base64; base64 не-JSON; лишнее поле в данных; пустая строка; длина `MaxLen+1` — все дают `errors.Is(err, cursor.ErrInvalid)`.

- [ ] **Step 5: Контракт и хендлеры (контракт)**

- `api/openapi.yaml`:
  - схема `Chat`: обязательное `last_activity_at` (`date-time`);
  - новый параметр `components.parameters.Cursor` (`in: query`, `name: cursor`, `schema: {type: string, maxLength: 512}`, описание «непрозрачная строка из next_cursor; не разбирайте её»);
  - `listMessages`: параметры `cursor`, `limit` (параметр `after` удалить); ответ `200` → схема `MessagePage {items: Message[], next_cursor: string, nullable}` — оба поля обязательные (`next_cursor: null` на последней странице — явно, а не отсутствием поля);
  - новая операция `GET /v1/chats` (`operationId: listChats`, тег `chats`): параметры `cursor`, `limit`; `200` → `ChatPage {items: Chat[], next_cursor}`; ошибки `400`, `401`, `500`, `503`;
  - регион `pagination` вокруг `MessagePage`, `ChatPage`, `Cursor`.
- `go generate ./...`.
- `cursors.go` в `chat/adapters/httpapi` (регион `cursors`): типы данных курсоров `messagesCursor{Chat string "chat"; Seq int64 "seq"}` (вид `"messages"`) и `chatsCursor{At time.Time "at"; ID string "id"}` (вид `"chats"`); функции `encodeMessagesCursor(chatID domain.ChatID, beforeSeq int64) (*string, error)` (0 → `nil`), `decodeMessagesCursor(raw *string, chatID domain.ChatID) (int64, error)` (`nil` → 0; ошибка `cursor.ErrInvalid`, чужой чат или `seq < 1` → `&domain.ValidationError{Field: "cursor", Reason: "курсор не годится: начните с первой страницы"}`) и пара для чатов (`ParseChatID` для `ID`; ошибка → то же). Курсор истории привязан к чату: курсор чата A, предъявленный для чата B, — 400.
- `ListMessages(w, r, id, params)` → `MessagePage` с `next_cursor`; `ListChats(w, r, params)` → `ChatPage`. Мапперы — `toMessagePage`, `toChatPage` (`items` через `make`). Узкий порт `chatLister` на use case и аргумент `New`.
- `main.go`: `chatusecase.NewListChats(chats)`, передать в `chathttp.New`.
- Тесты хендлеров: курсор с первой страницы открывает вторую; курсор чата A для чата B → 400 `invalid_params[0].name == "cursor"`; мусорный курсор → 400; `next_cursor` — `null` на последней странице (контрактная обёртка это и проверяет); `GET /v1/chats` — порядок и курсор.

Run: `go build ./... && go test -race ./... && golangci-lint run ./...` → PASS.

- [ ] **Step 6: Прогон через `curl`**

Алиса и Боб, 12 сообщений циклом, `GET …/messages?limit=5` → 5 сообщений и `next_cursor`; показать, что внутри (`echo $CUR | basenc --base64url -d` с дописыванием `=`, или `base64 -d` после замены символов); три новых сообщения; вторая страница по курсору — без дублей; курсор для другого чата → 400. `GET /v1/chats` с тремя чатами и `limit=2`. Реальный вывод — в отчёт.

- [ ] **Step 7: Урок `11-pagination.md`**

Frontmatter: `title: 1.11. Пагинация: offset, keyset и курсор`.

**Разделы:**
- **Проблема.** `GET /messages?after=…&limit=…` из 1a: клиент сам считает `seq`, историю показывает «от старых к новым», а мессенджер открывается на последних сообщениях; списка чатов нет вовсе. Наивное решение — `?page=3&size=50` — и разбор, почему оно ломается.
- **Теория:**
  - `offset-pagination`: `LIMIT/OFFSET`, почему БД читает и выбрасывает `OFFSET` строк (стоимость растёт с номером страницы — оценка на примере 1 млн сообщений; подробно — 2a), `page-drift` при вставках и удалениях — разбор на песочнице;
  - `keyset-pagination`: «после последнего ключа», индекс по ключу, стоимость не зависит от глубины; цена — нельзя прыгнуть на страницу 37 и нельзя дёшево посчитать «всего страниц» (и почему мессенджеру это не нужно);
  - стабильный строгий порядок: ключ из нескольких полей `(last_activity_at, id)`, сравнение кортежей; что случится без `id` (разобрать на трёх чатах с одним временем);
  - `cursor`: непрозрачность как контракт, версия формата, привязка к ресурсу и виду списка, лимит длины; подпись HMAC — когда нужна (курсор несёт то, что клиенту нельзя видеть или менять: фильтры прав доступа, внутренние id) и почему нам не нужна (подделка даёт лишь другую позицию, которую мы проверяем);
  - `limit+1` против `COUNT(*)`;
  - честное ограничение списка чатов: чат, который «всплыл» новым сообщением во время листания, может встретиться дважды или выпасть вниз уже прочитанной части — и почему это приемлемо (Slack, Telegram — так же; новые события придут доставкой в реальном времени, модуль 3);
  - направление «от новых к старым», и как клиент докачивает новые сообщения (не курсором назад, а доставкой — модуль 3).
- **Визуализация:** `<PaginationSandbox />` и 4 задания (получить дубль offset-ом; получить пропуск; повторить с курсором; найти в курсоре seq).
- **Код:** регионы `list-messages`, `list-before`, `cursors`, `cursor.go` целиком (регион `codec`), YAML `pagination`; сценарий `curl`.
- **Под капотом:** «Почему OFFSET дорогой» — как B-дерево индекса отвечает на «первые N строк после ключа» (спуск по дереву, затем чтение листа) и почему на «пропусти N» ему приходится пройти N записей (без счётчиков в узлах); ссылка вперёд на 2a (`EXPLAIN ANALYZE` на настоящем Postgres), Петров «Database Internals», гл. 2–4.
- **Ошибки:** курсор = номер страницы в base64; ключ без тай-брейкера; курсор, который клиент собирает сам; `next_cursor` отсутствует вместо `null`; `limit` без верхней границы.
- **Компромисс:** keyset/курсор — стабильность и цена страницы O(log n) против произвольного доступа и «всего страниц».
- **Самопроверка, источники:** Markus Winand, «Use The Index, Luke» — «We need tool support for keyset pagination» и «Paging Through Results»; Slack Engineering, «Evolving API Pagination at Slack»; GraphQL Cursor Connections Specification (Relay); Google AIP-158 (Pagination); Клеппман, гл. 3.
- **Что изменилось в мессенджере** (`… m01-l10 m01-l11`).

**Термины:** `offset-pagination`, `keyset-pagination`, `cursor`, `page-drift`.

- [ ] **Step 8: Проверка, коммит, тег**

Run: `npm test && npm run typecheck && npm run check:messenger && npm run build` → PASS.

```bash
git add -A messenger course
git commit -m "feat(messenger): курсорная пагинация истории и списка чатов; урок 1.11

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git tag m01-l11
```

---

### Task 8: Урок 1.12 «Идемпотентность» + ADR-0005

**Files:**
- Modify: `messenger/internal/chat/domain/{ids.go,message.go,chat.go,errors.go}` и тесты
- Modify: `messenger/internal/chat/usecase/{send_message.go,usecase_test.go,fakes_test.go}`, `storetest/storetest.go`
- Modify: `messenger/internal/chat/adapters/memory/store.go`
- Create: `messenger/internal/platform/idempotency/{doc.go,idempotency.go,middleware.go,middleware_test.go}`, `…/idempotency/memory/{store.go,store_test.go}`, `…/idempotency/storetest/storetest.go`
- Modify: `messenger/internal/platform/problem/problem.go` (+3 кода), `api/openapi.yaml`, `gen/`, `chat/adapters/httpapi/*`, `cmd/messenger/main.go`
- Create: `messenger/docs/adr/0005-идемпотентность.md`, `course/modules/01-go-service/12-idempotency.md`

**Interfaces:**
- Consumes: задачи 4–7.
- Produces:
  - `domain.ClientMessageID`, `domain.ParseClientMessageID(s) (ClientMessageID, error)`; `(domain.Message).ClientID() ClientMessageID`;
  - `(*domain.Chat).Post(id MessageID, clientID ClientMessageID, author UserID, text Text, sentAt time.Time)`;
  - `domain.ErrDuplicateMessage`, `*domain.DuplicateMessageError{Existing Message}`, `domain.ErrClientMessageIDReused`;
  - `SendMessageCommand{ChatID, AuthorID, ClientMessageID, Text string}` → `SendMessageResult{Message domain.Message; Created bool}`;
  - `idempotency.Store`, `idempotency.Record`, `idempotency.Clock`, `idempotency.Middleware(store, clock, log, routes ...string) func(http.Handler) http.Handler`, константы `Header`, `ReplayedHeader`, `KeyMaxLen`, `TTL`; `memory.NewStore()`; `storetest.Run(t, newStore)`;
  - коды `problem.CodeClientMessageIDReused` (409), `CodeIdempotencyInProgress` (409), `CodeIdempotencyKeyReused` (422).

- [ ] **Step 1: Домен — естественный ключ сообщения**

`ids.go` — добавить (по образцу `MessageID`):

```go
// ClientMessageID — id, который клиент генерирует сам, до отправки, и повторяет при
// каждой попытке. По нему сервер узнаёт повтор (как random_id в Telegram, client_msg_id в Slack).
type ClientMessageID struct{ value string }

func ParseClientMessageID(s string) (ClientMessageID, error) {
	if !isUUID(s) {
		return ClientMessageID{}, invalid("client_message_id", "должен быть UUID в нижнем регистре")
	}
	return ClientMessageID{value: s}, nil
}

func (id ClientMessageID) String() string { return id.value }
func (id ClientMessageID) IsZero() bool   { return id.value == "" }
```

`message.go`: поле `clientID ClientMessageID`, метод `ClientID()`.

`chat.go`, `Post`: новый параметр `clientID ClientMessageID` вторым; в `switch` проверки нулевых — `case clientID.IsZero(): return Message{}, invalid("client_message_id", errNotSet)`; в результат — `clientID: clientID`. Регион `post` сохранить.

`errors.go` — в `sentinels` добавить:

```go
	ErrDuplicateMessage      = errors.New("сообщение с этим client_message_id уже сохранено")
	ErrClientMessageIDReused = errors.New("client_message_id уже использован для другого сообщения")
```

и после `ValidationError` (регион `duplicate`):

```go
// DuplicateMessageError — хранилище уже видело этот client_message_id от этого автора
// в этом чате. Existing — сохранённое тогда сообщение: use case вернёт его клиенту.
type DuplicateMessageError struct {
	Existing Message
}

func (e *DuplicateMessageError) Error() string { return ErrDuplicateMessage.Error() }

func (e *DuplicateMessageError) Is(target error) bool { return target == ErrDuplicateMessage }
```

Тесты: `ParseClientMessageID` (таблица как у других id); `Post` с нулевым `clientID` → `client_message_id: не задано`; `errors.Is(&DuplicateMessageError{}, ErrDuplicateMessage)`. Поправить все вызовы `Post` в тестах домена, `storetest` и use case'ах.

- [ ] **Step 2: Хранилище — уникальность `(чат, автор, client_message_id)`**

`memory/store.go`: в `Store` — карта `byClient map[clientKey]domain.Message` (под тем же `mu`, комментарий к `mu` — «защищает все карты ниже»), тип:

```go
// clientKey — естественный ключ сообщения. В Postgres (2a) — уникальный индекс
// (chat_id, author_id, client_message_id).
type clientKey struct {
	chat   domain.ChatID
	author domain.UserID
	client domain.ClientMessageID
}
```

В `AppendMessage` под блокировкой, **после** проверки, что чат существует, и **до** проверки версии:

```go
	key := clientKey{chat: chat.ID(), author: msg.AuthorID(), client: msg.ClientID()}
	// Повтор проверяем раньше версии: пока клиент ждал ответа, в чат могли написать,
	// и повтор должен получить своё сообщение, а не конфликт версий.
	if existing, ok := s.byClient[key]; ok {
		return &domain.DuplicateMessageError{Existing: existing}
	}
```

и при успешной записи `s.byClient[key] = msg`. Комментарий к `AppendMessage` в порту (`ports.go`) дополнить: «Если сообщение с тем же (чат, автор, client_message_id) уже есть — `*domain.DuplicateMessageError` с ним».

`storetest` — новые проверки (полный код главной):

```go
// Несколько одновременных повторов одного сообщения: сохранено ровно одно, остальные
// получают DuplicateMessageError с ним же.
func testConcurrentDuplicateClientMessageID(t *testing.T, s Store) {
	const n = 50
	c := newDirect(t, 1, 1, 2)
	create(t, s, c)
	clientID := id(t, domain.ParseClientMessageID, "e000", 1)
	text, err := domain.NewText("один раз")
	if err != nil {
		t.Fatal(err)
	}
	// id и find зовут t.Fatal — его нельзя вызывать из чужой горутины, поэтому id готовим
	// заранее, а чат читаем напрямую через порт.
	msgIDs := make([]domain.MessageID, n)
	for i := range n {
		msgIDs[i] = id(t, domain.ParseMessageID, "a000", i+1)
	}
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		saved   int
		dupIDs  = map[domain.MessageID]int{}
		failure error
	)
	for i := range n {
		wg.Go(func() {
			for {
				cur, err := s.FindChat(context.Background(), c.ID())
				if err != nil {
					mu.Lock()
					failure = err
					mu.Unlock()
					return
				}
				v := cur.Version()
				m, err := cur.Post(msgIDs[i], clientID, cur.Members()[0], text, t0)
				if err != nil {
					mu.Lock()
					failure = err
					mu.Unlock()
					return
				}
				err = s.AppendMessage(context.Background(), cur, m, v)
				var dup *domain.DuplicateMessageError
				mu.Lock()
				switch {
				case err == nil:
					saved++
				case errors.As(err, &dup):
					dupIDs[dup.Existing.ID()]++
				case errors.Is(err, domain.ErrConcurrentUpdate):
					mu.Unlock()
					continue // проиграли гонку версий — пробуем снова, как use case
				default:
					failure = err
				}
				mu.Unlock()
				return
			}
		})
	}
	wg.Wait()
	if failure != nil {
		t.Fatal(failure)
	}
	if saved != 1 || len(dupIDs) > 1 {
		t.Fatalf("сохранено %d, разных «существующих» %d — ждали 1 и не больше 1", saved, len(dupIDs))
	}
	if got := seqs(t, s, c.ID()); len(got) != 1 {
		t.Fatalf("в истории %d сообщений, ждали 1", len(got))
	}
}
```

(`sync.WaitGroup.Go` — Go 1.25+; если линтер или версия против — `wg.Add(1); go func(){ defer wg.Done() … }()`.) Плюс `testDuplicateClientMessageID` (последовательно: второй `Append` с тем же ключом → `DuplicateMessageError`, `Existing` — первое; другой автор или другой чат с тем же `client_message_id` — **не** дубль).

- [ ] **Step 3: Use case `SendMessage`**

Изменения (регион `send-execute` сохранить):

```go
type SendMessageCommand struct {
	ChatID          string
	AuthorID        string
	ClientMessageID string
	Text            string
}

// SendMessageResult.Created == false — это повтор уже сохранённого сообщения:
// адаптер ответит 200 вместо 201, а клиент получит то же сообщение, что и в первый раз.
type SendMessageResult struct {
	Message domain.Message
	Created bool
}
```

В `Execute`: разобрать `clientID` сразу после автора; `chat.Post(msgID, clientID, author, text, sentAt)`; после `AppendMessage`:

```go
		err = uc.messages.AppendMessage(ctx, chat, msg, expected)
		if err == nil {
			return SendMessageResult{Message: msg, Created: true}, nil
		}
		var dup *domain.DuplicateMessageError
		if errors.As(err, &dup) {
			// Тот же ключ и тот же текст — это повтор; тот же ключ с другим текстом —
			// ошибка клиента: ключ одного сообщения переиспользован для другого.
			if dup.Existing.Text() != text {
				return SendMessageResult{}, domain.ErrClientMessageIDReused
			}
			return SendMessageResult{Message: dup.Existing}, nil
		}
```

Тесты (полный код главного):

```go
func TestSendMessageRetryReturnsExisting(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	c := directChat(t, e)
	ctx := context.Background()
	cmd := usecase.SendMessageCommand{ChatID: c.ID().String(), AuthorID: alice, ClientMessageID: clientID(1), Text: "дойдёт один раз"}
	first, err := e.send.Execute(ctx, cmd)
	if err != nil || !first.Created {
		t.Fatalf("первая попытка: %+v, %v", first, err)
	}
	// Между попытками в чат написал собеседник: версия чата другая, а повтор — тот же.
	if _, err := e.send.Execute(ctx, usecase.SendMessageCommand{ChatID: c.ID().String(), AuthorID: bob, ClientMessageID: clientID(2), Text: "привет"}); err != nil {
		t.Fatal(err)
	}
	again, err := e.send.Execute(ctx, cmd)
	if err != nil || again.Created || again.Message.ID() != first.Message.ID() || again.Message.Seq() != first.Message.Seq() {
		t.Fatalf("повтор: %+v, %v — ждали то же сообщение и Created=false", again, err)
	}
	cmd.Text = "другой текст"
	if _, err := e.send.Execute(ctx, cmd); !errors.Is(err, domain.ErrClientMessageIDReused) {
		t.Fatalf("тот же ключ с другим текстом: ждали ErrClientMessageIDReused, получили %v", err)
	}
}
```

где `clientID(n int) string` в `fakes_test.go` — `fmt.Sprintf("00000000-0000-7000-e000-%012d", n)`. Остальные тесты `SendMessage` — добавить `ClientMessageID` (в `TestSendMessageConcurrent` — разный у каждой горутины, иначе тест проверит другое); `TestSendMessageRetryPolicy` — повторы при `ErrConcurrentUpdate` по-прежнему с тем же `msgID` и `clientID`.

Run: `go test -race ./internal/chat/...` → PASS.

- [ ] **Step 4: `platform/idempotency` — порт, запись, хранилище**

`idempotency.go`:

```go
package idempotency

import (
	"context"
	"crypto/sha256"
	"net/http"
	"time"
)

const (
	Header         = "Idempotency-Key"
	ReplayedHeader = "Idempotent-Replayed"
	// KeyMaxLen — как у Stripe: UUID с запасом, но не произвольно длинная строка.
	KeyMaxLen = 255
	// TTL — сколько помним ключ. Клиент повторяет секунды и минуты, а не дни;
	// сутки — с запасом на клиентов, которые были офлайн.
	TTL = 24 * time.Hour
)

type Clock interface {
	Now() time.Time
}

// Record — что сервер помнит о ключе. Пока Done == false, запрос «в работе».
type Record struct {
	Fingerprint [sha256.Size]byte
	Done        bool
	Status      int
	Header      http.Header // только заголовки, которые нужно повторить: Content-Type, Location
	Body        []byte
	ExpiresAt   time.Time
}

// Store — хранилище ключей. scope отделяет ключи разных пользователей: ключ — это
// договорённость клиента с сервером, и совпадение ключей у двух клиентов — не повтор.
type Store interface {
	// Reserve атомарно: если записи нет или она просрочена к now — сохраняет rec
	// («в работе») и возвращает (rec, true); иначе — существующую запись и false.
	Reserve(ctx context.Context, scope, key string, rec Record, now time.Time) (Record, bool, error)
	// Complete сохраняет готовый ответ.
	Complete(ctx context.Context, scope, key string, rec Record) error
	// Release удаляет запись: обработка не удалась, повтор должен пройти заново.
	Release(ctx context.Context, scope, key string) error
}
```

`memory/store.go`:

```go
// Package memory — хранилище ключей идемпотентности в памяти процесса. Просроченные
// записи удаляются, когда к ним обращаются; фоновая очистка — забота урока о воркерах
// (модуль 1.5), а в модуле 2a хранилищем станет Postgres.
package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"messenger/internal/platform/idempotency"
)

type entryKey struct{ scope, key string }

type Store struct {
	mu      sync.Mutex // защищает records
	records map[entryKey]idempotency.Record
}

func NewStore() *Store { return &Store{records: make(map[entryKey]idempotency.Record)} }

func (s *Store) Reserve(ctx context.Context, scope, key string, rec idempotency.Record, now time.Time) (idempotency.Record, bool, error) {
	if err := ctx.Err(); err != nil {
		return idempotency.Record{}, false, fmt.Errorf("idempotency memory: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := entryKey{scope, key}
	if old, ok := s.records[k]; ok && now.Before(old.ExpiresAt) {
		return clone(old), false, nil
	}
	s.records[k] = clone(rec)
	return clone(rec), true, nil
}

func (s *Store) Complete(ctx context.Context, scope, key string, rec idempotency.Record) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("idempotency memory: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[entryKey{scope, key}] = clone(rec)
	return nil
}

func (s *Store) Release(ctx context.Context, scope, key string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("idempotency memory: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, entryKey{scope, key})
	return nil
}

// clone — запись хранит срез и карту: отданная наружу без копии, она менялась бы вместе
// с хранилищем. Header.Clone копирует и срезы значений, не только карту.
func clone(r idempotency.Record) idempotency.Record {
	r.Header = r.Header.Clone()
	r.Body = append([]byte(nil), r.Body...)
	return r
}
```

`storetest/storetest.go` — `Run(t, newStore func(t *testing.T) idempotency.Store)` с проверками: `Reserve` нового ключа → `true`; повторный `Reserve` → `false` и та же запись; разные `scope` с одним ключом не пересекаются; `Complete` → следующий `Reserve` видит `Done`; `Release` → следующий `Reserve` снова `true`; просроченная запись (`now` после `ExpiresAt`) → `Reserve` даёт `true`; **гонка**: 50 горутин `Reserve` одного ключа → ровно одна получила `true`; возвращённая запись — копия (правка `Body` не меняет хранилище); отменённый контекст → ошибка с `context.Canceled` у всех трёх методов. `memory/store_test.go` — `storetest.Run(t, func(*testing.T) idempotency.Store { return memory.NewStore() })`.

- [ ] **Step 5: Middleware `Idempotency-Key`**

`middleware.go`:

```go
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"messenger/internal/platform/httpjson"
	"messenger/internal/platform/principal"
	"messenger/internal/platform/problem"
)

// #region middleware
// Middleware делает POST-маршруты из routes идемпотентными по заголовку Idempotency-Key
// (как у Stripe; черновик IETF draft-ietf-httpapi-idempotency-key-header). Без заголовка —
// обычная обработка. Маршрут — шаблон ServeMux ("POST /v1/chats"): middleware подключается
// после маршрутизатора (через сгенерированный обработчик), где r.Pattern уже заполнен.
//
// Ограничение: запись ключа и бизнес-операция — разные операции. Упади процесс между
// ними — повтор выполнится заново. Атомарно станет в 2a, когда запись ключа переедет
// в транзакцию операции (ADR-0005).
func Middleware(store Store, clock Clock, log *slog.Logger, routes ...string) func(http.Handler) http.Handler {
	guarded := make(map[string]bool, len(routes))
	for _, r := range routes {
		guarded[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get(Header)
			if key == "" || !guarded[r.Pattern] {
				next.ServeHTTP(w, r)
				return
			}
			g := guard{store: store, clock: clock, log: log}
			g.serve(w, r, next, key)
		})
	}
}

// #endregion middleware

type guard struct {
	store Store
	clock Clock
	log   *slog.Logger
}

func (g guard) serve(w http.ResponseWriter, r *http.Request, next http.Handler, key string) {
	if !validKey(key) {
		g.write(w, r, problem.Invalid(Header, "от 1 до 255 печатных ASCII-символов"))
		return
	}
	scope, ok := principal.UserIDFrom(r.Context())
	if !ok {
		// Ключ без пользователя не к чему привязать. Сюда можно попасть, только если
		// middleware подключили раньше аутентификации, — это ошибка сборки, а не клиента.
		g.log.Error("idempotency: нет личности в context — проверьте порядок middleware", "path", r.URL.Path)
		g.write(w, r, problem.Problem{Code: problem.CodeInternal})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, httpjson.MaxBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			g.write(w, r, problem.Problem{Code: problem.CodeBodyTooLarge, Detail: "тело больше 1 МиБ"})
			return
		}
		problem.Respond(w, r, g.log, err, func(error) (problem.Problem, bool) { return problem.Problem{}, false })
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	fp := fingerprint(r, body)
	now := g.clock.Now()
	rec, created, err := g.store.Reserve(r.Context(), scope, key, Record{Fingerprint: fp, ExpiresAt: now.Add(TTL)}, now)
	if err != nil {
		problem.Respond(w, r, g.log, err, func(error) (problem.Problem, bool) { return problem.Problem{}, false })
		return
	}
	if !created {
		g.replay(w, r, rec, fp)
		return
	}
	g.run(w, r, next, scope, key, rec)
}

// run выполняет запрос впервые и запоминает ответ. После обработчика клиент мог уйти —
// с хранилищем говорим вне его отмены, иначе ключ навсегда остался бы «в работе».
func (g guard) run(w http.ResponseWriter, r *http.Request, next http.Handler, scope, key string, rec Record) {
	ctx := context.WithoutCancel(r.Context())
	cw := &captureWriter{ResponseWriter: w}
	defer func() {
		if v := recover(); v != nil {
			g.release(ctx, scope, key)
			panic(v) // ответит Recover снаружи
		}
	}()
	next.ServeHTTP(cw, r)
	// 5xx и «ничего не записано» (клиент ушёл) — запрос не удался: освобождаем ключ,
	// чтобы повтор прошёл заново. 2xx и 4xx — окончательный ответ, его и повторяем.
	if cw.status == 0 || cw.status >= http.StatusInternalServerError {
		g.release(ctx, scope, key)
		return
	}
	rec.Done, rec.Status, rec.Body = true, cw.status, cw.body.Bytes()
	rec.Header = http.Header{}
	for _, h := range []string{"Content-Type", "Location"} {
		if v := cw.Header().Get(h); v != "" {
			rec.Header.Set(h, v)
		}
	}
	if err := g.store.Complete(ctx, scope, key, rec); err != nil {
		g.log.Error("idempotency: не удалось сохранить ответ", "key", key, "err", err)
	}
}

func (g guard) replay(w http.ResponseWriter, r *http.Request, rec Record, fp [sha256.Size]byte) {
	switch {
	case rec.Fingerprint != fp:
		g.write(w, r, problem.Problem{Code: problem.CodeIdempotencyKeyReused, Detail: "ключ уже использован для другого запроса"})
	case !rec.Done:
		g.write(w, r, problem.Problem{Code: problem.CodeIdempotencyInProgress, Detail: "запрос с этим ключом ещё выполняется, повторите позже"})
	default:
		for k, v := range rec.Header {
			w.Header()[k] = v
		}
		w.Header().Set(ReplayedHeader, "true")
		w.WriteHeader(rec.Status)
		if _, err := w.Write(rec.Body); err != nil {
			g.log.Error("запись ответа", "path", r.URL.Path, "err", err)
		}
	}
}

func (g guard) release(ctx context.Context, scope, key string) {
	if err := g.store.Release(ctx, scope, key); err != nil {
		g.log.Error("idempotency: не удалось освободить ключ", "key", key, "err", err)
	}
}

func (g guard) write(w http.ResponseWriter, r *http.Request, p problem.Problem) {
	if err := problem.Write(w, r, p); err != nil {
		g.log.Error("запись ответа", "path", r.URL.Path, "err", err)
	}
}

// fingerprint отличает повтор от другого запроса с тем же ключом: тот же метод, путь
// с запросом и то же тело. Разделитель — байт 0: он не встречается ни в методе, ни в пути.
func fingerprint(r *http.Request, body []byte) [sha256.Size]byte {
	h := sha256.New()
	h.Write([]byte(r.Method))
	h.Write([]byte{0})
	h.Write([]byte(r.URL.RequestURI()))
	h.Write([]byte{0})
	h.Write(body)
	var out [sha256.Size]byte
	copy(out[:], h.Sum(nil))
	return out
}

func validKey(k string) bool {
	if k == "" || len(k) > KeyMaxLen {
		return false
	}
	for i := range len(k) {
		if k[i] < 0x20 || k[i] > 0x7e {
			return false
		}
	}
	return true
}

// captureWriter пропускает ответ клиенту и копит его для повтора.
type captureWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (c *captureWriter) WriteHeader(code int) {
	if c.status == 0 {
		c.status = code
	}
	c.ResponseWriter.WriteHeader(code)
}

func (c *captureWriter) Write(b []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
	}
	c.body.Write(b)
	return c.ResponseWriter.Write(b)
}

func (c *captureWriter) Unwrap() http.ResponseWriter { return c.ResponseWriter }
```

В `problem.go` добавить коды и строки реестра:

```go
	CodeClientMessageIDReused Code = "client_message_id_reused"
	CodeIdempotencyInProgress Code = "idempotency_in_progress"
	CodeIdempotencyKeyReused  Code = "idempotency_key_reused"
	…
	CodeClientMessageIDReused: {http.StatusConflict, "client_message_id уже использован"},
	CodeIdempotencyInProgress: {http.StatusConflict, "Запрос с этим ключом ещё выполняется"},
	CodeIdempotencyKeyReused:  {http.StatusUnprocessableEntity, "Ключ идемпотентности использован для другого запроса"},
```

**Импорт-цикл:** `idempotency` → `problem` → `middleware`; `middleware` не импортирует `idempotency` — цикла нет. `idempotency` → `httpjson` — ради `MaxBodyBytes` (одно знание о лимите тела).

`middleware_test.go` — помощник `serve(t, store, handler) http.Handler`, который ставит `principal` и маршрутизирует через настоящий `ServeMux` с шаблоном `"POST /v1/chats"` (иначе `r.Pattern` пуст). Тесты (полный код двух главных, остальные — по образцу):

```go
func TestIdempotencyConcurrent(t *testing.T) {
	store := memory.NewStore()
	var calls atomic.Int32
	release := make(chan struct{})
	h := serve(t, store, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		<-release // держим первый запрос «в работе», пока не придут остальные
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	const n = 10
	codes := make(chan int, n)
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, request("ключ-1", `{"kind":"group"}`))
			codes <- rec.Code
		})
	}
	// Все, кроме первого, должны ответить 409 и не ждать: ждём их, потом отпускаем первого.
	waitFor(t, func() bool { return len(codes) == n-1 })
	close(release)
	wg.Wait()
	close(codes)
	count := map[int]int{}
	for c := range codes {
		count[c]++
	}
	if calls.Load() != 1 || count[http.StatusCreated] != 1 || count[http.StatusConflict] != n-1 {
		t.Fatalf("обработчик вызван %d раз, ответы %v", calls.Load(), count)
	}
}

func TestIdempotencyReleasesAfterPanic(t *testing.T) {
	store := memory.NewStore()
	var calls atomic.Int32
	h := serve(t, store, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			panic("бум")
		}
		w.WriteHeader(http.StatusCreated)
	}))
	func() {
		defer func() { _ = recover() }() // в бою панику ловит Recover снаружи
		h.ServeHTTP(httptest.NewRecorder(), request("ключ-2", `{}`))
	}()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, request("ключ-2", `{}`))
	if rec.Code != http.StatusCreated || calls.Load() != 2 {
		t.Fatalf("после паники повтор: %d, вызовов %d — ждали 201 и 2", rec.Code, calls.Load())
	}
}
```

(`request(key, body)` — `httptest.NewRequest(POST, "/v1/chats", body)` с заголовком ключа; `waitFor` — опрос условия с `time.Sleep(time.Millisecond)` и пределом 2 с; в `serve` — `principal.WithUserID` и `mux.Handle("POST /v1/chats", idempotency.Middleware(store, clock, log, "POST /v1/chats")(h))`.)

Остальные тесты: повтор после успеха — тот же статус, тело байт в байт, `Location` и `Idempotent-Replayed: true`, обработчик вызван 1 раз; другой ключ — обработчик вызван снова; тот же ключ, другое тело — 422 `idempotency_key_reused`; другой пользователь с тем же ключом — обработчик вызван (scope); 500 от обработчика — ключ освобождён; 400 от обработчика — **сохранён** (повтор получает тот же 400); обработчик ничего не записал — освобождён; ключ из 256 символов или с `\n` — 400 `invalid_params[0].name == "Idempotency-Key"`; маршрут не из `routes` — middleware не вмешивается; тело больше 1 МиБ — 413; нет `principal` — 500 и строка в логе.

Run: `go test -race ./internal/platform/idempotency/...` → PASS.

- [ ] **Step 6: Контракт, хендлеры, сборка (контракт)**

- `api/openapi.yaml`:
  - `SendMessageRequest`: обязательное `client_message_id` (`string`, `format: uuid`, `x-go-type: string`, описание «генерирует клиент; одинаковый во всех повторах этого сообщения»);
  - `Message`: обязательное `client_message_id`;
  - `sendMessage`: ответ `200` («повтор уже сохранённого сообщения»), `409` — описание дополнить (`conflict`, `client_message_id_reused`);
  - параметр `components.parameters.IdempotencyKey` (`in: header`, `name: Idempotency-Key`, `required: false`, `schema: {type: string, minLength: 1, maxLength: 255}`) у `createChat`; заголовок `Idempotent-Replayed` в ответах `201`/`200` `createChat`; ответы `409` (`idempotency_in_progress`) и `422` (`idempotency_key_reused`, новый `components.responses.UnprocessableEntity`);
  - `ProblemCode`: + `client_message_id_reused`, `idempotency_in_progress`, `idempotency_key_reused`;
  - регион `idempotency` вокруг параметра и ответа 422.
- `go generate ./...`. Сгенерированный `CreateChat` получит `params gen.CreateChatParams` с `IdempotencyKey *string` — хендлер его не использует (заголовок обрабатывает middleware), комментарий об этом в хендлере.
- `chat/httpapi`: `SendMessage` → `ClientMessageID: req.ClientMessageId`; `Created` → 201, иначе 200; `toMessage` отдаёт `client_message_id`; `problemFor`: `ErrClientMessageIDReused` → `CodeClientMessageIDReused`. `Register(mux, mws...)` уже принимает middleware (задача 5).
- `main.go`: `idemStore := idemmemory.NewStore()`; `chatAPI.Register(mux, gen.MiddlewareFunc(idempotency.Middleware(idemStore, clk, log, "POST /v1/chats")))` (тип приводится: `gen.MiddlewareFunc` — `func(http.Handler) http.Handler`). Комментарий: «ключ только у создания чата: у сообщений — естественный ключ client_message_id».
- Тесты хендлеров: 201 и 200 для повтора `client_message_id`; 409 `client_message_id_reused`; в `handler_test` — `Idempotency-Key` через настоящий middleware: повтор `POST /v1/chats` создаёт один чат и отвечает тем же телом.

Run: `go build ./... && go test -race ./... && golangci-lint run ./...` → PASS.

- [ ] **Step 7: ADR-0005 и прогон через `curl`**

`0005-идемпотентность.md`: контекст (повторы неизбежны: таймаут клиента ≠ провал; at-least-once), варианты (ничего; `Idempotency-Key` на всё; естественный ключ на всё; естественный ключ сообщений + ключ для остальных), решение (последний), последствия: атомарность естественного ключа (уникальность в той же операции записи) против неатомарности middleware до 2a; ключи живут 24 ч; память растёт с числом ключей до фоновой очистки (1.5) или Postgres (2a); `client_message_id` обязателен — клиент обязан генерировать UUID до первой попытки.

`curl`: сообщение с `client_message_id` → 201; тот же запрос → 200, то же `id` и `seq`; тот же ключ, другой текст → 409; группа с `Idempotency-Key: $(uuidgen)` → 201; повтор → 201 + `Idempotent-Replayed: true`, тот же `id`; тот же ключ, другое название → 422; `GET /v1/chats` — групп ровно одна. Реальный вывод — в отчёт.

- [ ] **Step 8: Урок `12-idempotency.md`**

Frontmatter: `title: 1.12. Идемпотентность: повтор без последствий`.

**Разделы:**
- **Проблема.** Клиент отправил сообщение, сервер сохранил, ответ потерялся в метро. Клиент не знает, дошло ли, и повторяет — в чате два одинаковых сообщения. Деньги в платёжке — то же, только дороже. Таймаут — это «не знаю», а не «нет».
- **Теория:**
  - `idempotency` и `idempotent-method` (из 1.8): почему POST не идемпотентен по природе и почему его делают идемпотентным;
  - `at-least-once`: повторы есть всегда — у клиентов, балансировщиков, ретраев (модуль 5), брокеров (модуль 4); «ровно один раз» — это «как минимум один раз» + идемпотентный получатель;
  - приём 1 — `natural-key`: `random_id` в `messages.sendMessage` Telegram (core.telegram.org), `client_msg_id` в Slack; клиент генерирует UUID до первой попытки; уникальность в хранилище; атомарность — проверка и запись одной операцией;
  - приём 2 — `idempotency-key` (Stripe, черновик IETF): ключ от клиента, `request-fingerprint`, состояния записи «в работе» / «готово», ответы на повтор (тот же ответ; 409 «ещё выполняется»; 422 «другое тело»), что хранить (статус, нужные заголовки, тело), сколько (TTL), что делать после 5xx и паники; область ключа — пользователь;
  - где каждый атомарен: таблица «процесс упал между X и Y» для обоих приёмов; что изменит транзакция в 2a (Brandur Leach — «Implementing Stripe-like Idempotency Keys in Postgres», атомарные фазы);
  - почему 200 при повторе сообщения, а не 201 (ничего нового не создано) — и почему повтор `Idempotency-Key` отвечает **тем же** 201 (повторяем ответ, а не пересчитываем его).
- **Визуализация:** `<IdempotencySandbox />` и 4 задания.
- **Код:** регионы `duplicate`, `send-execute`, `middleware`, функции `run` и `replay` (регионы — добавить), YAML `idempotency`; сценарий `curl`.
- **Под капотом:** «Гонка за ключ»: почему проверка «есть ли запись» и запись «в работе» должны быть одной атомарной операцией (две горутины прочитали «нет» — обе выполнили), как это сделано под мьютексом сейчас и как будет в Postgres (`INSERT … ON CONFLICT DO NOTHING RETURNING`), почему второй ждущий запрос получает 409 сразу, а не ждёт первого (не держим соединения и горутины; клиент повторит с паузой — модуль 5).
- **Ошибки:** ключ генерировать на сервере; новый ключ на каждую попытку клиента; сохранять 5xx как окончательный ответ; ключ без области пользователя; отпечаток без тела; повтор с «пересчётом» ответа.
- **Компромисс:** естественный ключ — атомарен и бесплатен, но требует поля в модели; `Idempotency-Key` — универсален, но это отдельное хранилище, TTL и неатомарность до транзакций.
- **Самопроверка, источники:** Stripe API Reference — Idempotent requests; Brandur Leach, «Implementing Stripe-like Idempotency Keys in Postgres»; IETF draft-ietf-httpapi-idempotency-key-header (актуальная редакция — проверить номер); Telegram API — `messages.sendMessage` (`random_id`); Клеппман, гл. 11 («идемпотентность», «exactly-once»); Richardson, «Microservices Patterns», паттерн Idempotent Consumer.
- **Что изменилось в мессенджере** (`… m01-l11 m01-l12`).

**Термины:** `idempotency`, `idempotency-key`, `natural-key`, `at-least-once`, `request-fingerprint`.

- [ ] **Step 9: Проверка, коммит, тег**

Run: `npm test && npm run typecheck && npm run check:messenger && npm run build` → PASS.

```bash
git add -A messenger course
git commit -m "feat(messenger): client_message_id и Idempotency-Key, ADR-0005; урок 1.12

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git tag m01-l12
```

---

### Task 9: Урок 1.13 «Пароли и вход» + контекст `auth` + ADR-0004

**Files:**
- Create: `messenger/internal/kernel/ident/{ident.go,ident_test.go}`; Modify: `messenger/internal/user/domain/id.go`, `messenger/internal/chat/domain/ids.go` (вызов `ident.IsUUID`, копии `isUUID` удалить)
- Create: `messenger/internal/auth/domain/{doc.go,password.go,credentials.go,errors.go,password_test.go,credentials_test.go,errors_test.go}`
- Create: `messenger/internal/auth/usecase/{doc.go,ports.go,signup.go,login.go,usecase_test.go,fakes_test.go}`, `…/usecase/storetest/storetest.go`
- Create: `messenger/internal/auth/adapters/argon2id/{doc.go,argon2id.go,argon2id_test.go}`
- Create: `messenger/internal/auth/adapters/memory/{doc.go,store.go,store_test.go}`
- Create: `messenger/internal/auth/adapters/userbridge/{doc.go,bridge.go,bridge_test.go}`
- Create: `messenger/internal/auth/adapters/httpapi/{doc.go,handler.go,dto.go,errors.go,handler_test.go}` + `gen/`
- Create: `messenger/internal/user/usecase/find_by_username.go`; Modify: `user/usecase/{ports.go,usecase_test.go}`, `user/usecase/storetest/storetest.go`, `user/adapters/memory/store.go`, `user/adapters/httpapi/*` (без регистрации, + `GetMe`)
- Modify: `messenger/api/openapi.yaml`, `cmd/messenger/main.go`, `go.mod` (`golang.org/x/crypto@v0.57.0`)
- Create: `messenger/docs/adr/0004-регистрация-через-два-контекста.md`, `course/modules/01-go-service/13-passwords.md`

**Interfaces:**
- Consumes: задачи 4–8; `user/usecase.RegisterUser`.
- Produces (использует задача 10):
  - `ident.IsUUID(s string) bool`;
  - `authdomain.UserID` / `ParseUserID`, `Password` / `NewPassword`, `PasswordHash` / `NewPasswordHash`, `Credentials` / `NewCredentials`, ошибки `ErrInvalid`, `*ValidationError`, `ErrInvalidCredentials`, `ErrUsernameTaken`, `ErrNoSuchUser`, `ErrNoCredentials`, `ErrCredentialsExist`; константы `PasswordMinLen = 15`, `PasswordMaxLen = 128`, `PasswordMaxBytes = 4 * PasswordMaxLen`;
  - порты `PasswordHasher`, `ProfileRegistrar`, `UserLookup`, `CredentialsSaver`, `CredentialsFinder`, `TokenIssuer`; типы `Profile`, `Token{Value string; ExpiresAt time.Time}`;
  - `NewSignUp(hasher, profiles, creds) (*SignUp, error)` → `Execute(ctx, SignUpCommand{Username, DisplayName, Password}) (Profile, error)`;
  - `NewLogin(users, creds, hasher, tokens) (*Login, error)` → `Execute(ctx, LoginCommand{Username, Password}) (Token, error)`;
  - `argon2id.New(Params) (*Hasher, error)`, `argon2id.OWASPMinimum`;
  - `userbridge.New(register, find) (*Bridge, error)`;
  - `authhttp.New(signUp, log) (*Handler, error)` с `Register(mux, mws...)`;
  - `userusecase.NewFindByUsername(users UserByUsernameFinder)`, `FindByUsernameQuery{Username string}`.

- [ ] **Step 1: Общее ядро `kernel/ident` (правило трёх)**

Контексту `auth` нужен свой `UserID` — это была бы третья копия `isUUID` (комментарий в `chat/domain/ids.go` прямо обещает выносить на третьей). `internal/kernel/ident/ident.go`:

```go
// Package ident — общее ядро (shared kernel): как выглядит идентификатор в нашей системе.
// Это знание одно на все контексты, поэтому живёт в одном месте. Ядро намеренно крошечное
// и без зависимостей: менять его можно только по согласию всех контекстов (Эванс).
package ident

// IsUUID — канонический текстовый вид UUID: 36 символов, дефисы на позициях 8, 13, 18, 23,
// остальное — шестнадцатеричные цифры в нижнем регистре. Версию не проверяет: id других
// систем (и клиентские client_message_id) бывают UUIDv4.
func IsUUID(s string) bool {
	const uuidLen = 36
	if len(s) != uuidLen {
		return false
	}
	for i := range len(s) {
		c := s[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				return false
			}
		}
	}
	return true
}
```

`ident_test.go` — таблица (перенести случаи из тестов `user/domain` и `chat/domain`: верхний регистр, без дефисов, 35/37 символов, буква `g`, кириллица). В `user/domain/id.go` и `chat/domain/ids.go` — `ident.IsUUID(s)`, локальные `isUUID` удалить, комментарий про «третью копию» заменить ссылкой на `kernel/ident`.

Run: `go test ./internal/... && golangci-lint run ./...` → PASS (правила задачи 1 разрешают `domain → kernel/ident`).

- [ ] **Step 2: Домен `auth`**

`doc.go`: «контекст auth — учётные данные и выпуск токенов (Identity & Access по Вернону); профиль пользователя — в контексте user».

`errors.go`:

```go
package domain

import (
	"errors"
	"fmt"
)

var (
	ErrInvalid = errors.New("некорректные данные")
	// ErrInvalidCredentials — одна ошибка на «нет такого пользователя» и «не тот пароль»:
	// разные ответы рассказали бы атакующему, какие имена существуют.
	ErrInvalidCredentials = errors.New("неверное имя пользователя или пароль")
	ErrUsernameTaken      = errors.New("имя пользователя занято")
	// ErrNoSuchUser и ErrNoCredentials — внутренние: наружу они уходят как ErrInvalidCredentials.
	ErrNoSuchUser       = errors.New("пользователь не найден")
	ErrNoCredentials    = errors.New("учётные данные не найдены")
	ErrCredentialsExist = errors.New("учётные данные уже есть")
)

type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Reason) }

func (e *ValidationError) Is(target error) bool { return target == ErrInvalid }

func invalid(field, reason string) error { return &ValidationError{Field: field, Reason: reason} }
```

`password.go`:

```go
package domain

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// #region password-policy
const (
	// NIST SP 800-63B-4: пароль как единственный фактор — не короче 15 символов; верхнюю
	// границу верификатор обязан допускать не меньше 64. 128 — с запасом. Требований
	// к составу («цифра, заглавная, спецсимвол») нет сознательно: NIST их не рекомендует —
	// они делают пароли предсказуемее, а не сильнее. (Цифры — сверить с финальной редакцией.)
	PasswordMinLen = 15
	PasswordMaxLen = 128
	// PasswordMaxBytes — верхняя граница в байтах для пароля при входе (до 4 байт UTF-8
	// на символ): мегабайтную строку незачем отдавать медленной функции хеширования.
	PasswordMaxBytes = 4 * PasswordMaxLen
)

// Password — новый пароль, прошедший политику. Нужен только при регистрации и смене:
// при входе пароль сравнивают как есть — политика могла ужесточиться после регистрации.
type Password struct{ value string }

func NewPassword(s string) (Password, error) {
	if !utf8.ValidString(s) {
		return Password{}, invalid("password", "некорректный UTF-8")
	}
	if n := utf8.RuneCountInString(s); n < PasswordMinLen || n > PasswordMaxLen {
		return Password{}, invalid("password", fmt.Sprintf("от %d до %d символов", PasswordMinLen, PasswordMaxLen))
	}
	if strings.IndexFunc(s, unicode.IsControl) >= 0 {
		return Password{}, invalid("password", "управляющие символы запрещены")
	}
	return Password{value: s}, nil
}

// #endregion password-policy

// Plaintext — явный доступ к паролю: только для хеширования.
func (p Password) Plaintext() string { return p.value }

// String и GoString скрывают пароль: случайный fmt.Print или лог не раскроют его.
func (Password) String() string   { return "[пароль скрыт]" }
func (Password) GoString() string { return "domain.Password{[скрыт]}" }
```

`credentials.go`:

```go
package domain

import "messenger/internal/kernel/ident"

// UserID — пользователь глазами контекста auth. Свой тип, как и в chat: контексты
// не делят доменные типы, только формат id (kernel/ident).
type UserID struct{ value string }

func ParseUserID(s string) (UserID, error) {
	if !ident.IsUUID(s) {
		return UserID{}, invalid("user_id", "должен быть UUID в нижнем регистре")
	}
	return UserID{value: s}, nil
}

func (id UserID) String() string { return id.value }
func (id UserID) IsZero() bool   { return id.value == "" }

// PasswordHash — результат функции хеширования. Домен не знает алгоритма: строка
// непрозрачна, её понимает только PasswordHasher.
type PasswordHash struct{ value string }

func NewPasswordHash(s string) (PasswordHash, error) {
	if s == "" {
		return PasswordHash{}, invalid("password_hash", "не задано")
	}
	return PasswordHash{value: s}, nil
}

func (h PasswordHash) String() string { return h.value }
func (h PasswordHash) IsZero() bool   { return h.value == "" }

type Credentials struct {
	userID UserID
	hash   PasswordHash
}

func NewCredentials(userID UserID, hash PasswordHash) (Credentials, error) {
	if userID.IsZero() {
		return Credentials{}, invalid("user_id", "не задано")
	}
	if hash.IsZero() {
		return Credentials{}, invalid("password_hash", "не задано")
	}
	return Credentials{userID: userID, hash: hash}, nil
}

func (c Credentials) UserID() UserID             { return c.userID }
func (c Credentials) PasswordHash() PasswordHash { return c.hash }
```

Тесты домена (таблицы): `NewPassword` — 14 символов нет, 15 да, 15 кириллических да, 128 да, 129 нет, эмодзи считается одним символом, `\x00`/`\n` нет, невалидный UTF-8 нет; `fmt.Sprintf("%v %#v %s", p, p, p)` не содержит пароля; `ParseUserID`, `NewPasswordHash`, `NewCredentials` с нулевыми — ошибки; `errors.Is(&ValidationError{}, ErrInvalid)`.

- [ ] **Step 3: Порты и use case'ы `auth`**

`ports.go`:

```go
package usecase

import (
	"context"
	"time"

	"messenger/internal/auth/domain"
)

// #region ports
type PasswordHasher interface {
	Hash(p domain.Password) (domain.PasswordHash, error)
	// Verify сравнивает пароль с хешем. Время сравнения не зависит от того, в каком
	// байте они расходятся. Ошибка — только если хеш испорчен, а не «не совпал».
	Verify(plaintext string, hash domain.PasswordHash) (bool, error)
}

// Profile — профиль пользователя глазами auth: ровно то, что нужно ответу регистрации.
type Profile struct {
	ID          domain.UserID
	Username    string
	DisplayName string
	CreatedAt   time.Time
}

// ProfileRegistrar создаёт профиль в контексте user. Реализация — адаптер userbridge;
// в модуле 6 — вызов сервиса users.
type ProfileRegistrar interface {
	// Ошибки данных — *domain.ValidationError, занятое имя — domain.ErrUsernameTaken.
	RegisterProfile(ctx context.Context, username, displayName string) (Profile, error)
}

type UserLookup interface {
	// UserIDByUsername — domain.ErrNoSuchUser, если имени нет или оно некорректно.
	UserIDByUsername(ctx context.Context, username string) (domain.UserID, error)
}

type CredentialsSaver interface {
	// SaveCredentials — domain.ErrCredentialsExist, если у пользователя они уже есть.
	SaveCredentials(ctx context.Context, c domain.Credentials) error
}

type CredentialsFinder interface {
	// FindCredentials — domain.ErrNoCredentials, если их нет.
	FindCredentials(ctx context.Context, id domain.UserID) (domain.Credentials, error)
}

// Token — выпущенный токен доступа.
type Token struct {
	Value     string
	ExpiresAt time.Time
}

// TokenIssuer выпускает токен доступа. Как он устроен (JWT, подпись, срок) — дело адаптера.
type TokenIssuer interface {
	Issue(userID domain.UserID) (Token, error)
}

// #endregion ports
```

`signup.go`:

```go
package usecase

import (
	"context"
	"errors"
	"fmt"

	"messenger/internal/auth/domain"
)

type SignUpCommand struct {
	Username    string
	DisplayName string
	Password    string
}

type SignUp struct {
	hasher   PasswordHasher
	profiles ProfileRegistrar
	creds    CredentialsSaver
}

func NewSignUp(hasher PasswordHasher, profiles ProfileRegistrar, creds CredentialsSaver) (*SignUp, error) {
	if hasher == nil || profiles == nil || creds == nil {
		return nil, errors.New("sign up: nil-зависимость")
	}
	return &SignUp{hasher: hasher, profiles: profiles, creds: creds}, nil
}

// #region signup
func (uc *SignUp) Execute(ctx context.Context, cmd SignUpCommand) (Profile, error) {
	pw, err := domain.NewPassword(cmd.Password)
	if err != nil {
		return Profile{}, err
	}
	// Хешируем до создания профиля: если хешер сломан, узнаем это, ничего не создав.
	hash, err := uc.hasher.Hash(pw)
	if err != nil {
		return Profile{}, fmt.Errorf("sign up: хеширование: %w", err)
	}
	profile, err := uc.profiles.RegisterProfile(ctx, cmd.Username, cmd.DisplayName)
	if err != nil {
		return Profile{}, fmt.Errorf("sign up: %w", err)
	}
	creds, err := domain.NewCredentials(profile.ID, hash)
	if err != nil {
		// Без %w: пустой id от соседнего контекста — сбой сервера, а не ошибка данных клиента.
		return Profile{}, fmt.Errorf("sign up: некорректные учётные данные для профиля %q", profile.ID)
	}
	if err := uc.creds.SaveCredentials(ctx, creds); err != nil {
		// Профиль уже создан, а пароля у него нет — войти он не сможет. Это известная
		// неатомарность двух контекстов (ADR-0004); закроет её транзакция в модуле 2a.
		return Profile{}, fmt.Errorf("sign up: профиль %s создан без учётных данных: %w", profile.ID, err)
	}
	return profile, nil
}

// #endregion signup
```

`login.go`:

```go
package usecase

import (
	"context"
	"errors"
	"fmt"

	"messenger/internal/auth/domain"
)

// dummyPassword — пароль фиктивного хеша. Войти им нельзя: хеш не привязан ни к кому.
const dummyPassword = "фиктивный пароль для выравнивания времени входа"

type LoginCommand struct {
	Username string
	Password string
}

type Login struct {
	users  UserLookup
	creds  CredentialsFinder
	hasher PasswordHasher
	tokens TokenIssuer
	dummy  domain.PasswordHash
}

// NewLogin заранее считает фиктивный хеш тем же хешером: его параметры совпадают
// с настоящими, значит, и время проверки тоже.
func NewLogin(users UserLookup, creds CredentialsFinder, hasher PasswordHasher, tokens TokenIssuer) (*Login, error) {
	if users == nil || creds == nil || hasher == nil || tokens == nil {
		return nil, errors.New("login: nil-зависимость")
	}
	pw, err := domain.NewPassword(dummyPassword)
	if err != nil {
		return nil, fmt.Errorf("login: фиктивный пароль не проходит политику: %w", err)
	}
	dummy, err := hasher.Hash(pw)
	if err != nil {
		return nil, fmt.Errorf("login: фиктивный хеш: %w", err)
	}
	return &Login{users: users, creds: creds, hasher: hasher, tokens: tokens, dummy: dummy}, nil
}

// #region login
func (uc *Login) Execute(ctx context.Context, cmd LoginCommand) (Token, error) {
	if len(cmd.Password) > domain.PasswordMaxBytes {
		// Ничего не выдаёт: длину пароля атакующий знает и так.
		return Token{}, domain.ErrInvalidCredentials
	}
	id, err := uc.users.UserIDByUsername(ctx, cmd.Username)
	if errors.Is(err, domain.ErrNoSuchUser) {
		return Token{}, uc.reject(cmd.Password)
	}
	if err != nil {
		return Token{}, fmt.Errorf("login: %w", err)
	}
	creds, err := uc.creds.FindCredentials(ctx, id)
	if errors.Is(err, domain.ErrNoCredentials) {
		// Профиль без пароля — след неудачной регистрации (ADR-0004). Для клиента — то же «неверно».
		return Token{}, uc.reject(cmd.Password)
	}
	if err != nil {
		return Token{}, fmt.Errorf("login: %w", err)
	}
	ok, err := uc.hasher.Verify(cmd.Password, creds.PasswordHash())
	if err != nil {
		return Token{}, fmt.Errorf("login: %w", err)
	}
	if !ok {
		return Token{}, domain.ErrInvalidCredentials
	}
	tok, err := uc.tokens.Issue(id)
	if err != nil {
		return Token{}, fmt.Errorf("login: выпуск токена: %w", err)
	}
	return tok, nil
}

// reject тратит на отказ столько же, сколько настоящая проверка: иначе по времени
// ответа видно, есть ли пользователь (быстро — нет, медленно — есть, но пароль не тот).
func (uc *Login) reject(password string) error {
	if _, err := uc.hasher.Verify(password, uc.dummy); err != nil {
		return fmt.Errorf("login: фиктивная проверка: %w", err)
	}
	return domain.ErrInvalidCredentials
}

// #endregion login
```

`fakes_test.go`: `countingHasher` (хеш — `"h:" + пароль`, `Verify` сравнивает и считает вызовы в `atomic.Int32`, умеет отдавать ошибку), фейки `ProfileRegistrar` (карта имён, `ErrUsernameTaken`), `UserLookup`, хранилище учётных данных — `memory.NewStore()` из адаптера (как в тестах `chat/usecase`), `fixedIssuer`.

`usecase_test.go` — полный код главного:

```go
func TestLoginDoesNotRevealUsers(t *testing.T) {
	t.Parallel()
	e := newEnv(t) // регистрирует alice с паролем "правильный-пароль-длинный"
	tests := []struct {
		name, username, password string
	}{
		{"нет такого пользователя", "nobody", "правильный-пароль-длинный"},
		{"неверный пароль", "alice", "неправильный-пароль-длинный"},
		{"профиль без пароля (ADR-0004)", "orphan", "правильный-пароль-длинный"},
		{"некорректное имя", "Не Имя!", "правильный-пароль-длинный"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := e.hasher.verifies.Load()
			_, err := e.login.Execute(context.Background(), usecase.LoginCommand{Username: tt.username, Password: tt.password})
			if !errors.Is(err, domain.ErrInvalidCredentials) {
				t.Fatalf("ждали ErrInvalidCredentials, получили %v", err)
			}
			// Каждый отказ — ровно одна проверка хеша: настоящего или фиктивного.
			if got := e.hasher.verifies.Load() - before; got != 1 {
				t.Fatalf("проверок хеша %d, ждали 1 — время ответа выдаст, есть ли пользователь", got)
			}
		})
	}
}
```

(`newEnv` в этом пакете: хранилище, хешер, фейк профилей с `alice` и `orphan` — у `orphan` есть профиль, но нет учётных данных; `t.Parallel` внутри подтестов не включать — общий счётчик.)

Остальные тесты: `TestSignUpOrder` (фейк-хешер падает → профиль не создан; фейк-регистратор падает → учётные данные не сохранены; сохранение падает → ошибка содержит id профиля и `errors.Is` видит исходную), `TestSignUpPasswordPolicy` (14 символов → `ValidationError{Field: "password"}`, хешер не вызван), `TestSignUpUsernameTaken` (→ `ErrUsernameTaken`), `TestLoginSuccess` (токен от `fixedIssuer`), `TestLoginHugePassword` (больше `PasswordMaxBytes` → отказ без вызова хешера), `TestConstructorsRejectNil`.

`storetest/storetest.go` — `Run(t, newStore)` для `CredentialsSaver + CredentialsFinder`: сохранить и найти; второй раз — `ErrCredentialsExist`; не найдено — `ErrNoCredentials`; гонка 50 горутин за одного пользователя — ровно один успех; отменённый контекст — ошибка с `context.Canceled`, ничего не сохранено.

- [ ] **Step 4: Адаптеры `auth`**

`memory/store.go` — по образцу `user/adapters/memory` (`sync.RWMutex`, `map[domain.UserID]domain.Credentials`, `ctx.Err()` на входе, проверка и запись под одной блокировкой); `store_test.go` — `storetest.Run`.

`argon2id/argon2id.go`:

```go
// Package argon2id — PasswordHasher на Argon2id (golang.org/x/crypto/argon2), первом
// выборе OWASP для хранения паролей. Хеш — в формате PHC: параметры записаны в нём самом,
// поэтому их можно поднять для новых паролей, не сломав проверку старых.
package argon2id

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"

	"messenger/internal/auth/domain"
)

type Params struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltLen     uint32
	KeyLen      uint32
}

// #region params
// OWASPMinimum — минимальная конфигурация Argon2id из OWASP Password Storage Cheat Sheet
// (m = 19 МиБ, t = 2, p = 1; сверить с текущей редакцией). Соль 16 байт и ключ 32 байта —
// рекомендации RFC 9106.
var OWASPMinimum = Params{MemoryKiB: 19 * 1024, Iterations: 2, Parallelism: 1, SaltLen: 16, KeyLen: 32}

// #endregion params

const (
	// Границы параметров, прочитанных из хеша: подменённый хеш не должен заставить
	// сервер выделить гигабайты или крутиться минуты.
	maxMemoryKiB  = 1 << 20 // 1 ГиБ
	maxIterations = 16
	minKeyLen     = 16
	maxKeyLen     = 64
)

var b64 = base64.RawStdEncoding // PHC: стандартный алфавит без «=»

type Hasher struct {
	p    Params
	rand io.Reader
}

func New(p Params) (*Hasher, error) {
	switch {
	case p.Parallelism == 0 || p.Iterations == 0 || p.Iterations > maxIterations:
		return nil, errors.New("argon2id: нужны t от 1 до 16 и p ≥ 1")
	case p.MemoryKiB < 8*uint32(p.Parallelism) || p.MemoryKiB > maxMemoryKiB:
		return nil, errors.New("argon2id: память от 8·p КиБ до 1 ГиБ")
	case p.SaltLen < 16 || p.KeyLen < minKeyLen || p.KeyLen > maxKeyLen:
		return nil, errors.New("argon2id: соль от 16 байт, ключ от 16 до 64 байт")
	}
	return &Hasher{p: p, rand: rand.Reader}, nil
}

// #region hash
func (h *Hasher) Hash(pw domain.Password) (domain.PasswordHash, error) {
	salt := make([]byte, h.p.SaltLen)
	if _, err := io.ReadFull(h.rand, salt); err != nil {
		return domain.PasswordHash{}, fmt.Errorf("argon2id: соль: %w", err)
	}
	key := argon2.IDKey([]byte(pw.Plaintext()), salt, h.p.Iterations, h.p.MemoryKiB, h.p.Parallelism, h.p.KeyLen)
	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.p.MemoryKiB, h.p.Iterations, h.p.Parallelism, b64.EncodeToString(salt), b64.EncodeToString(key))
	return domain.NewPasswordHash(encoded)
}

func (h *Hasher) Verify(plaintext string, hash domain.PasswordHash) (bool, error) {
	p, salt, key, err := parse(hash.String())
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(plaintext), salt, p.Iterations, p.MemoryKiB, p.Parallelism, uint32(len(key)))
	// ConstantTimeCompare: обычное сравнение остановилось бы на первом несовпавшем байте,
	// и время ответа подсказало бы, сколько байт угадано.
	return subtle.ConstantTimeCompare(got, key) == 1, nil
}

// #endregion hash

func parse(s string) (Params, []byte, []byte, error) {
	fail := func(why string) (Params, []byte, []byte, error) {
		return Params{}, nil, nil, fmt.Errorf("argon2id: испорченный хеш: %s", why)
	}
	parts := strings.Split(s, "$") // "", "argon2id", "v=19", "m=…,t=…,p=…", соль, ключ
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return fail("не PHC-строка argon2id")
	}
	var v int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &v); err != nil || v != argon2.Version {
		return fail("версия")
	}
	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.MemoryKiB, &p.Iterations, &p.Parallelism); err != nil {
		return fail("параметры")
	}
	if p.Parallelism == 0 || p.Iterations == 0 || p.Iterations > maxIterations || p.MemoryKiB < 8*uint32(p.Parallelism) || p.MemoryKiB > maxMemoryKiB {
		return fail("параметры вне допустимых границ")
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil || len(salt) < 16 {
		return fail("соль")
	}
	key, err := b64.DecodeString(parts[5])
	if err != nil || len(key) < minKeyLen || len(key) > maxKeyLen {
		return fail("ключ")
	}
	return p, salt, key, nil
}
```

`argon2id_test.go` — с маленькими параметрами для скорости (`Params{MemoryKiB: 64, Iterations: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32}`): свой пароль проходит, чужой нет; два хеша одного пароля различаются (соль); хеш начинается с `$argon2id$v=19$m=64,t=1,p=1$`; хеш с другими параметрами проверяется по своим параметрам (параметры читаются из хеша); испорченные хеши (5 частей, `argon2i`, `v=16`, `m=999999999`, `t=0`, не-base64 соль, ключ 8 байт) → ошибка; `New` отвергает плохие параметры; `BenchmarkHashOWASP` с `OWASPMinimum` — для урока (реальные цифры).

`userbridge/bridge.go`:

```go
// Package userbridge — антикоррупционный слой между auth и user: переводит вызовы и ошибки
// одной модели в другую. Единственное место, где auth знает о контексте user (правило
// auth-context в .golangci.yml). В модуле 6 его заменит клиент сервиса users.
package userbridge

import (
	"context"
	"errors"
	"fmt"

	authdomain "messenger/internal/auth/domain"
	authusecase "messenger/internal/auth/usecase"
	userdomain "messenger/internal/user/domain"
	userusecase "messenger/internal/user/usecase"
)

// Узкие порты на use case'ы контекста user — объявлены здесь, у потребителя.
type Registrar interface {
	Execute(ctx context.Context, cmd userusecase.RegisterUserCommand) (userdomain.User, error)
}

type Finder interface {
	Execute(ctx context.Context, q userusecase.FindByUsernameQuery) (userdomain.User, error)
}

// Bridge реализует authusecase.ProfileRegistrar и authusecase.UserLookup.
type Bridge struct {
	register Registrar
	find     Finder
}

func New(register Registrar, find Finder) (*Bridge, error) {
	if register == nil || find == nil {
		return nil, errors.New("userbridge: nil-зависимость")
	}
	return &Bridge{register: register, find: find}, nil
}

// #region translate
func (b *Bridge) RegisterProfile(ctx context.Context, username, displayName string) (authusecase.Profile, error) {
	u, err := b.register.Execute(ctx, userusecase.RegisterUserCommand{Username: username, DisplayName: displayName})
	var invalid *userdomain.ValidationError
	switch {
	case err == nil:
	case errors.As(err, &invalid):
		// Ошибку данных переводим в модель auth с тем же полем: клиенту всё равно,
		// какой контекст проверял имя.
		return authusecase.Profile{}, &authdomain.ValidationError{Field: invalid.Field, Reason: invalid.Reason}
	case errors.Is(err, userdomain.ErrUsernameTaken):
		return authusecase.Profile{}, authdomain.ErrUsernameTaken
	default:
		return authusecase.Profile{}, fmt.Errorf("userbridge: %w", err)
	}
	id, err := toAuthID(u.ID())
	if err != nil {
		return authusecase.Profile{}, err
	}
	return authusecase.Profile{ID: id, Username: u.Username().String(), DisplayName: u.DisplayName().String(), CreatedAt: u.CreatedAt()}, nil
}

// #endregion translate

func (b *Bridge) UserIDByUsername(ctx context.Context, username string) (authdomain.UserID, error) {
	u, err := b.find.Execute(ctx, userusecase.FindByUsernameQuery{Username: username})
	var invalid *userdomain.ValidationError
	switch {
	case err == nil:
	case errors.As(err, &invalid), errors.Is(err, userdomain.ErrUserNotFound):
		// Некорректное имя для входа — то же «нет такого»: иначе по ответу видно правила имён.
		return authdomain.UserID{}, authdomain.ErrNoSuchUser
	default:
		return authdomain.UserID{}, fmt.Errorf("userbridge: %w", err)
	}
	return toAuthID(u.ID())
}

func toAuthID(id userdomain.UserID) (authdomain.UserID, error) {
	out, err := authdomain.ParseUserID(id.String())
	if err != nil {
		// Без %w: соседний контекст вернул не UUID — сбой сервера, а не ошибка клиента.
		return authdomain.UserID{}, fmt.Errorf("userbridge: контекст user вернул не UUID: %q", id.String())
	}
	return out, nil
}
```

`bridge_test.go`: фейки двух use case'ов user; все ветки перевода (успех, `ValidationError` с полем, `ErrUsernameTaken`, сбой с `errors.Is(err, context.Canceled)` сквозь обёртку; поиск: найден, не найден, некорректное имя, сбой).

- [ ] **Step 5: Контекст `user` — поиск по имени, `/me`**

- `user/usecase/ports.go`: `type UserByUsernameFinder interface { FindUserByUsername(ctx, username domain.Username) (domain.User, error) // ErrUserNotFound }`.
- `find_by_username.go`: `FindByUsernameQuery{Username string}`, `NewFindByUsername(users UserByUsernameFinder)`, `Execute` — `domain.NewUsername` → порт → обёртка `"find by username: %w"`.
- `user/adapters/memory/store.go`: `FindUserByUsername` через `byUsername`; `storetest` дополнить (найти по имени; нет — `ErrUserNotFound`; отменённый контекст).
- `user/adapters/httpapi`: регистрация уходит в `auth` — метод `RegisterUser`, узкий порт `userRegistrar` и аргумент `New` удаляются; новый `GetMe(w, r)` — личность из `principal` (свой помощник `caller`, как в `chat`) → `GetUser`. Тесты: `/v1/users/me` с личностью и без.
- Тест `user/usecase`: `FindByUsername` — найден, нет, некорректное имя (`ValidationError` поля `username`).

- [ ] **Step 6: Контракт, HTTP-адаптер `auth`, сборка (контракт)**

- `api/openapi.yaml`:
  - тег `auth` («Регистрация и вход»);
  - `POST /v1/users`: `tags: [auth]`, `operationId: signUp`, тело `SignUpRequest {username, display_name, password}` (`password`: `string`, `minLength: 15`, `maxLength: 128`, `format: password`, `writeOnly: true`), ответы как у прежней регистрации;
  - `GET /v1/users/me` (`tags: [users]`, `operationId: getMe`): `200` → `User`, `401`, `500`, `503`;
  - `RegisterUserRequest` удалить.
- `auth/adapters/httpapi/gen/config.yaml` с `include-tags: [auth]`, `generate.go` — как у соседей; `go generate ./...`.
- `auth/adapters/httpapi`: `Handler` с узким портом `signUpper`, `New(signUp, log)`, `Register(mux, mws...)`, `var _ gen.ServerInterface = (*Handler)(nil)`; `SignUp(w, r)` → 201 + `Location: /v1/users/{id}` + профиль; `problemFor`: `*authdomain.ValidationError` → `problem.Invalid`, `ErrUsernameTaken` → `CodeUsernameTaken`; `paramError` — как у соседей. Пароль **нигде** не логируется (тест: тело запроса с паролем → в буфере лога его нет).
- Правило `depguard` `adapters` уже запрещает `auth/adapters` из других адаптеров; `adapter-gen` из задачи 1 — дополнить путём `auth/adapters/httpapi/gen`, если механизм это требует.
- `main.go`: `credStore := authmemory.NewStore()`; `hasher, err := argon2id.New(argon2id.OWASPMinimum)`; `findByUsername := userusecase.NewFindByUsername(users)`; `bridge := userbridge.New(registerUser, findByUsername)`; `signUp := authusecase.NewSignUp(hasher, bridge, credStore)`; `authAPI := authhttp.New(signUp, log)`; `authAPI.Register(mux)`. `Login` в этой задаче **не** собирается — его маршрут появится в задаче 10. `publicRoutes` не меняется.

Run: `go mod tidy && go build ./... && go test -race ./... && golangci-lint run ./...` → PASS. Отдельно — нарушение правила: временно импортировать `messenger/internal/user/domain` в `auth/usecase/signup.go` → `golangci-lint` ругается текстом правила `auth-context`; вернуть.

- [ ] **Step 7: ADR-0004, бенчмарк, прогон через `curl`**

`0004-регистрация-через-два-контекста.md`: контекст (профиль — в `user`, учётные данные — в `auth`; одна HTTP-операция меняет два хранилища), варианты (пароль в `user`; регистрация в `user`, вызывающая `auth`; регистрация в `auth` через ACL `userbridge`; одна транзакция — сейчас невозможна: хранилища в памяти разные), решение (третий: в модели Вернона Identity & Access владеет регистрацией), последствия: окно неатомарности «профиль создан, пароль не сохранён» — такой пользователь не может войти, имя занято; вход для него отвечает тем же `invalid_credentials`; закрывается в 2a (unit of work, одна транзакция Postgres), в модуле 6 — сагой с компенсацией или outbox; порядок шагов выбран так, чтобы самый вероятный сбой (хешер) случался до побочных эффектов.

`go test -run '^$' -bench BenchmarkHashOWASP -benchmem ./internal/auth/adapters/argon2id/` — реальный вывод (время и память на один хеш) в отчёт; урок цитирует.

`curl`: регистрация с паролем из 14 символов → 400 `invalid_params[0].name == "password"`; с нормальным → 201; повтор имени → 409; `GET /v1/users/me` с `X-User-ID` (заглушка ещё жива) → профиль. Реальный вывод — в отчёт.

- [ ] **Step 8: Урок `13-passwords.md`**

Frontmatter: `title: 1.13. Пароли, вход и контекст auth`.

**Разделы:**
- **Проблема.** Утечки баз паролей случаются у всех (примеры крупных утечек с хранением в открытом виде или несолёным MD5/SHA-1 — выбрать 2 документированных случая со ссылками). Задача — сделать так, чтобы утёкшая база не превращалась в утёкшие пароли; и чтобы форма входа не рассказывала, кто у нас зарегистрирован.
- **Теория:**
  - `authentication` против `authorization`;
  - почему не хранить пароль, не шифровать его (ключ утечёт вместе с базой), не считать SHA-256 (миллиарды попыток в секунду на GPU — порядок величины со ссылкой на источник), что делает `salt` (радужные таблицы, одинаковые пароли);
  - `password-hashing`: медленные функции (bcrypt, scrypt, PBKDF2) и memory-hard (scrypt, Argon2); почему память важна против GPU/ASIC; `argon2id` = гибрид Argon2i и Argon2d; рекомендации OWASP (Argon2id → scrypt → bcrypt → PBKDF2 для FIPS) и почему bcrypt обрезает пароль после 72 байт;
  - параметры m, t, p: как выбирать под своё железо (цель — порядка десятков–сотен мс на хеш), реальное время из бенчмарка урока; формат PHC и переход на новые параметры при входе (перехеширование — описать, в коде не делаем, YAGNI до первой смены параметров);
  - политика паролей по NIST SP 800-63B-4: длина, без правил состава, без принудительной смены, проверка по списку утёкших паролей (у нас нет — модуль 6.7), нормализация Unicode (у нас нет — объяснить, чем грозит);
  - `user-enumeration`: через текст ответа и через `timing-attack`; фиктивный хеш; одинаковый ответ; что ещё выдаёт пользователей (регистрация «имя занято» — неизбежная утечка, и как с ней живут: rate limiting, CAPTCHA — модуль 6.7);
  - контекст `auth` (Identity & Access, Вернон), `anti-corruption-layer` `userbridge` (Эванс) и регистрация через два контекста: окно неатомарности, ADR-0004, `unit-of-work` в 2a и сага/outbox в модуле 6;
  - `shared-kernel` `kernel/ident` и правило трёх.
- **Визуализация:** `<LoginTiming />` и задания.
- **Код:** регионы `password-policy`, `signup`, `login`, `params`, `hash`, `translate`; бенчмарк с реальным выводом; сценарий `curl`.
- **Под капотом:** «Почему Argon2 требует памяти» — по RFC 9106: матрица блоков по 1 КиБ, проходы t, зависимость каждого блока от предыдущих (Argon2d — от данных, Argon2i — от индекса, Argon2id — первая половина первого прохода как i), почему атакующему невыгодно экономить память (компромисс время–память), почему i-режим стоек к атакам по побочным каналам; без формул, прозой.
- **Ошибки:** SHA-256 с солью «сойдёт»; одна соль на всех; сравнение хешей через `==`/`bytes.Equal`; разные ответы для «нет пользователя» и «не тот пароль»; ранний выход для несуществующего пользователя; логирование тела запроса; политика пароля при входе.
- **Компромисс:** параметры Argon2id — безопасность против задержки входа и нагрузки на CPU и память сервера (и DoS через вход — почему нужен rate limiting).
- **Самопроверка, источники:** OWASP Password Storage Cheat Sheet; NIST SP 800-63B-4 (§3.1.1); RFC 9106 (Argon2); OWASP Authentication Cheat Sheet (сообщения об ошибках); Vernon, «Implementing DDD», гл. 2 (Identity & Access); Evans, «DDD» — Anti-Corruption Layer, Shared Kernel; Фаулер, PoEAA — Unit of Work.
- **Что изменилось в мессенджере** (`… m01-l12 m01-l13`).

**Термины:** `authentication`, `authorization`, `password-hashing`, `salt`, `argon2id`, `timing-attack`, `user-enumeration`, `anti-corruption-layer`, `unit-of-work`, `shared-kernel`.

**Факты проверить:** NIST SP 800-63B-4 — номера разделов и цифры (15 / 64); OWASP — параметры Argon2id; поведение bcrypt после 72 байт в `golang.org/x/crypto/bcrypt` (возвращает ошибку `ErrPasswordTooLong` с v0.21? — проверить по исходникам v0.57.0); примеры утечек.

- [ ] **Step 9: Проверка, коммит, тег**

Run: `npm test && npm run typecheck && npm run check:messenger && npm run build` → PASS.

```bash
git add -A messenger course
git commit -m "feat(messenger): контекст auth — Argon2id, регистрация и вход, kernel/ident, ADR-0004; урок 1.13

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git tag m01-l13
```

---

### Task 10: Урок 1.14 «JWT» + `httpauth.Bearer` + `keygen` + сквозной тест + ADR-0003

**Files:**
- Create: `messenger/internal/auth/adapters/jwt/{doc.go,keys.go,signer.go,verifier.go,jwt_test.go}`
- Modify: `messenger/internal/platform/httpauth/{httpauth.go,httpauth_test.go}` (+`Bearer`, `Challenge`, `ErrInvalidToken`, `TokenVerifier`; `HeaderStub` удалить)
- Modify: `messenger/internal/auth/adapters/httpapi/*` (+`CreateToken`), `chat`/`user` `caller` (заголовок `WWW-Authenticate`)
- Modify: `messenger/internal/platform/problem/problem.go` (+`CodeInvalidCredentials`)
- Create: `messenger/cmd/keygen/main.go`; Create: `messenger/cmd/messenger/{app.go,app_test.go}`; Modify: `messenger/cmd/messenger/main.go`
- Modify: `messenger/api/openapi.yaml`, `gen/`, `go.mod` (`github.com/golang-jwt/jwt/v5@v5.3.1`), `messenger/.gitignore` (`*.pem`)
- Create: `messenger/docs/adr/0003-jwt-вместо-x-user-id.md`; Modify: `messenger/docs/adr/0002-x-user-id-до-jwt.md` (статус «заменён ADR-0003»)
- Create: `course/modules/01-go-service/14-jwt.md`

**Interfaces:**
- Consumes: задачи 4–9.
- Produces:
  - `authhttp.New(signUp, login, clock, log) (*Handler, error)` (сигнатура задачи 9 расширяется);
  - `jwt.Issuer`, `jwt.Audience`, `jwt.TTL`, `jwt.Leeway`; `jwt.NewSigner(key ed25519.PrivateKey, clock, ids) (*Signer, error)` (реализует `authusecase.TokenIssuer`); `jwt.NewVerifier(clock, keys ...ed25519.PublicKey) (*Verifier, error)` (реализует `httpauth.TokenVerifier`); `jwt.KeyID(ed25519.PublicKey) string`; `jwt.ParsePrivateKey([]byte) (ed25519.PrivateKey, error)`; `jwt.EncodePrivateKey(ed25519.PrivateKey) ([]byte, error)`;
  - `httpauth.TokenVerifier`, `httpauth.NewBearer(v) (*Bearer, error)`, `httpauth.ErrInvalidToken`, `httpauth.Challenge(err error) string`;
  - `newApp(cfg config, log *slog.Logger) (http.Handler, error)` в `cmd/messenger`.

- [ ] **Step 1: Тесты JWT — атаки**

`go get github.com/golang-jwt/jwt/v5@v5.3.1`. `jwt_test.go` (внешний пакет `jwt_test`; импорт библиотеки под именем `gojwt`) — полный код:

```go
package jwt_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"

	authdomain "messenger/internal/auth/domain"
	"messenger/internal/auth/adapters/jwt"
)

type clock struct{ t time.Time }

func (c *clock) Now() time.Time { return c.t }

type ids struct{}

func (ids) NewID() string { return "00000000-0000-7000-f000-000000000001" }

const alice = "00000000-0000-7000-8000-000000000001"

var t0 = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

func newKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return priv
}

func pub(k ed25519.PrivateKey) ed25519.PublicKey { return k.Public().(ed25519.PublicKey) }

// forge подписывает произвольные claims нашим ключом и нашим kid — так атакующий,
// укравший ключ, или наш же сервис с ошибкой выпустили бы «почти правильный» токен.
func forge(t *testing.T, key ed25519.PrivateKey, claims gojwt.RegisteredClaims) string {
	t.Helper()
	tok := gojwt.NewWithClaims(gojwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = jwt.KeyID(pub(key))
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func validClaims() gojwt.RegisteredClaims {
	return gojwt.RegisteredClaims{
		Issuer: jwt.Issuer, Subject: alice, Audience: gojwt.ClaimStrings{jwt.Audience},
		IssuedAt: gojwt.NewNumericDate(t0), NotBefore: gojwt.NewNumericDate(t0), ExpiresAt: gojwt.NewNumericDate(t0.Add(jwt.TTL)),
	}
}

func TestIssueAndVerify(t *testing.T) {
	key := newKey(t)
	clk := &clock{t: t0}
	signer, err := jwt.NewSigner(key, clk, ids{})
	if err != nil {
		t.Fatal(err)
	}
	uid, err := authdomain.ParseUserID(alice)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := signer.Issue(uid)
	if err != nil {
		t.Fatal(err)
	}
	if !tok.ExpiresAt.Equal(t0.Add(jwt.TTL)) {
		t.Fatalf("срок %v, ждали %v", tok.ExpiresAt, t0.Add(jwt.TTL))
	}
	v, err := jwt.NewVerifier(clk, pub(key))
	if err != nil {
		t.Fatal(err)
	}
	for _, at := range []time.Duration{0, jwt.TTL + jwt.Leeway - time.Second} {
		clk.t = t0.Add(at)
		sub, err := v.Verify(t.Context(), tok.Value)
		if err != nil || sub != alice {
			t.Fatalf("через %v: sub %q, ошибка %v", at, sub, err)
		}
	}
}

// #region attacks
func TestVerifierRejects(t *testing.T) {
	key, other := newKey(t), newKey(t)
	good := forge(t, key, validClaims())
	parts := strings.Split(good, ".")
	tests := []struct {
		name  string
		token func(t *testing.T) string
		now   time.Time
	}{
		{"alg: none", func(t *testing.T) string {
			tok := gojwt.NewWithClaims(gojwt.SigningMethodNone, validClaims())
			tok.Header["kid"] = jwt.KeyID(pub(key))
			s, err := tok.SignedString(gojwt.UnsafeAllowNoneSignatureType)
			if err != nil {
				t.Fatal(err)
			}
			return s
		}, t0},
		{"HS256 с открытым ключом в роли секрета", func(t *testing.T) string {
			tok := gojwt.NewWithClaims(gojwt.SigningMethodHS256, validClaims())
			tok.Header["kid"] = jwt.KeyID(pub(key))
			s, err := tok.SignedString([]byte(pub(key)))
			if err != nil {
				t.Fatal(err)
			}
			return s
		}, t0},
		{"чужой ключ, наш kid", func(t *testing.T) string {
			tok := gojwt.NewWithClaims(gojwt.SigningMethodEdDSA, validClaims())
			tok.Header["kid"] = jwt.KeyID(pub(key))
			s, err := tok.SignedString(other)
			if err != nil {
				t.Fatal(err)
			}
			return s
		}, t0},
		{"неизвестный kid", func(t *testing.T) string { return forge(t, other, validClaims()) }, t0},
		{"изменён payload", func(t *testing.T) string {
			c := validClaims()
			c.Subject = "00000000-0000-7000-8000-000000000666"
			forged := strings.Split(forge(t, key, c), ".")
			return parts[0] + "." + forged[1] + "." + parts[2] // чужие данные со старой подписью
		}, t0},
		{"обрезанная подпись", func(*testing.T) string { return good[:len(good)-4] }, t0},
		{"подпись не base64url", func(*testing.T) string { return parts[0] + "." + parts[1] + ".!!!" }, t0},
		{"просрочен сверх допуска", func(*testing.T) string { return good }, t0.Add(jwt.TTL + jwt.Leeway + time.Second)},
		{"выпущен в будущем сверх допуска", func(*testing.T) string { return good }, t0.Add(-jwt.Leeway - time.Minute)},
		{"чужой aud", func(t *testing.T) string {
			c := validClaims()
			c.Audience = gojwt.ClaimStrings{"billing-api"}
			return forge(t, key, c)
		}, t0},
		{"чужой iss", func(t *testing.T) string {
			c := validClaims()
			c.Issuer = "evil"
			return forge(t, key, c)
		}, t0},
		{"без exp", func(t *testing.T) string {
			c := validClaims()
			c.ExpiresAt = nil
			return forge(t, key, c)
		}, t0},
		{"без sub", func(t *testing.T) string {
			c := validClaims()
			c.Subject = ""
			return forge(t, key, c)
		}, t0},
		{"мусор", func(*testing.T) string { return "это.не.токен" }, t0},
		{"пусто", func(*testing.T) string { return "" }, t0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := jwt.NewVerifier(&clock{t: tt.now}, pub(key))
			if err != nil {
				t.Fatal(err)
			}
			if sub, err := v.Verify(t.Context(), tt.token(t)); err == nil {
				t.Fatalf("токен принят, sub %q", sub)
			}
		})
	}
}

// #endregion attacks

func TestKeyRoundTrip(t *testing.T) {
	key := newKey(t)
	pemBytes, err := jwt.EncodePrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	got, err := jwt.ParsePrivateKey(pemBytes)
	if err != nil || !got.Equal(key) {
		t.Fatalf("ключ после PEM другой: %v", err)
	}
	if _, err := jwt.ParsePrivateKey([]byte("-----BEGIN RSA PRIVATE KEY-----\nAAAA\n-----END RSA PRIVATE KEY-----\n")); err == nil {
		t.Fatal("RSA-ключ принят как Ed25519")
	}
}

// Отпечаток RFC 7638 для ключа из RFC 8037 (приложение A.3): kid = kPrK_qmxVWaYVA9wwBF6Iuo3vVzz7TxHCTwXBygrS4k.
func TestKeyIDMatchesRFC8037(t *testing.T) {
	x, err := base64.RawURLEncoding.DecodeString("11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo")
	if err != nil {
		t.Fatal(err)
	}
	if got := jwt.KeyID(ed25519.PublicKey(x)); got != "kPrK_qmxVWaYVA9wwBF6Iuo3vVzz7TxHCTwXBygrS4k" {
		t.Fatalf("kid %q", got)
	}
}
```

(Значения из RFC 8037 A.3 — сверить с текстом RFC при реализации.) Run: `go test ./internal/auth/adapters/jwt/` → FAIL (пакета нет).

- [ ] **Step 2: Реализация `jwt`**

`doc.go`: «выпуск и проверка JWT (RFC 7519) с подписью EdDSA (Ed25519, RFC 8037). Signer реализует порт TokenIssuer контекста auth, Verifier — порт TokenVerifier из platform/httpauth».

`keys.go`:

```go
package jwt

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
)

// KeyID — отпечаток открытого ключа по RFC 7638: SHA-256 от JWK с обязательными полями
// в алфавитном порядке. Детерминирован: один ключ — один kid на любой машине.
func KeyID(pub ed25519.PublicKey) string {
	jwk := fmt.Sprintf(`{"crv":"Ed25519","kty":"OKP","x":"%s"}`, base64.RawURLEncoding.EncodeToString(pub))
	sum := sha256.Sum256([]byte(jwk))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func ParsePrivateKey(pemBytes []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, errors.New("jwt: нужен PEM-блок PRIVATE KEY (PKCS#8)")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("jwt: PKCS#8: %w", err)
	}
	ed, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("jwt: ключ %T, нужен Ed25519", key)
	}
	return ed, nil
}

func EncodePrivateKey(key ed25519.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("jwt: PKCS#8: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}
```

`signer.go`:

```go
package jwt

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"

	"messenger/internal/auth/domain"
	"messenger/internal/auth/usecase"
)

// #region claims
const (
	Issuer   = "messenger"
	Audience = "messenger-api"
	// TTL короткий: отозвать выпущенный JWT нельзя (он проверяется без обращения к нам),
	// поэтому его срок — это и есть худшее время жизни украденного токена. Refresh-токены,
	// которые позволяют не логиниться каждые 15 минут, — модуль 6.7.
	TTL = 15 * time.Minute
	// Leeway — допуск на расхождение часов выпустившего и проверяющего.
	Leeway = 30 * time.Second
)

// #endregion claims

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() string
}

type Signer struct {
	key   ed25519.PrivateKey
	kid   string
	clock Clock
	ids   IDGenerator
}

func NewSigner(key ed25519.PrivateKey, clock Clock, ids IDGenerator) (*Signer, error) {
	if len(key) != ed25519.PrivateKeySize || clock == nil || ids == nil {
		return nil, errors.New("jwt signer: нужен ключ Ed25519, часы и генератор id")
	}
	return &Signer{key: key, kid: KeyID(key.Public().(ed25519.PublicKey)), clock: clock, ids: ids}, nil
}

// #region issue
func (s *Signer) Issue(userID domain.UserID) (usecase.Token, error) {
	// NumericDate — секунды; обрезаем сразу, чтобы ExpiresAt в ответе совпал с exp в токене.
	now := s.clock.Now().UTC().Truncate(time.Second)
	exp := now.Add(TTL)
	claims := gojwt.RegisteredClaims{
		Issuer:    Issuer,
		Subject:   userID.String(),
		Audience:  gojwt.ClaimStrings{Audience},
		ExpiresAt: gojwt.NewNumericDate(exp),
		NotBefore: gojwt.NewNumericDate(now),
		IssuedAt:  gojwt.NewNumericDate(now),
		ID:        s.ids.NewID(), // jti — чтобы в 6.7 можно было вести список отозванных
	}
	tok := gojwt.NewWithClaims(gojwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = s.kid
	signed, err := tok.SignedString(s.key)
	if err != nil {
		return usecase.Token{}, fmt.Errorf("jwt: подпись: %w", err)
	}
	return usecase.Token{Value: signed, ExpiresAt: exp}, nil
}

// #endregion issue
```

`verifier.go`:

```go
package jwt

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"

	gojwt "github.com/golang-jwt/jwt/v5"
)

type Verifier struct {
	keys  map[string]ed25519.PublicKey // kid → ключ; несколько — на время смены ключа
	clock Clock
}

func NewVerifier(clock Clock, keys ...ed25519.PublicKey) (*Verifier, error) {
	if clock == nil || len(keys) == 0 {
		return nil, errors.New("jwt verifier: нужны часы и хотя бы один ключ")
	}
	byKID := make(map[string]ed25519.PublicKey, len(keys))
	for _, k := range keys {
		if len(k) != ed25519.PublicKeySize {
			return nil, errors.New("jwt verifier: ключ не Ed25519")
		}
		byKID[KeyID(k)] = k
	}
	return &Verifier{keys: byKID, clock: clock}, nil
}

// #region verify
// Verify возвращает sub проверенного токена. Алгоритм — только EdDSA: поле alg пишет
// автор токена, и верить ему — значит дать атакующему выбрать none или HS256 с нашим
// открытым ключом в роли секрета. Ключ — только из своего списка по kid.
func (v *Verifier) Verify(_ context.Context, token string) (string, error) {
	var claims gojwt.RegisteredClaims
	_, err := gojwt.ParseWithClaims(token, &claims, v.key,
		gojwt.WithValidMethods([]string{gojwt.SigningMethodEdDSA.Alg()}),
		gojwt.WithIssuer(Issuer),
		gojwt.WithAudience(Audience),
		gojwt.WithExpirationRequired(),
		gojwt.WithIssuedAt(),
		gojwt.WithLeeway(Leeway),
		gojwt.WithTimeFunc(v.clock.Now),
	)
	if err != nil {
		return "", fmt.Errorf("jwt: %w", err)
	}
	if claims.Subject == "" {
		return "", errors.New("jwt: в токене нет sub")
	}
	return claims.Subject, nil
}

// #endregion verify

func (v *Verifier) key(t *gojwt.Token) (any, error) {
	kid, _ := t.Header["kid"].(string)
	k, ok := v.keys[kid]
	if !ok {
		return nil, fmt.Errorf("неизвестный kid %q", kid)
	}
	return k, nil
}
```

`ctx` в `Verify` не используется, но порт его требует: в модуле 6 проверка может ходить за ключами (JWKS). Run: `go test -race ./internal/auth/adapters/jwt/` → PASS.

- [ ] **Step 3: `httpauth.Bearer` вместо заглушки**

В `httpauth.go` — удалить регион `header-stub` и `HeaderStub`; добавить (регион `bearer`):

```go
// ErrInvalidToken — токен предъявлен, но не прошёл проверку.
var ErrInvalidToken = errors.New("токен не прошёл проверку")

// TokenVerifier — порт проверки токена, объявлен здесь, у потребителя. Реализация —
// auth/adapters/jwt; её подставляет composition root.
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (userID string, err error)
}

// Bearer — аутентификация по заголовку Authorization: Bearer <токен> (RFC 6750).
type Bearer struct{ verifier TokenVerifier }

func NewBearer(v TokenVerifier) (*Bearer, error) {
	if v == nil {
		return nil, errors.New("httpauth: nil-зависимость")
	}
	return &Bearer{verifier: v}, nil
}

func (b *Bearer) Authenticate(r *http.Request) (string, error) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	// Схема в HTTP нечувствительна к регистру (RFC 9110 §11.1). Другая схема (Basic) —
	// для нас то же, что отсутствие данных: их нечем проверить.
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return "", ErrNoCredentials
	}
	id, err := b.verifier.Verify(r.Context(), strings.TrimSpace(token))
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	return id, nil
}

// Challenge — значение WWW-Authenticate для ответа 401 (RFC 6750 §3): error="invalid_token",
// если токен был, но не прошёл; без error — если токена не было вовсе. Почему именно
// не прошёл — клиенту не сообщаем: это подсказка атакующему.
func Challenge(err error) string {
	if errors.Is(err, ErrInvalidToken) {
		return `Bearer realm="messenger", error="invalid_token"`
	}
	return `Bearer realm="messenger"`
}
```

`httpauth_test.go`: тесты заглушки заменить на: нет заголовка / `Basic …` / `Bearer` без токена → `ErrNoCredentials`; `bearer x` (регистр схемы) → токен `x` дошёл до фейк-верификатора; верификатор отказал → `errors.Is(err, ErrInvalidToken)` и исходная ошибка тоже видна; `Challenge` для обоих случаев.

- [ ] **Step 4: Вход `POST /v1/tokens`, контракт, `caller` (контракт)**

- `problem.go`: `CodeInvalidCredentials Code = "invalid_credentials"` → `{http.StatusUnauthorized, "Неверное имя пользователя или пароль"}`.
- `api/openapi.yaml`:
  - `securitySchemes`: `userIdHeader` удалить; `bearerAuth: {type: http, scheme: bearer, bearerFormat: JWT}`; глобальный `security: [{bearerAuth: []}]`;
  - `POST /v1/tokens` (`tags: [auth]`, `operationId: createToken`, `security: []`): тело `CreateTokenRequest {username, password}` (`password` — `writeOnly`, без `minLength`: при входе политику не применяем); `200` → `TokenResponse {access_token: string, token_type: enum [Bearer], expires_in: integer}` + заголовки `Cache-Control` и `Pragma`; `400`, `401` (`invalid_credentials`), `413`, `415`, `500`, `503`;
  - `components.responses.Unauthorized`: заголовок `WWW-Authenticate` (`required: true`);
  - `ProblemCode`: + `invalid_credentials`;
  - регион `security` вокруг `securitySchemes` и `createToken`.
- `go generate ./...`.
- `auth/adapters/httpapi`: узкий порт `loginer`, `New(signUp, login, log)`; `CreateToken(w, r)` — `httpjson.Decode` → `Login.Execute` → `200` с `Cache-Control: no-store` и `Pragma: no-cache` (RFC 6749 §5.1 — от OAuth 2.0 берём только форму ответа, не протокол; урок говорит это прямо), `expires_in` — секунды до `ExpiresAt` по часам сервера (часы — узкий порт `Clock`, приходит через `New(signUp, login, clock, log)`); `ErrInvalidCredentials` → `CodeInvalidCredentials` **и** заголовок `WWW-Authenticate: Bearer realm="messenger"`: RFC 9110 §15.5.2 требует его у любого ответа 401 (многие API этим пренебрегают — урок называет это ошибкой). Поэтому `401` у `createToken` — тот же `components.responses.Unauthorized` с обязательным заголовком.
- `chat` и `user` `caller`: при 401 ставить `WWW-Authenticate: httpauth.Challenge(httpauth.ErrNoCredentials)`.
- Тесты адаптера `auth`: успешный вход (фейк `Login`), заголовки `Cache-Control`/`Pragma`, `token_type`, `expires_in`; `invalid_credentials`; пароль не попадает в журнал.

- [ ] **Step 5: `keygen`, `app.go`, `main.go`**

`cmd/keygen/main.go`:

```go
// Command keygen создаёт закрытый ключ подписи JWT: Ed25519 в PKCS#8 PEM, права 0600.
// Существующий файл не перезаписывает: новый ключ сделал бы недействительными все
// выданные токены — такое действие должно быть осознанным (удалите файл сами).
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"os"

	"messenger/internal/auth/adapters/jwt"
)

func main() {
	out := flag.String("out", "", "куда записать закрытый ключ, например jwt.pem")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "keygen:", err)
		os.Exit(1)
	}
}

func run(out string) error {
	if out == "" {
		return errors.New("укажите -out")
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("генерация ключа: %w", err)
	}
	pemBytes, err := jwt.EncodePrivateKey(priv)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("файл ключа: %w", err)
	}
	if _, err := f.Write(pemBytes); err != nil {
		_ = f.Close()
		return fmt.Errorf("запись ключа: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("запись ключа: %w", err)
	}
	fmt.Printf("ключ записан в %s, kid %s\n", out, jwt.KeyID(pub))
	return nil
}
```

`cmd/messenger/app.go` (контракт): `type config struct{ Addr string; SigningKey ed25519.PrivateKey; HasherParams argon2id.Params }` (`HasherParams` по умолчанию `argon2id.OWASPMinimum`; меняет его только сквозной тест); `func loadConfig() (config, error)` — `MESSENGER_ADDR` (по умолчанию `127.0.0.1:8080`), `MESSENGER_JWT_KEY_FILE` обязателен: пусто → ошибка «задайте MESSENGER_JWT_KEY_FILE — ключ создаёт go run ./cmd/keygen -out jwt.pem»; чтение файла и `jwt.ParsePrivateKey`; `func newApp(cfg config, log *slog.Logger) (http.Handler, error)` — **вся** сборка из `run` (хранилища, use case'ы, `signer := jwt.NewSigner(cfg.SigningKey, clk, ids)`, `jwt.NewVerifier(clk, pub)`, `authusecase.NewLogin(bridge, credStore, hasher, signer)`, адаптеры, mux, цепочка с `httpauth.Middleware(bearer, publicRoutes, unauthenticated)`); регионы `wiring` и `chain` переезжают сюда (урок 1.7 ссылается на `wiring` — обновить путь `<<<` в 1.7 и 1.8). `publicRoutes = []string{"POST /v1/users", "POST /v1/tokens"}`. `unauthenticated(w, r, err)` ставит `WWW-Authenticate: httpauth.Challenge(err)` и пишет `CodeUnauthenticated` (`detail` одинаковый для любой причины). `main.go` — только `loadConfig`, `newApp`, сервер, сигналы. Комментарий у `defaultAddr` поменять: loopback по умолчанию остаётся — сервис без TLS (TLS — модуль 6.7 / шлюз); `MESSENGER_ADDR=:8080` для Docker (1c).

`.gitignore`: `*.pem` — ключ не коммитится.

- [ ] **Step 6: Сквозной тест**

`cmd/messenger/app_test.go` (пакет `main`) — сценарий урока целиком через `httptest.NewServer(openapitest.New(t).Wrap(handler))` и настоящий `http.Client`:
1. регистрация Алисы, Боба и Евы → 201;
2. вход Алисы → `access_token`; неверный пароль → 401 `invalid_credentials`; нет такого пользователя → тот же ответ байт в байт, кроме `request_id`;
3. `GET /v1/users/me` с токеном → Алиса; без токена → 401 с `WWW-Authenticate: Bearer realm="messenger"`; с испорченным токеном → 401 с `error="invalid_token"`;
4. личный чат Алисы и Боба; группа с `Idempotency-Key` → 201; повтор → 201 + `Idempotent-Replayed`; тот же ключ, другое тело → 422;
5. 7 сообщений с разными `client_message_id`; повтор одного → 200; тот же `client_message_id`, другой текст → 409;
6. история Бобом по 3 с курсором до конца — 7 сообщений без повторов; Ева читает → 403;
7. `GET /v1/chats` Алисы — два чата, сначала самый активный;
8. в конце — `if u := v.Uncovered(); len(u) > 0 { t.Fatalf("операции контракта без проверенного ответа: %v", u) }`.

Ключ — `ed25519.GenerateKey` в тесте; хешер — `argon2id.New` с маленькими параметрами через поле `config` (`config.HasherParams`, по умолчанию `argon2id.OWASPMinimum`) — иначе тест медленный; это единственная разница с боевой сборкой.

Если роутер kin-openapi сопоставит `/v1/users/me` с шаблоном `/v1/users/{id}` (и `Uncovered` покажет `GET /v1/users/me`) — разобраться по исходникам `routers/gorillamux` v0.149.0, исправить в `openapitest` (например, сортировкой путей перед построением роутера) и описать в отчёте.

Run: `go test -race ./cmd/messenger/` → PASS.

- [ ] **Step 7: ADR-0003, прогон через `curl`**

`0003-jwt-вместо-x-user-id.md`: контекст (заглушка ADR-0002; в модуле 6 токен проверяют шлюз и несколько сервисов), варианты (сессии в БД с непрозрачным токеном — как у Эдвардса; JWT HS256 с общим секретом; JWT с подписью открытым ключом; mTLS/OAuth2 — позже), решение (JWT EdDSA, 15 минут, `kid`, проверки из `verify`), последствия: нельзя мгновенно отозвать (срок 15 мин; список отозванных `jti` и refresh-токены — 6.7); нет JWKS (появится с потребителем — шлюзом модуля 6); ключ из файла `MESSENGER_JWT_KEY_FILE`, ротация — 6.7; `X-User-ID` удалён. В ADR-0002 — статус «заменён ADR-0003».

Прогон (вывод — в отчёт; урок цитирует): `go run ./cmd/keygen -out jwt.pem`; второй запуск — отказ; `MESSENGER_JWT_KEY_FILE=jwt.pem go run ./cmd/messenger`; без переменной — понятная ошибка запуска; сценарий шага 6 через `curl` (токен в переменную `TOKEN`, `jq` если есть); раскодировать заголовок и payload токена (`cut -d. -f2 | basenc --base64url -d`); запрос со старым `X-User-ID` → 401; подделка `sub` в payload → 401 `invalid_token`.

- [ ] **Step 8: Урок `14-jwt.md`**

Frontmatter: `title: 1.14. JWT: кто вы и почему мы вам верим`.

**Разделы:**
- **Проблема.** `X-User-ID` из 1.8: `curl -H 'X-User-ID: <id Боба>'` — и вы Боб. Нужен способ, при котором сервер может проверить личность, не веря клиенту на слово, — и в модуле 6 это должны уметь шлюз и несколько сервисов.
- **Теория:**
  - аутентификация против авторизации (напоминание из 1.13), что даёт вход: доказательство «это Алиса» на 15 минут;
  - сессии в БД (непрозрачный токен, Эдвардс, гл. 15–16 — почему он так выбрал: простота отзыва) против самодостаточного токена; таблица: отзыв, нагрузка на БД, распределённая проверка, размер;
  - `jwt` по RFC 7519/7515: три части, base64url, это **не** шифрование (JWE — отдельная история); `claim`'ы: `iss`, `sub`, `aud`, `exp`, `nbf`, `iat`, `jti` — зачем каждый;
  - `digital-signature`: HMAC (общий секрет — каждый проверяющий может и выпускать) против `public-key-cryptography` (закрытым подписывает только auth); почему Ed25519 (детерминированная подпись, короткие ключи, нет подводных камней RSA-паддинга и ECDSA-нонса); `kid` и смена ключей; отпечаток RFC 7638;
  - обязательные проверки и атаки: `alg: none`; путаница алгоритмов (HS256 с открытым ключом) — история уязвимостей библиотек 2015 г.; подмена `kid` (инъекция пути/SQL в поиске ключа); чужой `aud`; часы и `Leeway`; почему проверка порядка «сначала алгоритм и ключ — потом claims»;
  - `bearer-token` по RFC 6750: заголовок, `WWW-Authenticate`, почему не в query-строке (журналы, `Referer`); почему токен нельзя хранить в `localStorage` на вебе (XSS) — кратко, подробно 6.7;
  - `token-expiry` как замена отзыву; что придёт в 6.7: refresh-токены и их ротация, список отозванных `jti`, JWKS для шлюза;
  - эндпоинт выдачи токена: форма ответа по RFC 6749 §5.1 (`access_token`, `token_type`, `expires_in`, `no-store`) — и почему это **не** OAuth 2.0 (нет клиентов, грантов, scopes; OAuth/OIDC — 6.7);
  - что изменилось в архитектуре: одна реализация `Authenticator` заменила другую — хендлеры не тронуты (вспомнить обещание из 1.7 и 1.8); порт `TokenVerifier` у потребителя в `platform`, реализация в `auth`.
- **Визуализация:** `<JwtSandbox />` и 5 заданий (подделать `sub`; `alg: none`; промотать часы; чужой `aud`; найти, какое правило сработало первым).
- **Код:** регионы `claims`, `issue`, `verify`, `attacks`, `bearer`, YAML `security`; `keygen` целиком; сценарий `curl` с реальным выводом, включая раскодированный токен.
- **Под капотом:** «Как устроена подпись Ed25519» — прозой: кривая Эдвардса над полем по модулю 2²⁵⁵−19, закрытый ключ → скаляр и префикс через SHA-512, детерминированный нонс из префикса и сообщения (почему это убирает класс атак, сломавший подписи PlayStation 3 с ECDSA), проверка — уравнение на точках кривой; почему подпись 64 байта, ключ 32; ссылки на RFC 8032 и исходники `crypto/ed25519` в Go.
- **Ошибки:** доверять `alg` из токена; HS256 с секретом в нескольких сервисах; длинный срок «чтобы не логиниться»; хранить в JWT то, что не должен видеть клиент; проверять подпись, но не `aud`/`exp`; токен в URL; ключ в репозитории.
- **Компромисс:** самодостаточный токен — проверка без похода в БД и в сервис auth против невозможности мгновенного отзыва.
- **Самопроверка, источники:** RFC 7519, 7515, 8037, 7638, 6750, 6749 §5.1, 8725 (JWT Best Current Practices — главный); OWASP JSON Web Token Cheat Sheet; Эдвардс, «Let's Go Further» (гл. об аутентификации — токены в БД); Auth0 — «Critical vulnerabilities in JSON Web Token libraries» (2015); документация `golang-jwt/jwt/v5`.
- **Что изменилось в мессенджере** (`… m01-l13 m01-l14`) и итог части 1b: дерево `messenger/`, таблица «урок → что появилось»; что дальше — 1c (Docker изнутри и CI).

**Термины:** `jwt`, `claim`, `digital-signature`, `public-key-cryptography`, `bearer-token`, `token-expiry`.

- [ ] **Step 9: Проверка, коммит, тег**

Run: `go mod tidy && npm test && npm run typecheck && npm run check:messenger && npm run build` → PASS. `grep -rn "X-User-ID\|HeaderStub" messenger --include=*.go` → пусто (кроме, возможно, комментариев в тестах про «старый заголовок → 401»).

```bash
git add -A messenger course
git commit -m "feat(messenger): JWT на Ed25519, Bearer вместо X-User-ID, keygen, сквозной тест, ADR-0003; урок 1.14

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git tag m01-l14
```

---

### Task 11: Итоговая проверка, ссылки, README, бэклог

**Files:**
- Modify: `course/modules/01-go-service/index.md`, уроки 1.7–1.14 (ссылки «следующий урок»), `README.md`, `docs/superpowers/backlog-after-module-0.md`

- [ ] **Step 1: Теги**

```bash
for t in m01-l08 m01-l09 m01-l10 m01-l11 m01-l12 m01-l13 m01-l14; do
  git worktree add -f "/tmp/claude-1000/m01-check-$t" "$t" >/dev/null &&
  (cd "/tmp/claude-1000/m01-check-$t/messenger" && go build ./... && go test -race ./... >/dev/null && echo "$t OK") ;
  git worktree remove --force "/tmp/claude-1000/m01-check-$t"
done
```

Expected: семь строк `OK`. Падение — исправить в задаче, которой принадлежит тег, перенести тег (`git tag -f`), записать в отчёт.

- [ ] **Step 2: Полный прогон**

Run: `npm test && npm run typecheck && npm run check:go && npm run check:labs && npm run check:messenger && npm run build` → всё зелёное.

- [ ] **Step 3: Сценарий с нуля**

Свежий клон каталога во временную папку (или `git stash` не нужен — рабочее дерево чистое): `go run ./cmd/keygen -out /tmp/…/jwt.pem`, сервер, сценарий `curl` из урока 1.14 по тексту урока — вывод совпадает по форме (id, время, токены — другие).

- [ ] **Step 4: Playwright**

Уроки 1.8–1.14, `index.md`, `/glossary` — 375 и 1280 px, тёмная и светлая темы:
- нет горизонтальной прокрутки (особенно — длинные токены и курсоры в блоках кода и в песочнице `jwt`);
- строка «Модуль 1 · Урок N»;
- метки `<Part>` и цвета заголовков;
- песочницы реагируют на действия, плееры — на шаги;
- ⛶ с удержанием фокуса, возвратом на кнопку и **закрытием по настоящему нажатию Esc** (`browser_press_key`) — закрывает пункт бэклога.

Скриншоты смотреть, а не только сохранять.

- [ ] **Step 5: Связи, README, бэклог**

- `index.md`: цели модуля дополнить пунктами 1b; список уроков 1.8–1.14; абзац «уроки 1c появятся позже» оставить только про 1c.
- В конце уроков 1.7–1.14 — ссылка на следующий.
- `README.md`, раздел «Мессенджер»: `go run ./cmd/keygen -out jwt.pem`, `MESSENGER_JWT_KEY_FILE`, `go generate ./...` после правки контракта, `api/openapi.yaml`, теги до `m01-l14`.
- Бэклог: закрыть «имена полей против `{id}`», «405 текстом», «дублирование `queryInt`», «`Canceled` от живого клиента в тестах адаптеров», «Esc живой клавишей»; добавить новые отложенные пункты из отчётов задач 1–10 (в том числе: фоновая очистка ключей идемпотентности — 1.5; перехеширование паролей при смене параметров; проверка паролей по списку утёкших; JWKS; refresh-токены; rate limiting входа — 6.7).

- [ ] **Step 6: Коммит**

```bash
git add README.md docs course messenger
git commit -m "docs: связи уроков 1b, README мессенджера, бэклог

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Тег `m01-l14` на этот коммит **не** переносится, если коммит не меняет `messenger/` (если меняет — перенести и прогнать шаг 1 для `m01-l14`).
