import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { LEVELS, testLayersScenario, type Level } from './test-layers'

const steps = testLayersScenario()
const levelIds = new Set<Level>(LEVELS.map((l) => l.id))

describe('testLayersScenario', () => {
  it('от 6 до 10 шагов, у каждого непустая подпись и цена', () => {
    expect(steps.length).toBeGreaterThanOrEqual(6)
    expect(steps.length).toBeLessThanOrEqual(10)
    for (const s of steps) {
      expect(captionText(s.caption).trim().length).toBeGreaterThan(20)
      expect(s.state.cost.trim().length).toBeGreaterThan(0)
    }
  })

  it('шесть разных багов, у каждого — уровень из списка', () => {
    const bugs = new Set(steps.map((s) => s.state.bug))
    expect(bugs.size).toBe(6)
    for (const s of steps) expect(levelIds.has(s.state.caughtBy)).toBe(true)
  })

  it.each<[string, Level]>([
    ['NewText', 'domain-unit'],
    ['участник', 'usecase-fake'],
    ['верси', 'contract'],
    ['ErrNotMember', 'http'],
    ['мьютекс', 'race'],
    ['SQL', 'contract'],
  ])('баг про «%s» ловит уровень %s', (needle, level) => {
    const s = steps.find((x) => x.state.bug.includes(needle))
    expect(s).toBeDefined()
    expect(s!.state.caughtBy).toBe(level)
  })

  it('используются все пять уровней', () => {
    expect(new Set(steps.map((s) => s.state.caughtBy))).toEqual(levelIds)
  })
})

it('все термины в подписях есть в глоссарии', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  const ids = steps.flatMap((s) => captionTermIds(s.caption))
  expect(ids.length).toBeGreaterThanOrEqual(3)
  for (const id of ids) expect(known.has(id)).toBe(true)
})
