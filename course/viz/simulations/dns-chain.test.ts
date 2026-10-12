import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { DNS_LATENCY_MS, FIRST_ANSWER_MS, NODES, SECOND_QUERY_MS, dnsChainScenario } from './dns-chain'

const steps = dnsChainScenario()
const nodeIds = new Set(NODES.map((n) => n.id))
const isAnswerToStub = (s: (typeof steps)[number]): boolean => s.state.packet?.kind === 'answer' && s.state.packet.to === 'stub'

describe('dnsChainScenario', () => {
  it('около 12 шагов', () => {
    expect(steps.length).toBeGreaterThanOrEqual(10)
    expect(steps.length).toBeLessThanOrEqual(14)
  })

  it('время не убывает', () => {
    for (let i = 1; i < steps.length; i++) {
      expect(steps[i]!.state.elapsedMs).toBeGreaterThanOrEqual(steps[i - 1]!.state.elapsedMs)
    }
  })

  it('пакет прибывает в активный узел между существующими разными узлами', () => {
    for (const { state } of steps) {
      expect(nodeIds.has(state.active)).toBe(true)
      if (state.packet === null) continue
      expect(state.packet.to).toBe(state.active)
      expect(state.packet.from).not.toBe(state.packet.to)
      expect(nodeIds.has(state.packet.from)).toBe(true)
    }
  })

  it('первый полный ответ — на 20 + 3·20 + 20 = 100 мс', () => {
    const first = steps.find(isAnswerToStub)!
    expect(first.state.elapsedMs).toBe(100)
    expect(FIRST_ANSWER_MS).toBe(DNS_LATENCY_MS.stubRecursive + 3 * 2 * DNS_LATENCY_MS.recursiveAuthoritative + DNS_LATENCY_MS.stubRecursive)
    expect(FIRST_ANSWER_MS).toBe(100)
  })

  it('второй запрос добавляет ровно 40 мс', () => {
    const answers = steps.filter(isAnswerToStub)
    expect(answers).toHaveLength(2)
    expect(answers[1]!.state.elapsedMs - answers[0]!.state.elapsedMs).toBe(40)
    expect(SECOND_QUERY_MS).toBe(40)
  })

  it('кэш пуст до ответа auth и содержит api.messenger.example после него', () => {
    const authIdx = steps.findIndex((s) => s.state.packet?.from === 'auth')
    expect(authIdx).toBeGreaterThan(0)
    for (const s of steps.slice(0, authIdx)) expect(s.state.cache).toEqual([])
    for (const s of steps.slice(authIdx + 1)) expect(s.state.cache.some((e) => e.includes('api.messenger.example'))).toBe(true)
  })

  it('у шагов есть пояснения; рефералы идут только от root и tld', () => {
    for (const s of steps) {
      expect(captionText(s.caption).trim().length).toBeGreaterThan(20)
      if (s.state.packet?.kind === 'referral') expect(['root', 'tld']).toContain(s.state.packet.from)
    }
  })
})

it('все термины в подписях есть в глоссарии', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  const ids = steps.flatMap((s) => captionTermIds(s.caption))
  expect(ids.length).toBeGreaterThanOrEqual(5)
  for (const id of ids) expect(known.has(id)).toBe(true)
})
