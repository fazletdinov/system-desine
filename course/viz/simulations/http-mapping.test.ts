import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { STAGES, httpMappingScenario, type Stage } from './http-mapping'

const steps = httpMappingScenario()
const JSON_STAGES = new Set<Stage>(['json-in', 'dto', 'dto-out', 'json-out'])

describe('httpMappingScenario', () => {
  it('от 6 до 10 шагов, у каждого непустая подпись и форма', () => {
    expect(steps.length).toBeGreaterThanOrEqual(6)
    expect(steps.length).toBeLessThanOrEqual(10)
    for (const s of steps) {
      expect(captionText(s.caption).trim().length).toBeGreaterThan(20)
      expect(s.state.shape.trim().length).toBeGreaterThan(0)
    }
  })

  it('проходит все стадии по порядку', () => {
    const order = STAGES.map((s) => s.id)
    const seen = steps.map((s) => order.indexOf(s.state.stage))
    for (let i = 1; i < seen.length; i++) expect(seen[i]!).toBeGreaterThanOrEqual(seen[i - 1]!)
    expect(new Set(steps.map((s) => s.state.stage))).toEqual(new Set(order))
  })

  it('json встречается в форме только на стадиях JSON и DTO', () => {
    for (const s of steps) expect(/json/i.test(s.state.shape)).toBe(JSON_STAGES.has(s.state.stage))
  })

  it('одно и то же сообщение «привет» проходит через все представления', () => {
    for (const s of steps.filter((x) => !x.state.shape.includes('json.Marshal'))) expect(s.state.shape).toContain('привет')
  })

  it('есть шаг про json.Marshal доменной структуры: получится {}', () => {
    expect(steps.some((s) => s.state.shape.includes('json.Marshal') && s.state.shape.includes('{}'))).toBe(true)
  })
})

it('все термины в подписях есть в глоссарии', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  const ids = steps.flatMap((s) => captionTermIds(s.caption))
  expect(ids.length).toBeGreaterThanOrEqual(2)
  for (const id of ids) expect(known.has(id)).toBe(true)
})
