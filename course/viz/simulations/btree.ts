// Сценарии сцены 2.5: как PostgreSQL ищет и вставляет строки через индекс B-tree.
// Тексты и порядок шагов — из утверждённого прототипа. Каждый шаг — полный снимок кадра
// вместе с копией модели, поэтому переход назад и прыжок на любой шаг не требуют пересчёта.
import { markedCaption, stripTerms, type Caption } from '../core/caption'
import type { Step } from '../core/types'
import {
  buildModel,
  cloneModel,
  BUILD_FILL,
  heapInsert,
  isLeaf,
  LEAF_CAPACITY,
  leafInsert,
  leafOfKey,
  need,
  parentOf,
  pickChild,
  rowAt,
  rowKey,
  sepS,
  splitLeaf,
  tidS,
  type BtModel,
  type BtNode,
  type InnerEntry,
  type InnerNode,
  type LeafNode,
  type Tid,
} from './btree-model'

export type BtScenarioId = 'intro' | 'one' | 'dup' | 'range' | 'ios' | 'seq' | 'ins'
export type BtView = 'simple' | 'detail'

export interface BtScenarioMeta {
  id: BtScenarioId
  label: string
  learn: string
  sql: string
  note: Caption
  view: BtView
}

// cur — читается прямо сейчас, read — уже прочитано в этом запросе, hint — на это указывает текст шага.
export type Mark = 'cur' | 'read' | 'hint'

export interface BtFrame {
  model: BtModel
  learn: string
  title: string
  focus: string
  nodes: Record<string, Mark>
  pages: Record<number, Mark>
  hitEntries: number[]
  hintEntries: number[]
  hitRows: string[]
  tids: (readonly [key: number, page: number, lp: number])[]
  edges: (readonly [from: string, to: string])[]
  sibs: (readonly [from: string, to: string])[]
  vmChecked: Record<number, 'ok' | 'no'>
  vmCur: number | null
  notes: Record<string, Caption>
  counters: { index: number; heapAccess: number; heapPages: number[]; heapFetches: number }
  result: string[]
}

export const BT_SCENARIOS: readonly BtScenarioMeta[] = [
  {
    id: 'intro',
    label: 'Введение',
    learn: 'Что вообще происходит',
    view: 'simple',
    sql: `-- listChatsFirst (store.go): список чатов при открытии приложения
SELECT c.id, c.title, …
FROM chat.chats c
JOIN chat.members me ON me.chat_id = c.id
WHERE me.user_id = $1
ORDER BY c.last_activity_at DESC, c.id DESC
LIMIT $2;`,
    note: 'Разбираем условие me.user_id = $1: найти строки chat.members данного пользователя. В мессенджере user_id — uuid, в модели — числа: B-tree сравнивает uuid так же, просто это 16 байт.',
  },
  {
    id: 'one',
    label: 'Один чат пользователя',
    learn: 'Index Scan по одному ключу',
    view: 'detail',
    sql: `-- чаты пользователя 15 (часть listChatsFirst)
SELECT chat_id
FROM chat.members
WHERE user_id = 15;`,
    note: 'План: Index Scan using members_user_id_idx.',
  },
  {
    id: 'dup',
    label: 'Все чаты пользователя',
    learn: 'Повторяющиеся ключи: posting list',
    view: 'detail',
    sql: `-- пользователь 42 состоит в трёх чатах
SELECT chat_id
FROM chat.members
WHERE user_id = 42;`,
    note: 'Один ключ — несколько строк таблицы. Так у любого пользователя мессенджера.',
  },
  {
    id: 'range',
    label: 'Диапазон BETWEEN',
    learn: 'Диапазонный поиск по цепочке листьев',
    view: 'detail',
    sql: `SELECT chat_id, user_id
FROM chat.members
WHERE user_id BETWEEN 20 AND 50;`,
    note: 'Учебный запрос: по user_id мессенджер диапазоном не ищет. Диапазоном по индексу он читает ленту: WHERE chat_id = $1 AND seq < $2 по messages_pkey (урок 2.5).',
  },
  {
    id: 'ios',
    label: 'Index Only Scan',
    learn: 'Когда таблица не нужна',
    view: 'detail',
    sql: `-- нужен только факт строки: сколько чатов у пользователя
SELECT count(*)
FROM chat.members
WHERE user_id = 42;`,
    note: 'План: Index Only Scan using members_user_id_idx. На странице «Страницы» видна карта видимости (VM) у каждой страницы таблицы.',
  },
  {
    id: 'seq',
    label: 'Seq Scan и сравнение',
    learn: 'Seq Scan и когда индекс не помогает',
    view: 'simple',
    sql: `-- как увидеть Seq Scan на настоящей базе
SET enable_indexscan = off;
SET enable_bitmapscan = off;
SELECT chat_id FROM chat.members
WHERE user_id = 42;`,
    note: 'Настройки enable_* не запрещают план: планировщик выберет его, только если других путей нет (в EXPLAIN узел помечен Disabled: true; до PG 18 это делалось огромной добавкой к стоимости) — для экспериментов, не для продакшена.',
  },
  {
    id: 'ins',
    label: 'Вставка и сплит',
    learn: 'Как запись меняет индекс',
    view: 'detail',
    sql: `-- новый личный чат c13 между пользователями 45 и 53
INSERT INTO chat.members (chat_id, user_id)
VALUES ('c13', 45), ('c13', 53);`,
    note: 'Тот же INSERT, что делает store.go при создании чата. Сплит здесь — учебная симуляция: точку раздела PostgreSQL выбирает по свободному месту, а не «пополам по числу записей».',
  },
]

// Над холстом SQL сценария показан целиком, а подсвечено условие, которое разбирает сцена:
// хвост строки после WHERE (для INSERT — после VALUES). Возвращает куски текста, а не HTML,
// чтобы сцена выводила их обычной интерполяцией без v-html.
export interface SqlParts { before: string; cond: string; after: string }
const CONDITION = /^((?!\s*--).*?\b(?:WHERE|VALUES)\s+)(.*?);?[ \t]*$/m
export function sqlCondition(sql: string): SqlParts {
  const m = CONDITION.exec(sql)
  const [head, cond] = [m?.[1], m?.[2]]
  if (m === null || head === undefined || cond === undefined || cond === '') return { before: sql, cond: '', after: '' }
  const start = m.index + head.length
  return { before: sql.slice(0, start), cond, after: sql.slice(start + cond.length) }
}

// --- трассировщик кадров -----------------------------------------------------

// Всё, что кадр хранит помимо модели и текстовых полей шага.
type Work = Omit<BtFrame, 'model' | 'learn' | 'title' | 'focus'>

interface StepInfo {
  title: string
  learn: string
  did: string
  uses: string
  reads: string
  why: string
  focus: string
  quiz?: { q: string; a: string }
}

type Edit = (w: Work, m: BtModel) => void

const FIRST_SENTENCE = /^.*?[.!?](?=\s|$)/s

const push = <T>(list: T[], v: T): void => {
  if (!list.includes(v)) list.push(v)
}
const entryAt = (n: InnerNode, i: number): InnerEntry => need(n.entries[i], `записи ${i} в блоке ${n.block}`)
// «1 строка, 2 строки, 5 строк»
const plural = (n: number, one: string, few: string, many: string): string => {
  const d = n % 10
  if (n % 100 >= 11 && n % 100 <= 14) return many
  return d === 1 ? one : d >= 2 && d <= 4 ? few : many
}

function leafById(m: BtModel, id: string): LeafNode {
  const n = need(m.nodes[id], `узла ${id}`)
  if (!isLeaf(n)) throw new Error(`btree: узел ${id} не лист`)
  return n
}

function emptyWork(): Work {
  return {
    nodes: {},
    pages: {},
    hitEntries: [],
    hintEntries: [],
    hitRows: [],
    tids: [],
    edges: [],
    sibs: [],
    vmChecked: {},
    vmCur: null,
    notes: {},
    counters: { index: 0, heapAccess: 0, heapPages: [], heapFetches: 0 },
    result: [],
  }
}

// cur → read, подсказки прошлого шага гаснут: так на схеме видно только текущее и пройденное.
function settle(marks: Record<string, Mark>): void {
  for (const k of Object.keys(marks)) {
    if (marks[k] === 'cur') marks[k] = 'read'
    else if (marks[k] === 'hint') delete marks[k]
  }
}

class Tracer {
  readonly m = buildModel()
  readonly steps: Step<BtFrame>[] = []
  private w = emptyWork()
  // Термин подчёркивается только при первом появлении в сценарии, дальше — обычный текст.
  private readonly seen = new Set<string>()
  // Повторное обращение к той же странице подряд не считается новым чтением таблицы.
  private lastPage: number | null = null

  get now(): Work {
    return this.w
  }

  // «[[id|текст]]» — термин глоссария; повторное вхождение в сценарии остаётся обычным текстом.
  private caption(src: string): Caption {
    return markedCaption(src, this.seen)
  }

  step(info: StepInfo, edit?: Edit): void {
    const w = structuredClone(this.w)
    settle(w.nodes)
    settle(w.pages as Record<string, Mark>)
    w.hintEntries = []
    w.vmCur = null
    edit?.(w, this.m)
    this.w = w

    const plainWhat = stripTerms(info.did)
    const lead = FIRST_SENTENCE.exec(plainWhat)?.[0] ?? plainWhat
    const state: BtFrame = {
      ...structuredClone(w),
      model: cloneModel(this.m),
      learn: info.learn,
      title: info.title,
      focus: info.focus,
    }
    // Порядок вызовов = порядок показа, чтобы «первое появление» совпало с тем, что видит читатель.
    const what = this.caption(info.did)
    const uses = this.caption(info.uses)
    const reads = this.caption(info.reads)
    const why = this.caption(info.why)
    const quiz = info.quiz && { question: this.caption(info.quiz.q), answer: this.caption(info.quiz.a) }
    this.steps.push({
      state,
      caption: `${info.title}: ${lead}`,
      highlight: [info.focus],
      card: { what, uses, reads, why },
      ...(quiz && { quiz }),
    })
  }

  indexPage(w: Work, id: string, note?: Caption): void {
    w.nodes[id] = 'cur'
    w.counters.index++
    if (note !== undefined) w.notes[id] = note
  }

  hintNode(w: Work, id: string): void {
    w.nodes[id] ??= 'hint'
  }

  hintPage(w: Work, no: number): void {
    w.pages[no] ??= 'hint'
  }

  fetchRow(w: Work, t: Tid, key: number): void {
    const [page, lp] = t
    if (this.lastPage !== page) {
      w.counters.heapAccess++
      this.lastPage = page
    }
    push(w.counters.heapPages, page)
    w.pages[page] = 'cur'
    push(w.hitRows, rowKey(page, lp))
    w.tids.push([key, page, lp])
    w.result.push(need(rowAt(this.m, t), `строки ${tidS(t)}`).chat)
  }

  scanAll(w: Work, page: number): void {
    w.counters.heapAccess++
    w.counters.heapPages.push(page)
    w.pages[page] = 'cur'
  }
}

// Тексты шага спуска: сравнение ключа с разделителями, список веток и объяснение «почему».
function descentTexts(m: BtModel, inner: InnerNode, i: number, key: number, child: BtNode) {
  const next = inner.entries[i + 1]
  const rightSep = sepS(entryAt(inner, 1))
  const cmp =
    i === 0
      ? `${key} < ${rightSep}`
      : `${key} ≥ ${sepS(entryAt(inner, i))}` + (next ? ` и ${key} < ${sepS(next)}` : '')
  const list = inner.entries.map((e) => `${sepS(e)} → ${need(m.nodes[e.child], 'потомка').block}`).join(' · ')
  const range = `${i ? 'от ' + sepS(entryAt(inner, i)) : 'меньше ' + rightSep}${i && next ? ' до ' + sepS(next) + ' (не включая)' : ''}`
  const why =
    inner.id === m.root
      ? `Одна [[db-page|страница]] отсекает часть дерева: всё, что ≥ ${rightSep}, лежит в правой ветке, всё, что меньше, — в левой. В настоящем индексе в корне сотни разделителей, и один шаг отбрасывает почти всё дерево.`
      : `Внутренняя страница делит свою часть ключей ещё мельче. Разделитель — наименьший ключ поддерева справа от него, поэтому в блоке ${child.block} лежат ключи ${range}.`
  return { cmp, list: `записи «разделитель → блок»: ${list}`, why }
}

// Метастраница → корень → внутренние страницы; возвращает лист, в котором должен лежать ключ.
function descend(t: Tracer, key: number, opts: { meta?: boolean; lastWhyExtra?: string } = {}): LeafNode {
  const { m } = t
  let n = need(m.nodes[m.root], 'корня')
  if (opts.meta ?? true) {
    t.step(
      {
        title: 'Метастраница: где корень',
        learn: 'Метастраница индекса',
        did: `Открывает индекс members_user_id_idx и смотрит в [[metapage|метастраницу]] (блок 0): корень сейчас — блок ${n.block}, в дереве ${n.level + 1} уровня.`,
        uses: 'метастраница индекса — блок 0 его файла',
        reads: `адрес корня: блок ${n.block}`,
        why: 'Корень не прибит к одному месту: когда расщепляется сам корень, появляется новый, и его адрес переписывают в метастранице. Её копию PostgreSQL держит в памяти, поэтому в EXPLAIN это чтение обычно не видно.',
        focus: 'meta',
      },
      (w) => {
        w.nodes.meta = 'cur'
      },
    )
  }
  while (!isLeaf(n)) {
    const inner: InnerNode = n
    const i = pickChild(inner, key)
    const child = need(m.nodes[entryAt(inner, i).child], 'потомка')
    const isRoot = inner.id === m.root
    const tx = descentTexts(m, inner, i, key, child)
    t.step(
      {
        title: `${isRoot ? 'Корень' : 'Внутренняя страница'} (блок ${inner.block}): куда спускаться`,
        learn: isRoot ? 'Корень B-tree: выбор ветки' : 'Внутренняя страница: сужение поиска',
        did: `Читает [[db-page|страницу]] и ищет последний разделитель, который ≤ ${key}: ${tx.cmp} → идёт в блок ${child.block}.`,
        uses: isRoot ? 'корень индекса' : 'внутренняя страница индекса',
        reads: tx.list,
        why: isLeaf(child) && opts.lastWhyExtra ? tx.why + opts.lastWhyExtra : tx.why,
        focus: 'node:' + inner.id,
      },
      (w) => {
        t.indexPage(w, inner.id, `${tx.cmp} → блок ${child.block}`)
        w.edges.push([inner.id, child.id])
      },
    )
    n = child
  }
  return n
}

function planStep(t: Tracer, estimate: string, why: string, quiz?: StepInfo['quiz']): void {
  t.step({
    title: 'Разбор запроса и план',
    learn: 'Как PostgreSQL выбирает план',
    did: 'Парсер разбирает SQL, анализатор находит в системном каталоге таблицу chat.members и её индексы: members_pkey (chat_id, user_id) и members_user_id_idx (user_id). [[query-planner|Планировщик]] перебирает способы достать строки и оценивает стоимость каждого.',
    uses: 'системный каталог и статистика pg_stats',
    reads: estimate,
    why,
    focus: 'plan',
    quiz,
  })
}

// --- сценарии ----------------------------------------------------------------

function intro(): Step<BtFrame>[] {
  const t = new Tracer()
  const { m } = t
  const key42 = need(leafOfKey(m, 42), 'ключа 42').entry
  const hintAllPages = (w: Work) => m.pages.forEach((p) => t.hintPage(w, p.no))
  t.step(
    {
      title: 'Откуда берётся запрос',
      learn: 'Какой запрос мы разбираем',
      did: 'Пользователь открывает мессенджер → клиент шлёт GET /v1/chats → use case вызывает ListChatsByMember → pgx отправляет в PostgreSQL запрос listChatsFirst. Первое, что нужно базе, — найти строки chat.members, где user_id = этот пользователь.',
      uses: 'таблица chat.members (chat_id, user_id): кто в каком чате',
      reads: 'пока ничего',
      why: 'Этот запрос выполняется при каждом открытии приложения у каждого пользователя. Если он медленный — медленно всё. В модели таблица маленькая: 12 чатов по 2 участника = 24 строки (внизу схемы, данные учебные).',
      focus: 'table',
    },
    hintAllPages,
  )
  t.step(
    {
      title: 'Таблица лежит страницами',
      learn: 'Страница и heap',
      did: 'Таблица хранится в файле, нарезанном на [[db-page|страницы]] по 8 КБ. Читает и кэширует PostgreSQL тоже страницами: чтобы получить одну строку, надо прочитать её страницу целиком.',
      uses: '[[heap-table|heap]] — так называется файл страниц таблицы',
      reads: '—',
      why: 'Диск и ОС отдают данные блоками, буферный кэш (shared_buffers) хранит страницы. Поэтому стоимость запроса меряют в страницах. Heap («куча») здесь — не двоичная куча из алгоритмов, а файл, где строки лежат без порядка: куда нашлось место. У нас — в порядке создания чатов.',
      focus: 'page:0',
    },
    hintAllPages,
  )
  t.step(
    {
      title: 'Без индекса: прочитать всё',
      learn: 'Seq Scan',
      did: 'Чтобы найти строки с user_id = 42 без индекса, остаётся прочитать все страницы подряд и проверить каждую строку. Это [[seq-scan|Seq Scan]] (последовательное сканирование).',
      uses: '[[heap-table|heap]]',
      reads: 'все 6 страниц, 24 строки; совпали 3 — на страницах 0, 1 и 5',
      why: 'Строки не отсортированы по user_id. Остановиться раньше нельзя: дальше может найтись ещё одна. В лабе в chat.members 2 млн строк — это порядка 15 тыс. страниц (оценка: ~60 байт на строку).',
      focus: 'page:5',
      quiz: {
        q: 'Почему нельзя найти 42 бинарным поиском прямо по таблице?',
        a: 'Бинарный поиск работает только по упорядоченным данным. В heap строки лежат в порядке вставки (здесь — по мере создания чатов): 42 есть на страницах 0, 1 и 5 вперемешку с другими. Упорядоченная по user_id копия есть только в индексе — ради этого он и нужен.',
      },
    },
    (w) => {
      m.pages.forEach((p) => {
        w.pages[p.no] = 'read'
        p.rows
          .filter((r) => r.user === 42)
          .forEach((r) => {
            push(w.hitRows, rowKey(p.no, r.lp))
            w.result.push(r.chat)
          })
      })
      w.counters.heapAccess = m.pages.length
      w.counters.heapPages = m.pages.map((p) => p.no)
    },
  )
  t.step(
    {
      title: 'Идея индекса: отсортированные пары',
      learn: 'Индексная запись: ключ + TID',
      did: 'Индекс members_user_id_idx — отдельный файл. В нём лежат [[index|индексные записи]]: ключ (значение user_id) и [[tid|TID]] — адрес строки в таблице: номер страницы и номер позиции на ней, например (0,1). Записи отсортированы по ключу.',
      uses: 'листовые страницы индекса (нижний ряд дерева)',
      reads: '—',
      why: 'В отсортированном списке ключ находится быстро, а TID сразу говорит, какую страницу таблицы читать — одну, а не все. Индекс НЕ хранит строку целиком: только ключ и адрес. chat_id в нём нет.',
      focus: 'entry:42',
    },
    (w) => {
      w.pages = {}
      w.hitRows = []
      w.counters = { index: 0, heapAccess: 0, heapPages: [], heapFetches: 0 }
      w.result = []
      Object.values(m.nodes)
        .filter((n) => isLeaf(n))
        .forEach((n) => t.hintNode(w, n.id))
      push(w.hitEntries, 42)
      key42.tids.forEach(([p, lp]) => {
        w.tids.push([42, p, lp])
        push(w.hitRows, rowKey(p, lp))
      })
    },
  )
  t.step(
    {
      title: 'Как не листать сам список: B-дерево',
      learn: 'Зачем корень и внутренние страницы',
      did: 'Отсортированные записи сами занимают много страниц. Чтобы не листать их по одной, над листьями строят «оглавление»: внутренние страницы, а над ними — корень. Каждая запись оглавления — «разделитель → номер страницы уровнем ниже».',
      uses: '[[b-tree|B-tree]]: корень → внутренние страницы → листья',
      reads: '—',
      why: 'Страница оглавления вмещает сотни разделителей, поэтому каждый уровень сужает поиск в сотни раз. Миллионы записей — это всего 3 уровня. В лабе на 2 млн строк спуск по этому индексу читает ровно 3 страницы: в EXPLAIN [[bitmap-scan|Bitmap Index Scan]] … Buffers: shared hit=3.',
      focus: 'node:b8',
    },
    (w) => ['b8', 'b6', 'b7'].forEach((id) => t.hintNode(w, id)),
  )
  t.step(
    {
      title: 'Итог: кто что хранит',
      learn: 'Путь поиска целиком',
      did: 'Поиск по индексу: корень → внутренняя страница → лист → TID → страница таблицы. Индекс отвечает на вопрос «где лежит», таблица хранит «что лежит».',
      uses: 'индекс и heap вместе',
      reads: 'путь для 42: блоки 8 → 6 → 3, затем страницы таблицы 0, 1, 5',
      why: 'Индекс — отсортированная копия одного столбца с адресами. Он ускоряет чтение, но его обновляют при каждой вставке, и он занимает место (сценарий «Вставка»). Дальше — по шагам, как это делает PostgreSQL.',
      focus: 'tid',
      quiz: {
        q: 'Есть ли в индексе members_user_id_idx значение chat_id?',
        a: 'Нет. В индексной записи только ключ user_id и TID. За chat_id приходится идти в таблицу по TID. Обойтись без таблицы можно, только если запросу хватает самого ключа (сценарий «[[index-only-scan|Index Only Scan]]») или если столбец добавить в индекс через INCLUDE.',
      },
    },
    (w) => {
      ;['b8', 'b6', 'b3'].forEach((id) => (w.nodes[id] = 'read'))
      w.edges.push(['b8', 'b6'], ['b6', 'b3'])
      push(w.hitEntries, 42)
      key42.tids.forEach(([p, lp]) => {
        w.tids.push([42, p, lp])
        push(w.hitRows, rowKey(p, lp))
        w.pages[p] = 'read'
      })
    },
  )
  return t.steps
}

function one(): Step<BtFrame>[] {
  const t = new Tracer()
  const { m } = t
  planStep(
    t,
    'оценка: строк с user_id = 15 ≈ 1',
    'Оценка селективности: по pg_stats значение 15 редкое, строк ≈ 1. [[seq-scan|Seq Scan]] — 6 [[db-page|страниц]] подряд. Index Scan — 3 страницы индекса + 1 страница таблицы, но вразброс (random_page_cost = 4 против seq_page_cost = 1). План зависит от статистики, селективности, индексов и настроек. Честно: на нашей игрушечной таблице из 6 страниц реальный PostgreSQL выбрал бы Seq Scan — она меньше самого индекса. Модель изображает масштаб лабы (2 млн строк), где для такого пользователя план — Index Scan.',
    {
      q: 'Что изменится в плане, если у пользователя не 1, а 1000 чатов?',
      a: 'Строк много → много случайных чтений таблицы. В лабе для «тяжёлого» пользователя (1000 чатов) план стал другим: [[bitmap-scan|Bitmap Index Scan]] собирает все [[tid|TID]] из индекса (3 страницы), сортирует их по страницам, и Bitmap [[heap-table|Heap]] Scan читает 960 страниц таблицы по порядку, каждую один раз. Для «лёгкого» (20 чатов) — обычный Index Scan.',
    },
  )
  const leaf = descend(t, 15)
  const e = need(
    leaf.entries.find((x) => x.key === 15),
    'ключа 15',
  )
  const tid = need(e.tids[0], 'TID')
  const chat = need(rowAt(m, tid), 'строки').chat
  t.step(
    {
      title: `Лист (блок ${leaf.block}): поиск ключа`,
      learn: 'Листовая страница и TID',
      did: `Записи в листе отсортированы, PostgreSQL находит 15 двоичным поиском по записям страницы. Рядом с ключом — TID ${tidS(tid)}.`,
      uses: 'листовая страница индекса',
      reads: `[[index|индексная запись]] 15 → ${tidS(tid)}`,
      why: 'В листе лежит не строка, а её адрес. Следующий ключ — 19 > 15, значит других строк с 15 нет и дальше искать не нужно.',
      focus: 'entry:15',
      quiz: {
        q: 'Мы нашли ключ в листовой странице. Почему этого недостаточно, чтобы получить всю строку таблицы?',
        a: 'В индексной записи только ключ и TID. Запросу нужен chat_id — он есть только в строке таблицы. Кроме того, индекс не знает, видна ли строка нашей транзакции: версии строк и их xmin/xmax ([[mvcc|MVCC]], урок 2.4) хранятся в [[heap-table|heap]]. Поэтому Index Scan обязан сходить в таблицу.',
      },
    },
    (w) => {
      t.indexPage(w, leaf.id, `15 найден → ${tidS(tid)}`)
      push(w.hitEntries, 15)
    },
  )
  t.step(
    {
      title: `Таблица: строка по TID ${tidS(tid)}`,
      learn: 'Heap и TID',
      did: `Читает страницу ${tid[0]} таблицы, берёт ${tid[1]}-й указатель строки (line pointer) и по нему — саму строку. Проверяет, видна ли эта версия строки текущей транзакции.`,
      uses: 'страница [[heap-table|heap]]',
      reads: `строка (${chat}, 15)`,
      why: `TID — прямой адрес: страница ${tid[0]}, позиция ${tid[1]}. Никакого поиска по таблице — одно чтение страницы.`,
      focus: `row:${rowKey(tid[0], tid[1])}`,
    },
    (w) => t.fetchRow(w, tid, 15),
  )
  t.step({
    title: 'Результат',
    learn: 'Сколько это стоило',
    did: `Исполнитель возвращает chat_id = ${chat}. В настоящем listChatsFirst по этому chat_id дальше ищется сам чат: Index Scan using chats_pkey — такой же спуск, только по другому [[b-tree|B-tree]].`,
    uses: '—',
    reads: 'итого: 3 страницы индекса + 1 страница таблицы',
    why: 'Seq Scan прочитал бы все 6 страниц таблицы, а на 2 млн строк — порядка 15 тыс. В лабе для пользователя с 20 чатами: Index Scan using members_user_id_idx, Buffers: 23 (3 страницы спуска по индексу + по странице таблицы на каждый из 20 чатов), весь запрос — 0,2 мс (источник: labs/2a/results/messenger-explain.txt).',
    focus: `row:${rowKey(tid[0], tid[1])}`,
  })
  return t.steps
}

function dup(): Step<BtFrame>[] {
  const t = new Tracer()
  const { m } = t
  planStep(
    t,
    'оценка: строк с user_id = 42 ≈ 3',
    'Ожидается несколько строк — всё ещё мало по сравнению с размером таблицы, поэтому в масштабе лабы выбирается Index Scan. Чем больше строк ожидается, тем ближе выбор к [[bitmap-scan|Bitmap Heap Scan]] или [[seq-scan|Seq Scan]].',
  )
  const leaf = descend(t, 42, {
    lastWhyExtra: ' Разделитель 42 означает «в этом поддереве ключи ≥ 42», поэтому поиск 42 идёт именно сюда.',
  })
  const e = need(
    leaf.entries.find((x) => x.key === 42),
    'ключа 42',
  )
  t.step(
    {
      title: `Лист (блок ${leaf.block}): один ключ — три адреса`,
      learn: 'Дедупликация: posting list',
      did: `Ключ 42 записан в листе один раз со списком из трёх [[tid|TID]]: ${e.tids.map(tidS).join(' ')}. Это [[posting-list|posting list]] — дедупликация [[b-tree|B-tree]] (PostgreSQL 13+): одинаковые ключи хранятся одной записью, адреса внутри отсортированы. При CREATE INDEX склейка делается сразу, при обычных вставках — лениво, перед тем как лист пришлось бы делить.`,
      uses: 'листовая страница индекса',
      reads: `42 → ${e.tids.map(tidS).join(' ')}`,
      why: 'Пользователь мессенджера состоит во многих чатах — user_id повторяется постоянно. Без дедупликации на каждую пару была бы отдельная запись с копией ключа; с ней индекс заметно меньше. Если повторов больше, чем помещается в странице, они продолжаются в соседнем листе — тогда поиск идёт вправо по ссылке между листьями.',
      focus: 'entry:42',
      quiz: {
        q: 'Три строки пользователя 42 стоят в индексе рядом. Будут ли они рядом в таблице?',
        a: 'Нет: TID указывают на страницы 0, 1 и 5. Пользователь вступал в чаты в разное время, и строки ложились туда, где тогда было место. Поэтому 3 строки = 3 разные страницы таблицы = 3 случайных чтения.',
      },
    },
    (w) => {
      t.indexPage(w, leaf.id, `42 → posting list из ${e.tids.length} TID`)
      push(w.hitEntries, 42)
    },
  )
  e.tids.forEach((tid, i) => {
    const chat = need(rowAt(m, tid), 'строки').chat
    t.step(
      {
        title: `Таблица: TID ${tidS(tid)}`,
        learn: 'Переход по TID в heap',
        did: `Берёт ${i + 1}-й адрес из списка и читает страницу ${tid[0]} таблицы, позицию ${tid[1]}: строка (${chat}, 42).`,
        uses: 'страница [[heap-table|heap]]',
        reads: `строка (${chat}, 42)`,
        why: i
          ? 'Другая страница — новое чтение. Строки одного ключа разбросаны по таблице.'
          : 'Index Scan выдаёт строки по одной: адрес из индекса → страница таблицы → строка → следующий адрес.',
        focus: `row:${rowKey(tid[0], tid[1])}`,
      },
      (w) => t.fetchRow(w, tid, 42),
    )
  })
  t.step(
    {
      title: 'Конец: следующий ключ больше',
      learn: 'Когда поиск останавливается',
      did: 'После списка 42 в листе идёт ключ 47 > 42. Индекс отсортирован, значит строк с 42 больше нет — поиск закончен.',
      uses: 'та же листовая страница',
      reads: 'итого: 3 страницы индекса + 3 страницы таблицы',
      why: 'Остановка — заслуга сортировки: в таблице так рано остановиться нельзя.',
      focus: 'entry:47',
    },
    (w) => {
      w.hintEntries.push(47)
    },
  )
  return t.steps
}

function range(): Step<BtFrame>[] {
  const t = new Tracer()
  const { m } = t
  const [lo, hi] = [20, 50]
  planStep(
    t,
    'оценка: ≈ 10 строк из 24 — почти половина',
    'Честно: при такой доле строк планировщик выбрал бы [[bitmap-scan|Bitmap Heap Scan]] или [[seq-scan|Seq Scan]]. Здесь мы проходим Index Scan, чтобы увидеть механику диапазона.',
  )
  let leaf: LeafNode | undefined = descend(t, lo)
  let prev: LeafNode | undefined
  while (leaf) {
    const cur: LeafNode = leaf
    const from = prev
    const hits = cur.entries.filter((e) => e.key >= lo && e.key <= hi)
    const beyond = cur.entries.find((e) => e.key > hi)
    const lastKey = need(cur.entries.at(-1), 'ключа').key
    t.step(
      {
        title: `Лист (блок ${cur.block})${from ? ' — по ссылке вправо' : ''}`,
        learn: from ? 'Цепочка листьев' : 'Начало диапазона',
        did: !hits.length
          ? `Поиск пришёл в лист, где стоял бы ключ ${lo}: после ${lastKey}. Ключей ≥ ${lo} здесь нет — PostgreSQL переходит по ссылке на правого соседа, не поднимаясь к корню.`
          : `${from ? `Перешёл по ссылке из блока ${from.block}. ` : ''}В диапазон попали ${hits.map((e) => e.key).join(', ')}${beyond ? `; ключ ${beyond.key} > ${hi} — дальше идти не нужно` : ''}.`,
        uses: 'листовая страница и ссылка на соседний лист',
        reads: hits.length ? hits.map((e) => `${e.key} → ${e.tids.map(tidS).join(' ')}`).join('; ') : 'совпадений нет',
        why:
          from || !hits.length
            ? 'Листья связаны в цепочку в порядке ключей (ссылки влево и вправо). Поэтому диапазон читается проходом по листьям слева направо, а не новым спуском для каждого ключа.'
            : 'Спуск нашёл начало диапазона, дальше — только вправо по листьям.',
        focus: 'node:' + cur.id,
      },
      (w) => {
        t.indexPage(w, cur.id)
        hits.forEach((e) => push(w.hitEntries, e.key))
        if (from) w.sibs.push([from.id, cur.id])
        if (beyond) w.hintEntries.push(beyond.key)
      },
    )
    if (hits.length) {
      const list = hits.flatMap((e) => e.tids.map((tid) => ({ key: e.key, tid })))
      const first = need(list[0], 'TID')
      t.step(
        {
          title: `Таблица: ${list.length} ${plural(list.length, 'строка', 'строки', 'строк')} по TID`,
          learn: 'Случайные чтения таблицы',
          did: `По очереди проходит адреса ([[tid|TID]]): ${list.map((x) => `${x.key}→${tidS(x.tid)}`).join(', ')}. Страницы таблицы: ${list.map((x) => x.tid[0]).join(', ')}.`,
          uses: 'страницы [[heap-table|heap]]',
          reads: `строки ${list.map((x) => `${need(rowAt(m, x.tid), 'строки').chat}/${x.key}`).join(', ')}`,
          why: 'Ключи в индексе идут подряд, а их строки лежат вразброс: Index Scan скачет по страницам таблицы. Если подряд идут два адреса на одной странице, повторно её не читают.',
          focus: `page:${first.tid[0]}`,
        },
        (w) => list.forEach((x) => t.fetchRow(w, x.tid, x.key)),
      )
    }
    if (beyond || !cur.right) break
    prev = cur
    leaf = leafById(m, cur.right)
  }
  const { counters: c, result } = t.now
  t.step({
    title: 'Результат',
    learn: 'Узкий и широкий диапазон',
    did: `Вернул ${result.length} ${plural(result.length, 'строку', 'строки', 'строк')}. Индекс: ${c.index} страниц. Таблица: ${c.heapAccess} обращений к ${c.heapPages.length} разным страницам из ${m.pages.length}.`,
    uses: '—',
    reads: `chat_id: ${result.join(', ')}`,
    why: 'Для узкого диапазона это быстро. Для широкого — почти каждая строка стоит случайного чтения, и полный проход по таблице подряд оказывается дешевле.',
    focus: 'sib',
    quiz: {
      q: 'Почему индекс выгоден для узкого диапазона и может проиграть на широком?',
      a: `Каждый TID — чтение какой-то страницы таблицы, причём в случайном порядке. Здесь ${result.length} ${plural(result.length, 'строка', 'строки', 'строк')} дали ${c.heapAccess} обращений к ${c.heapPages.length} страницам из ${m.pages.length} — почти вся таблица, плюс страницы индекса. Если диапазон захватывает заметную долю таблицы, дешевле Seq Scan (подряд) или Bitmap Heap Scan (собрать TID, отсортировать по страницам, прочитать каждую один раз).`,
    },
  })
  return t.steps
}

function ios(): Step<BtFrame>[] {
  const t = new Tracer()
  const { m } = t
  planStep(
    t,
    'запросу не нужны столбцы кроме user_id',
    'Всё, что нужно запросу, есть в самом индексе: ключ user_id. Поэтому возможен [[index-only-scan|Index Only Scan]] — читать индекс и по возможности не ходить в таблицу. В отличие от Index Scan, которому за chat_id таблица нужна всегда.',
  )
  const leaf = descend(t, 42)
  const e = need(
    leaf.entries.find((x) => x.key === 42),
    'ключа 42',
  )
  t.step(
    {
      title: `Лист (блок ${leaf.block}): три адреса`,
      learn: 'Index Only Scan: что ещё нужно проверить',
      did: `Находит 42 → ${e.tids.map(tidS).join(' ')}. Значения брать из таблицы не нужно — но нужно знать, видны ли эти строки нашей транзакции.`,
      uses: 'листовая страница',
      reads: `42 → ${e.tids.map(tidS).join(' ')}`,
      why: 'Индекс не хранит информации о видимости (xmin/xmax живут в строке таблицы). Строка могла быть удалена или ещё не закоммичена. Вместо похода в таблицу PostgreSQL сначала смотрит в [[visibility-map|карту видимости]].',
      focus: 'entry:42',
    },
    (w) => {
      t.indexPage(w, leaf.id)
      push(w.hitEntries, 42)
    },
  )
  e.tids.forEach((tid) => {
    const visible = need(m.pages[tid[0]], 'страницы').allVisible
    t.step(
      {
        title: `Карта видимости: страница ${tid[0]} — ${visible ? 'all-visible' : 'нет отметки'}`,
        learn: 'Visibility map',
        did: visible
          ? `Бит all-visible у страницы ${tid[0]} стоит: все строки на ней видны всем транзакциям. В таблицу не идёт — строка ${tidS(tid)} засчитана.`
          : `Бит all-visible у страницы ${tid[0]} не стоит (на ней недавно меняли строки, [[vacuum|VACUUM]] ещё не отметил). Приходится прочитать страницу ${tid[0]} и проверить версию строки ${tidS(tid)}: это [[heap-table|Heap]] Fetches.`,
        uses: 'карта видимости (visibility map): 2 бита на страницу таблицы',
        reads: visible ? '1 бит карты видимости' : `бит карты + страница таблицы ${tid[0]}`,
        why: 'Карту видимости обновляет [[vacuum|VACUUM]], любая запись на страницу сбрасывает бит. Чем свежее VACUUM, тем меньше Index Only Scan ходит в таблицу.',
        focus: 'vm:' + tid[0],
      },
      (w) => {
        w.vmChecked[tid[0]] = visible ? 'ok' : 'no'
        w.vmCur = tid[0]
        if (visible) {
          push(w.hitRows, rowKey(tid[0], tid[1]))
          w.result.push('✓')
          return
        }
        t.fetchRow(w, tid, 42)
        w.counters.heapFetches++
        // Значение строки не нужно: count(*) считает факт, поэтому в результате остаётся отметка.
        w.result[w.result.length - 1] = '✓'
      },
    )
  })
  t.step({
    title: 'Результат: count = 3',
    learn: 'Index Only Scan против Index Scan',
    did: 'Возвращает 3. Прочитано 3 страницы индекса и 1 страница таблицы ([[heap-table|Heap]] Fetches: 1). Обычный Index Scan прочитал бы 3 страницы таблицы.',
    uses: '—',
    reads: 'count = 3',
    why: 'В лабе подзапрос «участники чата» идёт как Index Only Scan using members_pkey … Heap Fetches: 0: таблица после VACUUM, все страницы all-visible, в [[heap-table|heap]] не пошли ни разу (источник: labs/2a/results/messenger-explain.txt).',
    focus: 'vm:5',
    quiz: {
      q: 'Будет ли Index Only Scan по members_user_id_idx для SELECT chat_id FROM chat.members WHERE user_id = 42?',
      a: 'Нет: chat_id в этом индексе нет, взять его можно только из строки таблицы. Сделать индекс «покрывающим» можно так: CREATE INDEX … ON chat.members (user_id) INCLUDE (chat_id) — ценой размера индекса и стоимости записи.',
    },
  })
  return t.steps
}

function seq(): Step<BtFrame>[] {
  const t = new Tracer()
  const { m } = t
  t.step({
    title: 'План: Seq Scan',
    learn: 'Последовательное сканирование',
    did: 'Индексные способы выключены (или индекса нет) — остаётся читать таблицу целиком, [[db-page|страницу]] за страницей, и проверять условие у каждой строки.',
    uses: '[[heap-table|heap]]',
    reads: '—',
    why: '[[seq-scan|Seq Scan]] читает страницы подряд, а последовательное чтение дешевле случайного. Поэтому на большой доле строк он выигрывает.',
    focus: 'plan',
  })
  m.pages.forEach((p) => {
    const hit = p.rows.filter((r) => r.user === 42)
    t.step(
      {
        title: `Страница ${p.no}: ${hit.length ? 'есть совпадение' : 'нет совпадений'}`,
        learn: 'Seq Scan: каждая страница',
        did: `Читает страницу ${p.no} и проверяет все ${p.rows.length} строки: user_id ${p.rows.map((r) => r.user).join(', ')}.${hit.length ? ' 42 нашлась — но читаем дальше: без индекса неизвестно, есть ли ещё.' : ''}`,
        uses: 'страница [[heap-table|heap]]',
        reads: `${p.rows.length} строки`,
        why: 'Проверяется каждая строка таблицы, даже если ответ — одна строка.',
        focus: 'page:' + p.no,
      },
      (w) => {
        t.scanAll(w, p.no)
        hit.forEach((r) => {
          push(w.hitRows, rowKey(p.no, r.lp))
          w.result.push(r.chat)
        })
      },
    )
  })
  t.step({
    title: 'Сравнение',
    learn: 'Index Scan против Seq Scan',
    did: 'Seq Scan: 6 страниц таблицы, 24 проверенные строки. Index Scan (сценарий «Все чаты пользователя»): 3 страницы индекса + 3 страницы таблицы.',
    uses: '—',
    reads: 'результат одинаковый: c1, c4, c12',
    why: 'На 6 страницах разница копеечная — и реальный PostgreSQL здесь выбрал бы Seq Scan. Разница появляется с ростом таблицы: спуск по индексу почти не растёт (3–4 страницы на миллионы строк), а Seq Scan растёт вместе с таблицей.',
    focus: 'plan',
  })
  t.step({
    title: 'Когда индекс не помогает',
    learn: 'Когда планировщик не берёт индекс',
    did: 'Реальные планы из лабы (2 млн строк chat.members):\n• 20 чатов → Index Scan: 23 страницы, весь запрос — 0,2 мс;\n• 1000 чатов → [[bitmap-scan|Bitmap Heap Scan]]: 3 страницы индекса + 960 страниц таблицы, весь запрос — 2 мс.',
    uses: '—',
    reads: 'labs/2a/results/messenger-explain.txt',
    why: 'Индекс проигрывает, когда условие выбирает большую долю строк, когда таблица маленькая, когда статистика устарела и оценка врёт, когда условие не совпадает с индексом (функция над столбцом, другой тип, второй столбец составного индекса без первого).',
    focus: 'plan',
    quiz: {
      q: 'Когда [[query-planner|планировщик]] выберет Seq Scan, хотя индекс есть?',
      a: 'Когда по оценке это дешевле: условие выбирает заметную долю таблицы (случайных чтений больше, чем страниц в таблице), таблица крошечная, или индекс к условию не подходит — например WHERE lower(x) = … при индексе по x. Между ними есть [[bitmap-scan|Bitmap Heap Scan]]: [[tid|TID]] из индекса, но таблица читается по порядку страниц — его и выбрал план для пользователя с 1000 чатов.',
    },
  })
  return t.steps
}

function ins(): Step<BtFrame>[] {
  const t = new Tracer()
  const { m } = t
  // Новая строка попадает на новую страницу, только если последняя полна: сценарий рассчитан на 24 строки при 4 на странице.
  const newPage = m.pages.length
  const expected1: Tid = [newPage, 1]
  let t1: Tid | undefined
  let t2: Tid | undefined
  t.step(
    {
      title: 'Строка 1 → таблица',
      learn: 'Вставка в heap',
      did: `Сначала строка (c13, 45) кладётся в таблицу. [[db-page|Страницы]] 0–${newPage - 1} заполнены, поэтому таблица растёт на страницу ${newPage}, и строка получает [[tid|TID]] ${tidS(expected1)}.`,
      uses: '[[heap-table|heap]] и [[free-space-map|карта свободного места]] (FSM)',
      reads: `FSM: свободного места на страницах 0–${newPage - 1} нет`,
      why: 'Место для новой строки PostgreSQL ищет по карте свободного места, а не в конце по порядку ключа. Поэтому строки одного пользователя оказываются на разных страницах.',
      focus: `page:${newPage}`,
    },
    (w) => {
      t1 = heapInsert(m, 'c13', 45)
      if (t1[0] !== expected1[0] || t1[1] !== expected1[1]) throw new Error(`btree: ожидали TID ${tidS(expected1)}, получили ${tidS(t1)}`)
      w.result.push('(c13, 45)')
      w.pages[t1[0]] = 'cur'
      push(w.hitRows, rowKey(t1[0], t1[1]))
    },
  )
  const tid1 = need(t1, 'TID строки 1')
  let leaf = descend(t, 45, { meta: false })
  t.step(
    {
      title: `Лист (блок ${leaf.block}): запись 45 помещается`,
      learn: 'Вставка в лист',
      did: `Спуск привёл в блок ${leaf.block}. Там ${leaf.entries.length} ${plural(leaf.entries.length, 'запись', 'записи', 'записей')} из ${LEAF_CAPACITY} возможных — PostgreSQL вставляет 45 → ${tidS(tid1)} на своё место по порядку.`,
      uses: 'листовая страница',
      reads: `есть место: ${leaf.entries.length} из ${LEAF_CAPACITY}`,
      why: `При CREATE INDEX листья заполняются не до конца ([[fillfactor|fillfactor]] 90%), чтобы осталось место под вставки. В модели в каждом листе ${BUILD_FILL} записи из ${LEAF_CAPACITY} возможных. Индекс members_pkey получает свою запись так же — на схеме его нет.`,
      focus: 'entry:45',
    },
    (w) => {
      leafInsert(leafById(m, leaf.id), 45, tid1)
      t.indexPage(w, leaf.id)
      push(w.hitEntries, 45)
      w.tids.push([45, tid1[0], tid1[1]])
    },
  )
  t.step(
    {
      title: 'Строка 2 → таблица',
      learn: 'Вставка в heap',
      did: `Вторая строка (c13, 53) ложится на ту же новую страницу ${newPage}: TID ${tidS([newPage, 2])}.`,
      uses: '[[heap-table|heap]]',
      reads: `на странице ${newPage} есть место`,
      why: 'Обе строки одного INSERT вставляются по очереди: строка в heap, затем запись в каждый индекс таблицы.',
      focus: `page:${newPage}`,
    },
    (w) => {
      t2 = heapInsert(m, 'c13', 53)
      w.result.push('(c13, 53)')
      w.pages[t2[0]] = 'cur'
      push(w.hitRows, rowKey(t2[0], t2[1]))
    },
  )
  const tid2 = need(t2, 'TID строки 2')
  leaf = descend(t, 53, { meta: false })
  const splitId = `b${m.nextBlock}`
  t.step(
    {
      title: `Лист (блок ${leaf.block}) полон: сплит`,
      learn: 'Расщепление страницы',
      did: `В блоке ${leaf.block} уже ${leaf.entries.length} записей из ${LEAF_CAPACITY} — для 53 нет места. Сначала PostgreSQL пробует освободить место (убрать мёртвые записи, дедуплицировать) — здесь не помогает. Тогда он расщепляет страницу ([[b-tree-page-split|сплит]]): берёт новую (блок ${m.nextBlock}), переносит туда правую половину и перевязывает соседей.`,
      uses: 'две листовые страницы',
      reads: '—',
      why: 'Страница фиксированного размера, поэтому расти «вширь» нельзя — только делиться. После сплита обе половины наполовину пусты: следующие вставки сюда снова пройдут без сплита.',
      focus: 'node:' + splitId,
    },
    (w) => {
      const left = leafById(m, leaf.id)
      leafInsert(left, 53, tid2)
      const right = splitLeaf(m, left)
      w.nodes[left.id] = 'cur'
      w.nodes[right.id] = 'cur'
      w.counters.index++
      push(w.hitEntries, 53)
      w.tids.push([53, tid2[0], tid2[1]])
      w.sibs.push([left.id, right.id])
    },
  )
  const created = leafById(m, splitId)
  const separator = need(created.entries[0], 'ключа').key
  const parent = need(parentOf(m, leaf.id), 'родителя')
  t.step(
    {
      title: `Разделитель ${separator} уходит в родителя`,
      learn: 'Как дерево растёт',
      did: `Родитель (блок ${parent.block}) получает новую запись «${separator} → блок ${created.block}». Теперь поиск ключей ≥ ${separator} из блока ${parent.block} идёт в новый лист.`,
      uses: 'внутренняя страница',
      reads: `место в блоке ${parent.block} есть`,
      why: 'Если бы в родителе не хватило места, расщепился бы и он, и так вверх до корня. Когда расщепляется корень, над ним появляется новый — только так [[b-tree|B-tree]] растёт в высоту, поэтому все листья всегда на одной глубине.',
      focus: 'node:' + parent.id,
    },
    (w) => {
      const p = need(parentOf(m, leaf.id), 'родителя')
      const pos = p.entries.findIndex((e) => e.child === leaf.id)
      p.entries.splice(pos + 1, 0, { sep: separator, child: created.id })
      w.nodes[p.id] = 'cur'
      w.edges.push([p.id, created.id])
    },
  )
  t.step({
    title: 'Цена записи',
    learn: 'Индексы ускоряют чтение, но удорожают запись',
    did: 'Каждая вставленная строка chat.members трогает: страницу heap, лист members_pkey, лист members_user_id_idx (иногда со сплитом), проверки внешних ключей (поиск по chats_pkey и users_pkey), и всё это пишется в WAL до COMMIT.',
    uses: 'heap, все индексы таблицы, WAL',
    reads: '—',
    why: 'Каждый новый индекс — ещё одна запись на каждую вставку и место на диске и в кэше. В мессенджере индекс по chat.chats (last_activity_at) не завели именно поэтому: [[query-planner|планировщик]] его не брал, а обновление чата потеряло бы HOT (раздел «Цена бесполезного индекса: HOT» этого урока, labs/2a).',
    focus: 'node:' + created.id,
    quiz: {
      q: 'Почему индекс ускоряет SELECT, но замедляет INSERT?',
      a: 'SELECT читает 3–4 страницы индекса вместо всей таблицы. INSERT же обязан добавить запись в КАЖДЫЙ индекс таблицы: найти лист спуском, вставить, иногда расщепить страницу и обновить родителя — и всё это записать в WAL. Индексов больше — запись дороже, а места на диске и в shared_buffers они тоже занимают.',
    },
  })
  return t.steps
}

const BUILDERS: Record<BtScenarioId, () => Step<BtFrame>[]> = { intro, one, dup, range, ios, seq, ins }

export function btreeScenario(id: BtScenarioId): readonly Step<BtFrame>[] {
  return BUILDERS[id]()
}
