import { describe, expect, it } from 'vitest'
import { idempotencySimulation as sim, type IdemAction, type IdemMode } from './idempotency'

const run = (mode: IdemMode, ...actions: IdemAction[]) => actions.reduce(sim.apply, sim.apply(sim.initial(), { type: 'mode', mode }))
const sendA: IdemAction = { type: 'send', body: 'A' }

describe('idempotency', () => {
  it('без ключа: ответ потерян, повтор — второй чат', () => {
    const s = run('no-key', { type: 'lose' }, sendA, sendA)
    expect(s.chats).toBe(2)
    expect(s.attempts.map((a) => a.delivered)).toEqual([false, true])
  })
  it('с ключом: повтор получает тот же ответ, чат один', () => {
    const s = run('key', { type: 'lose' }, sendA, sendA)
    expect(s.chats).toBe(1)
    expect(s.attempts.map((a) => a.outcome)).toEqual(['created', 'replayed'])
    expect(s.attempts[1]?.status).toBe(201)
  })
  it('с ключом: параллельный повтор — 409, чат один', () => {
    const s = run('key', { type: 'parallel' })
    expect(s.chats).toBe(1)
    expect(s.attempts.map((a) => a.status).sort()).toEqual([201, 409])
  })
  it('без ключа: параллельные запросы — два чата', () => {
    expect(run('no-key', { type: 'parallel' }).chats).toBe(2)
  })
  it('тот же ключ, другое тело — 422', () => {
    const s = run('key', sendA, { type: 'send', body: 'B' })
    expect(s.attempts.at(-1)?.status).toBe(422)
    expect(s.chats).toBe(1)
  })
  it('после 5xx ключ свободен, повтор создаёт чат', () => {
    const s = run('key', { type: 'fail' }, sendA, sendA)
    expect(s.attempts.map((a) => a.status)).toEqual([500, 201])
    expect(s.chats).toBe(1)
    expect(s.record?.done).toBe(true)
  })
  it('сброс сохраняет режим', () => {
    expect(sim.apply(run('key', sendA), { type: 'reset' })).toMatchObject({ mode: 'key', chats: 0, attempts: [] })
  })
})
