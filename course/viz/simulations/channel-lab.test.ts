import { describe, expect, it } from 'vitest'
import { channelLab as sim, type ChannelLabAction, type ChannelLabState } from './channel-lab'

const run = (cap: 0 | 1 | 3, ...actions: ChannelLabAction[]): ChannelLabState =>
  actions.reduce(sim.apply, sim.apply(sim.initial(), { type: 'setCap', cap }))
const g = (s: ChannelLabState, id: string) => s.gs.find((x) => x.id === id)!
const send = (id: string, value: number): ChannelLabAction => ({ type: 'send', g: id, value })
const recv = (id: string): ChannelLabAction => ({ type: 'recv', g: id })
const close = (id: string): ChannelLabAction => ({ type: 'close', g: id })
const lastLog = (s: ChannelLabState) => s.log.at(-1)!

describe('channel-lab', () => {
  it('cap=0: ждущий получатель получает значение напрямую, буфер пуст', () => {
    const s = run(0, recv('G2'), send('G1', 7))
    expect(s.buf).toEqual([])
    expect(g(s, 'G2').got).toEqual({ value: 7, ok: true })
    expect(g(s, 'G2').state).toBe('running')
    expect(lastLog(s).kind).toBe('handoff')
  })

  it('cap=0: отправка без получателя блокирует отправителя', () => {
    const s = run(0, send('G1', 7))
    expect(g(s, 'G1').state).toBe('blocked-send')
    expect(s.sendq).toEqual([{ g: 'G1', value: 7 }])
  })

  it('cap=1: первая отправка уходит в буфер, вторая блокирует отправителя', () => {
    const s1 = run(1, send('G1', 1))
    expect(s1.buf).toEqual([1])
    expect(g(s1, 'G1').state).toBe('running')
    expect(lastLog(s1).kind).toBe('buffered')
    const s2 = sim.apply(s1, send('G2', 2))
    expect(g(s2, 'G2').state).toBe('blocked-send')
    expect(s2.sendq).toHaveLength(1)
  })

  it('получение из полного буфера: голова отдаётся, значение отправителя встаёт в хвост, отправитель просыпается', () => {
    const s = run(1, send('G1', 1), send('G2', 2), recv('G3'))
    expect(g(s, 'G3').got).toEqual({ value: 1, ok: true })
    expect(s.buf).toEqual([2])
    expect(s.sendq).toEqual([])
    expect(g(s, 'G2').state).toBe('running')
    expect(lastLog(s).kind).toBe('woke')
  })

  it('FIFO сохраняется: буфер cap=3 и двое ждущих отправителей', () => {
    const s = run(3, send('G1', 1), send('G1', 2), send('G1', 3), send('G2', 4), send('G3', 5), recv('G4'), recv('G4'))
    expect(s.buf).toEqual([3, 4, 5])
    expect(s.sendq).toEqual([])
  })

  it('получение из пустого канала блокирует получателя', () => {
    const s = run(0, recv('G2'))
    expect(g(s, 'G2').state).toBe('blocked-recv')
    expect(s.recvq).toEqual(['G2'])
  })

  it('close будит всех ждущих получателей значением null и ok=false', () => {
    const s = run(0, recv('G2'), recv('G3'), close('G1'))
    expect(s.closed).toBe(true)
    expect(s.recvq).toEqual([])
    for (const id of ['G2', 'G3']) {
      expect(g(s, id).got).toEqual({ value: null, ok: false })
      expect(g(s, id).state).toBe('running')
    }
  })

  it('close паникует ждущих отправителей: send on closed channel', () => {
    const s = run(0, send('G2', 5), close('G1'))
    expect(g(s, 'G2').state).toBe('panicked')
    expect(s.sendq).toEqual([])
    expect(s.log.some((l) => l.kind === 'panic' && l.text.includes('send on closed channel'))).toBe(true)
  })

  it('закрытый канал с данными: сначала данные с ok=true, затем ноль с ok=false', () => {
    const s = run(3, send('G1', 1), send('G1', 2), close('G1'), recv('G2'), recv('G3'), recv('G4'))
    expect(g(s, 'G2').got).toEqual({ value: 1, ok: true })
    expect(g(s, 'G3').got).toEqual({ value: 2, ok: true })
    expect(g(s, 'G4').got).toEqual({ value: null, ok: false })
    expect(lastLog(s).kind).toBe('zero')
  })

  it('отправка в закрытый канал — паника send on closed channel', () => {
    const s = run(1, close('G1'), send('G2', 1))
    expect(g(s, 'G2').state).toBe('panicked')
    expect(lastLog(s)).toMatchObject({ kind: 'panic', text: expect.stringContaining('send on closed channel') })
  })

  it('повторный close — паника close of closed channel', () => {
    const s = run(0, close('G1'), close('G2'))
    expect(g(s, 'G2').state).toBe('panicked')
    expect(lastLog(s).text).toContain('close of closed channel')
  })

  it('close nil-канала — паника close of nil channel', () => {
    const s = run(0, { type: 'setNil', nil: true }, close('G1'))
    expect(g(s, 'G1').state).toBe('panicked')
    expect(lastLog(s).text).toContain('close of nil channel')
  })

  it('nil-канал: и отправка, и получение блокируют навсегда', () => {
    const s = run(3, { type: 'setNil', nil: true }, send('G1', 1), recv('G2'))
    expect(g(s, 'G1').state).toBe('blocked-send')
    expect(g(s, 'G2').state).toBe('blocked-recv')
    expect(s.buf).toEqual([])
    expect(s.sendq).toEqual([])
    expect(s.recvq).toEqual([])
  })

  it('заблокированная горутина не принимает действий: состояние то же, в журнале «G2 заблокирована»', () => {
    const before = run(0, recv('G2'))
    const after = sim.apply(before, send('G2', 9))
    expect({ ...after, log: before.log }).toEqual(before)
    expect(lastLog(after).text).toContain('G2 заблокирована')
  })

  it('select с default на полном буфере не блокирует', () => {
    const full = run(1, send('G1', 1))
    const s = sim.apply(full, { type: 'selectDefault', g: 'G2', op: 'send', value: 2 })
    expect(g(s, 'G2').state).toBe('running')
    expect(s.sendq).toEqual([])
    expect(s.buf).toEqual([1])
    expect(lastLog(s).kind).toBe('default')
  })

  it('select с default на пустом канале не блокирует', () => {
    const s = run(0, { type: 'selectDefault', g: 'G2', op: 'recv' })
    expect(g(s, 'G2').state).toBe('running')
    expect(s.recvq).toEqual([])
    expect(lastLog(s).kind).toBe('default')
  })

  it('select с default выполняет операцию, если канал готов', () => {
    const s = run(1, { type: 'selectDefault', g: 'G1', op: 'send', value: 4 })
    expect(s.buf).toEqual([4])
    expect(lastLog(s).kind).toBe('buffered')
  })

  it('смена ёмкости и reset возвращают чистое состояние', () => {
    const dirty = run(1, send('G1', 1), close('G2'))
    expect(sim.apply(dirty, { type: 'setCap', cap: 3 })).toMatchObject({ cap: 3, closed: false, buf: [] })
    expect(sim.apply(dirty, { type: 'reset' })).toMatchObject({ cap: 1, closed: false, buf: [], log: [] })
  })

  it('steps отдаёт один кадр с подписью', () => {
    expect(sim.steps(sim.initial())).toHaveLength(1)
  })
})
