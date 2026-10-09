import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { CODE_LINE, LATENCY_MS, NODES, TOTAL_MS, requestPathScenario } from './request-path'

const steps = requestPathScenario()
const nodeIds = new Set(NODES.map((n) => n.id))

describe('requestPathScenario', () => {
  it('состоит из 15 шагов', () => {
    expect(steps).toHaveLength(15)
  })

  it('начинается на клиенте без пакета и с нулевым временем', () => {
    expect(steps[0]?.state).toEqual({ active: 'client', packet: null, elapsedMs: 0 })
  })

  it('время не убывает от шага к шагу', () => {
    for (let i = 1; i < steps.length; i++) {
      expect(steps[i]!.state.elapsedMs).toBeGreaterThanOrEqual(steps[i - 1]!.state.elapsedMs)
    }
  })

  it('пакет всегда прибывает в активный узел и ходит между разными существующими узлами', () => {
    for (const { state } of steps) {
      expect(nodeIds.has(state.active)).toBe(true)
      if (state.packet === null) continue
      expect(state.packet.to).toBe(state.active)
      expect(state.packet.from).not.toBe(state.packet.to)
      expect(nodeIds.has(state.packet.from)).toBe(true)
    }
  })

  it('заканчивается ответом на клиенте, итоговое время = сумма задержек', () => {
    const last = steps.at(-1)!
    expect(last.state.active).toBe('client')
    expect(last.state.packet?.kind).toBe('response')
    expect(last.state.elapsedMs).toBeCloseTo(TOTAL_MS)
    expect(TOTAL_MS).toBeCloseTo(LATENCY_MS.dnsLookup + 3 * LATENCY_MS.rtt + 4 * LATENCY_MS.internalHop + LATENCY_MS.diskFsync)
  })

  it('у каждого шага есть пояснение; строка кода — только на сервере приложения и в БД', () => {
    for (const step of steps) {
      expect(step.caption.trim().length).toBeGreaterThan(20)
      const onServer = step.state.active === 'app' || step.state.active === 'db'
      if (!onServer) expect(step.codeLine).toBeUndefined()
    }
  })

  it('итоговое время упомянуто в последнем пояснении', () => {
    expect(steps.at(-1)!.caption).toContain('144')
  })
})

describe('CODE_LINE и Go-код урока 0.3', () => {
  const lesson = readFileSync(new URL('../../modules/00-foundation/03-request-path.md', import.meta.url), 'utf8')
  const goBlock = /```go\n([\s\S]*?)\n```/.exec(lesson)?.[1] ?? ''
  const lines = goBlock.split('\n')

  it.each([
    ['entry', CODE_LINE.entry, 'func'],
    ['auth', CODE_LINE.auth, 'auth.UserID'],
    ['execute', CODE_LINE.execute, 'h.send.Execute'],
    ['respond', CODE_LINE.respond, 'WriteHeader'],
  ])('строка %s (№ %i) содержит %s', (_name, line, needle) => {
    expect(lines[line - 1]).toContain(needle)
  })
})
