import type { Caption, CaptionSegment } from '../core/caption'
import { term } from '../core/caption'
import { plural } from '../core/format'
import type { Simulation, Step } from '../core/types'

// Песочница урока 2.2: одни и те же факты мессенджера в двух схемах. «Одна большая таблица»
// messages_flat хранит сообщение вместе с именем автора и названием чата (как в лабе
// labs/2a/anomalies.sql — те же строки), 3НФ-разложение — users, chats, messages. Каждое действие
// применяется к обеим схемам сразу, поэтому переключатель показывает цену одного и того же
// изменения в каждой. Правда о мире — 3НФ: по ней видно, какие факты «одна большая таблица» потеряла.

export type Schema = 'flat' | 'nf3'

export interface FlatRow {
  messageId: number
  chatId: number
  chatTitle: string
  authorId: number
  authorName: string
  text: string
}

export interface User {
  id: number
  name: string
}

export interface Chat {
  id: number
  title: string
}

export interface Message {
  id: number
  chatId: number
  authorId: number
  text: string
}

// Три аномалии классической теории (Кодд; Hernandez, гл. 3): обновления, удаления, вставки.
export type AnomalyKind = 'update' | 'delete' | 'insert'

export const ANOMALY_TITLE: Record<AnomalyKind, string> = {
  update: 'Аномалия обновления',
  delete: 'Аномалия удаления',
  insert: 'Аномалия вставки',
}

export const ANOMALY_EFFECT: Record<AnomalyKind, string> = {
  update: 'несогласованность: у одного автора два имени',
  delete: 'потеря факта: вместе с сообщением пропало имя автора',
  insert: 'невозможность вставки: пользователя без сообщения хранить негде',
}

export type TableName = 'messages_flat' | 'users' | 'messages'

export interface RowRef {
  table: TableName
  key: number
}

// Итог одного действия в одной схеме.
export interface Outcome {
  changed: readonly RowRef[] // вставленные и изменённые строки
  removed: readonly RowRef[] // удалённые строки
  rejected: boolean // действие выполнить нельзя
  anomaly: AnomalyKind | null
}

export type ActionType = 'message' | 'register' | 'rename' | 'delete-last'

export interface LastAction {
  type: ActionType
  authorId: number
  flat: Outcome
  nf3: Outcome
}

// Почему факта об авторе нет в messages_flat: его не удалось вставить или он ушёл вместе с последним сообщением.
export type MissingReason = 'never-inserted' | 'deleted'

export interface NormalFormsState {
  schema: Schema
  authorId: number // автор, к которому применяются действия
  partial: boolean // переименование «по одной строке», как код, который правит только то, что показывает
  flat: readonly FlatRow[]
  users: readonly User[]
  chats: readonly Chat[]
  messages: readonly Message[]
  missing: Readonly<Record<number, MissingReason>>
  nextMessageId: number
  nextUserId: number
  renames: Readonly<Record<number, number>> // сколько раз переименовывали автора — для нового имени
  last: LastAction | null
}

export type NormalFormsAction =
  | { type: 'schema'; schema: Schema }
  | { type: 'author'; authorId: number }
  | { type: 'partial'; partial: boolean }
  | { type: 'message' }
  | { type: 'register' }
  | { type: 'rename' }
  | { type: 'delete-last' }
  | { type: 'reset' }

export interface NormalFormsFrame {
  schema: Schema
  authorId: number
  partial: boolean
  flat: readonly FlatRow[]
  users: readonly User[]
  chats: readonly Chat[]
  messages: readonly Message[]
  missing: Readonly<Record<number, MissingReason>>
  inconsistent: readonly number[] // author_id, у которых в messages_flat больше одного имени
  last: LastAction | null
}

// Те же данные, что в labs/2a/anomalies.sql: alice пишет трижды в двух чатах, bob — один раз.
const USERS: readonly User[] = [
  { id: 100, name: 'alice' },
  { id: 101, name: 'bob' },
]
const CHATS: readonly Chat[] = [
  { id: 10, title: 'Бэкенд' },
  { id: 20, title: 'Обед' },
]
const MESSAGES: readonly Message[] = [
  { id: 1, chatId: 10, authorId: 100, text: 'Кто смотрел ревью?' },
  { id: 2, chatId: 10, authorId: 101, text: 'Я, сейчас допишу' },
  { id: 3, chatId: 10, authorId: 100, text: 'Спасибо!' },
  { id: 4, chatId: 20, authorId: 100, text: 'Идём в 13:00?' },
]
// Новые пользователи — по очереди; после списка имена нумеруются.
const NEW_NAMES = ['carol', 'dan', 'erin', 'frank'] as const
const NEW_TEXTS = ['Привет!', 'Сейчас посмотрю', 'Готово', 'Ок', 'Созвон в 15:00'] as const
// Новые сообщения уходят в первый чат: какой именно — для аномалий не важно.
const DEFAULT_CHAT_ID = 10
const FIRST_NEW_USER_ID = 102

function flatten(messages: readonly Message[], users: readonly User[], chats: readonly Chat[]): FlatRow[] {
  return messages.map((m) => ({
    messageId: m.id,
    chatId: m.chatId,
    chatTitle: chats.find((c) => c.id === m.chatId)!.title,
    authorId: m.authorId,
    authorName: users.find((u) => u.id === m.authorId)!.name,
    text: m.text,
  }))
}

function start(): NormalFormsState {
  return {
    schema: 'flat',
    authorId: 100,
    partial: false,
    flat: flatten(MESSAGES, USERS, CHATS),
    users: USERS,
    chats: CHATS,
    messages: MESSAGES,
    missing: {},
    nextMessageId: MESSAGES.length + 1,
    nextUserId: FIRST_NEW_USER_ID,
    renames: {},
    last: null,
  }
}

const ok = (changed: RowRef[], removed: RowRef[] = []): Outcome => ({ changed, removed, rejected: false, anomaly: null })

// Функциональная зависимость author_id → author_name нарушена там, где у одного author_id несколько имён.
export function inconsistentAuthors(flat: readonly FlatRow[]): number[] {
  const names = new Map<number, Set<string>>()
  for (const r of flat) names.set(r.authorId, (names.get(r.authorId) ?? new Set()).add(r.authorName))
  return [...names].filter(([, set]) => set.size > 1).map(([id]) => id)
}

function withoutKey<V>(rec: Readonly<Record<number, V>>, key: number): Record<number, V> {
  const copy = { ...rec }
  delete copy[key]
  return copy
}

function sendMessage(s: NormalFormsState): NormalFormsState {
  const author = s.users.find((u) => u.id === s.authorId)!
  const chat = s.chats.find((c) => c.id === DEFAULT_CHAT_ID)!
  const id = s.nextMessageId
  const text = NEW_TEXTS[(id - 1) % NEW_TEXTS.length]!
  // В «одной большой таблице» имя автора и название чата копируются в новую строку: ещё одна копия факта.
  const row: FlatRow = { messageId: id, chatId: chat.id, chatTitle: chat.title, authorId: author.id, authorName: author.name, text }
  const ref = (table: TableName): RowRef[] => [{ table, key: id }]
  return {
    ...s,
    flat: [...s.flat, row],
    messages: [...s.messages, { id, chatId: chat.id, authorId: author.id, text }],
    missing: withoutKey(s.missing, author.id),
    nextMessageId: id + 1,
    last: { type: 'message', authorId: author.id, flat: ok(ref('messages_flat')), nf3: ok(ref('messages')) },
  }
}

function register(s: NormalFormsState): NormalFormsState {
  const id = s.nextUserId
  const index = id - FIRST_NEW_USER_ID
  const name = NEW_NAMES[index] ?? `user${index + 1}`
  // В messages_flat строка — это сообщение: без message_id, чата и текста её не вставить (NOT NULL, PRIMARY KEY).
  const flat: Outcome = { changed: [], removed: [], rejected: true, anomaly: 'insert' }
  return {
    ...s,
    users: [...s.users, { id, name }],
    missing: { ...s.missing, [id]: 'never-inserted' },
    nextUserId: id + 1,
    authorId: id,
    last: { type: 'register', authorId: id, flat, nf3: ok([{ table: 'users', key: id }]) },
  }
}

function rename(s: NormalFormsState): NormalFormsState {
  const n = (s.renames[s.authorId] ?? 0) + 1
  const user = s.users.find((u) => u.id === s.authorId)!
  const base = user.name.replace(/_v\d+$/, '')
  const name = `${base}_v${n + 1}`
  const rows = s.flat.filter((r) => r.authorId === s.authorId)
  // Частичная правка трогает только первую строку автора — как UPDATE … WHERE message_id = 1 в лабе.
  const targets = s.partial ? rows.slice(0, 1) : rows
  const ids = new Set(targets.map((r) => r.messageId))
  const flat = s.flat.map((r) => (ids.has(r.messageId) ? { ...r, authorName: name } : r))
  const broken = inconsistentAuthors(flat).includes(s.authorId)
  const flatOutcome: Outcome = {
    changed: targets.map((r) => ({ table: 'messages_flat', key: r.messageId })),
    removed: [],
    rejected: rows.length === 0,
    anomaly: broken ? 'update' : null,
  }
  return {
    ...s,
    flat,
    users: s.users.map((u) => (u.id === s.authorId ? { ...u, name } : u)),
    renames: { ...s.renames, [s.authorId]: n },
    last: { type: 'rename', authorId: s.authorId, flat: flatOutcome, nf3: ok([{ table: 'users', key: s.authorId }]) },
  }
}

const lastOf = <T>(items: readonly T[], match: (t: T) => boolean): T | undefined => [...items].reverse().find(match)

function deleteLast(s: NormalFormsState): NormalFormsState {
  const nfVictim = lastOf(s.messages, (m) => m.authorId === s.authorId)
  const flatVictim = lastOf(s.flat, (r) => r.authorId === s.authorId)
  const flat = flatVictim === undefined ? s.flat : s.flat.filter((r) => r.messageId !== flatVictim.messageId)
  const lost = flatVictim !== undefined && !flat.some((r) => r.authorId === s.authorId)
  const flatOutcome: Outcome =
    flatVictim === undefined
      ? { changed: [], removed: [], rejected: true, anomaly: null }
      : { changed: [], removed: [{ table: 'messages_flat', key: flatVictim.messageId }], rejected: false, anomaly: lost ? 'delete' : null }
  const nfOutcome: Outcome =
    nfVictim === undefined ? { changed: [], removed: [], rejected: true, anomaly: null } : ok([], [{ table: 'messages', key: nfVictim.id }])
  return {
    ...s,
    flat,
    messages: nfVictim === undefined ? s.messages : s.messages.filter((m) => m.id !== nfVictim.id),
    missing: lost ? { ...s.missing, [s.authorId]: 'deleted' } : s.missing,
    last: { type: 'delete-last', authorId: s.authorId, flat: flatOutcome, nf3: nfOutcome },
  }
}

function apply(s: NormalFormsState, a: NormalFormsAction): NormalFormsState {
  switch (a.type) {
    case 'schema':
      return { ...s, schema: a.schema }
    case 'author':
      return s.users.some((u) => u.id === a.authorId) ? { ...s, authorId: a.authorId } : s
    case 'partial':
      return { ...s, partial: a.partial }
    case 'message':
      return sendMessage(s)
    case 'register':
      return register(s)
    case 'rename':
      return rename(s)
    case 'delete-last':
      return deleteLast(s)
    case 'reset':
      return start()
  }
}

const rows = (n: number): string => `${n} ${plural(n, 'строка', 'строки', 'строк')}`

function nameOf(s: NormalFormsState, id: number): string {
  return s.users.find((u) => u.id === id)?.name ?? String(id)
}

function flatCaption(s: NormalFormsState, last: LastAction): CaptionSegment[] {
  const who = nameOf(s, last.authorId)
  const o = last.flat
  switch (last.type) {
    case 'message':
      return [{ text: `Новое сообщение ${who} — новая строка, и в неё снова скопированы имя автора и название чата. Копий одного факта стало больше: каждую придётся менять при переименовании.` }]
    case 'register':
      return [
        { text: `${ANOMALY_TITLE.insert}: ${who} зарегистрировался, но строка таблицы — это сообщение, и без message_id, чата и текста её не вставить. Факт «пользователь ${who} существует» хранить негде, пока он не напишет. ` },
        term('Нормальная форма', 'normal-form'),
        { text: ' лечит это отдельной таблицей пользователей.' },
      ]
    case 'rename':
      if (o.rejected) return [{ text: `Строк автора ${last.authorId} в таблице нет — менять нечего: имя уже потеряно или так и не было записано (см. «нет в таблице»).` }]
      if (o.anomaly === 'update') {
        return [
          { text: `${ANOMALY_TITLE.update}: обновлена ${rows(o.changed.length)} из нескольких, и у автора ${last.authorId} теперь два имени. Нарушена ` },
          term('функциональная зависимость', 'functional-dependency'),
          { text: ' author_id → author_name: какое имя верное, таблица уже не знает.' },
        ]
      }
      return [{ text: `Переименование ${who} изменило ${rows(o.changed.length)} — по одной на каждое его сообщение. Чем больше он писал, тем дороже правка и тем легче забыть одну из строк (включите «править одну строку»).` }]
    case 'delete-last':
      if (o.rejected) return [{ text: `У ${who} в этой таблице нет сообщений — удалять нечего.` }]
      if (o.anomaly === 'delete') {
        return [{ text: `${ANOMALY_TITLE.delete}: это было последнее сообщение ${who}, и вместе с ним исчезла единственная запись о том, что пользователь ${last.authorId} зовётся «${who}».` }]
      }
      return [{ text: `Удалено одно сообщение ${who}; другие его строки ещё хранят имя, так что факт пока не потерян.` }]
  }
}

function nf3Caption(s: NormalFormsState, last: LastAction): CaptionSegment[] {
  const who = nameOf(s, last.authorId)
  switch (last.type) {
    case 'message':
      return [{ text: `Новое сообщение ${who} — одна строка в messages: только ссылки author_id и chat_id, имя и название не копируются.` }]
    case 'register':
      return [{ text: `${who} — одна строка в users. Сообщений у него нет, и это никому не мешает: каждый факт хранится в своей таблице.` }]
    case 'rename':
      return [
        { text: `Переименование ${who} — ровно одна строка в users, сколько бы сообщений он ни написал. Противоречию взяться неоткуда: имя хранится в одном месте. Это и есть 3НФ: каждая ` },
        term('функциональная зависимость', 'functional-dependency'),
        { text: ' — от ключа своей таблицы.' },
      ]
    case 'delete-last':
      if (last.nf3.rejected) return [{ text: `У ${who} нет сообщений — удалять нечего, а сам пользователь на месте.` }]
      return [{ text: `Удалена одна строка messages. Пользователь ${who} остаётся в users: удаление сообщения не трогает факт о человеке.` }]
  }
}

function caption(s: NormalFormsState): Caption {
  if (s.last === null) {
    return s.schema === 'flat'
      ? [
          { text: 'Одна большая таблица: имя автора и название чата повторяются в каждой строке. Выберите автора и действие и смотрите, какие строки меняются и где возникает аномалия. Тот же сценарий — в лабе labs/2a/anomalies.sql. ' },
          term('Нормальная форма', 'normal-form'),
          { text: ' — переключатель «3НФ».' },
        ]
      : [{ text: 'Та же информация в 3НФ: пользователи, чаты и сообщения — отдельные таблицы, сообщения ссылаются на них по id. Выполните те же действия и сравните число изменённых строк.' }]
  }
  return s.schema === 'flat' ? flatCaption(s, s.last) : nf3Caption(s, s.last)
}

function frame(s: NormalFormsState): NormalFormsFrame {
  return {
    schema: s.schema,
    authorId: s.authorId,
    partial: s.partial,
    flat: s.flat,
    users: s.users,
    chats: s.chats,
    messages: s.messages,
    missing: s.missing,
    inconsistent: inconsistentAuthors(s.flat),
    last: s.last,
  }
}

export const normalFormsSimulation: Simulation<NormalFormsState, NormalFormsAction, NormalFormsFrame> = {
  initial: start,
  apply,
  steps: (s): readonly Step<NormalFormsFrame>[] => [{ state: frame(s), caption: caption(s) }],
}
