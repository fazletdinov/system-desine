import type { CaptionSegment } from '../core/caption'
import type { Simulation, Step } from '../core/types'

// Учебная модель TCP: задержка в одну сторону — 1 тик, кумулятивные ACK, быстрый повтор после трёх
// дубликатов, повтор по таймеру RTO. Нет SACK, slow start и точной формулы RTO — это упрощения,
// о которых урок говорит прямо.
export interface TcpConfig {
  segments: number
  window: number
  rto: number
}

export interface TcpState {
  config: TcpConfig
  dropSegments: readonly number[]
  dropAcks: readonly number[]
}

export type TcpAction =
  | { type: 'drop-segment'; seq: number }
  | { type: 'drop-ack'; ack: number }
  | { type: 'window'; size: number }
  | { type: 'reset' }

export type TcpEvent = 'send' | 'deliver' | 'ack' | 'dup-ack' | 'fast-retransmit' | 'timeout' | 'done'

export interface Packet {
  kind: 'segment' | 'ack'
  n: number
  lost: boolean
  retransmit: boolean
}

export interface TcpFrame {
  tick: number
  base: number
  nextSeq: number
  window: number
  expected: number
  received: readonly number[]
  dupAcks: number
  sent: readonly Packet[]
  arrived: readonly Packet[]
  events: readonly TcpEvent[]
}

export const DEFAULT_TCP: TcpConfig = { segments: 8, window: 4, rto: 6 }
const MAX_TICKS = 80
const DUP_ACK_THRESHOLD = 3

function toggle(list: readonly number[], n: number): number[] {
  return list.includes(n) ? list.filter((x) => x !== n) : [...list, n].sort((a, b) => a - b)
}

export function simulateTcp(state: TcpState): TcpFrame[] {
  const { segments, window, rto } = state.config
  const segLossUsed = new Set<number>()
  const ackLossUsed = new Set<number>()
  let base = 0
  let nextSeq = 0
  let expected = 0
  let dupAcks = 0
  let timerStart = 0
  const received = new Set<number>()
  let inTransit: Packet[] = []
  const frames: TcpFrame[] = []

  for (let tick = 0; tick <= MAX_TICKS; tick++) {
    const events: TcpEvent[] = []
    const arrived = inTransit.filter((p) => !p.lost)
    const sent: Packet[] = []
    const send = (p: Omit<Packet, 'lost'>, lost: boolean): void => {
      sent.push({ ...p, lost })
    }

    // 1. Получатель: принимает сегменты и отвечает кумулятивным ACK на каждый.
    for (const p of arrived.filter((x) => x.kind === 'segment').sort((a, b) => a.n - b.n)) {
      received.add(p.n)
      while (received.has(expected)) expected++
      events.push('deliver')
      const loseAck = state.dropAcks.includes(expected) && !ackLossUsed.has(expected)
      if (loseAck) ackLossUsed.add(expected)
      send({ kind: 'ack', n: expected, retransmit: false }, loseAck)
    }

    // 2. Отправитель: обрабатывает ACK.
    for (const p of arrived.filter((x) => x.kind === 'ack').sort((a, b) => a.n - b.n)) {
      if (p.n > base) {
        base = p.n
        dupAcks = 0
        timerStart = tick
        events.push('ack')
      } else if (p.n === base && base < nextSeq) {
        dupAcks++
        events.push('dup-ack')
        if (dupAcks === DUP_ACK_THRESHOLD) {
          send({ kind: 'segment', n: base, retransmit: true }, false)
          events.push('fast-retransmit')
          dupAcks = 0
          timerStart = tick
        }
      }
    }

    // 3. Таймер: самый старый неподтверждённый сегмент не подтверждён за RTO тиков.
    if (base < nextSeq && tick - timerStart >= rto && !events.includes('fast-retransmit')) {
      send({ kind: 'segment', n: base, retransmit: true }, false)
      events.push('timeout')
      timerStart = tick
    }

    // 4. Новые сегменты, пока позволяет окно.
    while (nextSeq < base + window && nextSeq < segments) {
      if (base === nextSeq) timerStart = tick
      const lose = state.dropSegments.includes(nextSeq) && !segLossUsed.has(nextSeq)
      if (lose) segLossUsed.add(nextSeq)
      send({ kind: 'segment', n: nextSeq, retransmit: false }, lose)
      events.push('send')
      nextSeq++
    }

    const done = base >= segments
    if (done) events.push('done')
    frames.push({ tick, base, nextSeq, window, expected, received: [...received].sort((a, b) => a - b), dupAcks, sent, arrived, events })
    if (done) break
    inTransit = sent
  }
  return frames
}

function caption(f: TcpFrame): CaptionSegment[] {
  const parts: CaptionSegment[] = [{ text: `Тик ${f.tick}. ` }]
  if (f.tick === 0) {
    parts.push({ text: 'Учебная модель: без SACK и slow start, RTO — фиксированное число тиков, а не формула из RFC. ' })
  }
  const segs = f.sent.filter((p) => p.kind === 'segment' && !p.retransmit).map((p) => p.n)
  if (f.events.includes('fast-retransmit')) {
    parts.push({ text: `Три дубликата ACK ${f.base} подряд — отправитель не ждёт таймера: ` }, { text: 'быстрый повтор', termId: 'retransmission' }, { text: ` сегмента ${f.base}. ` })
  } else if (f.events.includes('dup-ack')) {
    parts.push({ text: `Пришёл дубликат ACK ${f.base}: получатель всё ещё ждёт сегмент ${f.base}. ` })
  }
  if (f.events.includes('timeout')) {
    parts.push({ text: `Подтверждения нет уже ${DEFAULT_TCP.rto} тиков — повтор сегмента ${f.base} ` }, { text: 'по таймеру', termId: 'retransmission' }, { text: '. Дубликатов не хватило: окно слишком маленькое. ' })
  }
  if (segs.length > 0) parts.push({ text: `Отправлены сегменты ${segs.join(', ')} — столько, сколько позволяет окно (${f.window}). ` })
  if (f.events.includes('deliver')) parts.push({ text: `Получатель ждёт сегмент ${f.expected} и отвечает ACK ${f.expected}. ` })
  if (f.events.includes('done')) parts.push({ text: 'Все сегменты подтверждены.' })
  return parts
}

export const tcpSimulation: Simulation<TcpState, TcpAction, TcpFrame> = {
  initial: () => ({ config: { ...DEFAULT_TCP }, dropSegments: [], dropAcks: [] }),
  apply: (s, a) => {
    switch (a.type) {
      case 'drop-segment':
        return { ...s, dropSegments: toggle(s.dropSegments, a.seq) }
      case 'drop-ack':
        return { ...s, dropAcks: toggle(s.dropAcks, a.ack) }
      case 'window':
        return { ...s, config: { ...s.config, window: Math.min(8, Math.max(1, Math.trunc(a.size))) } }
      case 'reset':
        return tcpSimulation.initial()
    }
  },
  steps: (s) => simulateTcp(s).map((f): Step<TcpFrame> => ({ state: f, caption: caption(f) })),
}
