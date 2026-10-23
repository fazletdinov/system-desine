import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { CODE_LINE, LATENCY_MS, NODES, REQUEST_PATH_CONTEXT, REQUEST_PATH_HONESTY, REQUEST_PATH_REQUEST, TOTAL_MS, requestPathScenario } from './request-path'

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
      expect(captionText(step.caption).trim().length).toBeGreaterThan(20)
      const onServer = step.state.active === 'app' || step.state.active === 'db'
      if (!onServer) expect(step.codeLine).toBeUndefined()
    }
  })

  it('итоговое время упомянуто в последнем пояснении', () => {
    expect(captionText(steps.at(-1)!.caption)).toContain('144')
  })
})

it('все термины в подписях есть в глоссарии, и они есть хотя бы в пяти шагах', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  const ids = steps.flatMap((s) => captionTermIds(s.caption))
  expect(ids.length).toBeGreaterThanOrEqual(5)
  for (const id of ids) expect(known.has(id)).toBe(true)
})

it.each([
  [2, 'ttl'],
  [7, 'certificate'],
  [9, 'data-center'],
  [10, 'bearer-token'],
])('шаг %i объясняет термин %s', (no, id) => {
  expect(captionTermIds(steps[no - 1]!.caption)).toContain(id)
})

describe('строки контекста и честности', () => {
  const lesson = readFileSync(new URL('../../modules/00-foundation/03-request-path.md', import.meta.url), 'utf8')

  it('запрос над схемой совпадает с HTTP-запросом урока 0.3', () => {
    const httpBlock = /```http\n([\s\S]*?)\n```/.exec(lesson)?.[1] ?? ''
    const [requestLine = ''] = httpBlock.split('\n')
    const body = httpBlock.split('\n').at(-1) ?? ''
    const [method = '', path = ''] = requestLine.split(' ')
    expect(REQUEST_PATH_REQUEST).toBe(`${method} ${path} ${body}`)
    expect(REQUEST_PATH_CONTEXT).toContain('соединения ещё нет')
  })

  it('подпись честности называет все модельные задержки и не выдаёт их за замер', () => {
    for (const ms of ['20', '40', '0,5', '2']) expect(REQUEST_PATH_HONESTY).toContain(`${ms}\u00A0мс`)
    expect(REQUEST_PATH_HONESTY).toContain('модель')
    expect(REQUEST_PATH_HONESTY).toContain('не замер')
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
