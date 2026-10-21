import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { erBuildScenario, type ErFrame } from './er-build'

const steps = erBuildScenario()
const last = steps.at(-1)!.state
const rel = (f: ErFrame, parent: string, child: string) => f.relations.find((r) => r.parent === parent && r.child === child)

describe('последний шаг', () => {
  it('шесть сущностей в своих схемах', () => {
    expect(last.entities.map((e) => `${e.schema}.${e.id}`).sort()).toEqual(
      ['auth.credentials', 'chat.chats', 'chat.members', 'chat.messages', 'platform.idempotency_keys', 'users.users'].sort(),
    )
    expect(last.showSchemas).toBe(true)
  })

  it('связи с верной кардинальностью', () => {
    expect(rel(last, 'users', 'credentials')).toMatchObject({ kind: '1:1', column: 'user_id', fk: true })
    expect(rel(last, 'chats', 'messages')).toMatchObject({ kind: '1:N', column: 'chat_id', fk: true })
    expect(rel(last, 'users', 'members')).toMatchObject({ kind: '1:N', fk: true })
    expect(rel(last, 'chats', 'members')).toMatchObject({ kind: '1:N', fk: true })
    expect(last.manyToMany).toEqual([{ a: 'users', b: 'chats', via: 'members' }])
    expect(last.entities.find((e) => e.id === 'members')?.junction).toBe(true)
  })

  it('межсхемные FK — только в users.users (из auth.credentials и chat.members)', () => {
    const schemaOf = new Map(last.entities.map((e) => [e.id, e.schema]))
    const cross = last.relations.filter((r) => r.fk && schemaOf.get(r.parent) !== schemaOf.get(r.child))
    expect(cross.map((r) => `${r.child}->${r.parent}`).sort()).toEqual(['credentials->users', 'members->users'])
    expect(rel(last, 'users', 'messages')).toMatchObject({ column: 'author_id', fk: false })
    expect([...last.fresh].sort()).toEqual(cross.map((r) => r.id).sort())
  })

  it('ключи совпадают с рулингом 1: PK (chat_id, seq) и UNIQUE (id) у сообщений, PK (chat_id, user_id) у участников', () => {
    const pk = (id: string) => last.entities.find((e) => e.id === id)!.attrs.filter((a) => a.keys.includes('PK')).map((a) => a.name)
    expect(pk('messages')).toEqual(['chat_id', 'seq'])
    expect(pk('members')).toEqual(['chat_id', 'user_id'])
    expect(pk('credentials')).toEqual(['user_id'])
    expect(pk('idempotency_keys')).toEqual(['scope', 'key'])
    expect(last.entities.find((e) => e.id === 'messages')!.attrs.find((a) => a.name === 'id')!.keys).toEqual(['UQ'])
  })
})

it('модель только растёт; каждая связь ссылается на уже показанные сущности; у шага есть требование', () => {
  steps.forEach((s, i) => {
    const ids = new Set(s.state.entities.map((e) => e.id))
    for (const r of s.state.relations) expect(ids.has(r.parent) && ids.has(r.child)).toBe(true)
    for (const m of s.state.manyToMany) expect(ids.has(m.a) && ids.has(m.b) && ids.has(m.via)).toBe(true)
    expect(s.state.requirement.length).toBeGreaterThan(10)
    if (i > 0) expect(s.state.entities.length).toBeGreaterThanOrEqual(steps[i - 1]!.state.entities.length)
    expect(s.state.showSchemas).toBe(i === steps.length - 1)
  })
  expect(steps[0]!.state.entities.map((e) => e.id)).toEqual(['users'])
})

it('подписи: термины из глоссария, ключевые есть', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  const ids = steps.flatMap((s) => captionTermIds(s.caption))
  for (const id of ['entity', 'relationship', 'cardinality', 'er-diagram', 'foreign-key', 'bounded-context']) expect(ids).toContain(id)
  for (const id of ids) expect(known.has(id)).toBe(true)
  for (const s of steps) expect(captionText(s.caption).trim().length).toBeGreaterThan(40)
})
