# Модуль 1, часть 1a «Архитектура Go-сервиса и домен мессенджера» — план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Уроки 1.1–1.7 и первый код мессенджера в `messenger/`: домен «пользователи, чаты, сообщения», use case'ы, хранилища в памяти за портами, контрактные тесты, тонкий HTTP-адаптер, `depguard` на правила слоёв, проверка `npm run check:messenger`, две песочницы и пять плееров.

**Architecture:**
- **Мессенджер** — отдельный Go-модуль `messenger/` (module `messenger`, go 1.26). Раскладка по контекстам `user` и `chat`, внутри слои `domain` ← `usecase` ← `adapters`; сборка в `cmd/messenger/main.go`. Правила зависимостей проверяет `golangci-lint` (`depguard`).
- **Код растёт урок за уроком.** Задачи 4–10 — по одному уроку: код урока + текст урока + git-тег `m01-lNN` на завершающем коммите.
- **Визуализации** — чистые TS-модули в `course/viz/simulations/` с тестами Vitest, тонкие рендереры и сцены в `VizFrame` (задачи 2–3, до уроков).

**Tech Stack:**
- курс: VitePress 1.6.4, Vue 3.5.43, TypeScript 5.9.3, Vitest 3.2.7, Node 26;
- мессенджер: Go 1.26, `github.com/google/uuid` v1.6.0 (UUIDv7; в stdlib 1.26 генератора UUID нет — проверено по `$(go env GOROOT)/src`), `golangci-lint` 2.12.2.

**Spec:**
- `docs/superpowers/specs/2026-10-13-module-1a-service-architecture-design.md` — главный.
- `docs/superpowers/specs/2026-10-09-course-platform-design.md` — §1, §2.2–2.5.

**Отступления от стандарта плана (как в 0b):**
- **Домен, use case'ы, хранилища, контрактные тесты, песочницы и скрипт регионов** даны полным кодом.
- **HTTP-обработчики, `main.go`, рендереры, сцены, плееры и тексты уроков** даны **контрактом**: поведение, обязательные свойства, имена. Ревьюер проверяет контракт.

## Global Constraints

- Команды npm — из `/home/idel/DevGo/system-desine/course`; Go-команды мессенджера — из `/home/idel/DevGo/system-desine/messenger`. Версии — точные.
- Язык контента — русский; английский оригинал термина — в скобках при первом упоминании. Каждый термин при первом появлении в уроке — `<T id="…">` (id в двойных кавычках; неизвестный id — ошибка сборки). Сетевой `port` уже занят: порт из «портов и адаптеров» — `hexagonal-port`.
- Тексты уроков — по основной спецификации:
  - §1 «Голос автора»: доктор наук, объясняющий просто; упрощение без искажения, упрощения помечены `<Callout type="simplified">`;
  - §2.2 — все 10 правил, включая самодостаточность;
  - §2.3 — у каждой визуализации полноэкранный режим;
  - §2.4 — `<Part>` у каждого `##`; `<N>` для 3–8 ключевых чисел (только результат формулы или измерения, не константы и не операнды); `<Deep>` 150–500 слов (прозой, без кода).
- Чистые TS-модули не импортируют `vue` и не трогают DOM. В компонентах DOM — только в `onMounted`, обработчиках и `watch` без `immediate`; слушатели снимаются в `onBeforeUnmount`.
- Цвета только через CSS-переменные (`--viz-*`, `--vp-c-*`, `--course-*`, `--part-*`), обе темы, контраст текста ≥ 4,5:1, `prefers-reduced-motion`, 375 px без горизонтальной прокрутки, каждая сцена в `VizFrame`.
- Go-код мессенджера — стандарт пользователя (`~/.claude/CLAUDE.md`) и §2.5:
  - зависимости только внутрь: `domain` ← `usecase` ← `adapters` ← `cmd`; порт объявляется в пакете потребителя;
  - доменные структуры без тегов `json:`/`db:`; DTO и мапперы — в адаптерах;
  - один сценарий — один use case с методом `Execute`; конструкторы `NewX(deps…) (*X, error)` принимают интерфейсы и проверяют `nil`;
  - `context.Context` — первый аргумент всего, что ходит вовне; не хранится в структурах;
  - ошибки: доменные — `var ErrX = errors.New(…)` и `*ValidationError`; обёртка `fmt.Errorf("…: %w", err)` там, где добавляется контекст; на границах — `errors.Is/As`; паника только на старте;
  - сообщения ошибок — по-русски, со строчной буквы, без точки в конце;
  - у каждой горутины владелец и способ остановки; мьютекс рядом с полями, которые защищает;
  - `gofmt`, `go vet`, `golangci-lint run`, `go test -race` — чистые;
  - комментарии объясняют «почему»; все лимиты — именованные константы в `domain`.
- Код мессенджера в уроках — только `<<< @/../messenger/<путь>#<регион>{go}`, регионы `// #region имя` … `// #endregion имя`. Иллюстративные Go-фрагменты (антипримеры) — с `<!-- go-check -->` и проходят `npm run check:go`.
- Тег `m01-lNN` ставится исполнителем задачи урока на её последний коммит; в раундах исправлений — `git tag -f m01-lNN` на новый последний коммит задачи. На каждом теге `go build ./...` и `go test -race ./...` проходят.
- Вывод команд в уроках — реальный, полученный прогоном исполнителя.
- Коммиты на русском, заканчиваются строкой `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **Одновременная отправка в один чат.** Ожидается: `seq` уникальны и идут без пропусков; неудачи — только `ErrConcurrentUpdate` после 3 попыток; число сообщений в истории = числу успешных отправок. Покрывают тест в задаче 7 (`TestSendMessageConcurrent`) и контрактный тест в задаче 9.
2. **Личный чат для одной пары, созданный дважды одновременно.** Ожидается: один чат, второй вызов получает существующий (`Created=false`). Покрывает `TestCreateDirectChatConcurrent` в задаче 7.
3. **Текст сообщения на границах Unicode.** Ожидается: 4096 символов кириллицы или эмодзи проходят, 4097 — нет; невалидный UTF-8 и строка из одних пробелов отклоняются. Покрывает `TestNewText` в задаче 5.
4. **Тело HTTP-запроса.** Ожидается: больше 1 МиБ → 413; неизвестное поле или второй JSON-объект в теле → 400; нет или кривой `X-User-ID` → 401; сбой хранилища → 500 без деталей наружу. Покрывают тесты `httpjson` и обработчиков в задаче 10.
5. **Отменённый контекст.** Ожидается: use case возвращает ошибку, для которой `errors.Is(err, context.Canceled)`, хранилище ничего не записало. Покрывают тесты задачи 8.

---

## Структура файлов

```
messenger/
├── go.mod, go.sum, .gitignore, .golangci.yml
├── cmd/messenger/main.go                         # задача 10
├── internal/
│   ├── user/
│   │   ├── domain/   doc.go, id.go, user.go, errors.go (+ _test)          # 5, 6
│   │   ├── usecase/  doc.go, ports.go, register.go, get.go (+ _test)      # 7
│   │   │   └── storetest/ storetest.go                                    # 9
│   │   └── adapters/
│   │       ├── memory/  store.go (+ _test)                                # 7, 8, 9
│   │       └── httpapi/ handler.go, dto.go, errors.go (+ _test)           # 10
│   ├── chat/
│   │   ├── domain/   doc.go, ids.go, text.go, chat.go, message.go, errors.go (+ _test)
│   │   ├── usecase/  doc.go, ports.go, create_chat.go, send_message.go, list_messages.go (+ _test)
│   │   │   └── storetest/ storetest.go
│   │   └── adapters/
│   │       ├── memory/      store.go (+ _test)
│   │       ├── httpapi/     handler.go, dto.go, errors.go, auth.go (+ _test)
│   │       └── userchecker/ checker.go (+ _test)
│   └── platform/
│       ├── clock/     clock.go
│       ├── idgen/     idgen.go (+ _test)
│       ├── httpjson/  httpjson.go (+ _test)
│       └── httpserver/ server.go
└── docs/adr/ 0000-template.md, 0001-раскладка-по-контекстам.md, 0002-x-user-id-до-jwt.md
course/
├── scripts/check-messenger.ts, scripts/snippet-regions.ts (+ test)
├── package.json                                   # + "check:messenger"
├── glossary/terms.yaml                            # + термины 1a
├── viz/simulations/ deps.ts, ctx-tree.ts, validation-gates.ts, error-chain.ts,
│                    layer-flow.ts, test-layers.ts, http-mapping.ts (+ tests)
├── viz/renderers/  DepsView, CtxTreeView, GatesView, ErrorChainView, LayerFlowView,
│                   TestLayersView, HttpMappingView .vue
├── viz/scenes/     DepsSandbox, CtxTreeSandbox, ValidationGates, ErrorChain, LayerFlow,
│                   TestLayers, HttpMapping .vue
└── modules/01-go-service/ index.md, 01-architecture.md … 07-http-adapter.md
```

Порядок задач и теги:

| Задача | Содержание | Тег |
|---|---|---|
| 1 | Скелет `messenger/`, `.golangci.yml`, `check:messenger` с проверкой регионов, каркас модуля, глоссарий | — |
| 2 | Песочницы `deps` и `ctx-tree` | — |
| 3 | Плееры `validation-gates`, `error-chain`, `layer-flow`, `test-layers`, `http-mapping` | — |
| 4 | Урок 1.1 + ADR-0001 + демонстрация `depguard` | `m01-l01` |
| 5 | Урок 1.2 + домен | `m01-l02` |
| 6 | Урок 1.3 + доменные ошибки | `m01-l03` |
| 7 | Урок 1.4 + use case'ы, порты, хранилища в памяти, `clock`, `idgen` | `m01-l04` |
| 8 | Урок 1.5 + `context` до хранилища | `m01-l05` |
| 9 | Урок 1.6 + контрактные тесты | `m01-l06` |
| 10 | Урок 1.7 + HTTP-адаптер, `main.go`, ADR-0002 | `m01-l07` |
| 11 | Итоговая проверка, ссылки, README, бэклог | — |

---

### Task 1: Скелет мессенджера, линтер слоёв, `check:messenger`, каркас модуля, глоссарий

**Files:**
- Create: `messenger/go.mod`, `messenger/.gitignore`, `messenger/.golangci.yml`, `doc.go` в каждом пакете из «Структуры файлов» (кроме `storetest`, `cmd`), `messenger/docs/adr/0000-template.md`
- Create: `course/scripts/snippet-regions.ts`, `course/scripts/snippet-regions.test.ts`, `course/scripts/check-messenger.ts`
- Modify: `course/package.json` (скрипт), `course/glossary/terms.yaml`
- Create: `course/modules/01-go-service/index.md`

**Interfaces:**
- Produces: `findSnippetRefs(markdown: string): SnippetRef[]`, `hasRegion(source: string, region: string): boolean`, `interface SnippetRef { path: string; region: string | null; line: number }`; команда `npm run check:messenger`; пакеты-заглушки с `doc.go`; термины глоссария (список в шаге 6).

- [ ] **Step 1: Go-модуль и пакеты**

`messenger/go.mod`:

```
module messenger

go 1.26
```

`messenger/.gitignore`:

```
/messenger
/bin/
*.test
*.out
```

`doc.go` в каждом пакете — одна строка `package <имя>` и комментарий пакета о его слое. Пример `messenger/internal/chat/domain/doc.go`:

```go
// Package domain — ядро контекста «чаты»: чаты, сообщения и их правила.
// Пакет не знает ни про HTTP, ни про хранилище: импортирует только стандартную
// библиотеку. Это проверяет depguard (см. .golangci.yml).
package domain
```

Пакеты: `user/{domain,usecase}`, `user/adapters/{memory,httpapi}`, `chat/{domain,usecase}`, `chat/adapters/{memory,httpapi,userchecker}`, `platform/{clock,idgen,httpjson,httpserver}`. Имена пакетов — последний элемент пути.

- [ ] **Step 2: `.golangci.yml` с правилами слоёв**

```yaml
version: "2"

linters:
  default: standard
  enable:
    - depguard
    - errorlint
    - bodyclose
    - noctx
  settings:
    depguard:
      rules:
        domain:
          list-mode: lax
          files: ["**/internal/*/domain/**", "!$test"]
          deny:
            - pkg: messenger/internal
              desc: "домен не зависит от других пакетов проекта (урок 1.1, правило зависимостей)"
            - pkg: net/http
              desc: "домен не знает про транспорт: HTTP — дело адаптера"
            - pkg: database/sql
              desc: "домен не знает про хранилище: оно за портом в usecase"
            - pkg: encoding/json
              desc: "сериализация — в DTO адаптера, не в домене"
            - pkg: log/slog
              desc: "домен не логирует: он возвращает ошибки"
            - pkg: github.com/google/uuid
              desc: "домен не знает, как генерируются id: порт IDGenerator"
        usecase:
          list-mode: lax
          files: ["**/internal/*/usecase/**", "!$test"]
          deny:
            - pkg: messenger/internal/user/adapters
              desc: "use case зависит от порта, а не от адаптера"
            - pkg: messenger/internal/chat/adapters
              desc: "use case зависит от порта, а не от адаптера"
            - pkg: messenger/internal/platform
              desc: "время и id приходят через порты Clock и IDGenerator"
            - pkg: net/http
              desc: "use case не знает про транспорт"
            - pkg: database/sql
              desc: "use case не знает про хранилище"
            - pkg: github.com/google/uuid
              desc: "id приходят через порт IDGenerator"
        adapters:
          list-mode: lax
          files: ["**/internal/*/adapters/**", "!$test"]
          deny:
            - pkg: messenger/internal/user/adapters
              desc: "адаптеры не знают друг о друге: связь — через use case"
            - pkg: messenger/internal/chat/adapters
              desc: "адаптеры не знают друг о друге: связь — через use case"
        chat-context:
          list-mode: lax
          files: ["**/internal/chat/**", "!**/internal/chat/adapters/userchecker/**"]
          deny:
            - pkg: messenger/internal/user
              desc: "контекст chat не зависит от user: порт UserChecker и адаптер userchecker"
        user-context:
          list-mode: lax
          files: ["**/internal/user/**"]
          deny:
            - pkg: messenger/internal/chat
              desc: "контекст user не знает про чаты"
        platform:
          list-mode: lax
          files: ["**/internal/platform/**"]
          deny:
            - pkg: messenger/internal/user
              desc: "platform — общие технические пакеты, без бизнес-контекстов"
            - pkg: messenger/internal/chat
              desc: "platform — общие технические пакеты, без бизнес-контекстов"

formatters:
  enable:
    - gofmt
```

Исполнитель **проверяет каждое правило вживую**: во временной копии (`cp -r messenger "$TMP"`) добавляет по одному запрещённому импорту в непустой `.go`-файл соответствующего пакета (по одному на правило: domain → `net/http`, usecase → `messenger/internal/platform/clock`, `user/adapters/memory` → `messenger/internal/user/adapters/httpapi`, `chat/domain` → `messenger/internal/user/domain`, `platform/clock` → `messenger/internal/chat/domain`), запускает `golangci-lint run ./...` и убеждается, что каждый раз падает `depguard` с нужным `desc`. Если синтаксис `files`/`!$test` в 2.12.2 ведёт себя иначе — исправить конфиг, записать в отчёт. Вывод одного прогона сохранить в отчёт: он понадобится уроку 1.1.

- [ ] **Step 3: Тест разбора ссылок на регионы (падает)**

`course/scripts/snippet-regions.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { findSnippetRefs, hasRegion } from './snippet-regions'

describe('findSnippetRefs', () => {
  it('находит путь и регион, номер строки с 1', () => {
    const md = 'текст\n\n<<< @/../messenger/internal/chat/domain/chat.go#post{go}\n'
    expect(findSnippetRefs(md)).toEqual([{ path: '../messenger/internal/chat/domain/chat.go', region: 'post', line: 3 }])
  })
  it('ссылка без региона и без языка', () => {
    expect(findSnippetRefs('<<< @/../labs/0b/netem.sh')).toEqual([{ path: '../labs/0b/netem.sh', region: null, line: 1 }])
  })
  it('не трогает строки внутри блока кода', () => {
    expect(findSnippetRefs('```md\n<<< @/../x.go#a\n```')).toEqual([])
  })
})

describe('hasRegion', () => {
  const src = 'package x\n\n// #region post\nfunc A() {}\n// #endregion post\n'
  it('находит регион', () => expect(hasRegion(src, 'post')).toBe(true))
  it('не путает префикс', () => expect(hasRegion(src, 'pos')).toBe(false))
  it('требует закрывающую метку', () => expect(hasRegion('// #region a\n', 'a')).toBe(false))
})
```

Run: `npx vitest run scripts/snippet-regions.test.ts` → FAIL (модуля нет).

- [ ] **Step 4: Реализация**

`course/scripts/snippet-regions.ts`:

```ts
// Разбор ссылок VitePress `<<< @/путь#регион{язык}`. `@/` — каталог course/,
// поэтому путь возвращается относительно него.
export interface SnippetRef {
  path: string
  region: string | null
  line: number
}

const REF = /^<<<\s+@\/(\S+?)(?:#([\w-]+))?(?:\{[^}]*\})?\s*$/

export function findSnippetRefs(markdown: string): SnippetRef[] {
  const refs: SnippetRef[] = []
  let inFence = false
  markdown.split('\n').forEach((raw, i) => {
    const line = raw.trim()
    if (line.startsWith('```')) {
      inFence = !inFence
      return
    }
    if (inFence) return
    const m = REF.exec(line)
    if (m !== null) refs.push({ path: m[1] ?? '', region: m[2] ?? null, line: i + 1 })
  })
  return refs
}

const escape = (s: string): string => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

export function hasRegion(source: string, region: string): boolean {
  const name = escape(region)
  const open = new RegExp(`^\\s*(?://|#|<!--)\\s*#region\\s+${name}\\s*(?:-->)?\\s*$`, 'm')
  const close = new RegExp(`^\\s*(?://|#|<!--)\\s*#endregion(?:\\s+${name})?\\s*(?:-->)?\\s*$`, 'm')
  const at = source.search(open)
  return at !== -1 && close.test(source.slice(at))
}
```

Run: `npx vitest run scripts/snippet-regions.test.ts` → PASS.

- [ ] **Step 5: `check-messenger.ts` и скрипт npm**

`course/scripts/check-messenger.ts` — по образцу `check-labs.ts` (та же функция `run` с выводом stdout+stderr при ошибке и `process.exit(1)`), `cwd` = `../../messenger`:
1. `go version`;
2. `gofmt -l .` пусто, иначе список и выход 1;
3. `go vet ./...`;
4. `golangci-lint run ./...` (таймаут 300 с);
5. `go test -race ./...` (таймаут 600 с);
6. **регионы:** обойти все `*.md` в `course/modules/` (рекурсивно, `node:fs`), для каждой ссылки `findSnippetRefs` проверить, что файл `path.resolve(COURSE_DIR, ref.path)` существует и при `region !== null` — `hasRegion`. Ошибки собрать списком `файл.md:строка → путь#регион: нет файла | нет региона` и выйти с 1. Проверка покрывает и `labs/`, и `messenger/`.
7. Итог: `Мессенджер: gofmt, go vet, golangci-lint, go test -race и регионы <<< — без ошибок (ссылок: N).`

`course/package.json`, в `scripts`: `"check:messenger": "node scripts/check-messenger.ts"`.

Run: `npm run check:messenger` → проходит на пустом скелете; затем временно испортить регион в любом уроке 0b (`#region` → несуществующее имя) и убедиться, что скрипт падает с понятной строкой; вернуть.

- [ ] **Step 6: Каркас модуля и глоссарий**

`course/modules/01-go-service/index.md`:
- frontmatter `title: Модуль 1. Сервис на Go`, `order: 1`;
- `# Модуль 1. Сервис на Go`, абзац «зачем модуль» (бизнес-логика, которую не переписывают при смене БД, транспорта и API), список «В этом модуле вы:» (4–5 пунктов по спецификации 1a, §1), строка о том, что код живёт в `messenger/`, а каждый урок заканчивается тегом `m01-lNN` (`git checkout m01-l03`);
- ссылка `[Начать с урока 1.1](./01-architecture)` — **закомментирована** до задачи 4 (`<!-- … -->`), чтобы сборка не падала на мёртвой ссылке.

`course/glossary/terms.yaml` — термины в формате существующих (`term`, `en`, `short` ≤ 140 символов, `long` 2–5 предложений, простым языком, с аналогией и ссылкой «подробно — урок 1.N»):
`clean-architecture`, `dependency-rule`, `hexagonal-port`, `adapter`, `composition-root`, `big-ball-of-mud`, `ddd`, `bounded-context`, `ubiquitous-language`, `entity`, `value-object`, `aggregate`, `invariant`, `use-case`, `sentinel-error`, `error-wrapping`, `go-context`, `cancellation`, `deadline`, `data-race`, `race-detector`, `test-double`, `fake`, `mock`, `contract-test`, `table-driven-test`, `test-pyramid`, `optimistic-locking`, `mapper`, `linter`, `uuidv7`. Существующие `adr`, `dto`, `middleware` — перечитать и при необходимости дополнить ссылкой на модуль 1.

- [ ] **Step 7: Проверка и коммит**

Run: `npm test && npm run typecheck && npm run check:go && npm run check:labs && npm run check:messenger && npm run build` → всё зелёное.

```bash
git add messenger course
git commit -m "feat(messenger): скелет мессенджера, depguard на слои, check:messenger, каркас модуля 1

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Песочницы `deps` и `ctx-tree`

**Files:**
- Create: `course/viz/simulations/deps.ts`, `deps.test.ts`, `ctx-tree.ts`, `ctx-tree.test.ts`
- Create: `course/viz/renderers/DepsView.vue`, `CtxTreeView.vue`; `course/viz/scenes/DepsSandbox.vue`, `CtxTreeSandbox.vue`

**Interfaces:**
- Consumes: `Simulation<S, A, F>`, `Step<S>` из `viz/core/types.ts`; `Caption`, `term()` из `viz/core/caption.ts`; `VizFrame`, `Player`, `useNarrow` — как в `TcpSandbox.vue`.
- Produces: `depsSimulation`, `PACKAGES`, `CANDIDATES`, `checkImport()`; `ctxTreeSimulation`, `CTX_NODES`, `effectiveDeadline()`; сцены `<DepsSandbox />`, `<CtxTreeSandbox />` для уроков 1.1 и 1.5.

- [ ] **Step 1: Тесты `deps` (падают)**

`course/viz/simulations/deps.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { BASE_IMPORTS, CANDIDATES, checkImport, depsSimulation } from './deps'

describe('checkImport', () => {
  it('все импорты реального мессенджера разрешены', () => {
    for (const [from, to] of BASE_IMPORTS) expect(checkImport(from, to, BASE_IMPORTS)).toBeNull()
  })
  it.each([
    ['chat/domain', 'net/http', 'domain-pure'],
    ['chat/domain', 'chat/adapters/memory', 'cycle'],
    ['chat/usecase', 'chat/adapters/memory', 'usecase-ports'],
    ['chat/usecase', 'platform/clock', 'usecase-ports'],
    ['user/adapters/memory', 'user/adapters/httpapi', 'adapter-adapter'],
    ['chat/usecase', 'user/usecase', 'cross-context'],
    ['chat/adapters/userchecker', 'user/adapters/memory', 'adapter-adapter'],
    ['platform/clock', 'chat/domain', 'platform-pure'],
  ] as const)('%s → %s нарушает %s', (from, to, rule) => {
    expect(checkImport(from, to, BASE_IMPORTS)?.rule).toBe(rule)
  })
  it('каждый кандидат либо разрешён, либо называет правило и последствие', () => {
    for (const c of CANDIDATES) {
      const v = checkImport(c.from, c.to, BASE_IMPORTS)
      if (v !== null) expect(v.consequence.length).toBeGreaterThan(20)
    }
  })
})

describe('depsSimulation', () => {
  it('добавленный импорт становится ребром и даёт шаг с вердиктом', () => {
    const s = depsSimulation.apply(depsSimulation.initial(), { type: 'add', from: 'chat/domain', to: 'net/http' })
    const steps = depsSimulation.steps(s)
    expect(steps.at(-1)?.state.violation?.rule).toBe('domain-pure')
    expect(depsSimulation.steps(depsSimulation.apply(s, { type: 'reset' }))).toHaveLength(1)
  })
})
```

Run: `npx vitest run viz/simulations/deps.test.ts` → FAIL.

- [ ] **Step 2: Реализация `deps.ts`**

```ts
import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Simulation, Step } from '../core/types'

export type Pkg =
  | 'user/domain' | 'user/usecase' | 'user/adapters/memory' | 'user/adapters/httpapi'
  | 'chat/domain' | 'chat/usecase' | 'chat/adapters/memory' | 'chat/adapters/httpapi' | 'chat/adapters/userchecker'
  | 'platform/clock' | 'cmd/messenger' | 'net/http' | 'database/sql'

export type Layer = 'domain' | 'usecase' | 'adapter' | 'platform' | 'root' | 'stdlib'

export const PACKAGES: readonly { id: Pkg; layer: Layer }[] = [
  { id: 'user/domain', layer: 'domain' }, { id: 'chat/domain', layer: 'domain' },
  { id: 'user/usecase', layer: 'usecase' }, { id: 'chat/usecase', layer: 'usecase' },
  { id: 'user/adapters/memory', layer: 'adapter' }, { id: 'user/adapters/httpapi', layer: 'adapter' },
  { id: 'chat/adapters/memory', layer: 'adapter' }, { id: 'chat/adapters/httpapi', layer: 'adapter' },
  { id: 'chat/adapters/userchecker', layer: 'adapter' },
  { id: 'platform/clock', layer: 'platform' }, { id: 'cmd/messenger', layer: 'root' },
  { id: 'net/http', layer: 'stdlib' }, { id: 'database/sql', layer: 'stdlib' },
]

export type Edge = readonly [Pkg, Pkg]

// Импорты настоящего мессенджера на теге m01-l07 (кроме stdlib, кроме net/http у httpapi).
export const BASE_IMPORTS: readonly Edge[] = [
  ['user/usecase', 'user/domain'], ['chat/usecase', 'chat/domain'],
  ['user/adapters/memory', 'user/domain'], ['user/adapters/httpapi', 'user/usecase'], ['user/adapters/httpapi', 'user/domain'],
  ['chat/adapters/memory', 'chat/domain'], ['chat/adapters/httpapi', 'chat/usecase'], ['chat/adapters/httpapi', 'chat/domain'],
  ['chat/adapters/userchecker', 'chat/domain'], ['chat/adapters/userchecker', 'user/usecase'], ['chat/adapters/userchecker', 'user/domain'],
  ['user/adapters/httpapi', 'net/http'], ['chat/adapters/httpapi', 'net/http'],
  ['cmd/messenger', 'user/usecase'], ['cmd/messenger', 'chat/usecase'], ['cmd/messenger', 'user/adapters/memory'],
  ['cmd/messenger', 'user/adapters/httpapi'], ['cmd/messenger', 'chat/adapters/memory'], ['cmd/messenger', 'chat/adapters/httpapi'],
  ['cmd/messenger', 'chat/adapters/userchecker'], ['cmd/messenger', 'platform/clock'], ['cmd/messenger', 'net/http'],
]

export type Rule = 'cycle' | 'domain-pure' | 'usecase-ports' | 'adapter-adapter' | 'cross-context' | 'platform-pure'

export interface Violation {
  rule: Rule
  title: string
  consequence: string
}

const layerOf = (p: Pkg): Layer => PACKAGES.find((x) => x.id === p)?.layer ?? 'stdlib'
const contextOf = (p: Pkg): 'user' | 'chat' | null => (p.startsWith('user/') ? 'user' : p.startsWith('chat/') ? 'chat' : null)

function reachable(from: Pkg, to: Pkg, edges: readonly Edge[]): boolean {
  const seen = new Set<Pkg>([from])
  const queue: Pkg[] = [from]
  while (queue.length > 0) {
    const cur = queue.shift() as Pkg
    if (cur === to) return true
    for (const [a, b] of edges) if (a === cur && !seen.has(b)) { seen.add(b); queue.push(b) }
  }
  return false
}

const V = (rule: Rule, title: string, consequence: string): Violation => ({ rule, title, consequence })

// Порядок проверок важен: цикл — ошибка компиляции, она важнее нарушения стиля.
export function checkImport(from: Pkg, to: Pkg, edges: readonly Edge[]): Violation | null {
  if (from === to) return V('cycle', 'Пакет импортирует сам себя', 'Go не соберёт проект: import cycle not allowed.')
  if (reachable(to, from, edges))
    return V('cycle', 'Цикл импортов', `${to} уже зависит от ${from}. Go запрещает циклы: компилятор остановится с ошибкой import cycle not allowed.`)
  const fl = layerOf(from), tl = layerOf(to)
  if (fl === 'domain' && to !== from)
    return V('domain-pure', 'Домен зависит от внешнего мира',
      `Домен должен импортировать только стандартную библиотеку без транспорта и хранилищ. С этим импортом смена ${to} заставит переписывать бизнес-правила, а тесты домена потянут за собой инфраструктуру.`)
  if (fl === 'usecase' && (tl === 'adapter' || tl === 'platform' || tl === 'stdlib'))
    return V('usecase-ports', 'Use case зависит от конкретики',
      `Use case должен видеть только порты — интерфейсы, объявленные у него же. Зависимость от ${to} привяжет сценарий к одной реализации: заменить хранилище на Postgres или подменить часы в тесте станет нельзя без правки use case.`)
  if (fl === 'adapter' && tl === 'adapter')
    return V('adapter-adapter', 'Адаптер знает о другом адаптере',
      'Адаптеры — сменные детали снаружи. Если HTTP-адаптер знает про хранилище в памяти, замена хранилища сломает транспорт. Связь между ними идёт только через use case.')
  const fc = contextOf(from), tc = contextOf(to)
  if (fc !== null && tc !== null && fc !== tc && !(from === 'chat/adapters/userchecker' && (to === 'user/usecase' || to === 'user/domain')))
    return V('cross-context', 'Один контекст лезет внутрь другого',
      'Контексты user и chat должны разделиться на сервисы в модуле 6. Прямой импорт склеивает их: при распиле придётся переписывать код. Связь — через порт UserChecker и единственный адаптер userchecker, который станет gRPC-клиентом.')
  if (fl === 'platform' && fc === null && tc !== null)
    return V('platform-pure', 'Технический пакет знает про бизнес',
      'platform — общие технические детали (часы, генератор id). Зависимость от бизнес-контекста превращает его в свалку и тянет бизнес-логику во все сервисы.')
  return null
}

export const CANDIDATES: readonly { from: Pkg; to: Pkg }[] = [
  { from: 'chat/domain', to: 'net/http' }, { from: 'chat/domain', to: 'database/sql' },
  { from: 'chat/domain', to: 'chat/adapters/memory' }, { from: 'chat/usecase', to: 'chat/adapters/memory' },
  { from: 'chat/usecase', to: 'platform/clock' }, { from: 'chat/usecase', to: 'user/usecase' },
  { from: 'user/adapters/memory', to: 'user/adapters/httpapi' }, { from: 'chat/adapters/userchecker', to: 'user/adapters/memory' },
  { from: 'platform/clock', to: 'chat/domain' }, { from: 'chat/adapters/httpapi', to: 'chat/usecase' },
  { from: 'chat/adapters/memory', to: 'chat/domain' },
]

export interface DepsState { added: readonly Edge[] }
export type DepsAction = { type: 'add'; from: Pkg; to: Pkg } | { type: 'reset' }
export interface DepsFrame { edges: readonly Edge[]; last: Edge | null; violation: Violation | null }

const intro: Caption = [
  { text: 'Граф импортов мессенджера. Стрелки идут только внутрь — от адаптеров к use case и от use case к домену. Это ' },
  term('правило зависимостей', 'dependency-rule'),
  { text: '. Добавьте импорт из списка и посмотрите, что скажет проверка.' },
]

export const depsSimulation: Simulation<DepsState, DepsAction, DepsFrame> = {
  initial: () => ({ added: [] }),
  apply: (state, action) => (action.type === 'reset' ? { added: [] } : { added: [...state.added, [action.from, action.to]] }),
  steps: (state) => {
    const steps: Step<DepsFrame>[] = [{ state: { edges: BASE_IMPORTS, last: null, violation: null }, caption: intro }]
    let edges: readonly Edge[] = BASE_IMPORTS
    for (const edge of state.added) {
      const violation = checkImport(edge[0], edge[1], edges)
      edges = [...edges, edge]
      const caption: Caption = violation === null
        ? `${edge[0]} → ${edge[1]}: разрешено. Зависимость направлена внутрь — внешний слой знает о внутреннем, а не наоборот.`
        : `${edge[0]} → ${edge[1]}: ${violation.title}. ${violation.consequence}`
      steps.push({ state: { edges, last: edge, violation }, caption })
    }
    return steps
  },
}
```

Если тип `Step<S>` в `viz/core/types.ts` требует других полей (например, `id`), привести к нему — по образцу `tcp.ts`.

Run: `npx vitest run viz/simulations/deps.test.ts` → PASS.

- [ ] **Step 3: Тесты `ctx-tree` (падают)**

`course/viz/simulations/ctx-tree.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { ctxTreeSimulation as sim, effectiveDeadline } from './ctx-tree'

const run = (...actions: Parameters<typeof sim.apply>[1][]) => actions.reduce(sim.apply, sim.initial())

describe('ctx-tree', () => {
  it('дедлайн потомка не позже дедлайна родителя', () => {
    expect(effectiveDeadline('store')).toBeLessThanOrEqual(effectiveDeadline('request'))
    expect(effectiveDeadline('usecase')).toBe(effectiveDeadline('request'))
  })
  it('отмена родителя отменяет всех потомков, кроме отвязанных', () => {
    const s = run({ type: 'cancel', id: 'request' })
    expect(Object.keys(s.done).sort()).toEqual(['request', 'store', 'usecase', 'userchecker'])
    expect(s.done.audit).toBeUndefined()
  })
  it('отмена потомка не трогает родителя', () => {
    const s = run({ type: 'cancel', id: 'userchecker' })
    expect(s.done.userchecker?.cause).toBe('canceled')
    expect(s.done.usecase).toBeUndefined()
  })
  it('время доходит до дедлайна — причина deadline у узла и его потомков', () => {
    const s = run({ type: 'advance', ms: 100 })
    expect(s.done.userchecker?.cause).toBe('deadline')
    expect(s.done.request).toBeUndefined()
    const t = run({ type: 'advance', ms: 1000 })
    expect(t.done.request?.cause).toBe('deadline')
    expect(t.done.store?.cause).toBe('deadline')
    expect(t.done.audit).toBeUndefined()
  })
  it('Background отменить нельзя', () => {
    expect(run({ type: 'cancel', id: 'root' }).done.root).toBeUndefined()
  })
})
```

- [ ] **Step 4: Реализация `ctx-tree.ts`**

```ts
import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Simulation, Step } from '../core/types'

export type CtxId = 'root' | 'request' | 'usecase' | 'store' | 'userchecker' | 'audit'
export type CtxKind = 'background' | 'timeout' | 'value' | 'without-cancel'

export interface CtxNode {
  id: CtxId
  parent: CtxId | null
  kind: CtxKind
  timeoutMs?: number
  label: string
  work: string
}

// Сценарий урока 1.5: запрос с таймаутом 1 с, проверка участников с таймаутом 100 мс,
// аудит, который должен дописаться, даже если клиент ушёл.
export const CTX_NODES: readonly CtxNode[] = [
  { id: 'root', parent: null, kind: 'background', label: 'context.Background()', work: 'main' },
  { id: 'request', parent: 'root', kind: 'timeout', timeoutMs: 1000, label: 'WithTimeout(r.Context(), 1s)', work: 'HTTP-обработчик' },
  { id: 'usecase', parent: 'request', kind: 'value', label: 'тот же ctx', work: 'SendMessage.Execute' },
  { id: 'store', parent: 'usecase', kind: 'value', label: 'тот же ctx', work: 'хранилище: AppendMessage' },
  { id: 'userchecker', parent: 'usecase', kind: 'timeout', timeoutMs: 100, label: 'WithTimeout(ctx, 100ms)', work: 'UserChecker' },
  { id: 'audit', parent: 'request', kind: 'without-cancel', label: 'WithoutCancel(ctx)', work: 'запись в аудит' },
]

const node = (id: CtxId): CtxNode => CTX_NODES.find((n) => n.id === id) as CtxNode
const children = (id: CtxId): CtxNode[] => CTX_NODES.filter((n) => n.parent === id)

export function effectiveDeadline(id: CtxId): number {
  const n = node(id)
  if (n.kind === 'background' || n.kind === 'without-cancel') return n.timeoutMs ?? Infinity
  const parent = n.parent === null ? Infinity : effectiveDeadline(n.parent)
  return Math.min(parent, n.timeoutMs ?? Infinity)
}

export type Cause = 'canceled' | 'deadline'
export interface CtxState { nowMs: number; done: Partial<Record<CtxId, { cause: Cause; atMs: number }>>; last: string }
export type CtxAction = { type: 'cancel'; id: CtxId } | { type: 'advance'; ms: number } | { type: 'reset' }

function finish(done: CtxState['done'], id: CtxId, cause: Cause, atMs: number): CtxState['done'] {
  if (done[id] !== undefined || node(id).kind === 'background') return done
  let next = { ...done, [id]: { cause, atMs } }
  // WithoutCancel обрывает распространение: потомок не видит отмену родителя.
  for (const c of children(id)) if (c.kind !== 'without-cancel') next = finish(next, c.id, cause, atMs)
  return next
}

export const ctxTreeSimulation: Simulation<CtxState, CtxAction> = {
  initial: () => ({ nowMs: 0, done: {}, last: 'start' }),
  apply: (s, a) => {
    if (a.type === 'reset') return ctxTreeSimulation.initial()
    if (a.type === 'cancel') return { ...s, done: finish(s.done, a.id, 'canceled', s.nowMs), last: `cancel:${a.id}` }
    const nowMs = s.nowMs + a.ms
    let done = s.done
    // Сначала самые ранние дедлайны: причина у потомка — его собственный дедлайн, если он раньше.
    const byDeadline = [...CTX_NODES].sort((x, y) => effectiveDeadline(x.id) - effectiveDeadline(y.id))
    for (const n of byDeadline) {
      const d = effectiveDeadline(n.id)
      if (d <= nowMs) done = finish(done, n.id, 'deadline', d)
    }
    return { nowMs, done, last: `advance:${a.ms}` }
  },
  steps: (s) => [{ state: s, caption: captionFor(s) } satisfies Step<CtxState>],
}

function captionFor(s: CtxState): Caption {
  if (s.last === 'start')
    return [
      { text: 'Дерево ' }, term('контекстов', 'go-context'),
      { text: ': каждый WithTimeout или WithCancel создаёт потомка. Отмена идёт только вниз — от родителя к детям. Отмените узел или промотайте время.' },
    ]
  const stopped = CTX_NODES.filter((n) => s.done[n.id] !== undefined).map((n) => n.work)
  const live = CTX_NODES.filter((n) => n.kind !== 'background' && s.done[n.id] === undefined).map((n) => n.work)
  const what = s.last.startsWith('cancel') ? 'Вызвали cancel()' : `Прошло ${s.nowMs} мс`
  return [
    { text: `${what}. Остановились: ${stopped.length > 0 ? stopped.join(', ') : 'никто'}. Работают: ${live.length > 0 ? live.join(', ') : 'никто'}. ` },
    term('Дедлайн', 'deadline'),
    { text: ' потомка не может быть позже родительского, а ' },
    term('отмена', 'cancellation'),
    { text: ' потомка не трогает родителя.' },
  ]
}
```

Run: `npx vitest run viz/simulations/ctx-tree.test.ts` → PASS.

- [ ] **Step 5: Рендереры и сцены (контракт)**

- `DepsView.vue`: SVG-граф в 4 колонки по слоям (stdlib | domain | usecase/platform | adapter | root), пакеты — плашки с цветом слоя, рёбра — стрелки; последнее добавленное ребро — красное пунктирное при нарушении, зелёное при разрешении; на 375 px — колонки в 2 ряда или вертикальный список с отступами (без горизонтальной прокрутки).
- `DepsSandbox.vue`: `VizFrame` + `Player`; `<select>` с `CANDIDATES` (подпись «откуда → куда») и кнопки «Добавить импорт», «Сбросить»; все элементы управления с видимыми подписями и доступны с клавиатуры.
- `CtxTreeView.vue`: дерево узлов сверху вниз; у узла — метка, `work`, эффективный дедлайн («до 1000 мс», «без дедлайна»), состояние: работает / `context.Canceled` / `context.DeadlineExceeded` (текст, не только цвет); у `audit` — пометка «отвязан».
- `CtxTreeSandbox.vue`: кнопки «Отменить» у каждого узла (кроме `root`), «+50 мс», «+500 мс», «Сброс»; часы `t = N мс`.
- Обе сцены — в `VizFrame` с полноэкранным режимом, подписи `aria-live="polite"`.

- [ ] **Step 6: Проверка и коммит**

Run: `npm test && npm run typecheck && npm run build`; Playwright: временная страница с обеими сценами не нужна — сцены проверяются в задачах 4 и 8; здесь достаточно, что `vue-tsc` и тесты зелёные.

```bash
git add course/viz
git commit -m "feat(viz): песочницы «граф зависимостей» и «дерево контекстов»

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Плееры `validation-gates`, `error-chain`, `layer-flow`, `test-layers`, `http-mapping`

**Files:**
- Create: `course/viz/simulations/{validation-gates,error-chain,layer-flow,test-layers,http-mapping}.ts` (+ `.test.ts`)
- Create: рендереры `GatesView`, `ErrorChainView`, `LayerFlowView`, `TestLayersView`, `HttpMappingView` и сцены `ValidationGates`, `ErrorChain`, `LayerFlow`, `TestLayers`, `HttpMapping` (`.vue`)

**Interfaces:**
- Consumes: `Scenario<S>`, `Step<S>`, `term()`; имена типов и функций мессенджера из задач 5–10 (ниже) — подписи их называют, поэтому они должны совпадать с кодом.
- Produces: сцены для уроков 1.2, 1.3, 1.4, 1.6, 1.7.

Имена, на которые ссылаются подписи (определены в задачах 5–10): `domain.NewText`, `domain.NewUsername`, `domain.ParseChatID`, `*domain.ValidationError`, `domain.ErrInvalid`, `domain.ErrChatNotFound`, `domain.ErrNotMember`, `domain.ErrConcurrentUpdate`, `(*domain.Chat).Post`, `usecase.SendMessage.Execute`, `usecase.SendMessageCommand`, `MaxSendAttempts = 3`, `memory.Store.AppendMessage`, `httpapi` DTO `sendMessageRequest`/`messageResponse`, `httpjson.Decode`, `httpjson.MaxBodyBytes = 1 << 20`.

- [ ] **Step 1: Сценарии (контракт и обязательные тесты)**

Каждый — чистая функция `Scenario<S>` с подписями-`Caption` (термины через `term()`), 6–10 шагов. Тест на каждый: число шагов в диапазоне, у каждого шага непустая подпись, плюс содержательные проверки ниже.

1. **`validation-gates`** (урок 1.2). Состояние: `{ input: string; gate: 'json' | 'dto' | 'value-object' | 'aggregate' | 'stored'; verdict: 'pass' | 'reject'; reason?: string }`. Четыре прогона подряд:
   - `"   "` → отклонён на `value-object` (`NewText`: пустое после обрезки);
   - строка из 4097 букв «я» → отклонена на `value-object` (символы считаются рунами, не байтами: 4097 рун = 8194 байта);
   - `{"text": 42}` → отклонён на `json` (тип поля), до домена не дошёл;
   - нормальный текст от не-участника → проходит `value-object`, отклонён на `aggregate` (`Chat.Post` → `ErrNotMember`) — инвариант, которому нужно состояние чата.
   Тест: каждая из четырёх историй останавливается на указанных воротах.
2. **`error-chain`** (урок 1.3). Состояние: `{ layers: { where: string; message: string }[]; isNotFound: boolean; asValidation: boolean; status: number | null }`. Шаги: хранилище возвращает `domain.ErrChatNotFound` → use case оборачивает `fmt.Errorf("send message: %w", err)` → (вариант с ошибкой) кто-то обернул через `%v` — `errors.Is` больше не видит сигнальную ошибку → исправлено на `%w` → адаптер: `errors.Is(err, domain.ErrChatNotFound)` → 404 → второй сценарий: `*ValidationError{Field: "text"}` и `errors.As` → 400 с полем → `errors.Is(err, domain.ErrInvalid)` тоже `true` благодаря методу `Is`. Тест: на шаге с `%v` `isNotFound === false`, на остальных шагах цепочки — `true`; итоговые статусы 404 и 400.
3. **`layer-flow`** (урок 1.4). Состояние: `{ active: 'http' | 'usecase' | 'domain' | 'store'; sees: string[]; note: string; attempt: number }`. Шаги `POST /chats/{id}/messages`: обработчик (видит `http.Request`, DTO) → команда `SendMessageCommand` (строки) → use case разбирает в `ChatID`, `UserID`, `Text` → `FindChat` через порт `ChatFinder` → `chat.Post` (инвариант «участник», `seq = lastSeq + 1`, версия +1) → `AppendMessage(ctx, chat, msg, expectedVersion)` → конфликт версии (другой участник успел первым) → повтор, попытка 2 → успех → ответ 201. Тест: есть шаг с `attempt === 2`; `sees` у шага `domain` не содержит ничего про HTTP.
4. **`test-layers`** (урок 1.6). Состояние: `{ bug: string; caughtBy: 'domain-unit' | 'usecase-fake' | 'contract' | 'http' | 'race'; cost: string }`. Баги: «`NewText` пропускает 4097 символов» → юнит домена; «use case не проверяет участника» → тест use case на фейках; «хранилище не проверяет версию» → контрактный тест; «`ErrNotMember` маппится в 500» → тест обработчика; «запись в map без мьютекса» → `go test -race`; «SQL в будущем Postgres-адаптере перепутал порядок» → тот же контрактный тест в модуле 2a. Тест: шесть разных багов, у каждого уровень из перечисленных.
5. **`http-mapping`** (урок 1.7). Состояние: `{ stage: 'json-in' | 'dto' | 'command' | 'domain' | 'dto-out' | 'json-out'; shape: string }`. Шаги показывают одно сообщение в каждом представлении: JSON `{"text":"привет"}` → `sendMessageRequest{Text}` → `SendMessageCommand{ChatID, AuthorID, Text}` (ChatID — из пути, AuthorID — из заголовка `X-User-ID`) → `domain.Message` (неэкспортируемые поля, `seq`) → `messageResponse{ID, ChatID, AuthorID, Text, Seq, SentAt}` с тегами `json` → JSON-ответ. Отдельный шаг: «почему не `json.Marshal(domain.Message)`» — поля неэкспортируемые, а переименование поля в домене сломало бы API клиентам. Тест: `json` в `shape` есть только на стадиях `dto`, `dto-out`, `json-in`, `json-out`.

- [ ] **Step 2: Рендереры и сцены (контракт)**

Каждая сцена — `VizFrame` + `Player` + рендерер; узкая раскладка для 375 px; состояния обозначены и текстом, не только цветом. `GatesView` — горизонтальная (на узком — вертикальная) линия ворот с маркером ввода и вердиктом; `ErrorChainView` — стопка «конвертов» ошибок с текстом `Error()` и двумя индикаторами `errors.Is`/`errors.As`; `LayerFlowView` — четыре слоя-полосы с подсветкой активного и списком «что видит»; `TestLayersView` — пирамида уровней с отметкой, кто поймал баг; `HttpMappingView` — цепочка представлений с моноширинным `shape`.

- [ ] **Step 3: Проверка и коммит**

Run: `npm test && npm run typecheck && npm run build`.

```bash
git add course/viz
git commit -m "feat(viz): плееры модуля 1 — ворота валидации, цепочка ошибок, слои, тесты, маппинг HTTP

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Общие правила для уроков (задачи 4–10)

1. **Шаблон.** У каждого `##` — `<Part kind>` (`problem`, `theory`, `viz`, `code`, `mistakes`, `tradeoff`, `selfcheck`, `sources`, `practice`). Код урока — раздел `## <Part kind="code" /> Код: …`; изменения мессенджера — `## <Part kind="practice" /> Что изменилось в мессенджере` с деревом файлов, командой `git diff m01-l(NN-1) m01-lNN --stat` и её реальным выводом (для 1.1 — `git show --stat m01-l01`).
2. **Числа.** 3–8 `<N>` на урок.
3. **«Под капотом».** Минимум один `<Deep>` (150–500 слов); тема — в задаче.
4. **Самодостаточность.** Перечитать и закрыть места, где нужна книга; список — в отчёт.
5. **Код.** Код мессенджера — только `<<< @/../messenger/…#регион{go}`. Регионы добавляются в Go-файлы комментариями `// #region имя` / `// #endregion имя`. Антипримеры — с `<!-- go-check -->`; упрощения — `<Callout type="simplified">`.
6. **Вывод команд.** Реальный, из собственного прогона (`go test -race -v`, `golangci-lint`, `curl`).
7. **Источники.** «Автор, «Название», глава/раздел — что найти». Книги из основной спецификации: Мартин «Чистая архитектура»; Ханонов «Изучаем DDD»; Харсаньи «100 ошибок Go»; Эдвардс «Let's Go Further»; Фаулер (статьи); Кокберн «Hexagonal Architecture»; Найгард «Documenting Architecture Decisions»; документация Go (go.dev/blog, go.dev/ref/spec, go.dev/doc/articles/race_detector).
8. **Связи.** Назад — на 0.3 (путь запроса), 0.6 (требования), 0b.6 (таймауты сервера); вперёд — на 1b, 1.5, 2a, 2b, 3, 6, 6.7.
9. **Факты.** Перечисленные в задаче факты исполнитель **проверяет** по исходникам Go 1.26 (`$(go env GOROOT)/src`) или документации; неверный факт исправить и записать в отчёт.
10. **Проверка урока.** `npm test && npm run check:go && npm run check:labs && npm run check:messenger && npm run build`; Playwright 375 и 1280, обе темы: метки, `<Deep>` с клавиатуры, сцены работают, ⛶ с удержанием фокуса, нет горизонтальной прокрутки. Ссылку на следующий урок в конце урока ставить только когда он существует.
11. **Тег.** Последний коммит задачи — `git tag m01-lNN`; при исправлениях — `git tag -f m01-lNN`.

---

### Task 4: Урок 1.1 «Зачем архитектура» + ADR-0001

**Files:**
- Create: `course/modules/01-go-service/01-architecture.md`, `messenger/docs/adr/0001-раскладка-по-контекстам.md`
- Modify: `messenger/docs/adr/0000-template.md` (если нужно), `course/modules/01-go-service/index.md` (раскомментировать ссылку на 1.1)

**Interfaces:**
- Consumes: скелет и `.golangci.yml` (задача 1), `<DepsSandbox />` (задача 2), сохранённый вывод `depguard` из отчёта задачи 1.

- [ ] **Step 1: ADR**

`0000-template.md` — разделы: «Статус» (предложено / принято / заменено ADR-NNNN), «Контекст», «Рассмотренные варианты», «Решение», «Последствия» (плюсы, минусы, что станет сложнее). `0001-…` — по шаблону: раскладка по контекстам против раскладки по слоям и плоских пакетов (те три варианта, что обсуждались: превью раскладок), решение — по контекстам, последствия (модуль 6, `depguard`, дублирование портов `Clock`/`IDGenerator` в двух контекстах — сознательное). Статус «принято», дата 2026-10-13.

- [ ] **Step 2: Урок `01-architecture.md`**

Frontmatter: `title: 1.1. Зачем сервису архитектура`.

**Разделы:**
- **Проблема.** Обработчик «отправить сообщение» на 80 строк, который сам разбирает JSON, ходит в `database/sql`, проверяет участника и шлёт пуш (антипример с `<!-- go-check -->`, компилируется). Три жалобы: «поменяли формат ответа — сломали проверку участника», «тест требует живую БД», «в модуле 6 этот код не разрезать». Термин `big-ball-of-mud`.
- **Теория:**
  - слои и **правило зависимостей** (Мартин): исходный код зависит только от более внутреннего; инверсия зависимостей — интерфейс объявлен внутри, реализован снаружи;
  - порты и адаптеры (Кокберн, 2005), `hexagonal-port`, `adapter`; ведущие (HTTP) и ведомые (хранилище) адаптеры;
  - `composition-root`: единственное место, где создаются конкретные типы;
  - раскладка по контекстам против раскладки по слоям — с деревьями обоих вариантов; почему выбрали контексты (ADR-0001 подключить фрагментом или ссылкой на файл в репозитории);
  - как Go помогает: `internal/` (импорт только из своего дерева), запрет циклов импорта; как `depguard` превращает правило в проверку;
  - когда архитектура вредна: порог из стандарта — маленькая утилита (как `labs/0b`) живёт без слоёв; раскладываем, когда появляется второй источник данных или второй транспорт; у мессенджера впереди Postgres (2a), WebSocket (3), gRPC (6).
- **Визуализация:** `<DepsSandbox />` и 3 задания (добавить `chat/domain → net/http`; `chat/usecase → platform/clock`; `chat/adapters/httpapi → chat/usecase` — почему это разрешено).
- **Код:** `.golangci.yml` регионом с правилами `domain` и `chat-context` (добавить в файл YAML-комментарии `# #region domain-rule` / `# #endregion domain-rule` — `hasRegion` понимает `#`); демонстрация: добавляем в `chat/domain/doc.go` импорт `net/http`, запускаем `golangci-lint run ./...`, реальный вывод, откатываем.
- **Под капотом:** «Почему Go запрещает циклы импорта и как это помогает слоям» — инициализация пакетов в порядке зависимостей, скорость сборки, что делать при цикле (вынести интерфейс в потребителя).
- **Ошибки:** «слои ради слоёв» на утилите; пакеты `models`/`utils`/`common`; интерфейс рядом с реализацией «на будущее»; домен, который знает про JSON.
- **Компромисс:** больше файлов и косвенности против заменяемости и тестируемости.
- **Самопроверка** (4–5 вопросов на понимание), **источники**.

**Термины:** `big-ball-of-mud`, `clean-architecture`, `dependency-rule`, `hexagonal-port`, `adapter`, `composition-root`, `adr`, `linter`.

**Факты проверить:** спецификация Go о запрете циклов импорта; правило `internal/` (`go help internal` / документация `cmd/go`); год и название статьи Кокберна; формулировка правила зависимостей у Мартина (гл. 22).

- [ ] **Step 3: Проверка, коммит, тег**

```bash
git add course messenger
git commit -m "feat(course): урок 1.1 «Зачем сервису архитектура», ADR-0001

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git tag m01-l01
```

---

### Task 5: Урок 1.2 «Домен» + код домена

**Files:**
- Create: `messenger/internal/user/domain/{id.go,user.go,errors.go,user_test.go}`, `messenger/internal/chat/domain/{ids.go,text.go,chat.go,message.go,errors.go,chat_test.go,text_test.go}`
- Create: `course/modules/01-go-service/02-domain.md`

**Interfaces:**
- Produces (используют задачи 6–10):
  - `user/domain`: `UserID`, `ParseUserID(string) (UserID, error)`, `(UserID).String()`, `(UserID).IsZero()`; `Username`, `NewUsername`; `DisplayName`, `NewDisplayName`; `User`, `NewUser(id, username, displayName, createdAt) User` + геттеры `ID()`, `Username()`, `DisplayName()`, `CreatedAt()`; константы `UsernameMinLen = 3`, `UsernameMaxLen = 32`, `DisplayNameMaxLen = 64`.
  - `chat/domain`: `ChatID`, `MessageID`, `UserID` (+ `Parse…`, `String`, `IsZero`); `Text`, `NewText`; `Title`, `NewTitle`; `Kind` (`KindDirect`, `KindGroup`), `ParseKind`; `Chat`, `NewDirectChat`, `NewGroupChat`, методы `ID`, `Kind`, `Title`, `Members`, `IsMember`, `LastSeq`, `Version`, `CreatedAt`, `DirectKey`, `Post`; `DirectKeyFor(a, b UserID) string`; `Message` + геттеры `ID`, `ChatID`, `AuthorID`, `Text`, `Seq`, `SentAt`; константы `TextMaxLen = 4096`, `TitleMaxLen = 128`, `GroupMinMembers = 2`, `GroupMaxMembers = 200`, `ListLimitDefault = 50`, `ListLimitMax = 100`; `ErrNotMember`.

**Эволюция ошибок.** В этом уроке конструкторы возвращают ошибки через функцию `invalid(field, reason string) error`, которая в **l02** реализована как `errors.New(field + ": " + reason)`; `ErrNotMember` уже сигнальная (её надо сравнивать в тестах). В задаче 6 `invalid` меняется на `*ValidationError` — остальной код не трогается.

- [ ] **Step 1: Тесты домена (падают)**

`messenger/internal/chat/domain/text_test.go`:

```go
package domain_test

import (
	"strings"
	"testing"

	"messenger/internal/chat/domain"
)

func TestNewText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "обычный текст", in: "привет", want: "привет"},
		{name: "пробелы по краям обрезаются", in: "  привет \n", want: "привет"},
		{name: "пустая строка", in: "", wantErr: true},
		{name: "только пробелы", in: " \t\n ", wantErr: true},
		{name: "ровно лимит кириллицей", in: strings.Repeat("я", domain.TextMaxLen), want: strings.Repeat("я", domain.TextMaxLen)},
		{name: "лимит + 1 кириллицей", in: strings.Repeat("я", domain.TextMaxLen+1), wantErr: true},
		{name: "ровно лимит эмодзи", in: strings.Repeat("🙂", domain.TextMaxLen), want: strings.Repeat("🙂", domain.TextMaxLen)},
		{name: "невалидный UTF-8", in: "при\xffвет", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.NewText(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewText(%q): ждали ошибку, получили %q", tt.in, got.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("NewText(%q): неожиданная ошибка %v", tt.in, err)
			}
			if got.String() != tt.want {
				t.Fatalf("NewText(%q) = %q, ждали %q", tt.in, got.String(), tt.want)
			}
		})
	}
}
```

`messenger/internal/chat/domain/chat_test.go`:

```go
package domain_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"messenger/internal/chat/domain"
)

var t0 = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

func uid(t *testing.T, n int) domain.UserID {
	t.Helper()
	id, err := domain.ParseUserID(fmt.Sprintf("00000000-0000-7000-8000-%012d", n))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func cid(t *testing.T, n int) domain.ChatID {
	t.Helper()
	id, err := domain.ParseChatID(fmt.Sprintf("00000000-0000-7000-9000-%012d", n))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mid(t *testing.T, n int) domain.MessageID {
	t.Helper()
	id, err := domain.ParseMessageID(fmt.Sprintf("00000000-0000-7000-a000-%012d", n))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func text(t *testing.T, s string) domain.Text {
	t.Helper()
	x, err := domain.NewText(s)
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func TestNewDirectChat(t *testing.T) {
	t.Parallel()
	if _, err := domain.NewDirectChat(cid(t, 1), uid(t, 1), uid(t, 1), t0); err == nil {
		t.Fatal("личный чат с самим собой: ждали ошибку")
	}
	c, err := domain.NewDirectChat(cid(t, 1), uid(t, 2), uid(t, 1), t0)
	if err != nil {
		t.Fatal(err)
	}
	if c.Kind() != domain.KindDirect || len(c.Members()) != 2 || !c.IsMember(uid(t, 1)) || !c.IsMember(uid(t, 2)) {
		t.Fatalf("неверный чат: %+v", c.Members())
	}
	if domain.DirectKeyFor(uid(t, 1), uid(t, 2)) != domain.DirectKeyFor(uid(t, 2), uid(t, 1)) {
		t.Fatal("ключ личного чата должен не зависеть от порядка участников")
	}
}

func TestNewGroupChatMembers(t *testing.T) {
	t.Parallel()
	title, err := domain.NewTitle("Семья")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		others  int
		wantLen int
		wantErr bool
	}{
		{name: "создатель один — мало", others: 0, wantErr: true},
		{name: "создатель и один участник", others: 1, wantLen: 2},
		{name: "ровно максимум", others: domain.GroupMaxMembers - 1, wantLen: domain.GroupMaxMembers},
		{name: "максимум + 1", others: domain.GroupMaxMembers, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			others := make([]domain.UserID, 0, tt.others+1)
			for i := range tt.others {
				others = append(others, uid(t, i+2))
			}
			others = append(others, uid(t, 1)) // создатель в списке ещё раз — дубль схлопывается
			c, err := domain.NewGroupChat(cid(t, 1), title, uid(t, 1), others, t0)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ждали ошибку, получили %d участников", len(c.Members()))
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(c.Members()) != tt.wantLen || !c.IsMember(uid(t, 1)) {
				t.Fatalf("участников %d, ждали %d, создатель внутри: %v", len(c.Members()), tt.wantLen, c.IsMember(uid(t, 1)))
			}
		})
	}
}

func TestChatPost(t *testing.T) {
	t.Parallel()
	c, err := domain.NewDirectChat(cid(t, 1), uid(t, 1), uid(t, 2), t0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Post(mid(t, 1), uid(t, 3), text(t, "я не участник"), t0); !errors.Is(err, domain.ErrNotMember) {
		t.Fatalf("ждали ErrNotMember, получили %v", err)
	}
	if c.LastSeq() != 0 || c.Version() != 0 {
		t.Fatal("отклонённое сообщение не должно менять чат")
	}
	for i := 1; i <= 3; i++ {
		m, err := c.Post(mid(t, i), uid(t, 1), text(t, "привет"), t0)
		if err != nil {
			t.Fatal(err)
		}
		if m.Seq() != int64(i) || c.LastSeq() != int64(i) || c.Version() != int64(i) {
			t.Fatalf("сообщение %d: seq=%d lastSeq=%d version=%d", i, m.Seq(), c.LastSeq(), c.Version())
		}
	}
}

func TestMembersIsCopy(t *testing.T) {
	t.Parallel()
	c, err := domain.NewDirectChat(cid(t, 1), uid(t, 1), uid(t, 2), t0)
	if err != nil {
		t.Fatal(err)
	}
	m := c.Members()
	m[0] = uid(t, 9)
	if c.IsMember(uid(t, 9)) {
		t.Fatal("Members() должен отдавать копию: снаружи нельзя менять агрегат")
	}
}
```

`messenger/internal/user/domain/user_test.go` — табличные тесты `NewUsername` (`"ab"` ✗, `"abc"` ✓, 32 символа ✓, 33 ✗, `"Ivan"` ✗ — верхний регистр, `"иван"` ✗, `"ivan_99"` ✓, `"ivan-99"` ✗), `NewDisplayName` (`"  Иван  "` → `"Иван"`, пустое ✗, 64 руны ✓, 65 ✗, `"a\x00b"` ✗), `ParseUserID` (канонический UUID ✓, верхний регистр ✗, 35 символов ✗, без дефисов ✗).

Run (из `messenger/`): `go test ./internal/...` → FAIL (нет кода).

- [ ] **Step 2: Код `user/domain`**

`id.go`:

```go
package domain

// UserID — идентификатор пользователя. Обёртка-структура, а не `type UserID string`:
// строку можно привести к именованному строковому типу из любого пакета
// (`domain.UserID("мусор")`), а структуру с неэкспортируемым полем — нет.
// Получить UserID можно только через ParseUserID.
type UserID struct{ value string }

// ParseUserID принимает UUID в каноническом виде: 8-4-4-4-12, шестнадцатеричные цифры
// в нижнем регистре. Версию UUID домен не проверяет — генерация id не его забота.
func ParseUserID(s string) (UserID, error) {
	if !isUUID(s) {
		return UserID{}, invalid("user_id", "должен быть UUID в нижнем регистре")
	}
	return UserID{value: s}, nil
}

func (id UserID) String() string { return id.value }

// IsZero сообщает, что значение не было создано через ParseUserID (нулевое значение структуры).
func (id UserID) IsZero() bool { return id.value == "" }

func isUUID(s string) bool {
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

`user.go`:

```go
package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	UsernameMinLen    = 3
	UsernameMaxLen    = 32
	DisplayNameMaxLen = 64
)

// Username — уникальное имя для поиска и упоминаний: латиница в нижнем регистре, цифры, «_».
type Username struct{ value string }

// #region new-username
func NewUsername(s string) (Username, error) {
	if len(s) < UsernameMinLen || len(s) > UsernameMaxLen {
		return Username{}, invalid("username", fmt.Sprintf("длина от %d до %d символов", UsernameMinLen, UsernameMaxLen))
	}
	for i := range len(s) {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
			return Username{}, invalid("username", "только a–z, 0–9 и _")
		}
	}
	return Username{value: s}, nil
}

// #endregion new-username

func (u Username) String() string { return u.value }

// DisplayName — имя, которое видят собеседники. Любой язык, но без управляющих символов.
type DisplayName struct{ value string }

func NewDisplayName(s string) (DisplayName, error) {
	if !utf8.ValidString(s) {
		return DisplayName{}, invalid("display_name", "некорректный UTF-8")
	}
	s = strings.TrimSpace(s)
	if n := utf8.RuneCountInString(s); n == 0 || n > DisplayNameMaxLen {
		return DisplayName{}, invalid("display_name", fmt.Sprintf("от 1 до %d символов", DisplayNameMaxLen))
	}
	if strings.IndexFunc(s, unicode.IsControl) >= 0 {
		return DisplayName{}, invalid("display_name", "управляющие символы запрещены")
	}
	return DisplayName{value: s}, nil
}

func (d DisplayName) String() string { return d.value }

// User — сущность: её отличает id, а не набор полей.
type User struct {
	id          UserID
	username    Username
	displayName DisplayName
	createdAt   time.Time
}

// NewUser не возвращает ошибку: все аргументы уже прошли свои конструкторы.
func NewUser(id UserID, username Username, displayName DisplayName, createdAt time.Time) User {
	return User{id: id, username: username, displayName: displayName, createdAt: createdAt.UTC()}
}

func (u User) ID() UserID               { return u.id }
func (u User) Username() Username       { return u.username }
func (u User) DisplayName() DisplayName { return u.displayName }
func (u User) CreatedAt() time.Time     { return u.createdAt }
```

`errors.go` (вариант **l02**):

```go
package domain

import "errors"

// invalid — единая точка создания ошибок валидации. В уроке 1.3 она станет *ValidationError.
func invalid(field, reason string) error {
	return errors.New(field + ": " + reason)
}
```

- [ ] **Step 3: Код `chat/domain`**

`ids.go` — `ChatID`, `MessageID`, `UserID` по образцу `user/domain/id.go` (`Parse…` с полями ошибки `chat_id`, `message_id`, `user_id`, `String`, `IsZero`) и своя копия `isUUID` с комментарием: «Копия из user/domain — сознательная: контексты не делят код, чтобы в модуле 6 разойтись в разные сервисы. Третья копия — сигнал вынести в общий пакет (правило трёх)».

`text.go`:

```go
package domain

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	TextMaxLen  = 4096 // как у Telegram: длиннее — уже не сообщение, а документ
	TitleMaxLen = 128
)

// Text — текст сообщения. Длина считается в символах Unicode (рунах), а не в байтах:
// «я» в UTF-8 — два байта, эмодзи — четыре, а пользователь видит один символ.
type Text struct{ value string }

// #region new-text
func NewText(s string) (Text, error) {
	if !utf8.ValidString(s) {
		return Text{}, invalid("text", "некорректный UTF-8")
	}
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	if n == 0 {
		return Text{}, invalid("text", "пустое сообщение")
	}
	if n > TextMaxLen {
		return Text{}, invalid("text", fmt.Sprintf("не длиннее %d символов", TextMaxLen))
	}
	return Text{value: s}, nil
}

// #endregion new-text

func (t Text) String() string { return t.value }

// Title — название группы.
type Title struct{ value string }

func NewTitle(s string) (Title, error) {
	if !utf8.ValidString(s) {
		return Title{}, invalid("title", "некорректный UTF-8")
	}
	s = strings.TrimSpace(s)
	if n := utf8.RuneCountInString(s); n == 0 || n > TitleMaxLen {
		return Title{}, invalid("title", fmt.Sprintf("от 1 до %d символов", TitleMaxLen))
	}
	if strings.IndexFunc(s, unicode.IsControl) >= 0 {
		return Title{}, invalid("title", "управляющие символы запрещены")
	}
	return Title{value: s}, nil
}

func (t Title) String() string { return t.value }
```

`chat.go`:

```go
package domain

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

const (
	GroupMinMembers  = 2
	GroupMaxMembers  = 200
	ListLimitDefault = 50
	ListLimitMax     = 100
)

type Kind string

const (
	KindDirect Kind = "direct"
	KindGroup  Kind = "group"
)

func ParseKind(s string) (Kind, error) {
	switch k := Kind(s); k {
	case KindDirect, KindGroup:
		return k, nil
	default:
		return "", invalid("kind", `"direct" или "group"`)
	}
}

// #region chat
// Chat — агрегат: граница, внутри которой соблюдаются инварианты
// «отправитель — участник» и «seq идут подряд без пропусков».
// Менять чат можно только через его методы.
type Chat struct {
	id        ChatID
	kind      Kind
	title     Title
	members   []UserID // отсортированы, без повторов — для бинарного поиска
	lastSeq   int64
	version   int64 // растёт при каждом изменении — для оптимистической блокировки
	createdAt time.Time
}

// #endregion chat

func NewDirectChat(id ChatID, a, b UserID, createdAt time.Time) (*Chat, error) {
	if a == b {
		return nil, invalid("member_ids", "в личном чате два разных участника")
	}
	return &Chat{id: id, kind: KindDirect, members: normalizeMembers([]UserID{a, b}), createdAt: createdAt.UTC()}, nil
}

// NewGroupChat сам добавляет создателя в участники: группа, в которой нет её создателя,
// бессмысленна, а требовать это от клиента — лишняя ошибка на ровном месте.
func NewGroupChat(id ChatID, title Title, creator UserID, others []UserID, createdAt time.Time) (*Chat, error) {
	members := normalizeMembers(append([]UserID{creator}, others...))
	if len(members) < GroupMinMembers || len(members) > GroupMaxMembers {
		return nil, invalid("member_ids", fmt.Sprintf("в группе от %d до %d участников", GroupMinMembers, GroupMaxMembers))
	}
	return &Chat{id: id, kind: KindGroup, title: title, members: members, createdAt: createdAt.UTC()}, nil
}

func normalizeMembers(ids []UserID) []UserID {
	out := slices.Clone(ids)
	slices.SortFunc(out, compareUserID)
	return slices.Compact(out)
}

func compareUserID(a, b UserID) int { return strings.Compare(a.value, b.value) }

// DirectKeyFor — ключ пары для личного чата, не зависящий от порядка участников.
func DirectKeyFor(a, b UserID) string {
	if compareUserID(a, b) > 0 {
		a, b = b, a
	}
	return a.value + "|" + b.value
}

func (c *Chat) ID() ChatID           { return c.id }
func (c *Chat) Kind() Kind           { return c.kind }
func (c *Chat) Title() Title         { return c.title }
func (c *Chat) LastSeq() int64       { return c.lastSeq }
func (c *Chat) Version() int64       { return c.version }
func (c *Chat) CreatedAt() time.Time { return c.createdAt }

// Members отдаёт копию: срез, отданный наружу как есть, позволил бы менять агрегат в обход его правил.
func (c *Chat) Members() []UserID { return slices.Clone(c.members) }

func (c *Chat) IsMember(u UserID) bool {
	_, ok := slices.BinarySearchFunc(c.members, u, compareUserID)
	return ok
}

// DirectKey — ключ пары для личного чата; для группы ok == false.
func (c *Chat) DirectKey() (key string, ok bool) {
	if c.kind != KindDirect {
		return "", false
	}
	return DirectKeyFor(c.members[0], c.members[1]), true
}

// #region post
// Post — единственный способ добавить сообщение. Номер выдаёт агрегат,
// а не хранилище: правило «seq подряд» — бизнес-правило, ему место в домене.
func (c *Chat) Post(id MessageID, author UserID, text Text, sentAt time.Time) (Message, error) {
	if !c.IsMember(author) {
		return Message{}, ErrNotMember
	}
	c.lastSeq++
	c.version++
	return Message{id: id, chatID: c.id, authorID: author, text: text, seq: c.lastSeq, sentAt: sentAt.UTC()}, nil
}

// #endregion post
```

`message.go`:

```go
package domain

import "time"

// Message — неизменяемое значение: после Post его поля не меняются.
type Message struct {
	id       MessageID
	chatID   ChatID
	authorID UserID
	text     Text
	seq      int64
	sentAt   time.Time
}

func (m Message) ID() MessageID     { return m.id }
func (m Message) ChatID() ChatID    { return m.chatID }
func (m Message) AuthorID() UserID  { return m.authorID }
func (m Message) Text() Text        { return m.text }
func (m Message) Seq() int64        { return m.seq }
func (m Message) SentAt() time.Time { return m.sentAt }
```

`errors.go` (вариант **l02**):

```go
package domain

import "errors"

// ErrNotMember — нарушение инварианта агрегата: писать и читать может только участник.
var ErrNotMember = errors.New("пользователь не участник чата")

func invalid(field, reason string) error {
	return errors.New(field + ": " + reason)
}
```

Run: `go test -race ./internal/...` → PASS; `golangci-lint run ./...` → без замечаний.

- [ ] **Step 4: Урок `02-domain.md`**

Frontmatter: `title: 1.2. Домен: правила, которые нельзя нарушить`.

**Разделы:**
- **Проблема.** В базе нашлось сообщение из 0 символов и группа из одного человека: проверки были в HTTP-обработчике, а импорт из CSV их обошёл. Вывод: правило должно жить там, где его нельзя обойти.
- **Теория:**
  - DDD-lite (Ханонов): `ubiquitous-language` (словарь мессенджера: участник, личный чат, группа, сообщение, номер в чате), `bounded-context` (`user` и `chat` — что такое «пользователь» в каждом), почему «lite» — без событий и тактических паттернов, которые нам пока не нужны;
  - `entity` против `value-object` на примерах `User` и `Text`;
  - `invariant`; «невалидный объект нельзя создать»: конструктор `NewX(...) (X, error)` и обёртка-структура против `type X string` (код `id.go` регионом и антипример с приведением строки);
  - нулевое значение в Go и `IsZero` — где Go не даёт гарантии и как с этим жить;
  - валидация ввода (формат, лимиты — в объектах-значениях) против инвариантов, которым нужно состояние (участник чата) — `aggregate` как идея; подробно — урок 1.4;
  - руны против байтов: `utf8.RuneCountInString`, почему 4096 «я» — 8192 байта;
  - лимиты из урока 0.6 (200 байт на сообщение — среднее; 4096 — максимум) как именованные константы.
- **Визуализация:** `<ValidationGates />` и задания.
- **Код:** регионы `new-text`, `chat`, `post`, `new-username`; таблица тестов `TestNewText` регионом (добавить `// #region text-table` в тест) и реальный вывод `go test -race -run 'TestNewText' -v ./internal/chat/domain/`.
- **Под капотом:** «Неизменяемость в Go без `final`»: неэкспортируемые поля, копирование значений, срезы и мапы как скрытые ссылки (почему `Members()` отдаёт копию — тест `TestMembersIsCopy`).
- **Ошибки:** «анемичная модель» (структура с публичными полями и сервис рядом); валидация в обработчике; `type Username string` с «обещанием» звать конструктор; подсчёт длины в байтах.
- **Компромисс:** больше кода (геттеры, конструкторы) против гарантии, что домен всегда валиден.
- **Самопроверка, источники.**
- **Что изменилось в мессенджере** (`git diff m01-l01 m01-l02 --stat`).

**Термины:** `ddd`, `ubiquitous-language`, `bounded-context`, `entity`, `value-object`, `invariant`, `aggregate`, `table-driven-test`.

**Факты проверить:** `utf8.RuneCountInString` и поведение `strings.TrimSpace` на невалидном UTF-8; лимит Telegram 4096 символов (документация Bot API, `sendMessage`); главы Ханонова (ограниченные контексты — гл. 3, тактика — гл. 6).

- [ ] **Step 5: Проверка, коммит, тег**

```bash
git add messenger course
git commit -m "feat(messenger): домен пользователей и чатов; урок 1.2

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git tag m01-l02
```

---

### Task 6: Урок 1.3 «Ошибки» + доменные ошибки

**Files:**
- Modify: `messenger/internal/user/domain/errors.go`, `messenger/internal/chat/domain/errors.go`
- Create: `messenger/internal/user/domain/errors_test.go`, `messenger/internal/chat/domain/errors_test.go`
- Create: `course/modules/01-go-service/03-errors.md`

**Interfaces:**
- Consumes: `invalid()` из задачи 5.
- Produces: в обоих доменах `ErrInvalid`, `*ValidationError{Field, Reason string}` с методами `Error()` и `Is(error) bool`; `user/domain`: `ErrUserNotFound`, `ErrUsernameTaken`; `chat/domain`: `ErrChatNotFound`, `ErrNotMember`, `ErrConcurrentUpdate`, `ErrDirectChatExists`.

- [ ] **Step 1: Тест (падает)**

`messenger/internal/chat/domain/errors_test.go`:

```go
package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"messenger/internal/chat/domain"
)

func TestValidationErrorIsAndAs(t *testing.T) {
	t.Parallel()
	_, err := domain.NewText("")
	wrapped := fmt.Errorf("send message: %w", err)

	if !errors.Is(wrapped, domain.ErrInvalid) {
		t.Fatalf("errors.Is(%v, ErrInvalid) = false", wrapped)
	}
	var ve *domain.ValidationError
	if !errors.As(wrapped, &ve) {
		t.Fatalf("errors.As не нашёл *ValidationError в %v", wrapped)
	}
	if ve.Field != "text" {
		t.Fatalf("Field = %q, ждали text", ve.Field)
	}
	if errors.Is(wrapped, domain.ErrChatNotFound) {
		t.Fatal("ошибка валидации не должна совпадать с ErrChatNotFound")
	}
}

func TestPercentVBreaksChain(t *testing.T) {
	t.Parallel()
	// %v превращает ошибку в текст: цепочка обрывается, errors.Is её больше не видит.
	lost := fmt.Errorf("send message: %v", domain.ErrChatNotFound)
	if errors.Is(lost, domain.ErrChatNotFound) {
		t.Fatal("с %v цепочки быть не должно")
	}
}
```

Аналогичный `user/domain/errors_test.go` для `NewUsername` и `ErrInvalid`.

Run: `go test ./internal/...` → FAIL (`ErrInvalid` не определён).

- [ ] **Step 2: Реализация**

`messenger/internal/chat/domain/errors.go`:

```go
package domain

import (
	"errors"
	"fmt"
)

// #region sentinels
// Сигнальные ошибки — то, что вызывающий код различает через errors.Is.
// Текст — для людей и логов; полагаться на него в коде нельзя.
var (
	ErrInvalid          = errors.New("некорректные данные")
	ErrChatNotFound     = errors.New("чат не найден")
	ErrNotMember        = errors.New("пользователь не участник чата")
	ErrConcurrentUpdate = errors.New("чат изменился одновременно с вами")
	ErrDirectChatExists = errors.New("личный чат для этой пары уже есть")
)

// #endregion sentinels

// #region validation-error
// ValidationError несёт поле и причину — их HTTP-адаптер отдаст клиенту.
// Метод Is делает любую ValidationError равной ErrInvalid для errors.Is:
// вызывающему, которому всё равно, какое поле, хватает одной проверки.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Reason) }

func (e *ValidationError) Is(target error) bool { return target == ErrInvalid }

func invalid(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason}
}

// #endregion validation-error
```

`messenger/internal/user/domain/errors.go` — то же для `user`: `ErrInvalid`, `ErrUserNotFound = errors.New("пользователь не найден")`, `ErrUsernameTaken = errors.New("имя пользователя занято")`, `ValidationError`, `invalid`.

Run: `go test -race ./internal/...` → PASS; `golangci-lint run ./...` (в т. ч. `errorlint`) → чисто.

- [ ] **Step 3: Урок `03-errors.md`**

Frontmatter: `title: 1.3. Ошибки в Go: сигналы, обёртки и границы`.

**Разделы:**
- **Проблема.** Клиент получает 500 на «чат не найден», потому что где-то ошибку обернули через `%v` и `errors.Is` перестал её видеть; в логах одна ошибка пять раз — её логировали на каждом уровне.
- **Теория:**
  - ошибка как значение: интерфейс `error`, почему нет исключений;
  - `sentinel-error`: `var ErrX = errors.New(…)`, сравнение через `errors.Is`, а не `==` и не по тексту;
  - типизированные ошибки и `errors.As`; метод `Is` у типа (наш `ValidationError`);
  - `error-wrapping`: `%w` против `%v`, когда оборачивать (добавляем контекст: «send message: …»), когда возвращать как есть; несколько `%w` и `errors.Join` (Go 1.20);
  - правило «обработай или верни, но не оба»: логирует тот, кто принимает решение (адаптер), — 100 Go Mistakes № 52;
  - доменные ошибки против инфраструктурных; перевод в HTTP-статус — только в адаптере (таблица-набросок: `ErrInvalid` → 400, `ErrChatNotFound` → 404, `ErrNotMember` → 403, `ErrConcurrentUpdate` → 409, прочее → 500; реализация — урок 1.7);
  - паника — только на старте (стандарт).
- **Визуализация:** `<ErrorChain />` и задания.
- **Код:** регионы `sentinels`, `validation-error`; оба теста из шага 1 регионом и реальный вывод `go test -race -run 'Validation|PercentV' -v ./internal/chat/domain/`; `git diff m01-l02 m01-l03 -- messenger/internal/chat/domain/errors.go` — как одна функция `invalid` поменяла все ошибки валидации сразу.
- **Под капотом:** «Как устроены `errors.Is` и `errors.As`»: обход дерева через `Unwrap() error` и `Unwrap() []error`, порядок (в глубину), вызов метода `Is`/`As` у узла, почему `As` требует указатель на переменную нужного типа.
- **Ошибки:** сравнение по `err.Error()`; `%v` вместо `%w`; логирование на каждом уровне; возврат `nil`-указателя на тип ошибки как `error` (интерфейс не `nil`) — с `<!-- go-check -->`-примером; экспорт обёрнутых ошибок сторонних библиотек как части API.
- **Компромисс:** сигнальные ошибки — часть API пакета (их нельзя молча переименовать) против удобства различения.
- **Самопроверка, источники** (Харсаньи, гл. 7; go.dev/blog «Working with Errors in Go 1.13»; документация `errors`).
- **Что изменилось в мессенджере.**

**Термины:** `sentinel-error`, `error-wrapping`.

**Факты проверить:** по `$(go env GOROOT)/src/errors/wrap.go` — алгоритм `Is`/`As`; версия Go, где появились `errors.Join` и несколько `%w` (1.20); номер «ошибки» у Харсаньи про двойное логирование.

- [ ] **Step 4: Проверка, коммит, тег**

```bash
git add messenger course
git commit -m "feat(messenger): доменные ошибки — сигнальные и ValidationError; урок 1.3

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git tag m01-l03
```

---

### Task 7: Урок 1.4 «Use case'ы и порты» + хранилища в памяти

**Files:**
- Create: `messenger/internal/user/usecase/{ports.go,register.go,get.go,usecase_test.go}`
- Create: `messenger/internal/chat/usecase/{ports.go,create_chat.go,send_message.go,list_messages.go,fakes_test.go,usecase_test.go}`
- Create: `messenger/internal/user/adapters/memory/{store.go,store_test.go}`, `messenger/internal/chat/adapters/memory/{store.go,store_test.go}`
- Create: `messenger/internal/platform/clock/clock.go`, `messenger/internal/platform/idgen/{idgen.go,idgen_test.go}`
- Modify: `messenger/internal/chat/domain/chat.go` (снимок агрегата), `messenger/go.mod`, `messenger/go.sum`
- Create: `course/modules/01-go-service/04-usecases.md`

**Interfaces:**
- Consumes: домен и ошибки (задачи 5–6).
- Produces (используют задачи 8–10):
  - `chat/domain`: `ChatSnapshot{ID ChatID; Kind Kind; Title Title; Members []UserID; LastSeq, Version int64; CreatedAt time.Time}`, `(*Chat).Snapshot() ChatSnapshot`, `RestoreChat(ChatSnapshot) *Chat`.
  - `user/usecase`: порты `Clock`, `IDGenerator`, `UserCreator`, `UserFinder`; `RegisterUserCommand{Username, DisplayName string}`, `NewRegisterUser(UserCreator, Clock, IDGenerator) (*RegisterUser, error)`, `(*RegisterUser).Execute(ctx, RegisterUserCommand) (domain.User, error)`; `GetUserQuery{ID string}`, `NewGetUser(UserFinder) (*GetUser, error)`, `(*GetUser).Execute(ctx, GetUserQuery) (domain.User, error)`.
  - `chat/usecase`: порты `Clock`, `IDGenerator`, `UserChecker`, `ChatCreator`, `ChatFinder`, `DirectChatFinder`, `MessageAppender`, `MessageLister`; `CreateChatCommand{CreatorID, Kind, Title string; MemberIDs []string}`, `CreateChatResult{Chat *domain.Chat; Created bool}`, `NewCreateChat(ChatCreator, DirectChatFinder, UserChecker, Clock, IDGenerator) (*CreateChat, error)`; `SendMessageCommand{ChatID, AuthorID, Text string}`, `MaxSendAttempts = 3`, `NewSendMessage(ChatFinder, MessageAppender, Clock, IDGenerator) (*SendMessage, error)`; `ListMessagesQuery{ChatID, ReaderID string; AfterSeq int64; Limit int}`, `NewListMessages(ChatFinder, MessageLister) (*ListMessages, error)`.
  - `user/adapters/memory`: `NewStore() *Store` — реализует `UserCreator`, `UserFinder`.
  - `chat/adapters/memory`: `NewStore() *Store` — реализует `ChatCreator`, `ChatFinder`, `DirectChatFinder`, `MessageAppender`, `MessageLister`.
  - `platform/clock`: `type System struct{}`, `(System) Now() time.Time`. `platform/idgen`: `type UUIDv7 struct{}`, `(UUIDv7) NewID() string`.

- [ ] **Step 1: Снимок агрегата**

Хранилищу нужно скопировать агрегат и восстановить его, не трогая неэкспортируемые поля. В модуле 2a тем же снимком воспользуется Postgres-адаптер. В `chat/domain/chat.go` добавить:

```go
// ChatSnapshot — плоская копия состояния агрегата для хранилищ.
// Без тегов json/db: как раскладывать поля по колонкам, решает адаптер.
type ChatSnapshot struct {
	ID        ChatID
	Kind      Kind
	Title     Title
	Members   []UserID
	LastSeq   int64
	Version   int64
	CreatedAt time.Time
}

func (c *Chat) Snapshot() ChatSnapshot {
	return ChatSnapshot{
		ID: c.id, Kind: c.kind, Title: c.title, Members: slices.Clone(c.members),
		LastSeq: c.lastSeq, Version: c.version, CreatedAt: c.createdAt,
	}
}

// RestoreChat доверяет снимку: он получен из Snapshot и сохранён нашим же хранилищем,
// поэтому инварианты повторно не проверяются.
func RestoreChat(s ChatSnapshot) *Chat {
	return &Chat{
		id: s.ID, kind: s.Kind, title: s.Title, members: slices.Clone(s.Members),
		lastSeq: s.LastSeq, version: s.Version, createdAt: s.CreatedAt,
	}
}
```

Тест в `chat_test.go`: `RestoreChat(c.Snapshot())` даёт чат с теми же `Members`, `LastSeq`, `Version`, а изменение `Members` снимка не меняет исходный чат.

- [ ] **Step 2: Порты**

`messenger/internal/chat/usecase/ports.go`:

```go
package usecase

import (
	"context"
	"time"

	"messenger/internal/chat/domain"
)

// #region ports
// Порты объявлены здесь, у потребителя, и узкие — по одному-два метода:
// use case просит ровно то, что ему нужно, и не знает, кто это реализует.

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() string
}

// UserChecker отвечает, кого из пользователей не существует.
// Реализация — адаптер над контекстом user (урок 1.7), в модуле 6 — gRPC-клиент.
type UserChecker interface {
	MissingUsers(ctx context.Context, ids []domain.UserID) ([]domain.UserID, error)
}

type ChatCreator interface {
	// CreateChat сохраняет новый чат. Для личного чата, если для пары он уже есть,
	// возвращает domain.ErrDirectChatExists.
	CreateChat(ctx context.Context, chat *domain.Chat) error
}

type ChatFinder interface {
	// FindChat возвращает domain.ErrChatNotFound, если чата нет.
	FindChat(ctx context.Context, id domain.ChatID) (*domain.Chat, error)
}

type DirectChatFinder interface {
	FindDirectChat(ctx context.Context, a, b domain.UserID) (*domain.Chat, error)
}

type MessageAppender interface {
	// AppendMessage сохраняет сообщение и новое состояние чата, только если версия
	// чата в хранилище всё ещё expectedVersion; иначе — domain.ErrConcurrentUpdate.
	AppendMessage(ctx context.Context, chat *domain.Chat, msg domain.Message, expectedVersion int64) error
}

type MessageLister interface {
	// ListMessages возвращает до limit сообщений с seq > afterSeq по возрастанию seq.
	ListMessages(ctx context.Context, chatID domain.ChatID, afterSeq int64, limit int) ([]domain.Message, error)
}

// #endregion ports
```

`messenger/internal/user/usecase/ports.go` — `Clock`, `IDGenerator` (те же сигнатуры — порты дублируются в каждом контексте сознательно), `UserCreator{CreateUser(ctx, domain.User) error}` (`ErrUsernameTaken` при занятом имени), `UserFinder{FindUser(ctx, domain.UserID) (domain.User, error)}` (`ErrUserNotFound`).

- [ ] **Step 3: Тесты use case'ов на фейках (падают)**

`messenger/internal/chat/usecase/fakes_test.go`:

```go
package usecase_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"messenger/internal/chat/domain"
)

var t0 = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return t0 }

// seqIDs выдаёт предсказуемые UUID: тесты сравнивают id, а не случайные строки.
type seqIDs struct{ n atomic.Int64 }

func (g *seqIDs) NewID() string {
	return fmt.Sprintf("00000000-0000-7000-b000-%012d", g.n.Add(1))
}

// knownUsers — фейк UserChecker: «существуют» только перечисленные пользователи.
type knownUsers struct {
	mu  sync.Mutex
	ids map[domain.UserID]bool
}

func newKnownUsers(ids ...domain.UserID) *knownUsers {
	k := &knownUsers{ids: make(map[domain.UserID]bool)}
	for _, id := range ids {
		k.ids[id] = true
	}
	return k
}

func (k *knownUsers) MissingUsers(_ context.Context, ids []domain.UserID) ([]domain.UserID, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	var missing []domain.UserID
	for _, id := range ids {
		if !k.ids[id] {
			missing = append(missing, id)
		}
	}
	return missing, nil
}
```

`messenger/internal/chat/usecase/usecase_test.go` — тесты против настоящего `memory.Store` (он быстрый, без сети и диска — по стандарту это допустимый фейк порта) и фейков выше:

```go
package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"messenger/internal/chat/adapters/memory"
	"messenger/internal/chat/domain"
	"messenger/internal/chat/usecase"
)

const (
	alice = "00000000-0000-7000-8000-000000000001"
	bob   = "00000000-0000-7000-8000-000000000002"
	eve   = "00000000-0000-7000-8000-000000000003"
)

type env struct {
	create *usecase.CreateChat
	send   *usecase.SendMessage
	list   *usecase.ListMessages
}

func newEnv(t *testing.T) env {
	t.Helper()
	store := memory.NewStore()
	ids := &seqIDs{}
	users := newKnownUsers(mustUser(t, alice), mustUser(t, bob), mustUser(t, eve))
	create, err := usecase.NewCreateChat(store, store, users, fixedClock{}, ids)
	if err != nil {
		t.Fatal(err)
	}
	send, err := usecase.NewSendMessage(store, store, fixedClock{}, ids)
	if err != nil {
		t.Fatal(err)
	}
	list, err := usecase.NewListMessages(store, store)
	if err != nil {
		t.Fatal(err)
	}
	return env{create: create, send: send, list: list}
}

func mustUser(t *testing.T, s string) domain.UserID {
	t.Helper()
	id, err := domain.ParseUserID(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func directChat(t *testing.T, e env) *domain.Chat {
	t.Helper()
	res, err := e.create.Execute(context.Background(), usecase.CreateChatCommand{CreatorID: alice, Kind: "direct", MemberIDs: []string{bob}})
	if err != nil {
		t.Fatal(err)
	}
	return res.Chat
}

func TestCreateDirectChatTwiceReturnsExisting(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	first := directChat(t, e)
	res, err := e.create.Execute(context.Background(), usecase.CreateChatCommand{CreatorID: bob, Kind: "direct", MemberIDs: []string{alice}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Created || res.Chat.ID() != first.ID() {
		t.Fatalf("ждали существующий чат %s, получили %s (created=%v)", first.ID(), res.Chat.ID(), res.Created)
	}
}

func TestCreateDirectChatConcurrent(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	const n = 20
	ids := make([]domain.ChatID, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			res, err := e.create.Execute(context.Background(), usecase.CreateChatCommand{CreatorID: alice, Kind: "direct", MemberIDs: []string{bob}})
			if err != nil {
				t.Error(err)
				return
			}
			ids[i] = res.Chat.ID()
		})
	}
	wg.Wait()
	for _, id := range ids[1:] {
		if id != ids[0] {
			t.Fatalf("для одной пары создано несколько личных чатов: %s и %s", ids[0], id)
		}
	}
}

func TestCreateChatUnknownMember(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	_, err := e.create.Execute(context.Background(), usecase.CreateChatCommand{
		CreatorID: alice, Kind: "direct", MemberIDs: []string{"00000000-0000-7000-8000-000000000099"},
	})
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Field != "member_ids" {
		t.Fatalf("ждали ValidationError по member_ids, получили %v", err)
	}
}

func TestSendAndList(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	c := directChat(t, e)
	ctx := context.Background()
	for i := 1; i <= 3; i++ {
		m, err := e.send.Execute(ctx, usecase.SendMessageCommand{ChatID: c.ID().String(), AuthorID: alice, Text: fmt.Sprintf("сообщение %d", i)})
		if err != nil {
			t.Fatal(err)
		}
		if m.Seq() != int64(i) {
			t.Fatalf("seq = %d, ждали %d", m.Seq(), i)
		}
	}
	got, err := e.list.Execute(ctx, usecase.ListMessagesQuery{ChatID: c.ID().String(), ReaderID: bob, AfterSeq: 1, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Seq() != 2 || got[1].Seq() != 3 {
		t.Fatalf("ждали seq 2 и 3, получили %d сообщений", len(got))
	}
}

func TestNonMemberCannotSendOrRead(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	c := directChat(t, e)
	ctx := context.Background()
	_, err := e.send.Execute(ctx, usecase.SendMessageCommand{ChatID: c.ID().String(), AuthorID: eve, Text: "привет"})
	if !errors.Is(err, domain.ErrNotMember) {
		t.Fatalf("send: ждали ErrNotMember, получили %v", err)
	}
	_, err = e.list.Execute(ctx, usecase.ListMessagesQuery{ChatID: c.ID().String(), ReaderID: eve})
	if !errors.Is(err, domain.ErrNotMember) {
		t.Fatalf("list: ждали ErrNotMember, получили %v", err)
	}
}

func TestListLimits(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	c := directChat(t, e)
	tests := []struct {
		name  string
		after int64
		limit int
		field string
	}{
		{name: "отрицательный after", after: -1, field: "after"},
		{name: "limit больше максимума", limit: domain.ListLimitMax + 1, field: "limit"},
		{name: "отрицательный limit", limit: -1, field: "limit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := e.list.Execute(context.Background(), usecase.ListMessagesQuery{ChatID: c.ID().String(), ReaderID: alice, AfterSeq: tt.after, Limit: tt.limit})
			var ve *domain.ValidationError
			if !errors.As(err, &ve) || ve.Field != tt.field {
				t.Fatalf("ждали ValidationError по %s, получили %v", tt.field, err)
			}
		})
	}
}

func TestSendMessageConcurrent(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	c := directChat(t, e)
	const workers, perWorker = 8, 50
	var (
		mu        sync.Mutex
		ok, retry int
	)
	var wg sync.WaitGroup
	for w := range workers {
		wg.Go(func() {
			for i := range perWorker {
				author := alice
				if (w+i)%2 == 0 {
					author = bob
				}
				_, err := e.send.Execute(context.Background(), usecase.SendMessageCommand{ChatID: c.ID().String(), AuthorID: author, Text: "x"})
				mu.Lock()
				switch {
				case err == nil:
					ok++
				case errors.Is(err, domain.ErrConcurrentUpdate):
					retry++ // честный исход оптимистической блокировки под нагрузкой
				default:
					t.Error(err)
				}
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	if ok == 0 || ok+retry != workers*perWorker {
		t.Fatalf("успешно %d, конфликтов %d, всего ждали %d", ok, retry, workers*perWorker)
	}
	var seqs []int64
	for after := int64(0); ; {
		page, err := e.list.Execute(context.Background(), usecase.ListMessagesQuery{ChatID: c.ID().String(), ReaderID: alice, AfterSeq: after, Limit: domain.ListLimitMax})
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		for _, m := range page {
			seqs = append(seqs, m.Seq())
		}
		after = page[len(page)-1].Seq()
	}
	if len(seqs) != ok {
		t.Fatalf("в истории %d сообщений, успешных отправок %d", len(seqs), ok)
	}
	for i, s := range seqs {
		if s != int64(i+1) {
			t.Fatalf("seq идут с пропуском или повтором: позиция %d, seq %d", i, s)
		}
	}
	t.Logf("успешно %d, исчерпали %d попытки: %d", ok, usecase.MaxSendAttempts, retry)
}
```

Тесты `user/usecase` (`usecase_test.go`): регистрация возвращает пользователя с id от генератора и временем от часов; повтор имени → `errors.Is(err, domain.ErrUsernameTaken)`; плохое имя → `ValidationError` по `username`, хранилище не вызывалось (фейк-счётчик); `GetUser` неизвестного → `ErrUserNotFound`; кривой id → `ValidationError` по `user_id`. Конструкторы с `nil` зависимостью возвращают ошибку (табличный тест).

Run: `go test ./internal/...` → FAIL.

- [ ] **Step 4: Use case'ы `chat`**

`create_chat.go`:

```go
package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"messenger/internal/chat/domain"
)

type CreateChatCommand struct {
	CreatorID string
	Kind      string
	Title     string
	MemberIDs []string // без создателя; для личного чата — ровно один собеседник
}

type CreateChatResult struct {
	Chat    *domain.Chat
	Created bool // false — личный чат для этой пары уже был
}

type CreateChat struct {
	chats  ChatCreator
	direct DirectChatFinder
	users  UserChecker
	clock  Clock
	ids    IDGenerator
}

func NewCreateChat(chats ChatCreator, direct DirectChatFinder, users UserChecker, clock Clock, ids IDGenerator) (*CreateChat, error) {
	if chats == nil || direct == nil || users == nil || clock == nil || ids == nil {
		return nil, errors.New("create chat: nil-зависимость")
	}
	return &CreateChat{chats: chats, direct: direct, users: users, clock: clock, ids: ids}, nil
}

func (uc *CreateChat) Execute(ctx context.Context, cmd CreateChatCommand) (CreateChatResult, error) {
	kind, err := domain.ParseKind(cmd.Kind)
	if err != nil {
		return CreateChatResult{}, err
	}
	creator, err := domain.ParseUserID(cmd.CreatorID)
	if err != nil {
		return CreateChatResult{}, err
	}
	others := make([]domain.UserID, 0, len(cmd.MemberIDs))
	for _, s := range cmd.MemberIDs {
		id, err := domain.ParseUserID(s)
		if err != nil {
			return CreateChatResult{}, err
		}
		others = append(others, id)
	}
	if err := uc.ensureUsersExist(ctx, append([]domain.UserID{creator}, others...)); err != nil {
		return CreateChatResult{}, err
	}
	if kind == domain.KindDirect {
		return uc.createDirect(ctx, creator, others)
	}
	return uc.createGroup(ctx, creator, cmd.Title, others)
}

func (uc *CreateChat) ensureUsersExist(ctx context.Context, ids []domain.UserID) error {
	missing, err := uc.users.MissingUsers(ctx, ids)
	if err != nil {
		return fmt.Errorf("create chat: проверка участников: %w", err)
	}
	if len(missing) > 0 {
		names := make([]string, len(missing))
		for i, id := range missing {
			names[i] = id.String()
		}
		return &domain.ValidationError{Field: "member_ids", Reason: "нет таких пользователей: " + strings.Join(names, ", ")}
	}
	return nil
}

func (uc *CreateChat) createDirect(ctx context.Context, creator domain.UserID, others []domain.UserID) (CreateChatResult, error) {
	if len(others) != 1 {
		return CreateChatResult{}, &domain.ValidationError{Field: "member_ids", Reason: "для личного чата нужен ровно один собеседник"}
	}
	peer := others[0]
	if existing, err := uc.direct.FindDirectChat(ctx, creator, peer); err == nil {
		return CreateChatResult{Chat: existing}, nil
	} else if !errors.Is(err, domain.ErrChatNotFound) {
		return CreateChatResult{}, fmt.Errorf("create chat: %w", err)
	}
	id, err := uc.newChatID()
	if err != nil {
		return CreateChatResult{}, err
	}
	chat, err := domain.NewDirectChat(id, creator, peer, uc.clock.Now())
	if err != nil {
		return CreateChatResult{}, err
	}
	err = uc.chats.CreateChat(ctx, chat)
	if errors.Is(err, domain.ErrDirectChatExists) {
		// Гонка: между FindDirectChat и CreateChat чат для пары создал кто-то другой.
		existing, ferr := uc.direct.FindDirectChat(ctx, creator, peer)
		if ferr != nil {
			return CreateChatResult{}, fmt.Errorf("create chat: после гонки: %w", ferr)
		}
		return CreateChatResult{Chat: existing}, nil
	}
	if err != nil {
		return CreateChatResult{}, fmt.Errorf("create chat: %w", err)
	}
	return CreateChatResult{Chat: chat, Created: true}, nil
}

func (uc *CreateChat) createGroup(ctx context.Context, creator domain.UserID, rawTitle string, others []domain.UserID) (CreateChatResult, error) {
	title, err := domain.NewTitle(rawTitle)
	if err != nil {
		return CreateChatResult{}, err
	}
	id, err := uc.newChatID()
	if err != nil {
		return CreateChatResult{}, err
	}
	chat, err := domain.NewGroupChat(id, title, creator, others, uc.clock.Now())
	if err != nil {
		return CreateChatResult{}, err
	}
	if err := uc.chats.CreateChat(ctx, chat); err != nil {
		return CreateChatResult{}, fmt.Errorf("create chat: %w", err)
	}
	return CreateChatResult{Chat: chat, Created: true}, nil
}

func (uc *CreateChat) newChatID() (domain.ChatID, error) {
	id, err := domain.ParseChatID(uc.ids.NewID())
	if err != nil {
		return domain.ChatID{}, fmt.Errorf("create chat: генератор выдал не UUID: %w", err)
	}
	return id, nil
}
```

`send_message.go`:

```go
package usecase

import (
	"context"
	"errors"
	"fmt"

	"messenger/internal/chat/domain"
)

// MaxSendAttempts — сколько раз повторять отправку при конфликте версий.
// Больше трёх — признак сильной конкуренции за один чат: лучше честно вернуть
// ошибку (клиент повторит позже), чем крутиться в цикле (модуль 5 — ретраи с джиттером).
const MaxSendAttempts = 3

type SendMessageCommand struct {
	ChatID   string
	AuthorID string
	Text     string
}

type SendMessage struct {
	chats    ChatFinder
	messages MessageAppender
	clock    Clock
	ids      IDGenerator
}

func NewSendMessage(chats ChatFinder, messages MessageAppender, clock Clock, ids IDGenerator) (*SendMessage, error) {
	if chats == nil || messages == nil || clock == nil || ids == nil {
		return nil, errors.New("send message: nil-зависимость")
	}
	return &SendMessage{chats: chats, messages: messages, clock: clock, ids: ids}, nil
}

// #region send-execute
func (uc *SendMessage) Execute(ctx context.Context, cmd SendMessageCommand) (domain.Message, error) {
	chatID, err := domain.ParseChatID(cmd.ChatID)
	if err != nil {
		return domain.Message{}, err
	}
	author, err := domain.ParseUserID(cmd.AuthorID)
	if err != nil {
		return domain.Message{}, err
	}
	text, err := domain.NewText(cmd.Text)
	if err != nil {
		return domain.Message{}, err
	}
	msgID, err := domain.ParseMessageID(uc.ids.NewID())
	if err != nil {
		return domain.Message{}, fmt.Errorf("send message: генератор выдал не UUID: %w", err)
	}
	sentAt := uc.clock.Now()

	for attempt := 1; ; attempt++ {
		chat, err := uc.chats.FindChat(ctx, chatID)
		if err != nil {
			return domain.Message{}, fmt.Errorf("send message: %w", err)
		}
		expected := chat.Version()
		msg, err := chat.Post(msgID, author, text, sentAt)
		if err != nil {
			return domain.Message{}, err
		}
		err = uc.messages.AppendMessage(ctx, chat, msg, expected)
		if err == nil {
			return msg, nil
		}
		if !errors.Is(err, domain.ErrConcurrentUpdate) || attempt == MaxSendAttempts {
			return domain.Message{}, fmt.Errorf("send message (попытка %d): %w", attempt, err)
		}
	}
}

// #endregion send-execute
```

`list_messages.go`:

```go
package usecase

import (
	"context"
	"errors"
	"fmt"

	"messenger/internal/chat/domain"
)

type ListMessagesQuery struct {
	ChatID   string
	ReaderID string
	AfterSeq int64 // 0 — с начала
	Limit    int   // 0 — domain.ListLimitDefault
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

func (uc *ListMessages) Execute(ctx context.Context, q ListMessagesQuery) ([]domain.Message, error) {
	chatID, err := domain.ParseChatID(q.ChatID)
	if err != nil {
		return nil, err
	}
	reader, err := domain.ParseUserID(q.ReaderID)
	if err != nil {
		return nil, err
	}
	if q.AfterSeq < 0 {
		return nil, &domain.ValidationError{Field: "after", Reason: "не может быть отрицательным"}
	}
	limit := q.Limit
	if limit == 0 {
		limit = domain.ListLimitDefault
	}
	if limit < 1 || limit > domain.ListLimitMax {
		return nil, &domain.ValidationError{Field: "limit", Reason: fmt.Sprintf("от 1 до %d", domain.ListLimitMax)}
	}
	chat, err := uc.chats.FindChat(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	if !chat.IsMember(reader) {
		return nil, domain.ErrNotMember
	}
	msgs, err := uc.messages.ListMessages(ctx, chatID, q.AfterSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	return msgs, nil
}
```

- [ ] **Step 5: Use case'ы `user`**

`register.go` и `get.go` — по образцу `chat`:
- `RegisterUser.Execute`: `NewUsername` → `NewDisplayName` → `ParseUserID(uc.ids.NewID())` (ошибка генератора оборачивается) → `NewUser(…, uc.clock.Now())` → `CreateUser` (ошибка оборачивается `register user: %w`) → пользователь.
- `GetUser.Execute`: `ParseUserID(q.ID)` → `FindUser` (ошибка оборачивается `get user: %w`).
- Конструкторы проверяют `nil`.

- [ ] **Step 6: Хранилища в памяти**

`messenger/internal/chat/adapters/memory/store.go`:

```go
// Package memory — хранилище чатов в памяти процесса. Годится для тестов и v0;
// в модуле 2a его заменит Postgres, и use case'ы этого не заметят.
package memory

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"sync"

	"messenger/internal/chat/domain"
)

// #region store
type Store struct {
	mu       sync.RWMutex // защищает все три карты ниже
	chats    map[domain.ChatID]domain.ChatSnapshot
	direct   map[string]domain.ChatID // DirectKeyFor(a, b) → id личного чата
	messages map[domain.ChatID][]domain.Message
}

// #endregion store

func NewStore() *Store {
	return &Store{
		chats:    make(map[domain.ChatID]domain.ChatSnapshot),
		direct:   make(map[string]domain.ChatID),
		messages: make(map[domain.ChatID][]domain.Message),
	}
}

func (s *Store) CreateChat(_ context.Context, chat *domain.Chat) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.chats[chat.ID()]; ok {
		return fmt.Errorf("memory: чат %s уже существует", chat.ID())
	}
	key, isDirect := chat.DirectKey()
	if isDirect {
		if _, ok := s.direct[key]; ok {
			return domain.ErrDirectChatExists
		}
		s.direct[key] = chat.ID()
	}
	s.chats[chat.ID()] = chat.Snapshot()
	return nil
}

func (s *Store) FindChat(_ context.Context, id domain.ChatID) (*domain.Chat, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.chats[id]
	if !ok {
		return nil, domain.ErrChatNotFound
	}
	return domain.RestoreChat(snap), nil
}

func (s *Store) FindDirectChat(_ context.Context, a, b domain.UserID) (*domain.Chat, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.direct[domain.DirectKeyFor(a, b)]
	if !ok {
		return nil, domain.ErrChatNotFound
	}
	return domain.RestoreChat(s.chats[id]), nil
}

// #region append
func (s *Store) AppendMessage(_ context.Context, chat *domain.Chat, msg domain.Message, expectedVersion int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.chats[chat.ID()]
	if !ok {
		return domain.ErrChatNotFound
	}
	// Проверка версии и запись — под одной блокировкой: между ними никто не вклинится.
	if stored.Version != expectedVersion {
		return domain.ErrConcurrentUpdate
	}
	s.chats[chat.ID()] = chat.Snapshot()
	s.messages[chat.ID()] = append(s.messages[chat.ID()], msg)
	return nil
}

// #endregion append

func (s *Store) ListMessages(_ context.Context, chatID domain.ChatID, afterSeq int64, limit int) ([]domain.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	all := s.messages[chatID]
	// Сообщения лежат по возрастанию seq — ищем первое после afterSeq двоичным поиском.
	from := sort.Search(len(all), func(i int) bool { return all[i].Seq() > afterSeq })
	to := min(from+limit, len(all))
	return slices.Clone(all[from:to]), nil
}
```

`messenger/internal/user/adapters/memory/store.go` — `Store{mu sync.RWMutex; byID map[domain.UserID]domain.User; byUsername map[string]domain.UserID}`; `CreateUser` возвращает `domain.ErrUsernameTaken`, если имя занято; `FindUser` — `domain.ErrUserNotFound`. Тесты `store_test.go` в обоих пакетах: создание и поиск, занятые имя/пара, `AppendMessage` с устаревшей версией → `ErrConcurrentUpdate` и история не изменилась, `ListMessages` с `afterSeq` за концом → пустой срез.

- [ ] **Step 7: `clock` и `idgen`**

```bash
cd messenger && go get github.com/google/uuid@v1.6.0
```

`messenger/internal/platform/clock/clock.go`:

```go
// Package clock — реальные часы для composition root. Use case'ы получают время
// через свой порт Clock, поэтому в тестах их подменяют на фиксированные.
package clock

import "time"

type System struct{}

func (System) Now() time.Time { return time.Now().UTC() }
```

`messenger/internal/platform/idgen/idgen.go`:

```go
// Package idgen — генератор идентификаторов UUIDv7. Почему v7, а не v4, — модуль 2a:
// он растёт со временем, и индекс по нему в Postgres не фрагментируется.
package idgen

import "github.com/google/uuid"

type UUIDv7 struct{}

// NewID не возвращает ошибку: uuid.NewV7 падает только при сбое crypto/rand,
// а с Go 1.24 crypto/rand.Read при сбое аварийно завершает программу сам.
func (UUIDv7) NewID() string { return uuid.Must(uuid.NewV7()).String() }
```

`idgen_test.go`: 1000 id уникальны, каждый — канонический UUID в нижнем регистре (`uuid.Parse` без ошибки и `id == strings.ToLower(id)`; домен не импортировать — правило `depguard` «platform» действует и на тесты), версия (`uuid.MustParse(id).Version()`) равна 7, лексикографический порядок совпадает с порядком генерации в пределах разных миллисекунд (генерировать с `time.Sleep(time.Millisecond)` между 5 id).

**Факт проверить:** поведение `crypto/rand.Read` при сбое в Go 1.24+ (release notes 1.24, `crypto/rand`); если формулировка в комментарии неверна — исправить комментарий и, при необходимости, сигнатуру порта.

Run: `go test -race ./... && golangci-lint run ./...` → PASS.

- [ ] **Step 8: Урок `04-usecases.md`**

Frontmatter: `title: 1.4. Use case'ы и порты: сценарии отдельно от деталей`.

**Разделы:**
- **Проблема.** «Сервис чатов» на 15 методов с конструктором из 9 зависимостей; тест одного метода требует собрать всё. Плюс гонка: два участника отправили сообщения одновременно — оба получили `seq = 5`.
- **Теория:**
  - `use-case`: один сценарий — один тип с `Execute`; команда/запрос на входе (строки), разбор в объекты-значения внутри — валидация одинакова для HTTP, CLI и будущего gRPC;
  - порт у потребителя, узкие интерфейсы (ISP, «Interface Segregation»), почему `memory.Store` реализует пять портов, а use case видит один-два; «accept interfaces, return structs» (100 Go Mistakes № 6–7);
  - агрегат `Chat` как граница согласованности; `seq` выдаёт агрегат;
  - `optimistic-locking`: версия, `expectedVersion`, повтор до `MaxSendAttempts`; пессимистическая альтернатива (держать блокировку на весь сценарий) — и почему в распределённой системе она дороже;
  - снимок агрегата (`ChatSnapshot`/`RestoreChat`) — как хранилище копирует агрегат, не ломая инкапсуляцию;
  - порты `Clock` и `IDGenerator`: детерминированные тесты; UUIDv7 вместо v4 — кратко, подробно в 2a;
  - хранилище в памяти: `sync.RWMutex`, почему копии на входе и выходе.
- **Визуализация:** `<LayerFlow />` и задания.
- **Код:** регионы `ports`, `send-execute`, `store`, `append`; реальный вывод `go test -race -run TestSendMessageConcurrent -v ./internal/chat/usecase/` (строка `t.Logf` с числом конфликтов) — прогнать 3 раза и показать разброс.
- **Под капотом:** «Оптимистическая и пессимистическая блокировка» — что происходит при высокой конкуренции (конфликты растут), почему для чата это нормально (сообщения в один чат от многих людей одновременно — редкость), мостик к MVCC и `SELECT … FOR UPDATE` в 2b.
- **Ошибки:** «сервис-бог»; интерфейс репозитория на 20 методов рядом с реализацией; use case, возвращающий DTO с тегами `json`; `seq` от хранилища (`MAX(seq)+1`) — правило утекло из домена.
- **Компромисс:** оптимистическая блокировка — нет ожидания, но клиент иногда получает «повторите»; цена — счётчик версий и цикл.
- **Самопроверка, источники** (Мартин гл. 20 «Бизнес-правила»; Ханонов гл. 6 «Агрегаты»; Фаулер, PoEAA «Optimistic Offline Lock»; Харсаньи № 5–7).
- **Что изменилось в мессенджере.**

**Термины:** `use-case`, `optimistic-locking`, `uuidv7`, `fake` (кратко; подробно — 1.6).

- [ ] **Step 9: Проверка, коммит, тег**

```bash
git add messenger course
git commit -m "feat(messenger): use case'ы, порты, хранилища в памяти, оптимистическая блокировка; урок 1.4

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git tag m01-l04
```

---

### Task 8: Урок 1.5 «context» + отмена до хранилища

**Files:**
- Modify: `messenger/internal/chat/adapters/memory/store.go`, `messenger/internal/user/adapters/memory/store.go`, `messenger/internal/chat/usecase/send_message.go`
- Create: `messenger/internal/chat/usecase/context_test.go`, `messenger/internal/chat/adapters/memory/context_test.go`
- Create: `course/modules/01-go-service/05-context.md`

**Interfaces:**
- Consumes: всё из задачи 7.
- Produces: каждый метод обоих `memory.Store` возвращает `ctx.Err()` (обёрнутый: `fmt.Errorf("memory: %w", err)`), если контекст уже отменён, **до** любого изменения; `SendMessage` проверяет `ctx.Err()` перед каждой повторной попыткой.

- [ ] **Step 1: Тесты (падают)**

`messenger/internal/chat/adapters/memory/context_test.go`:

```go
package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"messenger/internal/chat/adapters/memory"
	"messenger/internal/chat/domain"
)

func TestCanceledContextWritesNothing(t *testing.T) {
	t.Parallel()
	store := memory.NewStore()
	a, _ := domain.ParseUserID("00000000-0000-7000-8000-000000000001")
	b, _ := domain.ParseUserID("00000000-0000-7000-8000-000000000002")
	id, _ := domain.ParseChatID("00000000-0000-7000-9000-000000000001")
	chat, err := domain.NewDirectChat(id, a, b, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.CreateChat(ctx, chat); !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateChat: ждали context.Canceled, получили %v", err)
	}
	if _, err := store.FindChat(context.Background(), id); !errors.Is(err, domain.ErrChatNotFound) {
		t.Fatalf("после отменённого CreateChat чат не должен появиться, получили %v", err)
	}

	expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelExpired()
	if _, err := store.FindChat(expired, id); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("FindChat: ждали DeadlineExceeded, получили %v", err)
	}
}
```

`messenger/internal/chat/usecase/context_test.go`: отправка с отменённым контекстом → `errors.Is(err, context.Canceled)`, а `ListMessages` после этого показывает 0 сообщений; фейк `MessageAppender`, который на первой попытке отменяет контекст (через переданную `cancel`) и возвращает `ErrConcurrentUpdate`, — `SendMessage` не делает вторую попытку и возвращает ошибку с `context.Canceled` (счётчик вызовов фейка = 1).

Run: `go test -race ./internal/chat/...` → FAIL.

- [ ] **Step 2: Реализация**

В каждый метод обоих `memory.Store` первой строкой:

```go
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("memory: %w", err) // для методов с результатом: return nil, fmt.Errorf(...)
	}
```

(параметр `_ context.Context` переименовать в `ctx`). В `SendMessage.Execute` в начале цикла, для `attempt > 1`:

```go
		if attempt > 1 {
			if err := ctx.Err(); err != nil {
				return domain.Message{}, fmt.Errorf("send message: перед попыткой %d: %w", attempt, err)
			}
		}
```

Run: `go test -race ./... && golangci-lint run ./...` → PASS.

- [ ] **Step 3: Урок `05-context.md`**

Frontmatter: `title: 1.5. context: как остановить работу, которая больше не нужна`.

**Разделы:**
- **Проблема.** Пользователь закрыл приложение, а сервер ещё 30 секунд ждёт медленную проверку участников и пишет в базу ответ, который никто не прочитает; под нагрузкой такие «сироты» съедают пул соединений (связь с 0b.6).
- **Теория:**
  - `go-context`: интерфейс (`Deadline`, `Done`, `Err`, `Value`), дерево контекстов;
  - `cancellation`: `WithCancel`, обязательный `defer cancel()` и почему `go vet` ругается (`lostcancel`);
  - `deadline`: `WithTimeout`/`WithDeadline`, дедлайн потомка ≤ дедлайна родителя; `context.Canceled` против `context.DeadlineExceeded`; `WithCancelCause`/`Cause` (1.20), `WithTimeoutCause` (1.21);
  - `WithoutCancel` (1.21) — когда работа должна пережить запрос (аудит), и риски; `AfterFunc` (1.21);
  - правила: первый аргумент, не хранить в структуре, `Value` — только для данных запроса (trace id, не логгер и не БД), ключ — неэкспортируемый тип; 100 Go Mistakes № 60–61;
  - откуда берётся контекст в сервере: `r.Context()` отменяется при разрыве соединения клиентом и после возврата из обработчика;
  - как отмена доходит до хранилища: проверка `ctx.Err()` до записи; в 2a драйвер Postgres сам прервёт запрос.
- **Визуализация:** `<CtxTreeSandbox />` и задания (отменить `request` — что с `audit`; промотать 100 мс — кто остановился и с какой ошибкой).
- **Код:** изменения регионами (добавить регион `ctx-check` в `AppendMessage`), оба теста, реальный вывод `go test -race -run 'Canceled|Context' -v ./internal/chat/...`.
- **Под капотом:** «Как контекст распространяет отмену» — по `$(go env GOROOT)/src/context/context.go`: `propagateCancel`, регистрация в `children` родителя `cancelCtx`, `AfterFunc` у родителя, иначе горутина, ждущая `parent.Done()`; почему `cancel()` нужно вызывать, даже если дедлайн истёк сам (освобождение ресурсов, таймер).
- **Ошибки:** `context.Background()` посреди обработчика; контекст в поле структуры; `Value` для передачи зависимостей; забытый `cancel`; игнорирование `ctx` в долгом цикле.
- **Компромисс:** проверки `ctx.Err()` в каждом методе — шум против гарантии «отменённый запрос ничего не пишет».
- **Самопроверка, источники** (go.dev/blog «Go Concurrency Patterns: Context»; документация `context`; Харсаньи № 60–61; Cox-Buday «Concurrency in Go», гл. 4 — пакет context).
- **Что изменилось в мессенджере.**

**Термины:** `go-context`, `cancellation`, `deadline`.

**Факты проверить:** версии появления `WithoutCancel`, `AfterFunc`, `WithCancelCause`, `WithTimeoutCause`; реализация `propagateCancel` в Go 1.26; анализатор `lostcancel` в `go vet`; номера у Харсаньи.

- [ ] **Step 4: Проверка, коммит, тег**

```bash
git add messenger course
git commit -m "feat(messenger): отмена через context до хранилища; урок 1.5

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git tag m01-l05
```

---

### Task 9: Урок 1.6 «Тесты» + контрактные тесты хранилищ

**Files:**
- Create: `messenger/internal/chat/usecase/storetest/storetest.go`, `messenger/internal/user/usecase/storetest/storetest.go`
- Modify: `messenger/internal/chat/adapters/memory/store_test.go`, `messenger/internal/user/adapters/memory/store_test.go` (тесты, дублирующие контракт, удалить — их заменяет вызов контракта)
- Create: `course/modules/01-go-service/06-tests.md`

**Interfaces:**
- Consumes: порты из задачи 7, правила контекста из задачи 8.
- Produces: `storetest.Store` (интерфейс-объединение портов) и `storetest.Run(t *testing.T, newStore func(t *testing.T) storetest.Store)` в каждом контексте. В модуле 2a Postgres-адаптер вызовет тот же `Run`.

- [ ] **Step 1: Контракт `chat`**

`messenger/internal/chat/usecase/storetest/storetest.go`:

```go
// Package storetest — контрактные тесты хранилища чатов: одна батарея проверок
// для любой реализации портов. Сейчас её проходит memory.Store, в модуле 2a —
// Postgres-адаптер. Если обе реализации проходят контракт, use case'ам всё равно,
// с какой из них работать.
package storetest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"messenger/internal/chat/domain"
	"messenger/internal/chat/usecase"
)

// Store — всё, что use case'ы чатов ждут от хранилища.
type Store interface {
	usecase.ChatCreator
	usecase.ChatFinder
	usecase.DirectChatFinder
	usecase.MessageAppender
	usecase.MessageLister
}

var t0 = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

// #region run
func Run(t *testing.T, newStore func(t *testing.T) Store) {
	t.Helper()
	tests := []struct {
		name string
		fn   func(t *testing.T, s Store)
	}{
		{"создание и поиск", testCreateAndFind},
		{"второй личный чат для пары", testDirectUnique},
		{"оптимистическая блокировка", testVersionConflict},
		{"история по seq", testListPaging},
		{"одновременная запись", testConcurrentAppend},
		{"отменённый контекст", testCanceledContext},
		{"найденный чат — копия", testFindReturnsCopy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.fn(t, newStore(t))
		})
	}
}

// #endregion run

func id[T any](t *testing.T, parse func(string) (T, error), prefix string, n int) T {
	t.Helper()
	v, err := parse(fmt.Sprintf("00000000-0000-7000-%s-%012d", prefix, n))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func newDirect(t *testing.T, n int) *domain.Chat {
	t.Helper()
	c, err := domain.NewDirectChat(id(t, domain.ParseChatID, "9000", n), id(t, domain.ParseUserID, "8000", 1), id(t, domain.ParseUserID, "8000", 2), t0)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func post(t *testing.T, c *domain.Chat, n int) domain.Message {
	t.Helper()
	text, err := domain.NewText(fmt.Sprintf("сообщение %d", n))
	if err != nil {
		t.Fatal(err)
	}
	m, err := c.Post(id(t, domain.ParseMessageID, "a000", n), id(t, domain.ParseUserID, "8000", 1), text, t0)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func testCreateAndFind(t *testing.T, s Store) {
	ctx := context.Background()
	c := newDirect(t, 1)
	if err := s.CreateChat(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err := s.FindChat(ctx, c.ID())
	if err != nil {
		t.Fatal(err)
	}
	if got.ID() != c.ID() || got.Kind() != c.Kind() || len(got.Members()) != 2 || got.Version() != 0 {
		t.Fatalf("найден не тот чат: %+v", got.Snapshot())
	}
	if _, err := s.FindChat(ctx, id(t, domain.ParseChatID, "9000", 99)); !errors.Is(err, domain.ErrChatNotFound) {
		t.Fatalf("ждали ErrChatNotFound, получили %v", err)
	}
}

func testDirectUnique(t *testing.T, s Store) {
	ctx := context.Background()
	if err := s.CreateChat(ctx, newDirect(t, 1)); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateChat(ctx, newDirect(t, 2)); !errors.Is(err, domain.ErrDirectChatExists) {
		t.Fatalf("ждали ErrDirectChatExists, получили %v", err)
	}
	a, b := id(t, domain.ParseUserID, "8000", 2), id(t, domain.ParseUserID, "8000", 1)
	got, err := s.FindDirectChat(ctx, a, b) // порядок участников не важен
	if err != nil || got.ID() != newDirect(t, 1).ID() {
		t.Fatalf("FindDirectChat: %v", err)
	}
}

func testVersionConflict(t *testing.T, s Store) {
	ctx := context.Background()
	c := newDirect(t, 1)
	if err := s.CreateChat(ctx, c); err != nil {
		t.Fatal(err)
	}
	first, _ := s.FindChat(ctx, c.ID())
	second, _ := s.FindChat(ctx, c.ID())
	if err := s.AppendMessage(ctx, first, post(t, first, 1), 0); err != nil {
		t.Fatal(err)
	}
	// second прочитан до записи first — его версия устарела.
	if err := s.AppendMessage(ctx, second, post(t, second, 2), 0); !errors.Is(err, domain.ErrConcurrentUpdate) {
		t.Fatalf("ждали ErrConcurrentUpdate, получили %v", err)
	}
	msgs, err := s.ListMessages(ctx, c.ID(), 0, 10)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("после конфликта в истории должно остаться 1 сообщение: %d, %v", len(msgs), err)
	}
}

func testListPaging(t *testing.T, s Store) {
	ctx := context.Background()
	c := newDirect(t, 1)
	if err := s.CreateChat(ctx, c); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 5; i++ {
		cur, _ := s.FindChat(ctx, c.ID())
		v := cur.Version()
		if err := s.AppendMessage(ctx, cur, post(t, cur, i), v); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		after    int64
		limit    int
		wantSeqs []int64
	}{
		{0, 2, []int64{1, 2}},
		{2, 2, []int64{3, 4}},
		{4, 10, []int64{5}},
		{5, 10, nil},
		{100, 10, nil},
	}
	for _, tt := range tests {
		msgs, err := s.ListMessages(ctx, c.ID(), tt.after, tt.limit)
		if err != nil {
			t.Fatal(err)
		}
		var got []int64
		for _, m := range msgs {
			got = append(got, m.Seq())
		}
		if fmt.Sprint(got) != fmt.Sprint(tt.wantSeqs) {
			t.Fatalf("after=%d limit=%d: seq %v, ждали %v", tt.after, tt.limit, got, tt.wantSeqs)
		}
	}
}

func testConcurrentAppend(t *testing.T, s Store) {
	ctx := context.Background()
	c := newDirect(t, 1)
	if err := s.CreateChat(ctx, c); err != nil {
		t.Fatal(err)
	}
	const writers = 16
	// Всё, что может вызвать t.Fatal, готовим до горутин: FailNow из чужой горутины
	// запрещён (go vet, анализатор testinggoroutine).
	author := id(t, domain.ParseUserID, "8000", 1)
	text, err := domain.NewText("одновременно")
	if err != nil {
		t.Fatal(err)
	}
	msgIDs := make([]domain.MessageID, writers)
	for w := range writers {
		msgIDs[w] = id(t, domain.ParseMessageID, "a000", w+1)
	}
	var (
		wg sync.WaitGroup
		mu sync.Mutex
		ok int
	)
	for w := range writers {
		wg.Go(func() {
			cur, err := s.FindChat(ctx, c.ID())
			if err != nil {
				t.Error(err)
				return
			}
			v := cur.Version()
			msg, err := cur.Post(msgIDs[w], author, text, t0)
			if err != nil {
				t.Error(err)
				return
			}
			err = s.AppendMessage(ctx, cur, msg, v)
			if err != nil && !errors.Is(err, domain.ErrConcurrentUpdate) {
				t.Error(err)
			}
			if err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	msgs, err := s.ListMessages(ctx, c.ID(), 0, domain.ListLimitMax)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != ok {
		t.Fatalf("успешных записей %d, в истории %d", ok, len(msgs))
	}
	for i, m := range msgs {
		if m.Seq() != int64(i+1) {
			t.Fatalf("seq с пропуском или повтором: позиция %d, seq %d", i, m.Seq())
		}
	}
}

func testCanceledContext(t *testing.T, s Store) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.CreateChat(ctx, newDirect(t, 1)); !errors.Is(err, context.Canceled) {
		t.Fatalf("ждали context.Canceled, получили %v", err)
	}
	if _, err := s.FindChat(context.Background(), newDirect(t, 1).ID()); !errors.Is(err, domain.ErrChatNotFound) {
		t.Fatalf("отменённая запись не должна сохраниться: %v", err)
	}
}

func testFindReturnsCopy(t *testing.T, s Store) {
	ctx := context.Background()
	c := newDirect(t, 1)
	if err := s.CreateChat(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, _ := s.FindChat(ctx, c.ID())
	post(t, got, 1) // меняем найденный агрегат, но не сохраняем
	again, _ := s.FindChat(ctx, c.ID())
	if again.Version() != 0 || again.LastSeq() != 0 {
		t.Fatal("изменение найденного чата без AppendMessage не должно попадать в хранилище")
	}
}
```

`messenger/internal/chat/adapters/memory/store_test.go` (контракт вместо ручных тестов; уникальные для памяти проверки, если есть, оставить):

```go
package memory_test

import (
	"testing"

	"messenger/internal/chat/adapters/memory"
	"messenger/internal/chat/usecase/storetest"
)

// #region contract
func TestStoreContract(t *testing.T) {
	storetest.Run(t, func(*testing.T) storetest.Store { return memory.NewStore() })
}

// #endregion contract
```

Контракт `user/usecase/storetest`: создание и поиск, занятое имя → `ErrUsernameTaken` (и первый пользователь не перезаписан), неизвестный id → `ErrUserNotFound`, отменённый контекст ничего не пишет, одновременная регистрация одного имени из 16 горутин — ровно одна успешная.

**`depguard`:** `storetest` лежит в `usecase` и импортирует `testing` — это разрешено (правило запрещает только адаптеры, `platform`, `net/http`, `database/sql`, `uuid`). Проверить, что `golangci-lint` чист.

Run: `go test -race -count=3 ./... && golangci-lint run ./...` → PASS. `-count=3` — гонки и конфликты проявляются не с первого раза.

- [ ] **Step 2: Урок `06-tests.md`**

Frontmatter: `title: 1.6. Тесты: что проверять и на каком уровне`.

**Разделы:**
- **Проблема.** Тесты на моках прошли, а в проде — дубли `seq`: мок «хранилища» не проверял версию, потому что мок — это не хранилище, а ожидания автора теста. Второй случай: тест, который падает через раз, — гонка, которую нашёл только `-race`.
- **Теория:**
  - `test-pyramid` и что тестировать на каком уровне (домен — юниты; use case — на фейках; адаптеры — контрактные и интеграционные, мало; HTTP — тесты обработчиков через `httptest`);
  - `test-double`: dummy, stub, `fake`, spy, `mock` (Фаулер, Месарош); почему мы предпочитаем фейки: проверяем поведение, а не последовательность вызовов; переусложнённый мок — сигнал плохого дизайна (стандарт);
  - `table-driven-test`, `t.Run`, `t.Parallel` и захват переменной цикла (с Go 1.22 больше не ловушка), `t.Helper`, `t.Cleanup`;
  - `contract-test`: одна батарея для всех реализаций порта; как он защищает замену хранилища в 2a;
  - `data-race` и `race-detector`: `go test -race`, почему `-count=3`, `testing.Short` для медленного;
  - что **не** тестировать: приватные детали, геттеры, код стандартной библиотеки.
- **Визуализация:** `<TestLayers />` и задания.
- **Код:** регионы `run`, `contract`; реальный вывод `go test -race -run TestStoreContract -v ./internal/chat/adapters/memory/`; намеренная поломка (во временной копии убрать проверку версии в `AppendMessage`) — реальный вывод упавшего контракта; намеренная гонка (убрать `mu.Lock` в `ListMessages`) — реальный вывод `-race` (`WARNING: DATA RACE`, сокращённо).
- **Под капотом:** «Как работает детектор гонок» — ThreadSanitizer, отношение happens-before, теневая память, почему он находит только случившиеся при прогоне гонки, цена (по документации Go: память ×5–10, время ×2–20).
- **Ошибки:** мок на каждый вызов; тест приватной функции; `time.Sleep` для синхронизации; общий фейк с состоянием между параллельными тестами; тесты без `-race` в CI (1c).
- **Компромисс:** фейк надо поддерживать как код, но он ловит больше, чем мок.
- **Самопроверка, источники** (Фаулер «Mocks Aren't Stubs», «The Practical Test Pyramid» (Ham Vocke); go.dev/doc/articles/race_detector; Харсаньи гл. 11).
- **Что изменилось в мессенджере.**

**Термины:** `test-pyramid`, `test-double`, `fake`, `mock`, `contract-test`, `data-race`, `race-detector` (`table-driven-test` введён в 1.2 — здесь ссылка назад).

**Факты проверить:** цифры накладных расходов в go.dev/doc/articles/race_detector; изменение семантики переменной цикла в Go 1.22; авторство статьи о пирамиде на martinfowler.com.

- [ ] **Step 3: Проверка, коммит, тег**

```bash
git add messenger course
git commit -m "test(messenger): контрактные тесты хранилищ; урок 1.6

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git tag m01-l06
```

---

### Task 10: Урок 1.7 «Тонкий HTTP-адаптер» + `main.go` + ADR-0002

**Files:**
- Create: `messenger/internal/platform/httpjson/{httpjson.go,httpjson_test.go}`, `messenger/internal/platform/httpserver/server.go`
- Create: `messenger/internal/user/adapters/httpapi/{handler.go,dto.go,errors.go,handler_test.go}`
- Create: `messenger/internal/chat/adapters/httpapi/{handler.go,dto.go,errors.go,auth.go,handler_test.go}`
- Create: `messenger/internal/chat/adapters/userchecker/{checker.go,checker_test.go}`
- Create: `messenger/cmd/messenger/main.go`, `messenger/docs/adr/0002-x-user-id-до-jwt.md`
- Modify: `course/modules/00-foundation/03-request-path.md` (ссылка на 1.7 у фрагмента `UserID`, пункт бэклога)
- Create: `course/modules/01-go-service/07-http-adapter.md`

**Interfaces:**
- Consumes: use case'ы и хранилища (задачи 7–9).
- Produces: бинарь `go run ./cmd/messenger` с API ниже.

**API v0 (все тела — JSON, `Content-Type: application/json; charset=utf-8`):**

| Метод и путь | Тело запроса | Ответ |
|---|---|---|
| `POST /users` | `{"username","display_name"}` | 201 `{"id","username","display_name","created_at"}` |
| `GET /users/{id}` | — | 200 тот же объект |
| `POST /chats` (`X-User-ID`) | `{"kind":"direct","member_ids":["…"]}` или `{"kind":"group","title":"…","member_ids":[…]}` | 201 (создан) или 200 (личный уже был) `{"id","kind","title","member_ids","created_at"}` (`title` опускается у личного) |
| `POST /chats/{id}/messages` (`X-User-ID`) | `{"text"}` | 201 `{"id","chat_id","author_id","text","seq","sent_at"}` |
| `GET /chats/{id}/messages?after=0&limit=50` (`X-User-ID`) | — | 200 `{"messages":[…]}` |

Время — RFC 3339 в UTC. Ошибки — `{"error":{"code":"…","message":"…","field":"…"}}` (`field` — только для валидации). Таблица маппинга (одинакова в обоих `httpapi/errors.go`, по своим ошибкам контекста):

| Ошибка | Статус | `code` |
|---|---|---|
| `errors.As(*ValidationError)` | 400 | `invalid_input` (+ `field`) |
| `ErrUserNotFound` / `ErrChatNotFound` | 404 | `not_found` |
| `ErrNotMember` | 403 | `not_member` |
| `ErrUsernameTaken` | 409 | `username_taken` |
| `ErrConcurrentUpdate` | 409 | `conflict` |
| `context.Canceled` | — (клиент ушёл; ответ не пишем, логируем на `Info`) | — |
| `context.DeadlineExceeded` | 503 | `timeout` |
| прочее | 500 | `internal` (сообщение общее, детали — только в лог `slog.Error` с путём и ошибкой) |
| ошибки `httpjson.Decode` | 400 / 413 / 415 | `bad_json` / `body_too_large` / `unsupported_media_type` |
| нет или кривой `X-User-ID` | 401 | `unauthenticated` |

- [ ] **Step 1: `httpjson` — тесты (падают) и реализация**

`messenger/internal/platform/httpjson/httpjson_test.go` — табличный тест `Decode` через `httptest.NewRequest`: корректное тело → nil; тело > `MaxBodyBytes` → `*DecodeError` со `Status == 413`; неизвестное поле → 400; два JSON-объекта подряд (`{"text":"a"}{"text":"b"}`) → 400; пустое тело → 400; `Content-Type: text/plain` → 415; `Content-Type: application/json; charset=utf-8` → nil. Тест `Write`: статус, заголовок, тело с переводом строки.

`messenger/internal/platform/httpjson/httpjson.go`:

```go
// Package httpjson — разбор и запись JSON на границе HTTP. Общий для адаптеров
// обоих контекстов: это техническая деталь транспорта, бизнес-правил здесь нет.
package httpjson

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
)

// MaxBodyBytes — лимит тела запроса. Сообщение — до 4096 символов (≤ 16 КиБ в UTF-8),
// группа — до 200 id по 36 байт; 1 МиБ — с большим запасом, но не даёт забить память.
const MaxBodyBytes = 1 << 20

// DecodeError — ошибка разбора тела; Status — HTTP-статус, который стоит вернуть.
type DecodeError struct {
	Status int
	Code   string
	Msg    string
}

func (e *DecodeError) Error() string { return e.Msg }

// #region decode
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	if ct := r.Header.Get("Content-Type"); ct != "" {
		mt, _, err := mime.ParseMediaType(ct)
		if err != nil || mt != "application/json" {
			return &DecodeError{Status: http.StatusUnsupportedMediaType, Code: "unsupported_media_type", Msg: "нужен Content-Type: application/json"}
		}
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return &DecodeError{Status: http.StatusRequestEntityTooLarge, Code: "body_too_large", Msg: fmt.Sprintf("тело больше %d байт", MaxBodyBytes)}
		}
		return &DecodeError{Status: http.StatusBadRequest, Code: "bad_json", Msg: "некорректный JSON: " + err.Error()}
	}
	// Второй объект в теле — почти всегда ошибка клиента; молча игнорировать его нельзя.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return &DecodeError{Status: http.StatusBadRequest, Code: "bad_json", Msg: "в теле должен быть ровно один JSON-объект"}
	}
	return nil
}

// #endregion decode

// Write пишет v как JSON. Ошибку кодирования возвращает вызывающему: заголовки уже
// отправлены, исправить ответ нельзя — остаётся только залогировать.
func Write(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return fmt.Errorf("httpjson: запись ответа: %w", err)
	}
	return nil
}
```

**Факт проверить:** что превышение лимита во втором `Decode` (после первого объекта) тоже даёт `*http.MaxBytesError`, а не просто ошибку разбора; если нет — проверять `MaxBytesError` и во втором вызове. Отсутствующий `Content-Type` пропускается сознательно (curl без `-H` удобнее в уроке) — записать это в ADR-0002 или в комментарий.

- [ ] **Step 2: Адаптер `userchecker`**

`messenger/internal/chat/adapters/userchecker/checker.go`:

```go
// Package userchecker — мост между контекстами: реализует порт chat/usecase.UserChecker
// поверх use case контекста user. Это единственное место, где chat знает про user
// (правило depguard «chat-context» делает для него исключение). В модуле 6 на его
// месте будет gRPC-клиент сервиса пользователей.
package userchecker

import (
	"context"
	"errors"
	"fmt"

	chatdomain "messenger/internal/chat/domain"
	userdomain "messenger/internal/user/domain"
	userusecase "messenger/internal/user/usecase"
)

// UserGetter — узкий порт на use case GetUser другого контекста.
type UserGetter interface {
	Execute(ctx context.Context, q userusecase.GetUserQuery) (userdomain.User, error)
}

type Checker struct{ users UserGetter }

func New(users UserGetter) (*Checker, error) {
	if users == nil {
		return nil, errors.New("userchecker: nil-зависимость")
	}
	return &Checker{users: users}, nil
}

func (c *Checker) MissingUsers(ctx context.Context, ids []chatdomain.UserID) ([]chatdomain.UserID, error) {
	var missing []chatdomain.UserID
	for _, id := range ids {
		_, err := c.users.Execute(ctx, userusecase.GetUserQuery{ID: id.String()})
		switch {
		case err == nil:
		case errors.Is(err, userdomain.ErrUserNotFound):
			missing = append(missing, id)
		default:
			return nil, fmt.Errorf("userchecker: %w", err)
		}
	}
	return missing, nil
}
```

`checker_test.go`: фейк `UserGetter` — существующий, отсутствующий, ошибка инфраструктуры (оборачивается, `errors.Is` её видит).

- [ ] **Step 3: HTTP-адаптеры (контракт)**

- **`chat/httpapi/auth.go`.** `const UserIDHeader = "X-User-ID"`; функция `authorID(r *http.Request) (string, bool)` — значение заголовка без пробелов по краям; формат UUID **не** проверяет (это сделает `ParseUserID` в use case, но ошибка `user_id` из use case для этого заголовка должна дать 401, а не 400 — обработчик разбирает его через `chat/domain.ParseUserID` сам, до вызова use case, и отвечает 401). Комментарий-предупреждение с ссылкой на ADR-0002: «заголовок присылает клиент — подделать его может любой; это заглушка до JWT (урок 1b)».
- **`dto.go` в обоих адаптерах.** Неэкспортируемые типы с тегами `json`: `registerUserRequest`, `userResponse`; `createChatRequest`, `chatResponse`, `sendMessageRequest`, `messageResponse`, `listMessagesResponse`; мапперы `toUserResponse(domain.User) userResponse`, `toChatResponse(*domain.Chat) chatResponse`, `toMessageResponse(domain.Message) messageResponse`. Время — `t.UTC().Format(time.RFC3339Nano)`. Регион `dto` для урока.
- **`errors.go` в обоих адаптерах.** `func writeError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error)` — таблица маппинга выше, через `errors.Is/As`, порядок: `DecodeError` → `ValidationError` → сигнальные → контекст → 500. Регион `error-map`.
- **`handler.go` в обоих адаптерах.** `type Handler struct{…}` с узкими интерфейсами на use case'ы, объявленными в адаптере (`type messageSender interface{ Execute(context.Context, usecase.SendMessageCommand) (domain.Message, error) }` и т. п.), `func New(…, log *slog.Logger) (*Handler, error)` с проверкой `nil`, `func (h *Handler) Register(mux *http.ServeMux)` с шаблонами `POST /chats`, `POST /chats/{id}/messages`, `GET /chats/{id}/messages` (и `POST /users`, `GET /users/{id}`). `r.PathValue("id")`; `after` и `limit` — `strconv.ParseInt`/`Atoi`, ошибка → 400 `invalid_input` с `field`. Каждый обработчик ≤ 30 строк: разобрать → вызвать use case с `r.Context()` → смаппить → записать; ошибка `httpjson.Write` — в `log.Error`. Регион `send-handler`.
- **`handler_test.go` в обоих адаптерах** — через `httptest.NewRecorder` и настоящий `http.ServeMux` с фейковыми use case'ами (структуры с полями-функциями): на каждую строку таблицы маппинга — тест статуса и `code`; 413 на большое тело; 401 без `X-User-ID` и с кривым; `POST /chats` → 201 при `Created=true` и 200 при `false`; 500 не раскрывает текст внутренней ошибки (`message` не содержит текста фейковой ошибки «секрет БД»); `GET` с `limit=abc` → 400 `field: limit`; метод `PUT /chats` → 405 (это делает сам `ServeMux` — тест фиксирует поведение).

- [ ] **Step 4: `httpserver` и `main.go` (контракт)**

- **`platform/httpserver/server.go`.** `func New(addr string, h http.Handler, log *slog.Logger) *http.Server` с `ReadHeaderTimeout: 5 * time.Second`, `ReadTimeout: 10 * time.Second`, `WriteTimeout: 10 * time.Second`, `IdleTimeout: 60 * time.Second`, `ErrorLog: slog.NewLogLogger(log.Handler(), slog.LevelWarn)` (значения обосновать комментарием со ссылкой на урок 0b.6). `func Run(ctx context.Context, srv *http.Server, shutdownTimeout time.Duration) error`: `ListenAndServe` в горутине (владелец — `Run`, результат через канал), ожидание `ctx.Done()` или ошибки сервера, затем `Shutdown` с `context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)`; `http.ErrServerClosed` — не ошибка. Комментарий: подробно — модуль 1.5.
- **`cmd/messenger/main.go`** — composition root, регион `wiring`:
  - `slog.New(slog.NewJSONHandler(os.Stdout, nil))`;
  - адрес из `MESSENGER_ADDR`, по умолчанию `:8080`;
  - `ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`;
  - сборка: `clock.System{}`, `idgen.UUIDv7{}`, оба `memory.NewStore()`, use case'ы, `userchecker.New(getUser)`, оба `httpapi.New`, `mux := http.NewServeMux()`, `Register`;
  - любая ошибка сборки — `log.Error` и `os.Exit(1)` (функция `run() error`, `main` только вызывает её и выходит с кодом — так `defer stop()` отработает);
  - лог «слушаю» с адресом.

- [ ] **Step 5: ADR-0002 и правка 0.3**

`0002-x-user-id-до-jwt.md` по шаблону: контекст (нужен автор сообщения, аутентификации ещё нет), варианты (без автора / `X-User-ID` / сразу JWT), решение (`X-User-ID` до 1b), последствия (любой клиент может представиться кем угодно — только для локального стенда; заменить в 1b; что изменится: только `chat/httpapi/auth.go` и middleware).

`course/modules/00-foundation/03-request-path.md`: у фрагмента с `UserID` (ошибка авторизации всегда 401) добавить одно предложение-ссылку: «как правильно разделять ошибки клиента (4xx) и сбои сервера (5xx) — в уроке 1.7, раздел о таблице ошибок» (якорь — реальный из собранного HTML). Пункт бэклога «`UserID`: ошибка всегда 401» отметить закрытым в задаче 11.

- [ ] **Step 6: Прогон через `curl`**

```bash
cd messenger && go run ./cmd/messenger &   # или в отдельном терминале
curl -s -X POST localhost:8080/users -H 'Content-Type: application/json' -d '{"username":"alice","display_name":"Алиса"}'
curl -s -X POST localhost:8080/users -H 'Content-Type: application/json' -d '{"username":"bob","display_name":"Боб"}'
curl -s -X POST localhost:8080/chats -H "X-User-ID: $ALICE" -H 'Content-Type: application/json' -d "{\"kind\":\"direct\",\"member_ids\":[\"$BOB\"]}"
curl -s -X POST "localhost:8080/chats/$CHAT/messages" -H "X-User-ID: $ALICE" -H 'Content-Type: application/json' -d '{"text":"Привет, Боб!"}'
curl -s "localhost:8080/chats/$CHAT/messages?after=0&limit=10" -H "X-User-ID: $BOB"
curl -s -i "localhost:8080/chats/$CHAT/messages" -H "X-User-ID: $EVE"     # 403 not_member (Eve зарегистрировать)
curl -s -i -X POST "localhost:8080/chats/$CHAT/messages" -H 'Content-Type: application/json' -d '{"text":"кто я?"}'   # 401
```

Переменные `ALICE`, `BOB`, `CHAT` — из ответов (`jq -r .id`, если `jq` есть; иначе вручную). Весь вывод и строки JSON-лога сервера сохранить в отчёт — урок цитирует их. Сервер остановить `kill %1` (или Ctrl+C) и убедиться, что в логе есть корректное завершение без ошибки.

Run: `go test -race ./... && golangci-lint run ./...` → PASS.

- [ ] **Step 7: Урок `07-http-adapter.md`**

Frontmatter: `title: 1.7. Тонкий HTTP-адаптер и сборка приложения`.

**Разделы:**
- **Проблема.** Обработчик знает про хранилище, отдаёт доменную структуру через `json.Marshal` (поля неэкспортируемые — пустой объект `{}`; если сделать их экспортируемыми — переименование поля в домене ломает клиентов), и на любую ошибку отвечает 500.
- **Теория:**
  - ведущий адаптер: разобрать → вызвать use case → смаппить ответ; «тонкий» — без бизнес-правил;
  - `http.ServeMux` с шаблонами (Go 1.22+): метод, `{id}`, `r.PathValue`, приоритет более специфичного шаблона, 405 с заголовком `Allow`;
  - `dto` и `mapper`: почему отдельные типы с тегами `json`; направление зависимости (адаптер знает домен, не наоборот);
  - разбор тела на границе: `MaxBytesReader`, `DisallowUnknownFields`, один объект; что из этого — защита (связь с 6.7, OWASP API Top 10 — кратко);
  - таблица «ошибка → статус» — единственное место перевода; 4xx против 5xx; не раскрывать внутренние ошибки; `context.Canceled` — клиент ушёл, отвечать некому; 503 на дедлайн;
  - `composition-root`: `main.go` собирает граф объектов; `run() error`; конфиг из окружения; `slog` JSON;
  - `X-User-ID` — заглушка и почему так нельзя в проде (ADR-0002); что изменится в 1b;
  - адаптер `userchecker` — исключение из правила «адаптеры не знают друг о друге» и почему оно безопасно;
  - таймауты сервера (ссылка на 0b.6) и минимальное корректное завершение (подробно — модуль 1.5).
- **Визуализация:** `<HttpMapping />` и задания.
- **Код:** регионы `decode`, `dto`, `error-map`, `send-handler`, `wiring`; сценарий `curl` целиком с реальным выводом и строками лога сервера.
- **Под капотом:** «Как `ServeMux` выбирает шаблон» — по `$(go env GOROOT)/src/net/http/pattern.go` и `routing_tree.go`: сравнение специфичности, почему конфликт двух одинаково специфичных шаблонов — паника при `Handle`, а не при запросе; `GODEBUG=httpmuxgo121`.
- **Ошибки:** доменная структура в ответе; `json.NewDecoder(r.Body)` без лимита; 500 с текстом ошибки БД наружу; логирование и возврат одной ошибки (урок 1.3); `http.DefaultServeMux` и глобальная регистрация (`init`-магия).
- **Компромисс:** DTO и мапперы — дублирование полей против свободы менять домен и API независимо.
- **Самопроверка, источники** (Эдвардс «Let's Go Further», гл. 3–4 — JSON, разбор и ошибки запроса; go.dev/blog «Routing Enhancements for Go 1.22»; документация `net/http`; OWASP API Security Top 10 2023 — API4 «Unrestricted Resource Consumption»).
- **Что изменилось в мессенджере** (`git diff m01-l06 m01-l07 --stat`) и итог части 1a: дерево `messenger/` целиком; что дальше — 1b (REST API по-взрослому, JWT).

**Термины:** `dto`, `mapper`, `middleware` (кратко — где он появится), `composition-root` (ссылка назад).

**Факты проверить:** правила приоритета шаблонов и поведение 405 в Go 1.22+ по исходникам; `GODEBUG=httpmuxgo121`; тип `*http.MaxBytesError` (Go 1.19); номер категории OWASP API Top 10 2023.

- [ ] **Step 8: Проверка, коммит, тег**

```bash
git add messenger course
git commit -m "feat(messenger): HTTP-адаптер, composition root, ADR-0002; урок 1.7

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git tag m01-l07
```

---

### Task 11: Итоговая проверка, ссылки, README, бэклог

**Files:**
- Modify: `README.md`, `docs/superpowers/backlog-after-module-0.md`, `course/modules/01-go-service/index.md`, уроки 1.1–1.7 (ссылки «следующий урок»), при необходимости `course/modules/00-foundation/06-messenger-requirements.md`

- [ ] **Step 1: Теги**

```bash
for t in m01-l01 m01-l02 m01-l03 m01-l04 m01-l05 m01-l06 m01-l07; do
  git worktree add -f "/tmp/claude-1000/m01-check-$t" "$t" >/dev/null &&
  (cd "/tmp/claude-1000/m01-check-$t/messenger" && go build ./... && go test -race ./... >/dev/null && echo "$t OK") ;
  git worktree remove --force "/tmp/claude-1000/m01-check-$t"
done
```

Expected: семь строк `OK`. Падение на теге — исправить в задаче, которой принадлежит тег, и перенести тег (`git tag -f`), записать в отчёт.

- [ ] **Step 2: Полный прогон**

Run: `npm test && npm run typecheck && npm run check:go && npm run check:labs && npm run check:messenger && npm run build` → всё зелёное.

- [ ] **Step 3: Сценарий с нуля**

`go run ./cmd/messenger` и сценарий `curl` из урока 1.7 по тексту урока; вывод совпадает по форме (id и время — другие). Демонстрация `depguard` из урока 1.1 по тексту урока даёт то же сообщение.

- [ ] **Step 4: Playwright**

Все уроки 1.1–1.7, `index.md`, `/glossary` — 375 и 1280 px, тёмная и светлая темы:
- нет горизонтальной прокрутки;
- строка «Модуль 1 · Урок N»;
- метки `<Part>` и цвета заголовков;
- каждая сцена реагирует (песочницы — на действия, плееры — на шаги);
- ⛶ с удержанием фокуса и возвратом на кнопку.

Скриншоты смотреть, а не только сохранять.

- [ ] **Step 5: Связи, README, бэклог**

- В конце каждого урока — ссылка на следующий; в `index.md` — список уроков 1.1–1.7 и пометка, что уроки 1b и 1c появятся позже.
- `06-messenger-requirements.md` (0.6): если там есть «код появится в модуле 1» — ссылка на `messenger/` и урок 1.1.
- `README.md`: раздел «Мессенджер» — `messenger/`, `go run ./cmd/messenger`, `npm run check:messenger`, теги `m01-lNN`, `golangci-lint` v2 как требование.
- Бэклог: закрыть «регионы `<<<` не проверяются» и «`UserID`: ошибка всегда 401»; добавить новые отложенные пункты из отчётов задач 1–10.

- [ ] **Step 6: Коммит**

```bash
git add README.md docs course messenger
git commit -m "docs: связи уроков модуля 1, README мессенджера, бэклог

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Тег `m01-l07` на этот коммит **не** переносится, если коммит не меняет `messenger/` (если меняет — перенести и прогнать шаг 1 для `m01-l07`).
