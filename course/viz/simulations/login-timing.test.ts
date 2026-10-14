import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { LOGIN_HIGHLIGHT_IDS, loginTimingScenario } from './login-timing'

const steps = loginTimingScenario()
const pick = (impl: 'naive' | 'safe', attempt: 'unknown-user' | 'wrong-password') => steps.find((s) => s.state.impl === impl && s.state.attempt === attempt)!.state

describe('loginTimingScenario', () => {
  it('от 8 до 10 кадров, у каждого подпись и шаги', () => {
    expect(steps.length).toBeGreaterThanOrEqual(8)
    expect(steps.length).toBeLessThanOrEqual(10)
    for (const s of steps) {
      expect(captionText(s.caption).trim().length).toBeGreaterThan(20)
      expect(s.state.steps.length).toBeGreaterThan(0)
    }
  })

  it('сначала naive, потом safe', () => {
    const impls = steps.map((s) => s.state.impl)
    expect(impls.indexOf('safe')).toBeGreaterThan(impls.lastIndexOf('naive'))
  })

  it('каждая подсветка - существующий id', () => {
    for (const s of steps) for (const h of s.highlight ?? []) expect(LOGIN_HIGHLIGHT_IDS).toContain(h)
  })

  it('naive: ответы различаются по тексту или статусу, время - больше чем в 10 раз', () => {
    const u = pick('naive', 'unknown-user')
    const w = pick('naive', 'wrong-password')
    expect(u.response.status !== w.response.status || u.response.text !== w.response.text).toBe(true)
    expect(w.elapsedMs).toBeGreaterThan(u.elapsedMs * 10)
  })

  it('safe: одинаковые статус, код и текст; время отличается не больше чем на 20%', () => {
    for (const s of steps.filter((x) => x.state.impl === 'safe')) expect(s.state.response).toEqual(pick('safe', 'unknown-user').response)
    const u = pick('safe', 'unknown-user')
    const w = pick('safe', 'wrong-password')
    expect(w.response).toEqual(u.response)
    expect(u.response.status).toBe(401)
    expect(u.response.code).toBe('invalid_credentials')
    expect(Math.abs(u.elapsedMs - w.elapsedMs) / Math.max(u.elapsedMs, w.elapsedMs)).toBeLessThanOrEqual(0.2)
  })
})

it('все термины в подписях есть в глоссарии', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  const ids = steps.flatMap((s) => captionTermIds(s.caption))
  expect(ids.length).toBeGreaterThanOrEqual(2)
  for (const id of ids) expect(known.has(id)).toBe(true)
})
