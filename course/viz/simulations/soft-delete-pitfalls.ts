import { term } from '../core/caption'
import type { Step } from '../core/types'

// Три ловушки мягкого удаления (deleted_at) на таблице users и её соседях. Имена, SQL и ответы базы —
// из labs/2a/history.sql (вывод — labs/2a/results/history.txt), столбцы сжаты ради ширины экрана.
// Не из лабы только последний шаг варианта «ссылки»: это шаблон процедуры мягкого удаления ($1), а не
// команда, которую лаба выполняет.

export type SoftDeleteVariant = 'unique' | 'references' | 'erasure'

// Как база держит уникальность имени: обычным ограничением или частичным индексом по живым строкам.
export type UsernameRule = 'unique' | 'partial'

export interface SdUser {
  id: number
  username: string
  displayName: string
  deleted: boolean
}

// Строка, которая ссылается на пользователя: сессия (ON DELETE CASCADE) или сообщение (NO ACTION).
export interface SdRef {
  table: 'sessions' | 'messages'
  key: string
  userId: number
}

// Строка журнала аудита alice: имя в старой и новой версии строки (null — версии нет или поле убрано
// при чистке).
export interface SdAudit {
  id: number
  op: 'INSERT' | 'UPDATE'
  oldName: string | null
  newName: string | null
}

export type SdOutcome = 'ok' | 'error' | 'rows'

export interface SdFrame {
  variant: SoftDeleteVariant
  rule: UsernameRule
  // Есть ли представление active_users — одно место, где записано «живой».
  view: boolean
  users: readonly SdUser[]
  refs: readonly SdRef[]
  // null — у варианта нет журнала аудита.
  audit: readonly SdAudit[] | null
  sql: string | null
  outcome: SdOutcome | null
  result: string | null
  sqlstate: string | null
  // Строки, в которых видна проблема этого шага: id пользователей и ключи ссылок.
  flagUsers: readonly number[]
  flagRefs: readonly string[]
  flagAudit: readonly number[]
  verdict: 'pending' | 'broken' | 'fixed'
}

export const UNIQUE_VIOLATION = '23505'
export const FK_VIOLATION = '23503'
export const PLAIN_KEY = 'users_username_key'
export const PARTIAL_KEY = 'users_username_active_key'
export const ERASED_NAME = 'alice'

const SOFT_T = term('мягкое удаление (soft delete)', 'soft-delete')
const SOFT_IN = term('Мягкое удаление (soft delete)', 'soft-delete')
const PARTIAL_T = term('частичный индекс (partial index)', 'partial-index')
const FK_T = term('внешний ключ (foreign key)', 'foreign-key')
const FK_IN = term('Внешний ключ (foreign key)', 'foreign-key')
const AUDIT_T = term('журнал аудита (audit log)', 'audit-log')
const ERASURE_T = term('право на удаление (right to erasure)', 'right-to-erasure')
const CONSTRAINT_T = term('ограничение (constraint)', 'constraint')
const TRIGGER_T = term('триггер (trigger)', 'db-trigger')

const user = (id: number, username: string, displayName: string, deleted = false): SdUser => ({ id, username, displayName, deleted })
const ALICE = user(1, 'alice', 'Алиса')
const ALICE_DELETED = { ...ALICE, deleted: true }
const BOB = user(2, 'bob', 'Боб')
const ALICE_NEW = user(4, 'alice', 'Другая Алиса')
const ALICE_ERASED = user(1, 'deleted_1', 'Удалённый пользователь', true)

const SESSION: SdRef = { table: 'sessions', key: 'tok-alice', userId: 1 }
const MSG_ALICE: SdRef = { table: 'messages', key: 'привет', userId: 1 }
const MSG_BOB: SdRef = { table: 'messages', key: 'привет, Алиса', userId: 2 }
const MSG_LATE: SdRef = { table: 'messages', key: 'пишу после удаления', userId: 1 }

interface FrameInit {
  rule?: UsernameRule
  view?: boolean
  users: readonly SdUser[]
  refs?: readonly SdRef[]
  audit?: readonly SdAudit[] | null
  sql?: string | null
  outcome?: SdOutcome | null
  result?: string | null
  sqlstate?: string | null
  flagUsers?: readonly number[]
  flagRefs?: readonly string[]
  flagAudit?: readonly number[]
  verdict?: SdFrame['verdict']
}

const frame = (variant: SoftDeleteVariant, f: FrameInit): SdFrame => ({
  variant,
  rule: f.rule ?? 'unique',
  view: f.view ?? false,
  users: f.users,
  refs: f.refs ?? [],
  audit: f.audit ?? null,
  sql: f.sql ?? null,
  outcome: f.outcome ?? null,
  result: f.result ?? null,
  sqlstate: f.sqlstate ?? null,
  flagUsers: f.flagUsers ?? [],
  flagRefs: f.flagRefs ?? [],
  flagAudit: f.flagAudit ?? [],
  verdict: f.verdict ?? 'pending',
})

const SOFT_DELETE_SQL = `UPDATE users SET deleted_at = now() WHERE username = 'alice'`
const REGISTER_SQL = `INSERT INTO users (username, display_name) VALUES ('alice', 'Другая Алиса')`
const dupError = (key: string): string => `ERROR: duplicate key value violates unique constraint "${key}" (SQLSTATE ${UNIQUE_VIOLATION})`

const u = (f: FrameInit): SdFrame => frame('unique', f)

const UNIQUE: readonly Step<SdFrame>[] = [
  {
    state: u({ users: [ALICE, BOB] }),
    caption: [
      { text: 'Таблица users с обычным ' },
      CONSTRAINT_T,
      { text: ' UNIQUE (username) и столбцом deleted_at: NULL — пользователь жив, время — когда его «удалили». Это ' },
      SOFT_T,
      { text: ': строку не стирают, а помечают, и все запросы должны её пропускать.' },
    ],
  },
  {
    state: u({ users: [ALICE_DELETED, BOB], sql: SOFT_DELETE_SQL, outcome: 'ok', result: 'UPDATE 1', flagUsers: [] }),
    caption: 'alice удаляет аккаунт. Для базы это обычный UPDATE: строка на месте, имя alice на месте, а в deleted_at появилось время. Строка перечёркнута только для нас — база не знает, что «удалена» значит «нет».',
  },
  {
    state: u({ users: [ALICE_DELETED, BOB], sql: REGISTER_SQL, outcome: 'error', result: dupError(PLAIN_KEY), sqlstate: UNIQUE_VIOLATION, flagUsers: [1], verdict: 'broken' }),
    caption: `Через год имя alice хочет занять другой человек. UNIQUE сравнивает со всеми строками таблицы, в том числе с «удалённой», и отвечает ${UNIQUE_VIOLATION}. Имя удалённого пользователя занято навсегда: регистрация отвечает «имя занято», хотя такого пользователя для всех остальных запросов нет.`,
  },
  {
    state: u({ rule: 'partial', users: [ALICE_DELETED, BOB], sql: `ALTER TABLE users DROP CONSTRAINT ${PLAIN_KEY};\nCREATE UNIQUE INDEX ${PARTIAL_KEY} ON users (username) WHERE deleted_at IS NULL`, outcome: 'ok', result: 'CREATE INDEX' }),
    caption: [
      { text: 'Исправление — уникальный ' },
      PARTIAL_T,
      { text: ': в нём лежат только строки, подходящие под WHERE deleted_at IS NULL, и уникальность проверяется только среди них. Строка удалённой alice из индекса выпала (урок 2.4 показал тот же приём на адресах почты).' },
    ],
  },
  {
    state: u({ rule: 'partial', users: [ALICE_DELETED, BOB, ALICE_NEW], sql: REGISTER_SQL, outcome: 'ok', result: 'INSERT 0 1' }),
    caption: 'Та же регистрация проходит: новая alice получает id 4 (номер 3 сгорел на неудачной вставке — identity не возвращает выданные значения). В таблице теперь две строки с именем alice, и это нормально: живая из них одна.',
  },
  {
    state: u({ rule: 'partial', users: [ALICE_DELETED, BOB, ALICE_NEW], sql: `INSERT INTO users (username, display_name) VALUES ('alice', 'Третья Алиса')`, outcome: 'error', result: dupError(PARTIAL_KEY), sqlstate: UNIQUE_VIOLATION, verdict: 'fixed' }),
    caption: `Вторая живая alice по-прежнему получает ${UNIQUE_VIOLATION} — правило «среди живых имя уникально» держит база. Обратите внимание на имя в ошибке: теперь это имя индекса ${PARTIAL_KEY}, и адаптер, который узнаёт ошибку по ConstraintName, нужно поменять вместе с миграцией.`,
  },
]

const r = (f: Omit<FrameInit, 'rule'>): SdFrame => frame('references', { rule: 'partial', ...f })
const REF_USERS = [ALICE_DELETED, BOB, ALICE_NEW]
const REF_BASE = [SESSION, MSG_ALICE, MSG_BOB]
const LOGIN_SQL = `SELECT id, display_name FROM users WHERE username = 'alice'`

const REFERENCES: readonly Step<SdFrame>[] = [
  {
    state: r({ users: REF_USERS, refs: REF_BASE }),
    caption: [
      { text: 'Продолжение: первая alice удалена мягко, новая зарегистрирована. Рядом две таблицы со ссылками на users: sessions (' },
      FK_T,
      { text: ' с ON DELETE CASCADE — сессии без пользователя не нужны) и messages (NO ACTION — сообщения переживают автора).' },
    ],
  },
  {
    state: r({ users: REF_USERS, refs: REF_BASE, sql: LOGIN_SQL, outcome: 'rows', result: '2 строки: id 1 (удалена) и id 4', flagUsers: [1, 4], verdict: 'broken' }),
    caption: 'Вход по имени написан до появления deleted_at — или его просто забыли поправить. Запрос находит обе строки alice. Какую из них считать «той», он не знает: в лучшем случае ошибка «найдено больше одной строки», в худшем — вход в аккаунт удалённого пользователя. Условие deleted_at IS NULL должно быть в каждом запросе, и забыть его можно в каждом.',
  },
  {
    state: r({ users: REF_USERS, refs: REF_BASE, sql: 'SELECT token, user_id FROM sessions', outcome: 'rows', result: '1 строка: tok-alice → user_id 1', flagRefs: ['tok-alice'], verdict: 'broken' }),
    caption: 'Сессия удалённой alice жива. ON DELETE CASCADE срабатывает на DELETE, а мягкое удаление — UPDATE: каскад не сработал, и токен по-прежнему пускает в аккаунт. Всё, что база делала бы при настоящем удалении сама, теперь должен сделать код — в той же транзакции, что и UPDATE deleted_at.',
  },
  {
    state: r({ users: REF_USERS, refs: [...REF_BASE, MSG_LATE], sql: `INSERT INTO messages (author_id, text) VALUES (1, 'пишу после удаления')`, outcome: 'ok', result: 'INSERT 0 1', flagRefs: ['пишу после удаления'], verdict: 'broken' }),
    caption: [
      FK_IN,
      { text: ' проверяет, что строка users с id 1 есть, а не что она жива: сообщение от удалённой alice принято без ошибки. Условие «автор не удалён» в FOREIGN KEY не записать — его держит только код (или триггер).' },
    ],
  },
  {
    state: r({ view: true, users: REF_USERS, refs: [...REF_BASE, MSG_LATE], sql: `CREATE VIEW active_users AS\n  SELECT id, username, display_name FROM users WHERE deleted_at IS NULL;\nSELECT id, display_name FROM active_users WHERE username = 'alice'`, outcome: 'rows', result: '1 строка: id 4 · план: Index Scan using users_username_active_key', flagUsers: [] }),
    caption: 'Защита от забытого условия — представление active_users: «живой» записан в одном месте, и запросы приложения читают из него. Бонус: план идёт по частичному индексу. Запрос к users без deleted_at IS NULL этот индекс использовать не может — лаба на 100 000 пользователей показывает для него Seq Scan, полный просмотр.',
  },
  {
    state: r({ view: true, users: REF_USERS, refs: [MSG_ALICE, MSG_BOB, MSG_LATE], sql: `-- процедура мягкого удаления пользователя $1\nBEGIN;\nUPDATE users SET deleted_at = now() WHERE id = $1;\nDELETE FROM sessions WHERE user_id = $1;\nCOMMIT`, outcome: 'ok', result: 'COMMIT', verdict: 'fixed' }),
    caption: 'Работа каскада переходит в код: мягкое удаление должно быть одной транзакцией, которая ставит deleted_at и сама удаляет сессии, — так его и надо было писать с самого начала. Сообщение «пишу после удаления» база уже не поймает: от него защищает только проверка в use case (автор жив) перед вставкой. Каждая из этих мер — правило, которое раньше держала база, а теперь держит дисциплина.',
  },
]

const e = (f: Omit<FrameInit, 'rule'>): SdFrame => frame('erasure', { rule: 'partial', ...f })
// История alice в журнале: вставка и мягкое удаление; номера — порядок строк, а не id из лабы.
const AUDIT_BEFORE: readonly SdAudit[] = [
  { id: 1, op: 'INSERT', oldName: null, newName: ERASED_NAME },
  { id: 2, op: 'UPDATE', oldName: ERASED_NAME, newName: ERASED_NAME },
]
const AUDIT_ERASED: readonly SdAudit[] = [...AUDIT_BEFORE, { id: 3, op: 'UPDATE', oldName: ERASED_NAME, newName: 'deleted_1' }]
const AUDIT_SCRUBBED: readonly SdAudit[] = AUDIT_ERASED.map((a) => ({ ...a, oldName: null, newName: null }))
const ERASE_USERS = [ALICE_DELETED, BOB, ALICE_NEW]
const ERASE_REFS = [SESSION, MSG_ALICE, MSG_BOB, MSG_LATE]
const ERASED_USERS = [ALICE_ERASED, BOB, ALICE_NEW]
const ERASED_REFS = [MSG_ALICE, MSG_BOB, MSG_LATE]

const ERASURE: readonly Step<SdFrame>[] = [
  {
    state: e({ users: ERASE_USERS, refs: ERASE_REFS, audit: AUDIT_BEFORE }),
    caption: [
      { text: 'Мягко удалённая alice (id 1) просит стереть её данные — это ' },
      ERASURE_T,
      { text: ' из GDPR и похожих законов. Смотреть придётся не только в users: на таблице висит ' },
      TRIGGER_T,
      { text: ', который пишет каждую правку строки в ' },
      AUDIT_T,
      { text: ' users_audit — старую и новую версию целиком, в JSONB. В истории alice уже две строки: вставка и мягкое удаление.' },
    ],
  },
  {
    state: e({ users: ERASE_USERS, refs: ERASE_REFS, audit: AUDIT_BEFORE, sql: `SELECT username, display_name, deleted_at IS NOT NULL AS deleted\nFROM users WHERE id = 1`, outcome: 'rows', result: '1 строка: alice | Алиса | deleted = t', flagUsers: [1], flagAudit: [1, 2], verdict: 'broken' }),
    caption: [
      SOFT_IN,
      { text: ' ничего не стёрло: оно прячет строку от запросов, но имя и отображаемое имя лежат в таблице, в журнале, в каждой реплике и резервной копии. Для закона «удалить» значит удалить, а не перестать показывать.' },
    ],
  },
  {
    state: e({ users: ERASE_USERS, refs: ERASE_REFS, audit: AUDIT_BEFORE, sql: 'DELETE FROM users WHERE id = 1', outcome: 'error', result: `ERROR: … violates foreign key constraint "messages_author_id_fkey" … Key (id)=(1) is still referenced from table "messages" (SQLSTATE ${FK_VIOLATION})`, sqlstate: FK_VIOLATION, flagRefs: ['привет', 'пишу после удаления'], verdict: 'broken' }),
    caption: `Настоящий DELETE упирается в сообщения alice: внешний ключ messages → users без каскада, и база отвечает ${FK_VIOLATION}. Стирать сообщения из чатов собеседников нельзя — это и их переписка. Перенос строки в архивную таблицу упрётся в тот же ключ: в лабе так не переносится bob.`,
  },
  {
    state: e({ users: ERASED_USERS, refs: ERASED_REFS, audit: AUDIT_ERASED, sql: `BEGIN;\nUPDATE users SET username = 'deleted_' || id,\n  display_name = 'Удалённый пользователь' WHERE id = 1;\nDELETE FROM sessions WHERE user_id = 1;\nCOMMIT`, outcome: 'ok', result: 'COMMIT', flagAudit: [1, 2, 3], verdict: 'broken' }),
    caption: 'Рабочий путь — обезличить: строка остаётся ради ссылок, а персональных данных в ней нет; сообщения показываются от deleted_1, сессия удалена явно. Но триггер аудита записал и эту правку — вместе со старой версией строки. В users имени alice больше нет, а в журнале оно во всех трёх строках её истории.',
  },
  {
    state: e({ users: ERASED_USERS, refs: ERASED_REFS, audit: AUDIT_SCRUBBED, sql: `UPDATE users_audit\nSET old_row = old_row - ARRAY['username', 'display_name'],\n    new_row = new_row - ARRAY['username', 'display_name']\nWHERE user_id = 1`, outcome: 'ok', result: 'UPDATE 3', verdict: 'fixed' }),
    caption: 'Процедура удаления проходит и по журналу: из строк истории убираются поля с персональными данными, а сами строки — кто, когда, какая операция — остаются. Это правка журнала, который приложение только дописывает, поэтому её выполняет отдельная роль базы. Резервные копии чистятся сроком хранения: удалённое уходит из них, когда истекает срок самой старой копии.',
  },
]

export function softDeleteScenario(variant: SoftDeleteVariant = 'unique'): readonly Step<SdFrame>[] {
  if (variant === 'references') return REFERENCES
  if (variant === 'erasure') return ERASURE
  return UNIQUE
}
