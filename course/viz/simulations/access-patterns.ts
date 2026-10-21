import type { Caption, CaptionSegment } from '../core/caption'
import { term } from '../core/caption'
import { formatNumber } from '../core/format'
import type { Simulation, Step } from '../core/types'

// Песочница урока 2.5: запросы мессенджера × индексы схемы. Для каждого запроса модель выбирает индекс,
// который читает меньше всего строк, и считает, сколько строк прочитано: точечный поиск, диапазон по
// префиксу индекса или полный проход таблицы (Seq Scan). Это не планировщик PostgreSQL: нет стоимостей
// страниц, статистики и bitmap-сканов. Число строк, подходящих под равенство по столбцам, оценивается
// как rows / Π distinct(col) — то же допущение о независимости столбцов, что делает и планировщик без
// расширенной статистики. Объёмы таблиц — модельные (мессенджер на миллион пользователей), не замер.

export type TableName = 'users.users' | 'auth.credentials' | 'chat.chats' | 'chat.members' | 'chat.messages' | 'platform.idempotency_keys'

export interface TableDef {
  rows: number
  distinct: Readonly<Record<string, number>> // число различных значений столбца
}

// Модельные объёмы: 10⁶ пользователей, у каждого ~6 чатов по ~3 участника, ~100 сообщений на чат,
// ключи идемпотентности за сутки хранения.
export const TABLES: Readonly<Record<TableName, TableDef>> = {
  'users.users': { rows: 1_000_000, distinct: { id: 1_000_000, username: 1_000_000 } },
  'auth.credentials': { rows: 1_000_000, distinct: { user_id: 1_000_000 } },
  'chat.chats': { rows: 2_000_000, distinct: { id: 2_000_000 } },
  'chat.members': { rows: 6_000_000, distinct: { chat_id: 2_000_000, user_id: 1_000_000 } },
  'chat.messages': { rows: 200_000_000, distinct: { chat_id: 2_000_000, id: 200_000_000, seq: 100_000, author_id: 1_000_000, client_message_id: 200_000_000 } },
  'platform.idempotency_keys': { rows: 5_000_000, distinct: { scope: 10, key: 5_000_000 } },
}

export interface IndexDef {
  id: string
  table: TableName
  columns: readonly string[]
  unique: boolean
  label: string // как индекс выглядит в DDL
}

export interface QueryDef {
  id: string
  title: string
  sql: string
  table: TableName
  eq: readonly string[] // столбцы с условием равенства
  orderBy?: string // столбец сортировки, если она нужна запросу
  limit?: number
  note: string // зачем этот запрос мессенджеру
}

// Каталог: паттерны доступа мессенджера (урок 2.5). Список чатов показан первой ступенью — чтением
// chat.members; затем по найденным chat_id читаются строки chat.chats по первичному ключу.
export const QUERIES: readonly QueryDef[] = [
  {
    id: 'feed',
    title: 'Лента чата',
    sql: 'SELECT … FROM chat.messages\nWHERE chat_id = $1 AND seq < $2\nORDER BY seq DESC LIMIT 50',
    table: 'chat.messages',
    eq: ['chat_id'],
    orderBy: 'seq',
    limit: 50,
    note: 'Открыть чат и листать историю вверх: keyset-пагинация по seq.',
  },
  {
    id: 'message-by-id',
    title: 'Сообщение по id',
    sql: 'SELECT … FROM chat.messages\nWHERE id = $1',
    table: 'chat.messages',
    eq: ['id'],
    note: 'Ссылка на сообщение, ответ на сообщение: клиент знает только id.',
  },
  {
    id: 'chats-of-member',
    title: 'Чаты пользователя',
    sql: 'SELECT chat_id FROM chat.members\nWHERE user_id = $1',
    table: 'chat.members',
    eq: ['user_id'],
    note: 'Первый экран приложения: список чатов. Затем строки chat.chats по найденным id.',
  },
  {
    id: 'is-member',
    title: 'Участник ли',
    sql: 'SELECT 1 FROM chat.members\nWHERE chat_id = $1 AND user_id = $2',
    table: 'chat.members',
    eq: ['chat_id', 'user_id'],
    note: 'Проверка доступа перед каждым чтением и отправкой.',
  },
  {
    id: 'user-by-username',
    title: 'Пользователь по имени',
    sql: 'SELECT … FROM users.users\nWHERE username = $1',
    table: 'users.users',
    eq: ['username'],
    note: 'Вход по имени и поиск собеседника.',
  },
  {
    id: 'credentials',
    title: 'Учётные данные',
    sql: 'SELECT password_hash FROM auth.credentials\nWHERE user_id = $1',
    table: 'auth.credentials',
    eq: ['user_id'],
    note: 'Вход: проверить пароль найденного пользователя.',
  },
  {
    id: 'idempotency',
    title: 'Ключ идемпотентности',
    sql: 'SELECT … FROM platform.idempotency_keys\nWHERE scope = $1 AND key = $2',
    table: 'platform.idempotency_keys',
    eq: ['scope', 'key'],
    note: 'Повтор запроса с тем же Idempotency-Key: вернуть сохранённый ответ.',
  },
]

// Индексы, которые не зависят от переключателей: схема мессенджера (рулинг 1 плана модуля 2a).
const BASE_INDEXES: readonly IndexDef[] = [
  { id: 'users_pkey', table: 'users.users', columns: ['id'], unique: true, label: 'PRIMARY KEY (id)' },
  { id: 'users_username_key', table: 'users.users', columns: ['username'], unique: true, label: 'UNIQUE (username)' },
  { id: 'credentials_pkey', table: 'auth.credentials', columns: ['user_id'], unique: true, label: 'PRIMARY KEY (user_id)' },
  { id: 'chats_pkey', table: 'chat.chats', columns: ['id'], unique: true, label: 'PRIMARY KEY (id)' },
  { id: 'members_pkey', table: 'chat.members', columns: ['chat_id', 'user_id'], unique: true, label: 'PRIMARY KEY (chat_id, user_id)' },
  { id: 'messages_client_id_key', table: 'chat.messages', columns: ['chat_id', 'author_id', 'client_message_id'], unique: true, label: 'UNIQUE (chat_id, author_id, client_message_id)' },
  { id: 'idempotency_keys_pkey', table: 'platform.idempotency_keys', columns: ['scope', 'key'], unique: true, label: 'PRIMARY KEY (scope, key)' },
]

// Варианты ключа сообщений.
export type MessagesKey = 'chat-seq' | 'id-only' | 'seq-chat'

export const MESSAGES_KEYS: Readonly<Record<MessagesKey, { title: string; indexes: readonly IndexDef[] }>> = {
  // Схема мессенджера: лента — один диапазон первичного ключа, поиск по id — второй индекс.
  'chat-seq': {
    title: 'PK (chat_id, seq) + UNIQUE (id)',
    indexes: [
      { id: 'messages_pkey', table: 'chat.messages', columns: ['chat_id', 'seq'], unique: true, label: 'PRIMARY KEY (chat_id, seq)' },
      { id: 'messages_id_key', table: 'chat.messages', columns: ['id'], unique: true, label: 'UNIQUE (id)' },
    ],
  },
  // «ID для всего»: первичный ключ — только id, ленту обслуживает лишь индекс по chat_id без seq.
  'id-only': {
    title: 'PK (id) + INDEX (chat_id)',
    indexes: [
      { id: 'messages_pkey', table: 'chat.messages', columns: ['id'], unique: true, label: 'PRIMARY KEY (id)' },
      { id: 'messages_chat_id_idx', table: 'chat.messages', columns: ['chat_id'], unique: false, label: 'INDEX (chat_id)' },
    ],
  },
  // Те же столбцы в обратном порядке: префикс индекса — seq, а запрос задаёт chat_id.
  'seq-chat': {
    title: 'PK (seq, chat_id) + UNIQUE (id)',
    indexes: [
      { id: 'messages_pkey', table: 'chat.messages', columns: ['seq', 'chat_id'], unique: true, label: 'PRIMARY KEY (seq, chat_id)' },
      { id: 'messages_id_key', table: 'chat.messages', columns: ['id'], unique: true, label: 'UNIQUE (id)' },
    ],
  },
}

export const MEMBERS_USER_INDEX: IndexDef = { id: 'members_user_id_idx', table: 'chat.members', columns: ['user_id'], unique: false, label: 'INDEX (user_id)' }

export interface Schema {
  messagesKey: MessagesKey
  membersUserIndex: boolean
}

export const MESSENGER_SCHEMA: Schema = { messagesKey: 'chat-seq', membersUserIndex: true }

export function indexesOf(schema: Schema): IndexDef[] {
  return [...BASE_INDEXES, ...MESSAGES_KEYS[schema.messagesKey].indexes, ...(schema.membersUserIndex ? [MEMBERS_USER_INDEX] : [])]
}

// point — уникальный индекс, равенство по всем его столбцам; range — диапазон по префиксу, порядок
// дан индексом; range-sort — диапазон, но сортировать придётся отдельно; full — полный проход таблицы.
export type AccessKind = 'point' | 'range' | 'range-sort' | 'full'

export interface Plan {
  kind: AccessKind
  index: IndexDef | null
  prefix: readonly string[] // столбцы индекса, по которым идёт поиск
  rowsRead: number
  rowsReturned: number
}

function matching(table: TableDef, cols: readonly string[]): number {
  const product = cols.reduce((p, c) => p * (table.distinct[c] ?? 1), 1)
  return Math.max(1, table.rows / product)
}

export function planWith(q: QueryDef, index: IndexDef | null): Plan {
  const table = TABLES[q.table]
  const matched = matching(table, q.eq)
  const returned = Math.min(matched, q.limit ?? matched)
  if (index === null) return { kind: 'full', index: null, prefix: [], rowsRead: table.rows, rowsReturned: returned }
  const prefix: string[] = []
  for (const c of index.columns) {
    if (!q.eq.includes(c)) break
    prefix.push(c)
  }
  if (prefix.length === 0) return { kind: 'full', index: null, prefix: [], rowsRead: table.rows, rowsReturned: returned }
  // Строки под префиксом читаются все, если префикс покрыл не все условия или порядок не дан индексом.
  const underPrefix = matching(table, prefix)
  const allEq = prefix.length === q.eq.length
  if (allEq && index.unique && prefix.length === index.columns.length) return { kind: 'point', index, prefix, rowsRead: 1, rowsReturned: 1 }
  const ordered = q.orderBy === undefined || index.columns[prefix.length] === q.orderBy
  if (allEq && ordered) return { kind: 'range', index, prefix, rowsRead: returned, rowsReturned: returned }
  return { kind: q.orderBy !== undefined && !ordered ? 'range-sort' : 'range', index, prefix, rowsRead: underPrefix, rowsReturned: returned }
}

// Лучший план: меньше всего прочитанных строк, при равенстве — индекс с меньшим числом столбцов (он
// меньше на диске); без подходящего индекса — полный проход.
export function bestPlan(q: QueryDef, indexes: readonly IndexDef[]): Plan {
  let best = planWith(q, null)
  for (const ix of indexes.filter((i) => i.table === q.table)) {
    const p = planWith(q, ix)
    const narrower = p.rowsRead === best.rowsRead && best.index !== null && ix.columns.length < best.index.columns.length
    if (p.rowsRead < best.rowsRead || narrower) best = p
  }
  return best
}

export interface QueryResult {
  query: QueryDef
  plan: Plan
  withoutIndex: Plan | null // что будет, если удалить индекс из плана; null — индекса в плане нет
}

export function evaluate(q: QueryDef, schema: Schema): QueryResult {
  const indexes = indexesOf(schema)
  const plan = bestPlan(q, indexes)
  const withoutIndex = plan.index === null ? null : bestPlan(q, indexes.filter((i) => i.id !== plan.index!.id))
  return { query: q, plan, withoutIndex }
}

export interface AccessPatternsState {
  queryId: string
  schema: Schema
}

export type AccessPatternsAction =
  | { type: 'query'; queryId: string }
  | { type: 'messages-key'; key: MessagesKey }
  | { type: 'members-user-index'; on: boolean }
  | { type: 'reset' }

export interface AccessPatternsFrame {
  schema: Schema
  selected: QueryResult
  all: readonly QueryResult[]
  tableIndexes: readonly IndexDef[] // индексы таблицы выбранного запроса
}

function apply(s: AccessPatternsState, a: AccessPatternsAction): AccessPatternsState {
  switch (a.type) {
    case 'query':
      return QUERIES.some((q) => q.id === a.queryId) ? { ...s, queryId: a.queryId } : s
    case 'messages-key':
      return { ...s, schema: { ...s.schema, messagesKey: a.key } }
    case 'members-user-index':
      return { ...s, schema: { ...s.schema, membersUserIndex: a.on } }
    case 'reset':
      return start()
  }
}

function start(): AccessPatternsState {
  return { queryId: QUERIES[0]!.id, schema: MESSENGER_SCHEMA }
}

export function formatRows(n: number): string {
  return formatNumber(Math.round(n))
}

function planText(r: QueryResult): CaptionSegment[] {
  const p = r.plan
  const t = r.query.table
  const ix = p.index?.label ?? ''
  switch (p.kind) {
    case 'point':
      return [{ text: `Точечный поиск по ${ix} таблицы ${t}: спуск по ` }, term('индексу', 'index'), { text: ' на 3–4 страницы и одна строка.' }]
    case 'range':
      return [
        { text: `Диапазон по ${ix}: префикс (${p.prefix.join(', ')}) находит начало, дальше строки идут подряд${r.query.orderBy ? ' уже в нужном порядке' : ''}. Прочитано ${formatRows(p.rowsRead)} — столько, сколько нужно.` },
      ]
    case 'range-sort':
      return [{ text: `${ix} находит строки по (${p.prefix.join(', ')}), но порядка по ${r.query.orderBy} в нём нет: прочитать все ${formatRows(p.rowsRead)}, отсортировать и отдать ${formatRows(p.rowsReturned)}.` }]
    case 'full':
      return [{ text: `Подходящего индекса нет: полный проход (Seq Scan) по ${t} — все ${formatRows(p.rowsRead)} строк ради ${formatRows(p.rowsReturned)}.` }]
  }
}

function withoutText(r: QueryResult): CaptionSegment[] {
  const w = r.withoutIndex
  if (w === null) return []
  if (w.kind === 'full') return [{ text: ` Без ${r.plan.index!.label} — полный проход: ${formatRows(w.rowsRead)} строк.` }]
  return [{ text: ` Без ${r.plan.index!.label} запрос обслужит ${w.index!.label}: ${formatRows(w.rowsRead)} строк.` }]
}

function hint(s: AccessPatternsState, r: QueryResult): CaptionSegment[] {
  if (r.query.id === 'feed' && s.schema.messagesKey === 'seq-chat') {
    return [{ text: ' Порядок столбцов составного ключа решает: B-дерево ищет по префиксу, а первым в PK стоит seq, поэтому для «сообщений чата» он бесполезен (skip scan из PostgreSQL 18 перебирал бы сотню тысяч разных seq). Выручает случайный индекс с префиксом chat_id — без порядка по seq. В среднем чате это 100 строк, в чате-канале на миллион сообщений — миллион строк ради 50.' }]
  }
  if (r.query.id === 'feed' && s.schema.messagesKey === 'id-only') {
    return [{ text: ' Ключ «id для всего» не знает про seq: сообщения чата разбросаны по индексу, и для ' }, term('keyset-пагинации', 'keyset-pagination'), { text: ' их приходится собирать и сортировать.' }]
  }
  if (r.query.id === 'message-by-id' && s.schema.messagesKey !== 'id-only') {
    return [{ text: ' Это цена первичного ключа (chat_id, seq): поиск по id идёт через второй индекс, который занимает место и замедляет каждую вставку.' }]
  }
  if (r.query.id === 'chats-of-member' && !s.schema.membersUserIndex) {
    return [{ text: ' Первичный ключ (chat_id, user_id) начинается с chat_id, и по user_id он не помогает. Нужен отдельный индекс members(user_id).' }]
  }
  if (r.query.id === 'idempotency') return [{ text: ' ' }, term('Ключ идемпотентности', 'idempotency-key'), { text: ' ищется по первичному ключу таблицы — отдельный индекс не нужен.' }]
  return []
}

function caption(s: AccessPatternsState, r: QueryResult): Caption {
  return [term('Паттерн доступа', 'access-pattern'), { text: ` «${r.query.title}». ` }, ...planText(r), ...withoutText(r), ...hint(s, r)]
}

function frame(s: AccessPatternsState): AccessPatternsFrame {
  const all = QUERIES.map((q) => evaluate(q, s.schema))
  const selected = all.find((r) => r.query.id === s.queryId)!
  return { schema: s.schema, selected, all, tableIndexes: indexesOf(s.schema).filter((i) => i.table === selected.query.table) }
}

export const accessPatternsSimulation: Simulation<AccessPatternsState, AccessPatternsAction, AccessPatternsFrame> = {
  initial: start,
  apply,
  steps: (s): readonly Step<AccessPatternsFrame>[] => {
    const f = frame(s)
    return [{ state: f, caption: caption(s, f.selected) }]
  },
}
