import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { LOST_MESSAGE, MESSAGES, RTO_TICKS, udpVsTcpScenario } from './udp-vs-tcp'

const steps = udpVsTcpScenario()
const last = steps.at(-1)!.state
const doneTick = (lane: 'tcp' | 'udp'): number => steps.find((s) => s.state[lane].delivered.length === (lane === 'tcp' ? 6 : 5))!.state.tick

describe('udpVsTcpScenario', () => {
  it('6–10 шагов, тики идут подряд с нуля', () => {
    expect(steps.length).toBeGreaterThanOrEqual(6)
    expect(steps.length).toBeLessThanOrEqual(10)
    steps.forEach((s, i) => expect(s.state.tick).toBe(i))
  })

  it('константы модели', () => {
    expect([MESSAGES, LOST_MESSAGE, RTO_TICKS]).toEqual([6, 3, 4])
  })

  it('в итоге TCP доставил 1–6 по порядку', () => {
    expect(last.tcp.delivered).toEqual([1, 2, 3, 4, 5, 6])
  })

  it('UDP доставил 1, 2, 4, 5, 6, а 3 потеряно навсегда', () => {
    expect(last.udp.delivered).toEqual([1, 2, 4, 5, 6])
    expect(last.udp.lost).toEqual([3])
  })

  it('TCP заканчивает позже UDP', () => {
    expect(doneTick('tcp')).toBeGreaterThan(doneTick('udp'))
  })

  it('пока сообщение 3 не повторено, TCP не отдаёт 4–6, а держит их в буфере', () => {
    const blocked = steps.filter((s) => s.state.tcp.lost.includes(3) && s.state.tcp.buffered.length > 0)
    expect(blocked.length).toBeGreaterThan(0)
    for (const s of steps) {
      expect(s.state.tcp.delivered.every((m) => m < 3 || s.state.tcp.delivered.includes(3))).toBe(true)
    }
  })

  it('доставленное TCP всегда идёт по порядку без дыр', () => {
    for (const s of steps) expect(s.state.tcp.delivered).toEqual(s.state.tcp.delivered.map((_, i) => i + 1))
  })

  it('у шагов есть пояснения', () => {
    for (const s of steps) expect(captionText(s.caption).trim().length).toBeGreaterThan(20)
  })
})

it('все термины в подписях есть в глоссарии', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  const ids = steps.flatMap((s) => captionTermIds(s.caption))
  expect(ids.length).toBeGreaterThanOrEqual(4)
  for (const id of ids) expect(known.has(id)).toBe(true)
})
