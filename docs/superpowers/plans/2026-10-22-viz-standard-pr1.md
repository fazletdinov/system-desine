# Мерка интерактива, PR 1: компоненты и сцена «B-tree изнутри» — план

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** общие компоненты мерки (карточка шага, объяснение по клику, «Проверь себя» в шаге, читаемая палитра с тестом контраста) и эталонная сцена «B-tree изнутри» в уроке 2.5, плюс короткие объяснения MVCC (2.4) и VACUUM (2.5).

**Architecture:** расширяем `Step<S>` необязательными `card`/`quiz`, добавляем тип `Explanation` и функцию объяснения сцены; `Player` сам рисует карточку и вопрос и получает слот `side` для панели объяснения. Сцена B-tree — как все сцены курса: чистая модель и сценарии в `viz/simulations/` (тесты на числа), рендерер в `viz/renderers/`, сборка в `viz/scenes/`, подключение в уроке.

**Tech Stack:** VitePress 1.6, Vue 3.5 (`<script setup lang="ts">`), TypeScript 5.9, vitest 5, `yaml`. Каталог — `course/`.

**Spec:** `docs/superpowers/specs/2026-10-22-viz-standard-design.md` (§3–§5, §7.2 — MVCC/VACUUM, §8). Прототип поведения и текстов: `docs/superpowers/specs/assets/2026-10-22-btree-prototype.html` — источник данных, порядка шагов и формулировок; тексты переносятся почти дословно (их одобрил пользователь), меняется только оформление терминов (`term()`).

## Global Constraints

- Русский язык во всём видимом тексте; комментарии в коде — по-русски, как в соседних файлах, объясняют «почему».
- Цвета — только через переменные `course/viz/palette.css` (`:root` и `.dark`); в SVG/CSS сцен никаких hex.
- Тёмная тема: второстепенный текст схем мятный, адреса/ссылки внутри структур — тёплый жёлтый, синего текста на синеватом фоне нет (синий — только рамки и линии).
- Контраст каждого токена текста схем ≥ 4,5:1 к `--viz-node-bg` и к `--vp-c-bg` в обеих темах.
- Проверка только на компьютере: 1280 px, обе темы. 375 px не проверяем.
- Учебные данные подписаны «учебные»; реальные числа — только из `labs/2a/results/messenger-explain.txt`, с указанием источника.
- Старые схемы без `card`/`quiz` выглядят и работают как раньше.
- Все проверки зелёные: `npm test`, `npm run typecheck`, `npm run build`, `npm run check:refs`, `npm run check:run`, `npm run check:go`, `npm run check:images` (из `course/`).
- Коммиты — с трейлером `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Ветка — `feat/viz-standard` (спецификация уже там). Push/PR — только по согласию пользователя.
- Снимки Playwright — только в `<repo>/.playwright-mcp/`, после проверки каталог удалить.

## Review Focus

1. Переход «Назад» в сценарии вставки после сплита — схема обязана вернуться к дереву из 5 листьев (кадры — полные снимки, не дельты). Тест — в Task 5.
2. Клик по элементу, затем смена шага — панель возвращается к элементу текущего шага; id, которого нет в модели этого кадра (например `node:b9` до сплита), даёт «нет объяснения», а не исключение. Тест — в Task 6.
3. Светлая тема: заливка «совпало», жёлтые адреса и мятные подписи читаются и на белом. Тест контраста — Task 1; глазами — Task 10.
4. Ответ «Проверь себя» скрыт заново при каждом шаге (не «утекает» с предыдущего шага). Проверка — Task 7 (ключ компонента) и Task 10.
5. Старые сцены после смены палитры: ни одна не потеряла читаемость и не сломалась раскладкой. Проход — Task 10.

---

### Task 1: Палитра и тест контраста

**Files:**
- Modify: `course/viz/palette.css`
- Create: `course/viz/core/contrast.ts`, `course/viz/core/contrast.test.ts`
- Modify: `course/viz/README.md` (таблица сущностей: строки «Второстепенный текст», «Адрес / ссылка в структуре»)

**Interfaces:**
- Produces: CSS-переменные `--viz-muted` (тёмная тема — мятный), новая `--viz-addr` (адрес/ссылка: TID, «→ блок N»), новая `--viz-hit-bg` (заливка «совпало с условием») и `--viz-hit-text`; `contrastRatio(a: string, b: string): number` (hex `#rrggbb`).

- [ ] **Step 1: Тест.** `contrast.test.ts`: `contrastRatio('#ffffff', '#000000')` ≈ 21 (до 0,01); `contrastRatio('#777777', '#ffffff')` ≈ 4,48. Второй `describe` читает `palette.css` (через `fs`, разбор блоков `:root {…}` и `.dark {…}` регуляркой) и проверяет для тем light/dark: каждый из `--viz-text`, `--viz-muted`, `--viz-addr`, `--viz-hit-text` к `--viz-node-bg` и к фону страницы ≥ 4,5; `--viz-hit-text` к `--viz-hit-bg` ≥ 4,5. Фон страницы: тёмная — `--vp-c-bg` из `course/.vitepress/theme/custom.css` (`#0b1220`), светлая — `#ffffff` (VitePress по умолчанию; в тесте комментарий, откуда значение).
- [ ] **Step 2:** `cd course && npx vitest run viz/core/contrast.test.ts` — FAIL (нет модуля / нет токенов).
- [ ] **Step 3:** `contrast.ts` — относительная яркость WCAG 2.x. В `palette.css`: `.dark --viz-muted: #a8d8bb`; `--viz-addr`: тёмная `#e8c27a`, светлая — тёмно-янтарный, проходящий 4,5:1 к белому (например `#92400e`); `--viz-hit-bg`/`--viz-hit-text`: тёмная `#1d5a3a`/`#9ff0c1`, светлая — подобрать (например `#d1fae5`/`#065f46`). Светлый `--viz-muted` не трогать, если проходит.
- [ ] **Step 4:** тест PASS; `npm test` целиком PASS.
- [ ] **Step 5:** README: строки таблицы + абзац «Правило текста: синий — только для рамок и линий; второстепенный текст — `--viz-muted`, адреса — `--viz-addr`». Коммит `feat(viz): читаемая палитра текста схем и тест контраста`.

### Task 2: Типы мерки: карточка, вопрос, объяснение

**Files:**
- Modify: `course/viz/core/types.ts`
- Create: `course/viz/core/explain.ts`, `course/viz/core/explain.test.ts`

**Interfaces:**
- Produces:
  ```ts
  // types.ts
  export interface StepCard { what: Caption; uses: Caption; reads: Caption; why: Caption }
  export interface StepQuiz { question: Caption; answer: Caption }
  export interface Step<S> { state: S; caption: Caption; highlight?: readonly string[]; codeLine?: number; card?: StepCard; quiz?: StepQuiz }
  // explain.ts
  export interface ExplainSection { heading: string; body: Caption }
  export interface Explanation { title: string; kind: string; sections: readonly ExplainSection[]; here?: Caption; real?: Caption }
  export type Explain<S> = (id: string, step: Step<S>) => Explanation | null
  export function stepTermIds<S>(step: Step<S>): string[]          // caption + все части card + quiz
  export function explanationTermIds(e: Explanation): string[]      // sections + here + real
  ```

- [ ] **Step 1: Тест** `explain.test.ts`: шаг с `caption` из `term('MVCC','mvcc')`, `card.why` с `term('TID','tid')`, `quiz.answer` с `term('VACUUM','vacuum')` → `stepTermIds` = `['mvcc','tid','vacuum']` (порядок: caption, card what→uses→reads→why, quiz question→answer); шаг без card/quiz → только термины caption; `explanationTermIds` собирает из `sections[*].body`, `here`, `real`.
- [ ] **Step 2:** FAIL. **Step 3:** реализация на `captionTermIds`. **Step 4:** PASS + `npm run typecheck`. **Step 5:** коммит `feat(viz): типы карточки шага, вопроса и объяснения`.

### Task 3: Термины в глоссарии

**Files:**
- Modify: `course/glossary/terms.yaml`

**Interfaces:**
- Produces: статьи с id: `mvcc`, `vacuum`, `dead-tuple`, `tuple`, `tid`, `visibility-map`, `heap-table`, `db-page`, `b-tree`, `seq-scan`, `index-scan`, `bitmap-scan`, `query-planner`, `fillfactor`, `posting-list`, `metapage`, `free-space-map`. Уже есть и используются как есть: `index`, `index-only-scan`, `hot-update`, `b-tree-page-split`, `explain`, `snapshot`, `transaction`, `wal`.

- [ ] **Step 1:** добавить статьи по схеме файла (`term`, `en`, `short`, `long`, `links`; в `long` нельзя `<T>`, ссылки — `[текст](#id)`). Содержание — из объяснений прототипа (функция `explain` и тексты шагов) и §7.2 спецификации, на примере мессенджера. `heap-table` и существующая `heap-allocation` — в `long` взаимная оговорка «не путать». `links` — документация PostgreSQL (`https://www.postgresql.org/docs/current/…`: routine-vacuuming, mvcc-intro, storage-page-layout, storage-vm, btree, indexes-index-only-scans, using-explain).
- [ ] **Step 2:** `npm test` (тесты глоссария и подписей) и `npm run build` — PASS; страница `/glossary` показывает новые статьи (`npm run dev`, глазами одна-две).
- [ ] **Step 3:** коммит `feat(glossary): термины устройства PostgreSQL для сцены B-tree`.

### Task 4: Модель B-tree и таблицы

**Files:**
- Create: `course/viz/simulations/btree-model.ts`, `course/viz/simulations/btree-model.test.ts`

**Interfaces:**
- Produces:
  ```ts
  export type Tid = readonly [page: number, lp: number]
  export interface HeapRow { lp: number; chat: string; user: number }
  export interface HeapPage { no: number; rows: HeapRow[]; allVisible: boolean }
  export interface LeafEntry { key: number; tids: Tid[] }
  export interface InnerEntry { sep: number | null; child: string }   // null = «−∞»
  export interface LeafNode { id: string; block: number; level: 0; entries: LeafEntry[]; left: string | null; right: string | null }
  export interface InnerNode { id: string; block: number; level: number; entries: InnerEntry[]; left: string | null; right: string | null }
  export type BtNode = LeafNode | InnerNode
  export interface BtModel { pages: HeapPage[]; nodes: Record<string, BtNode>; root: string; nextBlock: number }
  export const CHATS: readonly (readonly [number, number])[]  // [[42,15],[27,84],[3,63],[42,99],[19,50],[8,71],[63,36],[23,88],[27,95],[47,31],[56,90],[42,77]]
  export const PER_PAGE = 4, LEAF_CAPACITY = 5, BUILD_FILL = 4
  export function buildModel(): BtModel
  export function isLeaf(n: BtNode): n is LeafNode
  export function pickChild(n: InnerNode, key: number): number   // индекс последней записи с sep ≤ key (sep null — всегда подходит)
  export function leafFor(m: BtModel, key: number): LeafNode
  export function leafOrder(m: BtModel): string[]                // по ссылкам right от самого левого листа
  export function parentOf(m: BtModel, id: string): InnerNode | undefined
  export function rowAt(m: BtModel, t: Tid): HeapRow | undefined
  export function heapInsert(m: BtModel, chat: string, user: number): Tid       // мутирует m
  export function leafInsert(leaf: LeafEntry[] | LeafNode, key: number, tid: Tid): void  // мутирует; дубликат → в tids
  export function splitLeaf(m: BtModel, leaf: LeafNode): LeafNode                // правая половина в новый блок; родителя НЕ трогает
  export function cloneModel(m: BtModel): BtModel                                // глубокая копия (structuredClone)
  ```
  Построение как в прототипе (`buildModel`): строки по 4 на страницу в порядке чатов, позиции с 1; `allVisible` = false только у страницы 5; листья по 4 ключа (блоки 1–5); внутренние: блок 6 → листья 1–3, блок 7 → листья 4–5, корень блок 8 → 6, 7; разделитель записи i>0 — наименьший ключ поддерева; соседние ссылки у листьев и у блоков 6↔7; `nextBlock = 9`.

- [ ] **Step 1: Тест** (таблица `it.each` где уместно): 6 страниц × 4 строки; `rowAt([0,2])` = `{lp:2, chat:'c1', user:15}`; лист с 42 — блок 3, `tids` = `[[0,1],[1,3],[5,3]]`; корень `b8` разделители `[null, 63]`; `b6` — `[null, 23, 42]`; `b7` — `[null, 88]`; `leafOrder` = `['b1'…'b5']`; `pickChild(b6, 20)` = 0, `(b6, 42)` = 2, `(b8, 77)` = 1; `leafFor(15).block` = 1; `heapInsert('c13',45)` = `[6,1]`, затем `[6,2]`, страница 6 `allVisible` false; вставка 45 и 53 в лист 3 и `splitLeaf` → лист 3 `[42,45,47]`, новый `b9` `[50,53,56]`, `b3.right='b9'`, `b9.right='b4'`, `b4.left='b9'`, `nextBlock=10`; `cloneModel` независим (мутация копии не меняет оригинал).
- [ ] **Step 2:** FAIL. **Step 3:** реализация. **Step 4:** PASS. **Step 5:** коммит `feat(viz): модель B-tree и heap для сцены 2.5`.

### Task 5: Сценарии сцены

**Files:**
- Create: `course/viz/simulations/btree.ts`, `course/viz/simulations/btree.test.ts`

**Interfaces:**
- Consumes: Task 2 (`Step`, `StepCard`, `StepQuiz`), Task 3 (id терминов), Task 4 (модель).
- Produces:
  ```ts
  export type BtScenarioId = 'intro' | 'one' | 'dup' | 'range' | 'ios' | 'seq' | 'ins'
  export type BtView = 'simple' | 'detail'
  export interface BtScenarioMeta { id: BtScenarioId; label: string; learn: string; sql: string; note: Caption; view: BtView }
  export const BT_SCENARIOS: readonly BtScenarioMeta[]   // порядок и подписи — как вкладки прототипа
  export type Mark = 'cur' | 'read' | 'hint'
  export interface BtFrame {
    model: BtModel                     // снимок на этом шаге (после изменений шага)
    learn: string                      // «Сейчас изучаю»
    title: string                      // заголовок шага
    focus: string                      // id элемента для панели по умолчанию
    nodes: Record<string, Mark>        // ключи: id узла или 'meta'
    pages: Record<number, Mark>
    hitEntries: number[]               // ключи «совпало с условием»
    hintEntries: number[]              // ключи, на которые указывает текст шага (пунктир)
    hitRows: string[]                  // 'p:lp'
    tids: (readonly [key: number, page: number, lp: number])[]   // нарисованные TID-стрелки
    edges: (readonly [from: string, to: string])[]               // путь спуска
    sibs: (readonly [from: string, to: string])[]                // пройденные ссылки соседей
    vmChecked: Record<number, 'ok' | 'no'>
    vmCur: number | null
    notes: Record<string, Caption>     // «в этом запросе» для панели, по id узла
    counters: { index: number; heapAccess: number; heapPages: number[]; heapFetches: number }
    result: string[]
  }
  export function btreeScenario(id: BtScenarioId): readonly Step<BtFrame>[]
  ```
  Шаги, тексты, вопросы, порядок — как в прототипе (функции `SC.push(…build)`), с переносом полей: `did`→`card.what`, `uses`→`card.uses`, `reads`→`card.reads`, `why`→`card.why`, `quiz.q/a`→`quiz.question/answer`, `title`→`frame.title` и `caption` (= заголовок + первое предложение «что»). Первое появление термина в сценарии — через `term()` на id из Task 3 (страница, heap, Seq Scan, индексная запись→`index`, TID, B-tree, метастраница, posting list, карта видимости, VACUUM, MVCC, fillfactor, FSM, планировщик, Bitmap Scan, Index Only Scan, сплит→`b-tree-page-split`). Счётчик `heapAccess` растёт, только когда страница отличается от предыдущей прочитанной (как в прототипе); метастраница в `index` не считается.

- [ ] **Step 1: Тест** — числа §5 спецификации по последнему шагу каждого сценария:
  - `one`: 7 шагов; `index` 3, `heapAccess` 1, `result` `['c1']`;
  - `dup`: `index` 3, `heapAccess` 3, `result` `['c1','c4','c12']`;
  - `range`: `result.length` 10, `index` 5, `heapAccess` 9, `heapPages.length` 6;
  - `ios`: `index` 3, `heapFetches` 1, `result.length` 3;
  - `seq`: `heapAccess` 6, `index` 0, `result` `['c1','c4','c12']`;
  - `ins`: последний кадр — `b9` `[50,53,56]`, в `b6` запись `{sep:50, child:'b9'}`; **кадр шага «Лист полон: сплит»** ещё без записи 50 в `b6`; **кадр шага 1** — 5 листьев (снимки не делят объекты: проверить `!==` моделей соседних шагов);
  - каждый шаг каждого сценария: есть `card`, все `stepTermIds` есть в глоссарии (как в `constraint-race.test.ts`), `focus` непустой;
  - число шагов с `quiz` по сценариям: intro 2, one 2, dup 1, range 1, ios 1, seq 1, ins 1.
- [ ] **Step 2:** FAIL. **Step 3:** реализация (трассировщик кадров — как класс `Tr` прототипа: кадр — копия прошлого, `cur`→`read`, затем изменения шага, `cloneModel` в кадр). **Step 4:** PASS. **Step 5:** коммит `feat(viz): сценарии сцены B-tree`.

### Task 6: Объяснения элементов

**Files:**
- Create: `course/viz/simulations/btree-explain.ts`, `course/viz/simulations/btree-explain.test.ts`

**Interfaces:**
- Consumes: Task 2 (`Explain`, `Explanation`), Task 4, Task 5 (`BtFrame`).
- Produces: `export const explainBtree: Explain<BtFrame>` для id `meta`, `node:<id>`, `entry:<key>`, `row:<p>:<lp>`, `page:<p>`, `vm:<p>`, `sib`, `tid`, `table`, `plan`; `export const BT_SIMPLIFICATIONS: Explanation` («Что в модели упрощено»). Тексты — из `explain()` и `SIMPL` прототипа; разделы «Что это / Роль в PostgreSQL / Что хранит / Связи / Пример на числах»; `here` — из `frame.notes`; `real` — блоки «В реальности» прототипа.

- [ ] **Step 1: Тест:** корень на шаге 1 сценария `one` — `title` «Корень», в разделе «Пример на числах» есть «блок 6» и «блок 7»; `entry:42` — упоминает `(0,1)`, `(1,3)`, `(5,3)`; `row:0:2` — «(c1, 15)»; `node:b9` на первом шаге `ins` → `null` (узла ещё нет), на последнем — объяснение листа с «блок 9»; неизвестный id `foo` → `null`; все `explanationTermIds` по всем id всех кадров есть в глоссарии.
- [ ] **Step 2:** FAIL. **Step 3:** реализация. **Step 4:** PASS. **Step 5:** коммит `feat(viz): объяснения элементов сцены B-tree`.

### Task 7: Компоненты мерки в плеере

**Files:**
- Create: `course/viz/components/StepCard.vue`, `course/viz/components/StepQuiz.vue`, `course/viz/components/ExplainPanel.vue`
- Modify: `course/viz/components/Player.vue`

**Interfaces:**
- Consumes: Task 2.
- Produces:
  - `StepCard` props `{ card: StepCard }` — сетка «Что происходит / Какая структура / Что читается / Почему так», части через `<T>` для сегментов-терминов (как подпись в `Player`).
  - `StepQuiz` props `{ quiz: StepQuiz }` — вид и поведение `SelfCheck.vue` (ответ скрыт, кнопка «Сначала ответьте сами, затем откройте ответ»).
  - `ExplainPanel` props `{ explanation: Explanation | null; fallback?: string }` — заголовок, `kind`, разделы, `here` (рамка `--viz-active`), `real` (рамка `--viz-cache`); `null` → `fallback` («Кликните на элемент схемы — здесь появится объяснение»).
  - `Player`: если у шага есть `card` — `StepCard` под подписью; `quiz` — `StepQuiz` с `:key="state.index"` (сброс при смене шага); новый слот `side` (`{ step, index }`) — колонка справа от холста при ширине контейнера ≥ 900 px, иначе под холстом; новое событие `step` (`index: number`) при каждой смене индекса.
- [ ] **Step 1:** реализация; общий рендер `Caption` с `<T>` вынести в маленький компонент `CaptionText.vue` (используют `Player`, `StepCard`, `StepQuiz`, `ExplainPanel`).
- [ ] **Step 2:** `npm run typecheck`, `npm test`, `npm run build` — PASS; любая старая сцена (например урок 2.4, `ConstraintRace`) в `npm run dev` выглядит как раньше.
- [ ] **Step 3:** коммит `feat(viz): карточка шага, вопрос и панель объяснения в плеере`.

### Task 8: Рендерер сцены

**Files:**
- Create: `course/viz/renderers/BtreeView.vue`

**Interfaces:**
- Consumes: Task 4, Task 5.
- Produces: `BtreeView` props `{ frame: BtFrame; view: BtView; selected: string | null }`, emits `select: [id: string]`.

- [ ] **Step 1:** SVG по прототипу (функции `layout` и `render`): ряды «корень / внутренние страницы / листья — индексные записи / таблица chat.members (heap)», метастраница справа сверху (только `detail`), рёбра спуска, ссылки соседей (в `detail` все, в `simple` только пройденные), записи листьев и строки heap кликабельны (атрибут `data-explain` с id, обработчик emit), TID-стрелки из `frame.tids` и для `selected` (запись ↔ строки), карта видимости-чип у страниц heap в `detail`. Классы/цвета только из палитры: рамка «сейчас» — `--viz-client`, «прочитано» — `--viz-active`, «совпало» — `--viz-hit-bg`/`--viz-hit-text`, TID — `--viz-db`, адреса и «→ блок N» — `--viz-addr`, подписи рядов и заголовки страниц — `--viz-muted`. Анимация пульса — только без `prefers-reduced-motion`.
- [ ] **Step 2:** `npm run typecheck` PASS. **Step 3:** коммит `feat(viz): рендерер сцены B-tree`.

### Task 9: Сцена в уроке 2.5, MVCC и VACUUM в тексте

**Files:**
- Create: `course/viz/scenes/BtreeInside.vue`
- Modify: `course/modules/02-database/05-access-patterns.md`, `course/modules/02-database/04-constraints.md`, `course/viz/README.md`, `docs/superpowers/backlog-after-module-0.md`

**Interfaces:**
- Consumes: Tasks 5–8.

- [ ] **Step 1: Сцена.** `BtreeInside.vue`: `VizFrame` → `VariantTabs` (сценарии из `BT_SCENARIOS`), `SegmentedToggle` «Схема / Страницы» (значение по умолчанию — `view` сценария, сбрасывается при смене сценария), кнопка «Что в модели упрощено» (показывает `BT_SIMPLIFICATIONS` в панели), над холстом строка: «Сейчас изучаю: …», SQL сценария (моноширинный блок, подсвеченная часть условия), `note`, счётчики («Страниц индекса прочитано», «Обращений к таблице (разных страниц: N из M)», `Heap Fetches` в `ios`), подпись «данные и номера страниц — учебные». `Player` со слотом `side` → `ExplainPanel` (`explainBtree(selected ?? step.state.focus, step)`); `selected` сбрасывается по событию `step`. Ширина — блок шире колонки статьи, если помещается на 1280 px без прокрутки (решение исполнителя).
- [ ] **Step 2: Урок 2.5.** Новый раздел `## <Part kind="viz" /> Визуализация: B-tree изнутри` перед разделом песочницы access-patterns: 1–2 абзаца «что это и как смотреть» (как у песочницы) и `<BtreeInside />`. В разделе песочницы и в теории «Список чатов: индекс, которого нет» — не пересказывать устройство спуска/Bitmap заново: заменить повтор ссылкой на сцену (числа EXPLAIN из лабы в тексте остаются — это реальный замер). В `<Deep>` про HOT: первое упоминание VACUUM — 2–4 предложения: убирает мёртвые версии строк, освобождает место на странице, ставит отметки в карте видимости, без него таблица и индексы растут; `<T id="vacuum">`, `<T id="visibility-map">`; «подробно — модуль 2b».
- [ ] **Step 3: Урок 2.4.** Место с расшифровкой MVCC в скобках (раздел про гонку и индекс): заменить скобки абзацем — версия строки вместо перезаписи, xmin/xmax (кто создал / кто удалил), снимок определяет, какие версии видит транзакция, старые версии остаются «мёртвыми» до `VACUUM`; `<T id="mvcc">`, `<T id="snapshot">`, `<T id="dead-tuple">`.
- [ ] **Step 4: README** — раздел «Мерка» (таблица §3 спецификации и подвиды механизм/поток, кратко) и раздел «Карточка, объяснение, вопрос» (как подключить: поля шага, `Explain<S>`, слот `side`). **Бэклог** — §8 спецификации.
- [ ] **Step 5:** `npm test`, `npm run typecheck`, `npm run build`, `npm run check:refs`, `npm run check:run`, `npm run check:go`, `npm run check:images` — PASS. Коммит `feat(course): сцена B-tree в уроке 2.5, MVCC и VACUUM в тексте`.

### Task 10: Проверка в браузере

**Files:** — (только проверка; правки — в файлы предыдущих задач, отдельным коммитом)

- [ ] **Step 1:** `npm run dev`; Playwright, окно 1280×900. Урок 2.5, тёмная и светлая темы: все 7 сценариев от первого до последнего шага; на каждом шаге — нет ошибок в консоли, карточка и подпись есть, подписи на схеме не перекрываются; клик по записи 42 подсвечивает строки `(0,1)`, `(1,3)`, `(5,3)`; клик по строке подсвечивает запись; смена шага сбрасывает выделение и скрывает ответ вопроса; в `ins` «Назад» после сплита возвращает 5 листьев; переключатель «Схема/Страницы» работает.
- [ ] **Step 2:** тёмная тема, все уроки со сценами (`grep -l "viz/scenes" course/modules -r`): по снимку каждой сцены на первом шаге — текст читается, раскладка не сломана палитрой. Проблемы — исправить или, если это доводка схемы по мерке, записать в бэклог «модуль N» для будущих PR.
- [ ] **Step 3:** удалить `<repo>/.playwright-mcp/`. Коммит исправлений, если были: `fix(viz): …`.
