import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { PROBLEM_SOURCES, problemMappingScenario } from './problem-mapping'

const steps = problemMappingScenario()
const problems = steps.filter((s) => s.state.problem !== null)
const REQUIRED = ['type', 'title', 'status', 'detail', 'instance', 'code', 'request_id']

describe('problemMappingScenario', () => {
  it('от 8 до 10 кадров, у каждого подпись и ошибка', () => {
    expect(steps.length).toBeGreaterThanOrEqual(8)
    expect(steps.length).toBeLessThanOrEqual(10)
    for (const s of steps) {
      expect(captionText(s.caption).trim().length).toBeGreaterThan(20)
      expect(s.state.error.length).toBeGreaterThan(0)
    }
  })

  it('все пять источников встречаются и подсветка - источник', () => {
    expect(new Set(steps.map((s) => s.state.source))).toEqual(new Set(PROBLEM_SOURCES))
    for (const s of steps) expect(s.highlight).toEqual([s.state.source])
  })

  it('каждый problem содержит обязательные поля, type по шаблону', () => {
    expect(problems.length).toBeGreaterThanOrEqual(5)
    for (const { state } of problems) {
      const p = state.problem!
      for (const k of REQUIRED) expect(p).toHaveProperty(k)
      expect(p.type).toBe(`https://messenger.example/problems/${String(p.code)}`)
      for (const f of state.highlightFields) expect(p).toHaveProperty(f)
    }
  })

  it('невалидный ввод: invalid_params называет поле', () => {
    const byName = (src: string) => problems.find((s) => s.state.source === src)!.state.problem as { code: string; status: number; invalid_params: { name: string }[] }
    expect(byName('domain')).toMatchObject({ status: 400, code: 'invalid_input' })
    expect(byName('domain').invalid_params[0]!.name).toBe('text')
    expect(byName('param')).toMatchObject({ status: 400, code: 'invalid_input' })
    expect(byName('param').invalid_params[0]!.name).toBe('limit')
  })

  it('роутер, паника, большое тело: свои статусы и коды', () => {
    const p = (src: string) => problems.find((s) => s.state.source === src)!.state
    expect(p('router').problem).toMatchObject({ status: 405, code: 'method_not_allowed' })
    expect(p('router').headers.join(' ')).toContain('Allow:')
    expect(p('panic').problem).toMatchObject({ status: 500, code: 'internal' })
    expect(p('decode').problem).toMatchObject({ status: 413, code: 'body_too_large' })
  })

  it('у 500 detail не содержит текста исходной ошибки', () => {
    const s = problems.find((x) => x.state.source === 'panic')!.state
    expect(String(s.problem!.detail)).not.toContain(s.error)
    expect(JSON.stringify(s.problem)).not.toContain('runtime error')
  })
})

it('все термины в подписях есть в глоссарии', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  const ids = steps.flatMap((s) => captionTermIds(s.caption))
  expect(ids.length).toBeGreaterThanOrEqual(2)
  for (const id of ids) expect(known.has(id)).toBe(true)
})
