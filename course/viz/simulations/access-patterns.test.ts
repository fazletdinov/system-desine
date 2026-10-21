import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import {
  MESSENGER_SCHEMA,
  MESSAGES_KEYS,
  QUERIES,
  TABLES,
  accessPatternsSimulation as sim,
  evaluate,
  type AccessPatternsAction,
  type MessagesKey,
  type Schema,
} from './access-patterns'

const query = (id: string) => QUERIES.find((q) => q.id === id)!
const run = (...actions: AccessPatternsAction[]) => actions.reduce(sim.apply, sim.initial())

describe('access-patterns: схема мессенджера', () => {
  it('лента чата — диапазон по PK (chat_id, seq), читается ровно страница', () => {
    const r = evaluate(query('feed'), MESSENGER_SCHEMA)
    expect(r.plan.kind).toBe('range')
    expect(r.plan.index!.label).toBe('PRIMARY KEY (chat_id, seq)')
    expect(r.plan.rowsRead).toBe(50)
  })

  it('сообщение по id — точечный поиск через UNIQUE (id), без него — полный проход', () => {
    const r = evaluate(query('message-by-id'), MESSENGER_SCHEMA)
    expect(r.plan.kind).toBe('point')
    expect(r.plan.index!.label).toBe('UNIQUE (id)')
    expect(r.withoutIndex!.kind).toBe('full')
    expect(r.withoutIndex!.rowsRead).toBe(TABLES['chat.messages'].rows)
  })

  it('чаты пользователя — через индекс members(user_id); без него — полный проход', () => {
    const withIx = evaluate(query('chats-of-member'), MESSENGER_SCHEMA)
    expect(withIx.plan.kind).toBe('range')
    expect(withIx.plan.index!.label).toBe('INDEX (user_id)')
    expect(withIx.plan.rowsRead).toBe(6)
    const without = evaluate(query('chats-of-member'), { ...MESSENGER_SCHEMA, membersUserIndex: false })
    expect(without.plan.kind).toBe('full')
    expect(without.plan.rowsRead).toBe(TABLES['chat.members'].rows)
  })

  it('точечные запросы обслуживают первичные ключи и UNIQUE', () => {
    for (const id of ['is-member', 'user-by-username', 'credentials', 'idempotency']) {
      const r = evaluate(query(id), MESSENGER_SCHEMA)
      expect(r.plan.kind, id).toBe('point')
      expect(r.plan.rowsRead, id).toBe(1)
    }
  })

  it('в схеме мессенджера ни один запрос не идёт полным проходом', () => {
    for (const q of QUERIES) expect(evaluate(q, MESSENGER_SCHEMA).plan.kind, q.id).not.toBe('full')
  })
})

describe('access-patterns: другие ключи сообщений', () => {
  const schema = (key: MessagesKey): Schema => ({ ...MESSENGER_SCHEMA, messagesKey: key })

  it('PK (id) + INDEX (chat_id): лента читает все сообщения чата и сортирует', () => {
    const r = evaluate(query('feed'), schema('id-only'))
    expect(r.plan.kind).toBe('range-sort')
    expect(r.plan.index!.label).toBe('INDEX (chat_id)')
    expect(r.plan.rowsRead).toBe(100)
    expect(r.plan.rowsReturned).toBe(50)
    expect(evaluate(query('message-by-id'), schema('id-only')).plan.index!.label).toBe('PRIMARY KEY (id)')
  })

  it('PK (seq, chat_id): ключ с чужим префиксом ленте не помогает', () => {
    const r = evaluate(query('feed'), schema('seq-chat'))
    expect(r.plan.index!.label).not.toBe('PRIMARY KEY (seq, chat_id)')
    expect(r.plan.kind).toBe('range-sort')
  })

  it('варианты ключа меняют только индексы сообщений', () => {
    for (const key of Object.keys(MESSAGES_KEYS) as MessagesKey[]) {
      for (const id of ['chats-of-member', 'is-member', 'user-by-username', 'credentials', 'idempotency']) {
        expect(evaluate(query(id), schema(key)).plan).toEqual(evaluate(query(id), MESSENGER_SCHEMA).plan)
      }
    }
  })
})

describe('access-patterns: песочница', () => {
  it('действия выбирают запрос и меняют схему; сброс возвращает схему мессенджера', () => {
    const s = run({ type: 'query', queryId: 'chats-of-member' }, { type: 'members-user-index', on: false })
    const f = sim.steps(s)[0]!.state
    expect(f.selected.plan.kind).toBe('full')
    expect(f.tableIndexes.map((i) => i.label)).toEqual(['PRIMARY KEY (chat_id, user_id)'])
    expect(captionText(sim.steps(s)[0]!.caption)).toContain('members(user_id)')
    expect(run({ type: 'messages-key', key: 'id-only' }, { type: 'reset' }).schema).toEqual(MESSENGER_SCHEMA)
    expect(run({ type: 'query', queryId: 'нет такого' }).queryId).toBe('feed')
  })

  it('термины подписей есть в глоссарии', () => {
    const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
    const ids = new Set<string>()
    for (const key of Object.keys(MESSAGES_KEYS) as MessagesKey[]) {
      for (const on of [true, false]) {
        for (const q of QUERIES) {
          const s = run({ type: 'messages-key', key }, { type: 'members-user-index', on }, { type: 'query', queryId: q.id })
          const caption = sim.steps(s)[0]!.caption
          expect(captionText(caption).length).toBeGreaterThan(0)
          for (const id of captionTermIds(caption)) ids.add(id)
        }
      }
    }
    for (const id of ids) expect(known.has(id), id).toBe(true)
  })
})
