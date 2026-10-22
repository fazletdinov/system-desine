// Объяснения элементов сцены B-tree: что нажатый элемент такое, зачем он и что в нём сейчас.
// Тексты — из утверждённого прототипа; чистая функция от кадра, Vue-панель только рисует результат.
// Перевод строки в тексте раздела («\n») — отдельная строка списка; панель показывает его как есть.
import type { Caption } from '../core/caption'
import type { Explain, Explanation, ExplainSection } from '../core/explain'
import { markedCaption, type BtFrame } from './btree'
import {
  blockOf,
  isLeaf,
  LEAF_CAPACITY,
  leafOfKey,
  need,
  parentOf,
  pickChild,
  rowAt,
  sepS,
  tidS,
  type BtModel,
  type BtNode,
  type InnerNode,
  type LeafNode,
} from './btree-model'

const DIGITS = /^\d+$/
const EXAMPLE_LEFT_OF_SEP = 5
const EXAMPLE_RIGHT_OF_SEP = 3

// Термин подчёркивается один раз на объяснение: следующие вхождения идут обычным текстом.
class Builder {
  private readonly seen = new Set<string>()

  text(src: string): Caption {
    return markedCaption(src, this.seen)
  }

  section(heading: string, src: string): ExplainSection {
    return { heading, body: this.text(src) }
  }

  build(title: string, kind: string, rows: readonly (readonly [string, string])[], extra: { here?: Caption; real?: string } = {}): Explanation {
    const sections = rows.map(([h, v]) => this.section(h, v))
    return {
      title,
      kind,
      sections,
      ...(extra.here !== undefined && { here: extra.here }),
      ...(extra.real !== undefined && { real: this.text(extra.real) }),
    }
  }
}

function innerExplanation(b: Builder, f: BtFrame, n: InnerNode, here: Caption | undefined): Explanation {
  const m = f.model
  const isRoot = n.id === m.root
  const second = n.entries[1]?.sep ?? 0
  const lastSep = n.entries.at(-1)?.sep ?? 0
  // Два примера на числах: ключ чуть левее второго разделителя (уходит в первую ветку)
  // и ключ правее последнего (уходит в последнюю). Смещения любые, лишь бы ключ не совпал с разделителем.
  const low = second - EXAMPLE_LEFT_OF_SEP
  const high = lastSep + EXAMPLE_RIGHT_OF_SEP
  const lookup = (key: number): string => {
    const e = need(n.entries[pickChild(n, key)], 'записи')
    return `Ищем ${key}: последний разделитель ≤ ${key} — «${sepS(e)}» → блок ${blockOf(m, e.child)}.`
  }
  const down = n.entries.map((e) => `блок ${blockOf(m, e.child)}`).join(', ')
  const up = isRoot ? 'На неё указывает [[metapage|метастраница]].' : `Вверх: блок ${parentOf(m, n.id)?.block}.`
  return b.build(
    isRoot ? 'Корень' : 'Внутренняя страница',
    `страница индекса · блок ${n.block} · уровень ${n.level}`,
    [
      [
        'Что это',
        isRoot
          ? 'Верхняя страница [[b-tree|B-tree]] индекса. Любой поиск по индексу начинается с неё.'
          : 'Промежуточный уровень дерева между корнем и листьями — «оглавление» для своей части ключей.',
      ],
      [
        'Роль в PostgreSQL',
        isRoot
          ? 'Делит весь диапазон ключей на части и говорит, на какую страницу уровнем ниже идти.'
          : 'Сужает поиск: из диапазона, который ей отдал родитель, выбирает одну дочернюю страницу.',
      ],
      [
        'Что хранит',
        `Записи «разделитель → номер дочерней страницы»:\n${n.entries.map((e) => `${sepS(e)} → блок ${blockOf(m, e.child)}`).join('\n')}\n«−∞» у первой записи значит «всё, что меньше следующего разделителя». В модели разделитель — наименьший ключ своего поддерева.`,
      ],
      ['Связи', `Вниз: ${down}. ${up} Соседние страницы одного уровня тоже связаны ссылками.`],
      ['Пример на числах', `${lookup(low)}\n${lookup(high)}`],
    ],
    {
      ...(here !== undefined && { here }),
      real: 'В странице 8 КБ сотни разделителей, поэтому 3 уровня покрывают миллионы записей. В лабе 2a (2 001 000 строк members) спуск по members_user_id_idx — 3 страницы (labs/2a/results/messenger-explain.txt).',
    },
  )
}

function leafExplanation(b: Builder, m: BtModel, n: LeafNode, here: Caption | undefined): Explanation {
  const side = (id: string | null, none: string): string => (id === null ? none : `блок ${blockOf(m, id)}`)
  const parent = parentOf(m, n.id)
  return b.build(
    'Листовая страница',
    `страница индекса · блок ${n.block} · уровень 0`,
    [
      ['Что это', 'Нижний уровень B-tree. Только здесь лежат индексные записи.'],
      ['Роль в PostgreSQL', 'Хранит пары «ключ → адрес строки». Поиск заканчивается здесь, дальше — в таблицу по [[tid|TID]] (кроме Index Only Scan, если страница all-visible).'],
      [
        'Что хранит',
        `Записей: ${n.entries.length} из ${LEAF_CAPACITY} (в модели):\n${n.entries.map((e) => `${e.key} → ${e.tids.map(tidS).join(' ')}`).join('\n')}`,
      ],
      [
        'Связи',
        `Соседи: ${side(n.left, 'нет (самый левый)')} ← → ${side(n.right, 'нет (самый правый)')}. Родитель: ${parent ? `блок ${parent.block}` : '— (ещё не привязан)'}. TID ведут в [[heap-table|heap]].`,
      ],
      ['Почему ссылки на соседей', 'Диапазон (BETWEEN, ORDER BY) читается проходом по листьям слева направо без возврата к корню.'],
    ],
    {
      ...(here !== undefined && { here }),
      // В модели листья на 4 записи из 5; в PostgreSQL похожий запас задаёт fillfactor индекса.
      real: 'При CREATE INDEX листья заполняются не доверху: [[fillfactor|fillfactor]] B-tree по умолчанию 90 %, остальное — запас под будущие вставки, чтобы не делить страницу сразу ([[b-tree-page-split|сплит]]). В модели тот же приём: 4 записи из 5.',
    },
  )
}

function nodeExplanation(b: Builder, f: BtFrame, id: string): Explanation | null {
  const n: BtNode | undefined = f.model.nodes[id]
  if (n === undefined) return null
  const note = f.notes[id]
  return isLeaf(n) ? leafExplanation(b, f.model, n, note) : innerExplanation(b, f, n, note)
}

function entryExplanation(b: Builder, m: BtModel, key: number): Explanation | null {
  const found = leafOfKey(m, key)
  if (found === undefined) return null
  const { leaf, entry } = found
  const many = entry.tids.length > 1
  const rows = entry.tids.map((t) => {
    const r = rowAt(m, t)
    return `стр. ${t[0]}, поз. ${t[1]}${r ? ` (${r.chat})` : ''}`
  })
  return b.build(`Индексная запись ${key}`, `лист · блок ${leaf.block}`, [
    ['Что это', 'Ключ индекса + адрес строки таблицы ([[tid|TID]]).'],
    [
      'Что хранит',
      `ключ user_id = ${key} и ${many ? `[[posting-list|posting list]] из ${entry.tids.length} TID` : 'один TID'}: ${entry.tids.map(tidS).join(' ')}. Не хранит chat_id и не знает, видна ли строка транзакции.`,
    ],
    ['Откуда взялась', 'Когда INSERT положил строку в heap и она получила адрес, PostgreSQL добавил в каждый индекс таблицы запись с этим адресом.'],
    ['Связи', `Строки таблицы: ${rows.join('; ')} — подсвечены на схеме.`],
    ...(many
      ? ([
          [
            'Повторы ключа',
            // CREATE INDEX/REINDEX склеивают сразу; обычные вставки — лениво, только перед сплитом.
            'Одинаковые ключи PostgreSQL 13+ хранит одной записью со списком адресов (дедупликация). При CREATE INDEX склейка делается сразу; при обычных вставках — лениво: только когда лист заполнился и иначе пришлось бы его делить. Адреса в списке отсортированы.',
          ],
        ] as const)
      : []),
  ])
}

function rowExplanation(b: Builder, m: BtModel, p: number, lp: number): Explanation | null {
  const r = rowAt(m, [p, lp])
  if (r === undefined) return null
  const leaf = leafOfKey(m, r.user)?.leaf
  return b.build(`Строка (${r.chat}, ${r.user})`, `heap · страница ${p} · позиция ${lp}`, [
    ['Что это', 'Строка (в PostgreSQL — [[tuple|кортеж]]) таблицы chat.members.'],
    ['Адрес', `[[tid|TID]] (${p},${lp}): страница ${p}, ${lp}-й указатель строки на странице. В базе его видно как столбец ctid.`],
    ['Что хранит', `chat_id = ${r.chat}, user_id = ${r.user} и служебный заголовок: xmin/xmax — какая транзакция создала эту версию и какая удалила; по ним снимок решает, видна ли она ([[mvcc|MVCC]], урок 2.4).`],
    ['Откуда на неё ссылка', `Из индексной записи ${r.user} в листе блок ${leaf?.block ?? '?'} (подсвечена). Без этой ссылки найти строку можно только перебором всей таблицы.`],
  ])
}

// Смысл отметки all-visible и правило её жизни — одно знание для карточек страницы и карты видимости.
const visibilityStatus = (allVisible: boolean): string =>
  allVisible
    ? 'all-visible ✓ — все строки видны всем транзакциям; [[index-only-scan|Index Only Scan]] может не заглядывать в страницу'
    : 'отметки нет — после последнего VACUUM строки на странице меняли (или VACUUM её ещё не обходил); Index Only Scan будет её читать и проверять видимость'
const VISIBILITY_RULE = 'Отметку ставит [[vacuum|VACUUM]]; любое изменение страницы (INSERT, UPDATE, DELETE) её снимает.'

function pageExplanation(b: Builder, m: BtModel, no: number): Explanation | null {
  const p = m.pages[no]
  if (p === undefined) return null
  return b.build(`Страница таблицы ${p.no}`, 'heap · 8 КБ', [
    ['Что это', 'Кусок файла таблицы фиксированного размера (8 КБ). Единица чтения, записи и кэширования ([[db-page|страница]]).'],
    [
      'Heap',
      '«Куча» здесь — не структура данных из алгоритмов, а файл, где строки лежат без порядка, туда, где было место (свободное место подсказывает [[free-space-map|карта свободного места]]).',
    ],
    ['Что хранит', `заголовок, массив указателей строк и сами строки. Здесь ${p.rows.length}: ${p.rows.map((r) => `(${r.chat}, ${r.user})`).join(', ')}.`],
    ['Карта видимости', `${visibilityStatus(p.allVisible)}. ${VISIBILITY_RULE}`],
    ['Связи', 'На строки ссылаются TID из листьев всех индексов таблицы.'],
  ])
}

function visibilityExplanation(b: Builder, m: BtModel, no: number): Explanation | null {
  const p = m.pages[no]
  if (p === undefined) return null
  return b.build(`Карта видимости: страница ${p.no}`, 'отметка рядом с heap-страницей', [
    ['Что это', 'Два бита на каждую страницу таблицы в отдельном файле [[visibility-map|карты видимости]]; нас интересует бит all-visible.'],
    [
      'Статус',
      visibilityStatus(p.allVisible),
    ],
    ['Кто ставит и снимает', VISIBILITY_RULE],
    ['Почему не в индексе', 'В индексе нет xmin/xmax, поэтому о видимости строки он сам сказать не может — на помощь приходит эта карта.'],
  ])
}

const TID = (b: Builder): Explanation =>
  b.build('TID — ссылка из индекса на строку', 'tuple identifier · ctid', [
    ['Что это', '[[tid|Адрес строки]] в таблице: (номер страницы, номер позиции на ней).'],
    ['Зачем', 'Индекс хранит только ключ. Чтобы получить остальные столбцы и проверить видимость, PostgreSQL идёт по TID прямо на нужную страницу heap — одно чтение вместо перебора.'],
    ['Цена', 'Каждый TID — отдельное (обычно случайное) чтение страницы таблицы. Поэтому индекс выгоден, когда строк мало.'],
    ['На схеме', 'Зелёные линии из листа вниз, в таблицу.'],
  ])

const SIBLINGS = (b: Builder): Explanation =>
  b.build('Ссылки между листьями', 'right-link / left-link', [
    ['Что это', 'Каждый лист знает номер блока соседа слева и справа.'],
    ['Зачем', 'Диапазонный поиск и ORDER BY читают листья подряд, не поднимаясь к корню. Ещё они помогают поиску при параллельном [[b-tree-page-split|сплите]] (алгоритм Lehman–Yao).'],
    ['На схеме', 'Пунктир между листьями; синий — по нему шли в текущем запросе.'],
  ])

const META = (b: Builder, m: BtModel): Explanation => {
  const root = need(m.nodes[m.root], 'корня')
  return b.build('Метастраница', 'блок 0 файла индекса', [
    ['Что это', 'Служебная страница в начале файла индекса ([[metapage|metapage]]).'],
    ['Роль в PostgreSQL', 'Хранит адрес корня и высоту дерева. Спуск по дереву начинается с корня, а его номер PostgreSQL узнаёт отсюда.'],
    ['Что хранит', `корень — блок ${root.block}, уровень корня — ${root.level} (листья — уровень 0)`],
    ['Связи', '→ корень. Сама не хранит ключей.'],
    ['Зачем отдельно', 'Корень может смениться: когда расщепляется корень, сверху появляется новый. Тогда в метастранице меняют номер корня — остальным страницам ничего искать заново не нужно.'],
  ], { real: 'Метастраницу процесс кэширует в памяти, поэтому в EXPLAIN BUFFERS спуск — 3 страницы, без неё.' })
}

const TABLE = (b: Builder): Explanation =>
  b.build('Таблица chat.members', 'таблица мессенджера (миграция 00001_init.sql)', [
    ['Что это', 'Связь «многие ко многим» чатов и пользователей: строка (chat_id, user_id) = «пользователь состоит в чате».'],
    ['Роль', 'По ней мессенджер отвечает на два вопроса: «кто в чате» (индекс members_pkey (chat_id, user_id)) и «в каких чатах пользователь» (индекс members_user_id_idx (user_id)).'],
    ['Что хранит', 'В модели 24 строки на 6 страницах. В лабе — 2 млн строк.'],
    ['Связи', 'Строки лежат в [[heap-table|heap]] (внизу схемы). Индексы ссылаются на них по TID.'],
  ])

const PLAN = (b: Builder): Explanation =>
  b.build(
    'Планировщик',
    'часть PostgreSQL, которая решает, КАК выполнить запрос',
    [
      ['Что это', 'SQL говорит, ЧТО нужно. [[query-planner|План]] — как это достать: читать таблицу целиком ([[seq-scan|Seq Scan]]), спускаться по индексу или собрать TID и читать таблицу по порядку ([[bitmap-scan|Bitmap Scan]]).'],
      [
        'Роль',
        'Оценивает число строк по статистике (pg_stats: число различных значений, частые значения, гистограмма) и считает стоимость каждого способа в условных единицах (seq_page_cost = 1 — последовательное чтение страницы, random_page_cost = 4 — случайное).',
      ],
      ['Что важно', 'Индекс — не приказ, а возможность. План зависит от селективности условия (какая доля строк таблицы под него подходит), размера таблицы, свежести статистики и настроек. Посмотреть выбор: EXPLAIN (ANALYZE, BUFFERS).'],
    ],
    {
      real: 'Лаба 2a (labs/2a/results/messenger-explain.txt): один и тот же запрос дал Index Scan для пользователя с 20 чатами и Bitmap Heap Scan для пользователя с 1000 чатов.',
    },
  )

// Не привязана к кадру: панель открывает её по отдельной ссылке «Что в модели упрощено».
export const BT_SIMPLIFICATIONS: Explanation = new Builder().build(
  'Что в модели упрощено',
  'чтобы отличать учебную модель от настоящего PostgreSQL',
  [
    [
      'Данные придуманы',
      '12 чатов по 2 участника, 24 строки. user_id — числа вместо uuid. Номера блоков и TID учебные, но устроены как настоящие: [[tid|TID]] = (страница, позиция), позиции с 1; в базе их видно через SELECT ctid, * FROM chat.members.',
    ],
    [
      'Размеры',
      'Страница — 8 КБ: в ней сотни индексных записей и порядка 130 строк members. В модели — 4–5. Зато высота дерева та же, что в лабе на 2 млн строк: 3 уровня.',
    ],
    [
      'Логика ≠ файл',
      'Блоки индекса лежат в файле подряд по номерам: 0 — [[metapage|метастраница]], дальше страницы всех уровней вперемешку, в том порядке, в каком их заводил CREATE INDEX: корень получает номер рано (в двухуровневом индексе — обычно блок 3). В модели номера для наглядности идут по уровням: листья, внутренние, корень. «Уровни» на схеме — логическая связь по номерам блоков, а не расположение на диске.',
    ],
    [
      'Опущено',
      'верхняя граница страницы (high key); заголовок страницы и массив указателей; усечение разделителей и TID как часть ключа (PG 12+); алгоритм выбора точки сплита; блокировки и «шаг вправо» при параллельных вставках; [[wal|WAL]].',
    ],
    ['Видимость', '[[mvcc|MVCC]] только упомянута: у каждой строки в heap есть xmin/xmax (урок 2.4), в индексе их нет.'],
    [
      'План',
      'Выбор плана описан словами. На деле его считает формула стоимости с seq_page_cost, random_page_cost, effective_cache_size и статистикой. На нашей крошечной таблице реальный PostgreSQL почти всегда выбрал бы Seq Scan.',
    ],
  ],
)

export const explainBtree: Explain<BtFrame> = (id, step) => {
  const f = step.state
  const b = new Builder()
  const [kind, a, c] = id.split(':')
  switch (kind) {
    case 'meta':
      return META(b, f.model)
    case 'table':
      return TABLE(b)
    case 'plan':
      return PLAN(b)
    case 'tid':
      return TID(b)
    case 'sib':
      return SIBLINGS(b)
    case 'node':
      return a === undefined ? null : nodeExplanation(b, f, a)
    case 'entry':
      return a !== undefined && DIGITS.test(a) ? entryExplanation(b, f.model, Number(a)) : null
    case 'row':
      return a !== undefined && c !== undefined && DIGITS.test(a) && DIGITS.test(c) ? rowExplanation(b, f.model, Number(a), Number(c)) : null
    case 'page':
      return a !== undefined && DIGITS.test(a) ? pageExplanation(b, f.model, Number(a)) : null
    case 'vm':
      return a !== undefined && DIGITS.test(a) ? visibilityExplanation(b, f.model, Number(a)) : null
    default:
      return null
  }
}
