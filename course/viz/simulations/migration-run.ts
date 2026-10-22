import { term } from '../core/caption'
import type { Step } from '../core/types'

// Как мессенджер применяет миграции (рулинг 7 плана модуля 2a): подкоманда `messenger migrate up`
// на goose v3 (goose.NewProvider, *sql.DB через pgx/v5/stdlib на одно соединение), отдельный сервис
// compose `migrate`, от которого messenger зависит с condition: service_completed_successfully;
// сервер при старте сверяет версию схемы с migrations.Required и без неё не стартует.
// goose ведёт таблицу goose_db_version и выполняет каждую SQL-миграцию в своей транзакции
// вместе с записью версии, поэтому сбой посередине откатывает и DDL, и запись версии.
// Миграция 00002 — условный пример (в мессенджере её нет): показывает сбой и повтор.

export type MigVariant = 'first' | 'failure' | 'outdated'
export type FileStatus = 'applied' | 'pending' | 'running' | 'failed'
export type StmtStatus = 'pending' | 'done' | 'waiting' | 'failed'
export type ProcState = 'not-started' | 'running' | 'exit-0' | 'exit-1' | 'serving' | 'blocked'

export interface MigFile {
  version: number
  name: string
  status: FileStatus
}

export interface MigStmt {
  sql: string
  status: StmtStatus
}

export interface MigTx {
  version: number
  state: 'open' | 'committed' | 'rolled-back'
  stmts: readonly MigStmt[]
}

export interface VersionRow {
  version_id: number
  is_applied: boolean
}

export interface DbTable {
  name: string
  // tx — создана в незафиксированной транзакции: другие сеансы её не видят.
  visibility: 'committed' | 'tx'
}

export interface MigFrame {
  variant: MigVariant
  files: readonly MigFile[]
  tx: MigTx | null
  // null — таблицы goose_db_version ещё нет.
  versions: readonly VersionRow[] | null
  tables: readonly DbTable[]
  // Чужая блокировка, мешающая миграции.
  lock: string | null
  migrate: ProcState
  server: ProcState
  // Версия, без которой бинарь не стартует (migrations.Required).
  required: number
  log: string
}

export const VERSION_TABLE = 'goose_db_version'
export const LOCK_TIMEOUT = '5s'
export const INIT_TABLES = ['users.users', 'auth.credentials', 'chat.chats', 'chat.members', 'chat.messages', 'platform.idempotency_keys'] as const
export const EDITS_TABLE = 'chat.message_edits'

// Версия схемы — последняя применённая запись goose_db_version (строка 0 goose вставляет при создании таблицы).
export function schemaVersion(f: MigFrame): number | null {
  if (f.versions === null) return null
  return f.versions.filter((r) => r.is_applied).reduce((m, r) => Math.max(m, r.version_id), 0)
}

const MIGRATION = term('миграция (migration)', 'migration')
const TX = term('транзакции (transaction)', 'transaction')
const CRASH_ONLY = term('crash-only', 'crash-only')

const F_INIT = '00001_init.sql'
const F_EDITS = '00002_message_edits.sql'
const file = (version: number, name: string, status: FileStatus): MigFile => ({ version, name, status })
const tables = (names: readonly string[], txOnly: readonly string[] = []): DbTable[] => [
  ...names.map((name) => ({ name, visibility: 'committed' as const })),
  ...txOnly.map((name) => ({ name, visibility: 'tx' as const })),
]
const V0: VersionRow = { version_id: 0, is_applied: true }
const V1: VersionRow = { version_id: 1, is_applied: true }
const V2: VersionRow = { version_id: 2, is_applied: true }

const INIT_STMTS = ['CREATE SCHEMA users; … auth; … chat; … platform', 'CREATE TABLE users.users (…)', 'CREATE TABLE auth.credentials (…)', 'CREATE TABLE chat.chats, chat.members, chat.messages (…)', 'CREATE TABLE platform.idempotency_keys (…)']
const stmts = (list: readonly string[], status: StmtStatus): MigStmt[] => list.map((sql) => ({ sql, status }))
const INSERT_V = (v: number): string => `INSERT INTO ${VERSION_TABLE} (version_id, is_applied) VALUES (${v}, true)`

const base = (variant: MigVariant, required: number): Omit<MigFrame, 'files' | 'tx' | 'versions' | 'tables' | 'migrate' | 'server' | 'log'> => ({ variant, lock: null, required })

const FIRST: readonly Step<MigFrame>[] = [
  {
    state: { ...base('first', 1), files: [file(1, F_INIT, 'pending')], tx: null, versions: null, tables: [], migrate: 'running', server: 'blocked', log: '$ messenger migrate up' },
    caption: [
      { text: 'Пустая база, первый запуск compose. Сервер messenger не стартует сам: он ждёт сервис migrate (condition: service_completed_successfully). Тот выполняет подкоманду messenger migrate up: она открывает *sql.DB через pgx/v5/stdlib на одно соединение и передаёт его goose вместе со встроенными в бинарь файлами. ' },
      MIGRATION,
      { text: ' 00001_init.sql ещё не применена.' },
    ],
  },
  {
    state: { ...base('first', 1), files: [file(1, F_INIT, 'pending')], tx: null, versions: [V0], tables: [], migrate: 'running', server: 'blocked', log: `создана таблица ${VERSION_TABLE}` },
    caption: `Таблицы версий нет — goose создаёт ${VERSION_TABLE} и кладёт в неё строку с версией 0. Дальше эта таблица — единственный источник правды о том, какие миграции применены: goose сравнивает её с файлами и применяет миграции с версиями выше текущей, по порядку номеров. Пропуск в середине (файл с номером ниже уже применённого, out-of-order) goose по умолчанию отвергает.`,
  },
  {
    state: { ...base('first', 1), files: [file(1, F_INIT, 'running')], tx: { version: 1, state: 'open', stmts: stmts(INIT_STMTS, 'done') }, versions: [V0], tables: tables([], INIT_TABLES), migrate: 'running', server: 'blocked', log: `применяется ${F_INIT}` },
    caption: [
      { text: 'goose открывает ' },
      TX,
      { text: ' и выполняет операторы миграции: схемы и шесть таблиц. В PostgreSQL DDL транзакционен: таблицы уже созданы, но видны только внутри этой транзакции. Другой сеанс сейчас их не найдёт.' },
    ],
  },
  {
    state: { ...base('first', 1), files: [file(1, F_INIT, 'applied')], tx: { version: 1, state: 'committed', stmts: [...stmts(INIT_STMTS, 'done'), { sql: INSERT_V(1), status: 'done' }] }, versions: [V0, V1], tables: tables(INIT_TABLES), migrate: 'running', server: 'blocked', log: `${F_INIT} применена` },
    caption: `Последний оператор той же транзакции — запись версии 1 в ${VERSION_TABLE}. COMMIT делает видимыми сразу и таблицы, и версию: состояния «таблицы есть, а версия не записана» (или наоборот) не бывает.`,
  },
  {
    state: { ...base('first', 1), files: [file(1, F_INIT, 'applied')], tx: null, versions: [V0, V1], tables: tables(INIT_TABLES), migrate: 'exit-0', server: 'running', log: 'migrate: код выхода 0; запускается messenger' },
    caption: 'Применять больше нечего, migrate выходит с кодом 0, и compose запускает сервер. Повторный migrate up ничего не изменит: все версии из файлов уже есть в таблице. Это и делает шаг безопасным для каждого деплоя.',
  },
  {
    state: { ...base('first', 1), files: [file(1, F_INIT, 'applied')], tx: null, versions: [V0, V1], tables: tables(INIT_TABLES), migrate: 'exit-0', server: 'serving', log: 'messenger: версия схемы 1, нужна 1 — запуск' },
    caption: [
      { text: 'Сервер при старте проверяет базу: соединение (Ping) и версию схемы. Бинарю нужна версия 1 (migrations.Required), в базе 1 — сервер начинает принимать запросы. Проверка стоит до открытия порта: несовместимая схема обнаруживается до первого запроса, а не на нём.' },
    ],
  },
]

const EDIT_STMTS = [`SET LOCAL lock_timeout = '${LOCK_TIMEOUT}'`, `CREATE TABLE ${EDITS_TABLE} (…)`, 'ALTER TABLE chat.messages ADD COLUMN edited_at timestamptz']
const editStmts = (...status: StmtStatus[]): MigStmt[] => EDIT_STMTS.map((sql, i) => ({ sql, status: status[i] ?? 'pending' }))
const REPORT_LOCK = 'долгий отчётный SELECT по chat.messages держит ACCESS SHARE'
const FAILURE_FILES = (s: FileStatus): MigFile[] => [file(1, F_INIT, 'applied'), file(2, F_EDITS, s)]

const FAILURE: readonly Step<MigFrame>[] = [
  {
    state: { ...base('failure', 2), lock: REPORT_LOCK, files: FAILURE_FILES('pending'), tx: null, versions: [V0, V1], tables: tables(INIT_TABLES), migrate: 'running', server: 'blocked', log: '$ messenger migrate up' },
    caption: `Новый выпуск принёс условную миграцию ${F_EDITS}: таблица истории правок и столбец edited_at у сообщений. В базе версия 1. В это же время аналитик гоняет долгий отчёт по chat.messages — обычный SELECT, который держит самую слабую блокировку ACCESS SHARE.`,
  },
  {
    state: { ...base('failure', 2), lock: REPORT_LOCK, files: FAILURE_FILES('running'), tx: { version: 2, state: 'open', stmts: editStmts('done', 'done') }, versions: [V0, V1], tables: tables(INIT_TABLES, [EDITS_TABLE]), migrate: 'running', server: 'blocked', log: `применяется ${F_EDITS}` },
    caption: `Транзакция миграции началась. Первым оператором миграция ограничивает ожидание блокировок: SET LOCAL lock_timeout = '${LOCK_TIMEOUT}' действует только до конца этой транзакции. Таблица ${EDITS_TABLE} создана — пока внутри транзакции.`,
  },
  {
    state: { ...base('failure', 2), lock: REPORT_LOCK, files: FAILURE_FILES('running'), tx: { version: 2, state: 'open', stmts: editStmts('done', 'done', 'waiting') }, versions: [V0, V1], tables: tables(INIT_TABLES, [EDITS_TABLE]), migrate: 'running', server: 'blocked', log: 'ALTER TABLE ждёт блокировку chat.messages' },
    caption: 'ALTER TABLE … ADD COLUMN без значения по умолчанию меняет только метаданные и выполняется мгновенно, но ему нужна блокировка ACCESS EXCLUSIVE, а она несовместима даже с ACCESS SHARE отчёта. ALTER встаёт в очередь — и все новые запросы к chat.messages, включая чтение ленты, встают за ним. Без lock_timeout мессенджер ждал бы, пока не кончится отчёт. Подробно об этом — модуль 2b.',
  },
  {
    state: { ...base('failure', 2), lock: REPORT_LOCK, files: FAILURE_FILES('failed'), tx: { version: 2, state: 'rolled-back', stmts: editStmts('done', 'done', 'failed') }, versions: [V0, V1], tables: tables(INIT_TABLES), migrate: 'exit-1', server: 'blocked', log: 'ERROR: canceling statement due to lock timeout (SQLSTATE 55P03)' },
    caption: [
      { text: `Через ${LOCK_TIMEOUT.replace('s', ' с')} ALTER отменяется с ошибкой 55P03 (lock_not_available). Вся ` },
      TX,
      { text: ` миграции откатывается: ${EDITS_TABLE} исчезает, хотя её CREATE TABLE прошёл, а версия в ${VERSION_TABLE} осталась 1. База в том же состоянии, что до запуска. migrate выходит с кодом 1, и compose не запускает новый сервер — старая схема и старый код остаются согласованными.` },
    ],
  },
  {
    state: { ...base('failure', 2), files: FAILURE_FILES('running'), tx: { version: 2, state: 'open', stmts: editStmts('done', 'done', 'done') }, versions: [V0, V1], tables: tables(INIT_TABLES, [EDITS_TABLE]), migrate: 'running', server: 'blocked', log: '$ messenger migrate up   # повтор' },
    caption: `Отчёт закончился, блокировки нет. Повтор — тот же migrate up, файл миграции не менялся: goose видит версию 1 и снова применяет ${F_EDITS} с начала. Править можно только неприменённую миграцию; применённую не трогают никогда — изменения идут новой миграцией.`,
  },
  {
    state: { ...base('failure', 2), files: [file(1, F_INIT, 'applied'), file(2, F_EDITS, 'applied')], tx: { version: 2, state: 'committed', stmts: [...editStmts('done', 'done', 'done'), { sql: INSERT_V(2), status: 'done' }] }, versions: [V0, V1, V2], tables: tables([...INIT_TABLES, EDITS_TABLE]), migrate: 'exit-0', server: 'running', log: `${F_EDITS} применена; запускается messenger` },
    caption: `COMMIT: таблица, столбец и версия 2 появились разом. migrate выходит с кодом 0, и только теперь стартует сервер, которому нужна версия 2.`,
  },
]

const OUTDATED: readonly Step<MigFrame>[] = [
  {
    state: { ...base('outdated', 2), files: FAILURE_FILES('pending'), tx: null, versions: [V0, V1], tables: tables(INIT_TABLES), migrate: 'not-started', server: 'running', log: '$ messenger   # migrate up не запускали' },
    caption: `Новый бинарь (ему нужна версия 2) запустили в обход шага миграций — вручную или в окружении, где migrate забыли. В базе версия 1: таблицы ${EDITS_TABLE} и столбца edited_at нет.`,
  },
  {
    state: { ...base('outdated', 2), files: FAILURE_FILES('pending'), tx: null, versions: [V0, V1], tables: tables(INIT_TABLES), migrate: 'not-started', server: 'running', log: `messenger: Ping — ок; версия схемы из ${VERSION_TABLE}: 1` },
    caption: `Сервер создаёт пул соединений, проверяет его Ping и читает версию схемы из ${VERSION_TABLE}: 1.`,
  },
  {
    state: { ...base('outdated', 2), files: FAILURE_FILES('pending'), tx: null, versions: [V0, V1], tables: tables(INIT_TABLES), migrate: 'not-started', server: 'exit-1', log: 'messenger: версия схемы 1, бинарю нужна 2 — примените миграции; код выхода 1' },
    caption: [
      { text: '1 меньше 2 — сервер отказывается стартовать: пишет «примените миграции» и выходит с кодом 1, не открыв порт. Без этой проверки он бы запустился и отвечал 500 (42P01, undefined_table) на каждый запрос к истории правок, а оркестратор считал бы его здоровым. Громкий отказ при старте — принцип ' },
      CRASH_ONLY,
      { text: ': ошибка видна сразу, выкатка останавливается, старые экземпляры продолжают работать.' },
    ],
  },
  {
    state: { ...base('outdated', 2), files: [file(1, F_INIT, 'applied'), file(2, F_EDITS, 'applied')], tx: { version: 2, state: 'committed', stmts: [...editStmts('done', 'done', 'done'), { sql: INSERT_V(2), status: 'done' }] }, versions: [V0, V1, V2], tables: tables([...INIT_TABLES, EDITS_TABLE]), migrate: 'exit-0', server: 'exit-1', log: '$ messenger migrate up — версия схемы 2' },
    caption: [
      { text: 'Оператор выполняет пропущенный шаг: migrate up применяет 00002 в одной ' },
      TX,
      { text: ', версия схемы — 2.' },
    ],
  },
  {
    state: { ...base('outdated', 2), files: [file(1, F_INIT, 'applied'), file(2, F_EDITS, 'applied')], tx: null, versions: [V0, V1, V2], tables: tables([...INIT_TABLES, EDITS_TABLE]), migrate: 'exit-0', server: 'serving', log: 'messenger: версия схемы 2, нужна 2 — запуск' },
    caption: 'Перезапущенный сервер видит 2 ≥ 2 и стартует. Проверка именно «не меньше», а не «равно»: старый бинарь (ему нужна 1) спокойно работает на схеме 2. Так во время выкатки старые и новые экземпляры живут на одной базе — если каждая миграция совместима с предыдущей версией кода (expand/contract, модуль 2b).',
  },
]

export function migrationRunScenario(variant: MigVariant = 'first'): readonly Step<MigFrame>[] {
  return variant === 'failure' ? FAILURE : variant === 'outdated' ? OUTDATED : FIRST
}
