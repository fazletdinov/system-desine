import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { GATES, INPUTS, validationGatesScenario, type Gate } from './validation-gates'

const steps = validationGatesScenario()
const lastOf = (input: string) => steps.filter((s) => s.state.input === input).at(-1)!

describe('validationGatesScenario', () => {
  it('от 6 до 10 шагов, у каждого непустая подпись', () => {
    expect(steps.length).toBeGreaterThanOrEqual(6)
    expect(steps.length).toBeLessThanOrEqual(10)
    for (const s of steps) expect(captionText(s.caption).trim().length).toBeGreaterThan(20)
  })

  it.each<[keyof typeof INPUTS, Gate]>([
    ['blank', 'value-object'],
    ['tooLong', 'value-object'],
    ['wrongType', 'json'],
    ['stranger', 'aggregate'],
  ])('история %s останавливается на воротах %s', (key, gate) => {
    const last = lastOf(INPUTS[key])
    expect(last.state.gate).toBe(gate)
    expect(last.state.verdict).toBe('reject')
    expect(last.state.reason).toBeTruthy()
  })

  it('четыре прогона идут подряд в заданном порядке', () => {
    const order = [...new Set(steps.map((s) => s.state.input))]
    expect(order.slice(0, 4)).toEqual([INPUTS.blank, INPUTS.tooLong, INPUTS.wrongType, INPUTS.stranger])
  })

  it('не-участник проходит объект-значение и падает только на агрегате', () => {
    const own = steps.filter((s) => s.state.input === INPUTS.stranger)
    expect(own.some((s) => s.state.gate === 'value-object' && s.state.verdict === 'pass')).toBe(true)
  })

  it('отказ бывает только на последних воротах шага: дальше отклонённый ввод не идёт', () => {
    const order = GATES.map((g) => g.id)
    for (const input of Object.values(INPUTS)) {
      const own = steps.filter((s) => s.state.input === input)
      const rejectAt = own.findIndex((s) => s.state.verdict === 'reject')
      if (rejectAt === -1) continue
      expect(rejectAt).toBe(own.length - 1)
      for (const s of own) expect(order.indexOf(s.state.gate)).toBeLessThanOrEqual(order.indexOf(own[rejectAt]!.state.gate))
    }
  })

  it('длинный текст: подпись говорит про руны и 8194 байта', () => {
    const text = steps.filter((s) => s.state.input === INPUTS.tooLong).map((s) => captionText(s.caption)).join(' ')
    expect(text).toContain('4097')
    expect(text).toContain('8194')
  })
})

it('все термины в подписях есть в глоссарии', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  const ids = steps.flatMap((s) => captionTermIds(s.caption))
  expect(ids.length).toBeGreaterThanOrEqual(3)
  for (const id of ids) expect(known.has(id)).toBe(true)
})
