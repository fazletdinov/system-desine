import { term } from '../core/caption'
import type { Step } from '../core/types'

// Модель мессенджера растёт по требованиям урока 0.6 (и 1.12 для ключей идемпотентности).
// Имена схем, таблиц и столбцов — из рулинга 1 плана модуля 2a (миграция 00001_init.sql):
// показаны только ключевые столбцы, остальные атрибуты сущности опущены ради ширины 375 px.

export type SchemaName = 'users' | 'auth' | 'chat' | 'platform'
export type EntityId = 'users' | 'credentials' | 'chats' | 'members' | 'messages' | 'idempotency_keys'
export type KeyMark = 'PK' | 'FK' | 'UQ'

export interface ErAttr {
  name: string
  keys: readonly KeyMark[]
}

export interface ErEntity {
  id: EntityId
  schema: SchemaName
  attrs: readonly ErAttr[]
  // Таблица-связка, которой реализована связь «многие ко многим».
  junction?: boolean
}

// Связь «родитель — потомок»: у родителя ровно одна строка на строку потомка (NOT NULL),
// у потомка — ноль или одна (1:1) либо ноль или много (1:N) строк на родителя.
export interface ErRelation {
  id: string
  parent: EntityId
  child: EntityId
  kind: '1:1' | '1:N'
  column: string
  // false — логическая ссылка без FOREIGN KEY (её держит код, а не база).
  fk: boolean
}

// M:N не хранится одной ссылкой: это пара связей 1:N через таблицу-связку.
export interface ErManyToMany {
  a: EntityId
  b: EntityId
  via: EntityId
}

export interface ErFrame {
  requirement: string
  source: string
  entities: readonly ErEntity[]
  relations: readonly ErRelation[]
  manyToMany: readonly ErManyToMany[]
  // Что появилось на этом шаге: id сущностей и связей.
  fresh: readonly string[]
  // Последний шаг: видны границы контекстов — схемы PostgreSQL.
  showSchemas: boolean
}

const attr = (name: string, ...keys: KeyMark[]): ErAttr => ({ name, keys })

const USERS: ErEntity = { id: 'users', schema: 'users', attrs: [attr('id', 'PK'), attr('username', 'UQ'), attr('display_name')] }
const CREDENTIALS: ErEntity = { id: 'credentials', schema: 'auth', attrs: [attr('user_id', 'PK', 'FK'), attr('password_hash')] }
const CHATS: ErEntity = { id: 'chats', schema: 'chat', attrs: [attr('id', 'PK'), attr('kind'), attr('direct_key', 'UQ'), attr('last_seq')] }
const MEMBERS: ErEntity = { id: 'members', schema: 'chat', attrs: [attr('chat_id', 'PK', 'FK'), attr('user_id', 'PK', 'FK')], junction: true }
const MESSAGES: ErEntity = { id: 'messages', schema: 'chat', attrs: [attr('chat_id', 'PK', 'FK'), attr('seq', 'PK'), attr('id', 'UQ'), attr('author_id')] }
const IDEMPOTENCY: ErEntity = { id: 'idempotency_keys', schema: 'platform', attrs: [attr('scope', 'PK'), attr('key', 'PK'), attr('state'), attr('expires_at')] }

const R_CREDENTIALS: ErRelation = { id: 'users-credentials', parent: 'users', child: 'credentials', kind: '1:1', column: 'user_id', fk: true }
const R_MEMBERS_USER: ErRelation = { id: 'users-members', parent: 'users', child: 'members', kind: '1:N', column: 'user_id', fk: true }
const R_MEMBERS_CHAT: ErRelation = { id: 'chats-members', parent: 'chats', child: 'members', kind: '1:N', column: 'chat_id', fk: true }
const R_MESSAGES: ErRelation = { id: 'chats-messages', parent: 'chats', child: 'messages', kind: '1:N', column: 'chat_id', fk: true }
const R_AUTHOR: ErRelation = { id: 'users-messages', parent: 'users', child: 'messages', kind: '1:N', column: 'author_id', fk: false }
const M_MEMBERSHIP: ErManyToMany = { a: 'users', b: 'chats', via: 'members' }


const ENTITY = term('сущность (entity)', 'entity')
const RELATIONSHIP = term('связь (relationship)', 'relationship')
const CARDINALITY = term('кардинальность (cardinality)', 'cardinality')
const ER = term('ER-диаграмме', 'er-diagram')
const SURROGATE = term('суррогатный ключ (surrogate key)', 'surrogate-key')
const FK = term('внешний ключ (foreign key)', 'foreign-key')
const IDEMPOTENCY_KEY = term('ключ идемпотентности (idempotency key)', 'idempotency-key')
const CONTEXT = term('ограниченный контекст (bounded context)', 'bounded-context')

interface Draft {
  requirement: string
  source: string
  add: readonly (ErEntity | ErRelation | ErManyToMany)[]
  caption: Step<ErFrame>['caption']
  showSchemas?: boolean
  // Подсветить уже существующие связи (последний шаг: межсхемные FK).
  highlight?: readonly string[]
}

const isEntity = (x: ErEntity | ErRelation | ErManyToMany): x is ErEntity => 'attrs' in x
const isRelation = (x: ErEntity | ErRelation | ErManyToMany): x is ErRelation => 'parent' in x

const DRAFTS: readonly Draft[] = [
  {
    requirement: 'Пользователь регистрируется и входит под своим именем',
    source: 'урок 0.6, требование 1',
    add: [USERS],
    caption: [
      { text: 'Модель начинают с требований, а не с экранов. В первом требовании есть существительное «пользователь» — это ' },
      ENTITY,
      { text: ': то, о чём системе нужно помнить, у чего есть идентичность и атрибуты. На ' },
      ER,
      { text: ' сущность — прямоугольник, ниже — её атрибуты. id — ' },
      SURROGATE,
      { text: ' (PK, UUIDv7 из приложения): имя пользователя тоже уникально (UQ), но может смениться, а ссылки на пользователя меняться не должны.' },
    ],
  },
  {
    requirement: 'Вход по паролю; хеш пароля хранится отдельно от профиля',
    source: 'урок 0.6, требование 1; урок 1.13',
    add: [CREDENTIALS, R_CREDENTIALS],
    caption: [
      { text: 'Учётные данные — отдельная сущность, а между ней и пользователем — ' },
      RELATIONSHIP,
      { text: ' «один к одному»: у пользователя не больше одной записи, у записи ровно один пользователь. ' },
      CARDINALITY,
      { text: ' читается по концам линии («воронья лапка»): две черты у users — «ровно один», кружок и черта у credentials — «ноль или один». 1:1 обеспечивает общий первичный ключ: user_id здесь и PK, и ' },
      FK,
      { text: ', вторую запись для того же пользователя база не примет. Почему не столбец в users: пароль принадлежит другому контексту (auth) и читается только при входе.' },
    ],
  },
  {
    requirement: 'Личные чаты 1:1 и групповые чаты до 1 000 участников',
    source: 'урок 0.6, требования 2 и 3',
    add: [CHATS],
    caption: 'Чат — третья сущность. Личный и групповой чат различаются атрибутом kind, а не двумя таблицами: сообщения, участники и лента у них общие. direct_key — отсортированная пара id собеседников, у личного чата она есть и уникальна (UQ), у группы — NULL: так база не даст завести второй личный чат той же пары. last_seq — номер последнего сообщения, денормализованный ради ленты (урок 2.2).',
  },
  {
    requirement: 'Пользователь состоит во многих чатах, в чате много участников',
    source: 'урок 0.6, требование 3',
    add: [MEMBERS, R_MEMBERS_USER, R_MEMBERS_CHAT, M_MEMBERSHIP],
    caption: [
      { text: 'Это ' },
      RELATIONSHIP,
      { text: ' «многие ко многим» (M:N). Одним столбцом её не выразить: список чатов в строке пользователя нарушил бы атомарность значений (1НФ, урок 2.2). Поэтому появляется таблица-связка members: строка — факт «пользователь u состоит в чате c». M:N распадается на две связи 1:N, у каждой свой ' },
      FK,
      { text: ', а первичный ключ (chat_id, user_id) не даёт добавить одного участника дважды.' },
    ],
  },
  {
    requirement: 'История сообщений чата с пагинацией',
    source: 'урок 0.6, требование 5',
    add: [MESSAGES, R_MESSAGES, R_AUTHOR],
    caption: [
      { text: 'Сообщение принадлежит ровно одному чату, в чате их много: 1:N, и ' },
      FK,
      { text: ' chat_id лежит на стороне «многих». Первичный ключ — (chat_id, seq): лента чата читается одним диапазоном индекса (урок 2.5); id сообщения остаётся уникальным (UQ) для поиска по нему. Пунктир author_id — ссылка на автора без FK: межконтекстные внешние ключи в мессенджере идут только из auth.credentials и chat.members, а что автор — участник чата, проверяет use case перед вставкой.' },
    ],
  },
  {
    requirement: 'Повтор запроса после обрыва связи не создаёт дубль',
    source: 'урок 1.12',
    add: [IDEMPOTENCY],
    caption: [
      { text: 'Последняя сущность пришла не из продуктовых требований, а из надёжности: ' },
      IDEMPOTENCY_KEY,
      { text: ' запоминает ответ на запрос, чтобы повтор с тем же ключом получил тот же ответ. Ключ записи — (scope, key): ключ задаёт клиент, а scope отделяет ключи разных пользователей. Связей у таблицы нет: она обслуживает HTTP-слой и ничего не знает о чатах и пользователях. Итого шесть сущностей.' },
    ],
  },
  {
    requirement: 'Каждый контекст владеет своими данными',
    source: 'урок 1.2',
    add: [],
    showSchemas: true,
    highlight: [R_CREDENTIALS.id, R_MEMBERS_USER.id],
    caption: [
      { text: 'Последний шаг — границы. Каждый ' },
      CONTEXT,
      { text: ' из модуля 1 получает свою схему PostgreSQL: users, auth, chat, platform. Таблицу видно по имени (chat.messages), а права можно выдавать на схему целиком. Две выделенные линии пересекают границу: внешние ключи из auth.credentials и chat.members в users.users. Это осознанная связь контекстов (идентичность пользователя — общий язык): пока она есть, схемы нельзя вынести в отдельные базы. Первый шаг при выделении сервиса — снять эти FK и проверять пользователя через порт.' },
    ],
  },
]

function build(): readonly Step<ErFrame>[] {
  const entities: ErEntity[] = []
  const relations: ErRelation[] = []
  const manyToMany: ErManyToMany[] = []
  return DRAFTS.map((d) => {
    const fresh: string[] = [...(d.highlight ?? [])]
    for (const x of d.add) {
      if (isEntity(x)) {
        entities.push(x)
        fresh.push(x.id)
      } else if (isRelation(x)) {
        relations.push(x)
        fresh.push(x.id)
      } else {
        manyToMany.push(x)
      }
    }
    return {
      state: {
        requirement: d.requirement,
        source: d.source,
        entities: [...entities],
        relations: [...relations],
        manyToMany: [...manyToMany],
        fresh,
        showSchemas: d.showSchemas === true,
      },
      caption: d.caption,
    }
  })
}

const STEPS = build()

export function erBuildScenario(): readonly Step<ErFrame>[] {
  return STEPS
}
