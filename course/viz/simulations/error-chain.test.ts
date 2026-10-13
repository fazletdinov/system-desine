import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { CHAT_NOT_FOUND_TEXT, errorChainScenario } from './error-chain'

const steps = errorChainScenario()
// Первая история — «чат не найден»; вторая начинается с первой ValidationError.
const split = steps.findIndex((s) => s.state.asValidation)
const notFound = steps.slice(0, split)
const validation = steps.slice(split)
const isPercentV = (s: (typeof steps)[number]): boolean => s.state.layers.some((l) => l.where.includes('%v'))

describe('errorChainScenario', () => {
  it('от 6 до 10 шагов, у каждого непустая подпись и хотя бы один слой', () => {
    expect(steps.length).toBeGreaterThanOrEqual(6)
    expect(steps.length).toBeLessThanOrEqual(10)
    for (const s of steps) {
      expect(captionText(s.caption).trim().length).toBeGreaterThan(20)
      expect(s.state.layers.length).toBeGreaterThan(0)
    }
  })

  it('есть обе истории', () => {
    expect(split).toBeGreaterThan(0)
    expect(notFound.length).toBeGreaterThan(0)
  })

  it('с %v errors.Is не видит ErrChatNotFound, на остальных шагах цепочки — видит', () => {
    const v = notFound.filter(isPercentV)
    expect(v).toHaveLength(1)
    expect(v[0]!.state.isNotFound).toBe(false)
    for (const s of notFound.filter((x) => !isPercentV(x))) expect(s.state.isNotFound).toBe(true)
  })

  it('isNotFound совпадает с тем, есть ли сигнальная ошибка в цепочке', () => {
    for (const s of steps) expect(s.state.isNotFound).toBe(s.state.layers.some((l) => l.message === CHAT_NOT_FOUND_TEXT))
  })

  it('с %v текст Error() тот же, что с %w, — разница не видна глазами', () => {
    const v = notFound.find(isPercentV)!
    const w = notFound.find((s) => s.state.layers.length > 1)!
    expect(v.state.layers[0]!.message).toBe(w.state.layers[0]!.message)
  })

  it('итоговые статусы: 404 для «не найден» и 400 для валидации', () => {
    expect(notFound.at(-1)!.state.status).toBe(404)
    expect(validation.at(-1)!.state.status).toBe(400)
    for (const s of validation) {
      expect(s.state.isNotFound).toBe(false)
      expect(s.state.asValidation).toBe(true)
    }
  })

  it('внешний конверт содержит текст всех внутренних', () => {
    for (const { state } of steps) {
      for (let i = 1; i < state.layers.length; i++) expect(state.layers[i - 1]!.message).toContain(state.layers[i]!.message)
    }
  })
})

it('все термины в подписях есть в глоссарии', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  const ids = steps.flatMap((s) => captionTermIds(s.caption))
  expect(ids.length).toBeGreaterThanOrEqual(2)
  for (const id of ids) expect(known.has(id)).toBe(true)
})
