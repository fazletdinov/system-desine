import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { DEFAULT_TCP, MAX_TICKS, simulateTcp, tcpSimulation, type TcpState } from './tcp'

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
    expect(lastTick(s)).toBeLessThan(MAX_TICKS)
  })

  it('окно 8 и потеря сегмента 0 — ровно один быстрый повтор за весь прогон', () => {
    const s = state({ config: { ...DEFAULT_TCP, window: 8 }, dropSegments: [0] })
    expect(events(s).filter((x) => x.e === 'fast-retransmit')).toHaveLength(1)
    expect(simulateTcp(s).at(-1)!.events).toContain('done')
  })
})

const timeoutCaption = (s: TcpState): string => {
  const step = tcpSimulation.steps(s).find((st) => st.state.events.includes('timeout'))
  expect(step).toBeDefined()
  return captionText(step!.caption)
}
const captions = (s: TcpState): string[] => tcpSimulation.steps(s).map((st) => captionText(st.caption))

describe('подписи', () => {
  it('таймаут при малом окне: причина — мало сегментов после потери, RTO из конфигурации', () => {
    const text = timeoutCaption(state({ config: { ...DEFAULT_TCP, window: 2, rto: 5 }, dropSegments: [1] }))
    expect(text).toContain('меньше трёх')
    expect(text).toContain('5 тиков')
    expect(text).not.toContain('потерян ACK')
  })

  it('таймаут при потере хвоста потока (сегмент 7, окно 4): причина — мало сегментов', () => {
    const text = timeoutCaption(state({ dropSegments: [7] }))
    expect(text).toContain('меньше трёх')
    expect(text).not.toContain('окно слишком маленькое')
  })

  it('таймаут после потери последнего ACK: причина — потерян ACK', () => {
    const text = timeoutCaption(state({ dropAcks: [8] }))
    expect(text).toContain('Потерян ACK')
    expect(text).not.toContain('меньше трёх')
  })

  it('подпись называет потерянный сегмент, кумулятивный смысл ACK и простой', () => {
    const all = captions(state({ config: { ...DEFAULT_TCP, window: 2 }, dropSegments: [1] }))
    expect(all.some((t) => t.includes('Сегмент 1 потерян в сети'))).toBe(true)
    expect(all.some((t) => t.includes('подтверждает всё до сегмента 0 включительно'))).toBe(true)
    expect(all.some((t) => t.includes('отправитель ждёт') && t.includes('До таймера осталось'))).toBe(true)
  })

  it('согласует число при буфере вне порядка: один сегмент и несколько', () => {
    const one = captions(state({ config: { ...DEFAULT_TCP, window: 2 }, dropSegments: [0] }))
    expect(one.some((t) => t.includes('Сегмент 1 пришёл не по порядку и лежит в буфере'))).toBe(true)
    const many = captions(state({ dropSegments: [1] }))
    expect(many.some((t) => t.includes('Сегменты 2, 3 пришли не по порядку и лежат в буфере'))).toBe(true)
  })

  it('потерянный ACK назван в подписи', () => {
    expect(captions(state({ dropAcks: [2] })).some((t) => t.includes('ACK 2 потерян в сети'))).toBe(true)
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
