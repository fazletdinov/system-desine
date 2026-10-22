import { term } from '../core/caption'
import type { Step } from '../core/types'

// Два экземпляра мессенджера одновременно регистрируют имя alice (POST /v1/users).
// Имена — из мессенджера и плана модуля 2a: ограничение users_username_key (явное имя в 00001_init.sql),
// user/domain.ErrUsernameTaken → userbridge → auth/domain.ErrUsernameTaken → problem username_taken → 409.

export type RaceVariant = 'code-check' | 'unique'
export type InstanceId = 'A' | 'B'
export type InstanceState = 'running' | 'waiting' | 'done' | 'failed'

export interface RaceInstance {
  id: InstanceId
  // Последний оператор SQL этого экземпляра и его результат.
  sql: string | null
  result: string | null
  state: InstanceState
  http: 201 | 409 | null
}

export interface RaceRow {
  id: string
  username: string
  by: InstanceId
  // false — строка вставлена в незафиксированной транзакции: её видит только сам вставивший.
  committed: boolean
}

export interface RaceFrame {
  variant: RaceVariant
  unique: boolean
  instances: readonly [RaceInstance, RaceInstance]
  rows: readonly RaceRow[]
  actor: InstanceId | null
  sqlstate: string | null
  // Сколько звеньев перевода ошибки уже пройдено (индекс в ERROR_CHAIN), -1 — ещё нет.
  chain: number
  verdict: 'pending' | 'broken' | 'held'
}

export const USERNAME = 'alice'
export const CONSTRAINT = 'users_username_key'
export const UNIQUE_VIOLATION = '23505'

// Путь ошибки от базы до клиента: каждое звено знает только соседей (порты, а не драйвер).
export const ERROR_CHAIN = [
  { layer: 'pgx', text: `*pgconn.PgError{Code: "${UNIQUE_VIOLATION}", ConstraintName: "${CONSTRAINT}"}` },
  { layer: 'user/adapters/postgres', text: 'user/domain.ErrUsernameTaken' },
  { layer: 'auth/adapters/userbridge', text: 'auth/domain.ErrUsernameTaken' },
  { layer: 'auth/adapters/httpapi', text: '409 application/problem+json, type …/problems/username_taken' },
] as const

const SELECT = `SELECT id FROM users.users WHERE username = $1`
// Остальные столбцы (display_name, created_at) опущены ради ширины экрана.
const INSERT = `INSERT INTO users.users (id, username, …) VALUES ($1, $2, …)`

const CONSTRAINT_T = term('ограничение (constraint)', 'constraint')
const TX = term('транзакции (transaction)', 'transaction')
const INDEX = term('индексом (index)', 'index')
const PROBLEM = term('problem details', 'problem-details')
const CONSTRAINT_IN = term('Ограничения (constraint)', 'constraint')

const inst = (id: InstanceId, state: InstanceState, sql: string | null = null, result: string | null = null, http: 201 | 409 | null = null): RaceInstance => ({ id, sql, result, state, http })
const row = (id: string, by: InstanceId, committed = true): RaceRow => ({ id, username: USERNAME, by, committed })

const frame = (variant: RaceVariant, a: RaceInstance, b: RaceInstance, rows: RaceRow[], extra: Partial<Pick<RaceFrame, 'actor' | 'sqlstate' | 'chain' | 'verdict'>> = {}): RaceFrame => ({
  variant,
  unique: variant === 'unique',
  instances: [a, b],
  rows,
  actor: extra.actor ?? null,
  sqlstate: extra.sqlstate ?? null,
  chain: extra.chain ?? -1,
  verdict: extra.verdict ?? 'pending',
})

const ROW_A = '0192…a1'
const ROW_B = '0192…b7'

const cc = (a: RaceInstance, b: RaceInstance, rows: RaceRow[], extra: Parameters<typeof frame>[4] = {}): RaceFrame => frame('code-check', a, b, rows, extra)

const CODE_CHECK: readonly Step<RaceFrame>[] = [
  {
    state: cc(inst('A', 'running', null, 'POST /v1/users {"username":"alice"}'), inst('B', 'running', null, 'POST /v1/users {"username":"alice"}'), []),
    caption: [
      { text: 'Два экземпляра сервиса за балансировщиком почти одновременно получили регистрацию с одним именем alice. Уникальность имени здесь проверяет код: «сначала SELECT, нашли — ErrUsernameTaken, не нашли — INSERT». ' },
      CONSTRAINT_IN,
      { text: ' UNIQUE на столбце username нет. В in-memory хранилище из модуля 1 проверка и запись шли под одним мьютексом, но мьютекс живёт в памяти одного процесса, а экземпляров два.' },
    ],
  },
  {
    state: cc(inst('A', 'running', SELECT, '0 строк: имя свободно'), inst('B', 'running', null, 'POST /v1/users {"username":"alice"}'), [], { actor: 'A' }),
    caption: 'Экземпляр A проверяет имя. Строк нет — с точки зрения A имя свободно, и это правда в момент проверки.',
  },
  {
    state: cc(inst('A', 'running', SELECT, '0 строк: имя свободно'), inst('B', 'running', SELECT, '0 строк: имя свободно'), [], { actor: 'B' }),
    caption: [
      { text: 'Через миллисекунду ту же проверку делает B. A ещё не вставил строку, поэтому B тоже видит «свободно». Это гонка (race condition) вида «проверь, потом действуй» (check-then-act): между проверкой и записью есть окно, и чужая запись в него помещается. Уровень изоляции REPEATABLE READ не спасает: снимок B тоже не содержит строки A. Поймал бы только SERIALIZABLE, ценой повторов транзакций (модуль 2b).' },
    ],
  },
  {
    state: cc(inst('A', 'done', INSERT, 'INSERT 0 1', 201), inst('B', 'running', SELECT, '0 строк: имя свободно'), [row(ROW_A, 'A')], { actor: 'A' }),
    caption: 'A вставляет пользователя и фиксирует транзакцию, клиент A получает 201 Created. Пока всё выглядит правильно.',
  },
  {
    state: cc(inst('A', 'done', INSERT, 'INSERT 0 1', 201), inst('B', 'done', INSERT, 'INSERT 0 1', 201), [row(ROW_A, 'A'), row(ROW_B, 'B')], { actor: 'B' }),
    caption: 'B вставляет свою строку. Базе нечего возразить: правило «имя уникально» живёт только в коде, а код B своё уже проверил. Клиент B тоже получает 201 Created.',
  },
  {
    state: cc(inst('A', 'done', INSERT, 'INSERT 0 1', 201), inst('B', 'done', INSERT, 'INSERT 0 1', 201), [row(ROW_A, 'A'), row(ROW_B, 'B')], { verdict: 'broken' }),
    caption: 'Итог: два пользователя с именем alice, и никто об этом не узнал — ни ошибки, ни строки в журнале. Вход по имени теперь найдёт одного из двух, второй не войдёт никогда. Блокировка в коде (распределённый мьютекс) закрыла бы окно только для этого сервиса, но не для ручного SQL, скрипта миграции или второго приложения. Правило, которое должно держаться при любом писателе, место в базе.',
  },
]

const uq = (a: RaceInstance, b: RaceInstance, rows: RaceRow[], extra: Parameters<typeof frame>[4] = {}): RaceFrame => frame('unique', a, b, rows, extra)
const A_DONE = inst('A', 'done', 'COMMIT', 'COMMIT', 201)
const B_FAILED = inst('B', 'failed', INSERT, `ERROR: duplicate key value violates unique constraint "${CONSTRAINT}" (SQLSTATE ${UNIQUE_VIOLATION})`)
const B_409 = { ...B_FAILED, http: 409 as const }

const UNIQUE: readonly Step<RaceFrame>[] = [
  {
    state: uq(inst('A', 'running', null, 'POST /v1/users {"username":"alice"}'), inst('B', 'running', null, 'POST /v1/users {"username":"alice"}'), []),
    caption: [
      { text: 'Те же два запроса, но в миграции у таблицы есть ' },
      CONSTRAINT_T,
      { text: ` UNIQUE (username) с явным именем ${CONSTRAINT}. База поддерживает его уникальным ` },
      INDEX,
      { text: '. Предварительного SELECT нет: формат имени проверяет домен, занятость — только база. Регистрация идёт в одной ' },
      TX,
      { text: ': пользователь и учётные данные записываются вместе (порт Transactor).' },
    ],
  },
  {
    state: uq(inst('A', 'running', `BEGIN; ${INSERT}`, 'INSERT 0 1, транзакция открыта'), inst('B', 'running', null, 'POST /v1/users {"username":"alice"}'), [row(ROW_A, 'A', false)], { actor: 'A' }),
    caption: 'A вставляет строку. Запись alice уже лежит в уникальном индексе, но транзакция A не зафиксирована: другие сеансы строку не видят. A продолжает — дальше запись в auth.credentials.',
  },
  {
    state: uq(inst('A', 'running', `BEGIN; ${INSERT}`, 'INSERT 0 1, транзакция открыта'), inst('B', 'waiting', `BEGIN; ${INSERT}`, 'ждёт: в индексе уже есть alice от незавершённой транзакции A'), [row(ROW_A, 'A', false)], { actor: 'B' }),
    caption: 'B вставляет то же имя. Проверка уникальности находит в индексе alice от транзакции, которая ещё не закончилась. Ошибку сразу дать нельзя: если A откатится, имя свободно. Поэтому B ждёт исхода транзакции A. Именно здесь окно «проверь, потом действуй» и закрывается: проверка и запись — одно действие внутри индекса.',
  },
  {
    state: uq(A_DONE, inst('B', 'waiting', `BEGIN; ${INSERT}`, 'ждёт: в индексе уже есть alice от незавершённой транзакции A'), [row(ROW_A, 'A')], { actor: 'A' }),
    caption: 'A записал учётные данные и зафиксировал транзакцию: строка alice видна всем, клиент A получает 201 Created.',
  },
  {
    state: uq(A_DONE, B_FAILED, [row(ROW_A, 'A')], { actor: 'B', sqlstate: UNIQUE_VIOLATION, chain: 0 }),
    caption: `Транзакция A зафиксирована — значит, конфликт настоящий. INSERT в B завершается ошибкой: код SQLSTATE ${UNIQUE_VIOLATION} (unique_violation), имя ограничения ${CONSTRAINT}. Транзакция B откатывается целиком: от B в базе не осталось ничего.`,
  },
  {
    state: uq(A_DONE, B_FAILED, [row(ROW_A, 'A')], { actor: 'B', sqlstate: UNIQUE_VIOLATION, chain: 1 }),
    caption: `Postgres-адаптер user переводит ошибку драйвера в доменную: errors.As достаёт *pgconn.PgError, и если Code = "${UNIQUE_VIOLATION}", а ConstraintName = "${CONSTRAINT}", возвращается user/domain.ErrUsernameTaken. Текст сообщения не разбирают: он зависит от языка сервера (lc_messages) и версии. Имя ограничения проверяют, потому что 23505 бывает и у других уникальных ключей таблицы, а «повтор id» — уже ошибка сервера, а не «имя занято».`,
  },
  {
    state: uq(A_DONE, B_FAILED, [row(ROW_A, 'A')], { actor: 'B', sqlstate: UNIQUE_VIOLATION, chain: 2 }),
    caption: 'Дальше ошибка идёт по уже знакомой с модуля 1 цепочке. Контекст auth не импортирует домен user: мост userbridge переводит user/domain.ErrUsernameTaken в собственную ошибку auth — auth/domain.ErrUsernameTaken.',
  },
  {
    state: uq(A_DONE, B_409, [row(ROW_A, 'A')], { actor: 'B', sqlstate: UNIQUE_VIOLATION, chain: 3 }),
    caption: [
      { text: 'HTTP-адаптер auth переводит её в ' },
      PROBLEM,
      { text: ' с кодом username_taken и статусом 409 Conflict. Use case не изменился ни строкой: он и раньше ждал ErrUsernameTaken от порта.' },
    ],
  },
  {
    state: uq(A_DONE, B_409, [row(ROW_A, 'A')], { sqlstate: UNIQUE_VIOLATION, chain: 3, verdict: 'held' }),
    caption: [
      { text: 'Итог: один пользователь alice, клиент A получил 201, клиент B — 409 и может выбрать другое имя. ' },
      CONSTRAINT_T,
      { text: ' держит правило при любом писателе: втором экземпляре, ручном SQL, скрипте. Предварительный SELECT можно добавить как оптимизацию (не тратить хеш пароля на занятое имя), но гарантию даёт только база.' },
    ],
  },
]

export function constraintRaceScenario(variant: RaceVariant = 'code-check'): readonly Step<RaceFrame>[] {
  return variant === 'unique' ? UNIQUE : CODE_CHECK
}
