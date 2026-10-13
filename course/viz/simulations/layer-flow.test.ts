import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { LAYERS, layerFlowScenario } from './layer-flow'

const steps = layerFlowScenario()
const HTTP_WORDS = /http|json|x-user-id|request|заголов|dto/i

describe('layerFlowScenario', () => {
  it('от 6 до 10 шагов, у каждого непустая подпись и непустой список «видит»', () => {
    expect(steps.length).toBeGreaterThanOrEqual(6)
    expect(steps.length).toBeLessThanOrEqual(10)
    for (const s of steps) {
      expect(captionText(s.caption).trim().length).toBeGreaterThan(20)
      expect(s.state.sees.length).toBeGreaterThan(0)
      expect(LAYERS.map((l) => l.id)).toContain(s.state.active)
    }
  })

  it('начинается и заканчивается в HTTP-обработчике', () => {
    expect(steps[0]!.state.active).toBe('http')
    expect(steps.at(-1)!.state.active).toBe('http')
  })

  it('есть повтор: попытка 2 после конфликта версии', () => {
    expect(steps.some((s) => s.state.attempt === 2)).toBe(true)
    const firstRetry = steps.findIndex((s) => s.state.attempt === 2)
    expect(steps.slice(0, firstRetry).some((s) => s.state.attempt === 1)).toBe(true)
  })

  it('номер попытки не убывает', () => {
    for (let i = 1; i < steps.length; i++) expect(steps[i]!.state.attempt).toBeGreaterThanOrEqual(steps[i - 1]!.state.attempt)
  })

  it('домен не видит ничего из HTTP', () => {
    const domain = steps.filter((s) => s.state.active === 'domain')
    expect(domain.length).toBeGreaterThan(0)
    for (const s of domain) for (const item of s.state.sees) expect(item).not.toMatch(HTTP_WORDS)
  })

  it('use case тоже не видит HTTP', () => {
    for (const s of steps.filter((x) => x.state.active === 'usecase')) for (const item of s.state.sees) expect(item).not.toMatch(HTTP_WORDS)
  })
})

it('все термины в подписях есть в глоссарии', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  const ids = steps.flatMap((s) => captionTermIds(s.caption))
  expect(ids.length).toBeGreaterThanOrEqual(3)
  for (const id of ids) expect(known.has(id)).toBe(true)
})
