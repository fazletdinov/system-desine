import { describe, expect, it } from 'vitest'
import { BASE_IMPORTS, CANDIDATES, checkImport, depsSimulation } from './deps'

describe('checkImport', () => {
  it('все импорты реального мессенджера разрешены', () => {
    for (const [from, to] of BASE_IMPORTS) expect(checkImport(from, to, BASE_IMPORTS)).toBeNull()
  })
  it.each([
    ['chat/domain', 'net/http', 'domain-pure'],
    ['chat/domain', 'chat/adapters/memory', 'cycle'],
    ['chat/usecase', 'chat/adapters/memory', 'usecase-ports'],
    ['chat/usecase', 'platform/clock', 'usecase-ports'],
    ['user/adapters/memory', 'user/adapters/httpapi', 'adapter-adapter'],
    ['chat/usecase', 'user/usecase', 'cross-context'],
    ['chat/adapters/userchecker', 'user/adapters/memory', 'adapter-adapter'],
    ['platform/clock', 'chat/domain', 'platform-pure'],
  ] as const)('%s → %s нарушает %s', (from, to, rule) => {
    expect(checkImport(from, to, BASE_IMPORTS)?.rule).toBe(rule)
  })
  it('каждый кандидат либо разрешён, либо называет правило и последствие', () => {
    for (const c of CANDIDATES) {
      const v = checkImport(c.from, c.to, BASE_IMPORTS)
      if (v !== null) expect(v.consequence.length).toBeGreaterThan(20)
    }
  })
})

describe('depsSimulation', () => {
  it('добавленный импорт становится ребром и даёт шаг с вердиктом', () => {
    const s = depsSimulation.apply(depsSimulation.initial(), { type: 'add', from: 'chat/domain', to: 'net/http' })
    const steps = depsSimulation.steps(s)
    expect(steps.at(-1)?.state.violation?.rule).toBe('domain-pure')
    expect(depsSimulation.steps(depsSimulation.apply(s, { type: 'reset' }))).toHaveLength(1)
  })
})
