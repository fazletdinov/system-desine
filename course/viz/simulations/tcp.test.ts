import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds } from '../core/caption'
import { DEFAULT_TCP, simulateTcp, tcpSimulation, type TcpState } from './tcp'

const state = (over: Partial<TcpState> = {}): TcpState => ({ ...tcpSimulation.initial(), ...over })
const events = (s: TcpState) => simulateTcp(s).flatMap((f) => f.events.map((e) => ({ tick: f.tick, e })))
const lastTick = (s: TcpState) => simulateTcp(s).at(-1)!.tick

describe('simulateTcp', () => {
  it('A: без потерь 8 сегментов окном 4 доходят за 4 тика без повторов', () => {
    const s = state()
    expect(lastTick(s)).toBe(4)
    expect(events(s).some((x) => x.e === 'fast-retransmit' || x.e === 'timeout')).toBe(false)
    expect(simulateTcp(s).at(-1)!.events).toContain('done')
  })

  it('B: потеря сегмента 2 при окне 4 — быстрый повтор на тике 4, без таймаута', () => {
    const s = state({ dropSegments: [2] })
    expect(events(s)).toContainEqual({ tick: 4, e: 'fast-retransmit' })
    expect(events(s).some((x) => x.e === 'timeout')).toBe(false)
    expect(lastTick(s)).toBe(8)
  })

  it('C: окно 2 и потеря сегмента 1 — дубликатов не хватает, повтор по таймеру', () => {
    const s = state({ config: { ...DEFAULT_TCP, window: 2 }, dropSegments: [1] })
    const ev = events(s)
    expect(ev).toContainEqual({ tick: 8, e: 'timeout' })
    expect(ev.some((x) => x.e === 'fast-retransmit')).toBe(false)
    expect(simulateTcp(s).at(-1)!.events).toContain('done')
  })

  it('D: потеря ACK 2 покрывается следующим кумулятивным ACK — повторов нет', () => {
    const s = state({ dropAcks: [2] })
    expect(events(s).some((x) => x.e === 'fast-retransmit' || x.e === 'timeout')).toBe(false)
    expect(lastTick(s)).toBe(4)
  })

  it('всегда заканчивается не позже ограничителя', () => {
    const s = state({ config: { ...DEFAULT_TCP, window: 1 }, dropSegments: [0, 1, 2, 3, 4, 5, 6, 7] })
    expect(simulateTcp(s).at(-1)!.events).toContain('done')
  })
})

describe('tcpSimulation.apply', () => {
  it('повторное «потерять» снимает потерю, окно ограничено 1..8, reset возвращает начальное', () => {
    let s = tcpSimulation.initial()
    s = tcpSimulation.apply(s, { type: 'drop-segment', seq: 3 })
    expect(s.dropSegments).toEqual([3])
    s = tcpSimulation.apply(s, { type: 'drop-segment', seq: 3 })
    expect(s.dropSegments).toEqual([])
    expect(tcpSimulation.apply(s, { type: 'window', size: 20 }).config.window).toBe(8)
    expect(tcpSimulation.apply(s, { type: 'window', size: 0 }).config.window).toBe(1)
    expect(tcpSimulation.apply({ ...s, dropAcks: [1] }, { type: 'reset' })).toEqual(tcpSimulation.initial())
  })
})

describe('шаги', () => {
  it('у каждого шага есть подпись, все термины — из глоссария', () => {
    const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
    const steps = tcpSimulation.steps(state({ dropSegments: [2] }))
    expect(steps.length).toBeGreaterThan(3)
    for (const st of steps) for (const id of captionTermIds(st.caption)) expect(known.has(id)).toBe(true)
  })
})
