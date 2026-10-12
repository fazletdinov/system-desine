import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { HEADER_BYTES, PAYLOAD_BYTES, TOTAL_BYTES, encapsulationScenario, totalBytes } from './encapsulation'

const steps = encapsulationScenario()

describe('encapsulationScenario', () => {
  it('состоит из 6 шагов', () => {
    expect(steps).toHaveLength(6)
  })

  it('на шаге 0 только HTTP-запрос в 200 байт', () => {
    expect(steps[0]!.state.layers.map((l) => l.id)).toEqual(['http'])
    expect(totalBytes(steps[0]!.state.layers)).toBe(200)
  })

  it('размер на шаге k = 200 + заголовки добавленных слоёв', () => {
    const order = ['tls', 'tcp', 'ip', 'eth'] as const
    let expected = PAYLOAD_BYTES
    for (let k = 1; k <= 4; k++) {
      expected += HEADER_BYTES[order[k - 1]!]
      expect(totalBytes(steps[k]!.state.layers)).toBe(expected)
    }
  })

  it('размеры слоёв: TLS 22, TCP 32, IP 20, Ethernet 18', () => {
    expect(HEADER_BYTES).toEqual({ tls: 22, tcp: 32, ip: 20, eth: 18 })
  })

  it('итог: 292 байта, подпись упоминает 292 и около 68%', () => {
    const last = steps.at(-1)!
    expect(TOTAL_BYTES).toBe(200 + 22 + 32 + 20 + 18)
    expect(totalBytes(last.state.layers)).toBe(292)
    const text = captionText(last.caption)
    expect(text).toContain('292')
    expect(text).toContain('68%')
  })
})

it('все термины в подписях есть в глоссарии', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  const ids = steps.flatMap((s) => captionTermIds(s.caption))
  expect(ids.length).toBeGreaterThanOrEqual(4)
  for (const id of ids) expect(known.has(id)).toBe(true)
})
