import { describe, expect, it } from 'vitest'
import { ctxTreeSimulation as sim, effectiveDeadline } from './ctx-tree'

const run = (...actions: Parameters<typeof sim.apply>[1][]) => actions.reduce(sim.apply, sim.initial())

describe('ctx-tree', () => {
  it('дедлайн потомка не позже дедлайна родителя', () => {
    expect(effectiveDeadline('store')).toBeLessThanOrEqual(effectiveDeadline('request'))
    expect(effectiveDeadline('usecase')).toBe(effectiveDeadline('request'))
  })
  it('отмена родителя отменяет всех потомков, кроме отвязанных', () => {
    const s = run({ type: 'cancel', id: 'request' })
    expect(Object.keys(s.done).sort()).toEqual(['request', 'store', 'usecase', 'userchecker'])
    expect(s.done.audit).toBeUndefined()
  })
  it('отмена потомка не трогает родителя', () => {
    const s = run({ type: 'cancel', id: 'userchecker' })
    expect(s.done.userchecker?.cause).toBe('canceled')
    expect(s.done.usecase).toBeUndefined()
  })
  it('время доходит до дедлайна — причина deadline у узла и его потомков', () => {
    const s = run({ type: 'advance', ms: 100 })
    expect(s.done.userchecker?.cause).toBe('deadline')
    expect(s.done.request).toBeUndefined()
    const t = run({ type: 'advance', ms: 1000 })
    expect(t.done.request?.cause).toBe('deadline')
    expect(t.done.store?.cause).toBe('deadline')
    expect(t.done.audit).toBeUndefined()
  })
  it('Background отменить нельзя', () => {
    expect(run({ type: 'cancel', id: 'root' }).done.root).toBeUndefined()
  })
})
