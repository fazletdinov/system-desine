import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { PIPELINE_IDS, requestPipelineScenario, type PipelineScenario } from './request-pipeline'

const steps = requestPipelineScenario()
const of = (s: PipelineScenario) => steps.filter((x) => x.state.scenario === s)

describe('requestPipelineScenario', () => {
  it('от 10 до 14 кадров, у каждого есть подпись', () => {
    expect(steps.length).toBeGreaterThanOrEqual(10)
    expect(steps.length).toBeLessThanOrEqual(14)
    for (const s of steps) expect(captionText(s.caption).trim().length).toBeGreaterThan(20)
  })

  it('пять сценариев подряд, в каждом 2-3 кадра', () => {
    const order = steps.map((s) => s.state.scenario).filter((v, i, a) => a.indexOf(v) === i)
    expect(order).toEqual(['ok', 'panic', 'no-identity', 'unknown-route', 'wrong-method'])
    for (const id of order) expect([2, 3]).toContain(of(id).length)
  })

  it('каждая подсветка - существующий слой', () => {
    for (const s of steps) for (const h of s.highlight ?? []) expect(PIPELINE_IDS).toContain(h)
  })

  it('паника: ответ 500 и строка лога со статусом 500', () => {
    const r = of('panic').find((s) => s.state.at === 'response')
    expect(r?.state.status).toBe(500)
    expect(r?.state.log.some((l) => l.includes('500'))).toBe(true)
  })

  it('без личности цепочка не доходит до хендлера', () => {
    expect(of('no-identity').some((s) => s.state.at === 'handler')).toBe(false)
    expect(of('no-identity').some((s) => s.state.status === 401)).toBe(true)
  })

  it('неверный метод: 405', () => {
    expect(of('wrong-method').some((s) => s.state.status === 405)).toBe(true)
  })

  it('успех: 201 и строка лога', () => {
    const r = of('ok').find((s) => s.state.at === 'response')
    expect(r?.state.status).toBe(201)
    expect(r?.state.log.length).toBeGreaterThan(0)
  })

  it('у каждого кадра есть requestId', () => {
    for (const s of steps) expect(s.state.requestId.length).toBeGreaterThan(0)
  })
})

it('все термины в подписях есть в глоссарии', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  const ids = steps.flatMap((s) => captionTermIds(s.caption))
  expect(ids.length).toBeGreaterThanOrEqual(2)
  for (const id of ids) expect(known.has(id)).toBe(true)
})
