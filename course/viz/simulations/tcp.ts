import { term, type CaptionSegment } from '../core/caption'
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
  rto: number
  // Сколько тиков осталось до таймера; null — в полёте ничего нет, таймер не идёт.
  timerLeft: number | null
  // Почему повтор пошёл по таймеру, а не по дубликатам ACK.
  timeoutCause: TimeoutCause | null
}

export type TimeoutCause = 'few-dups' | 'ack-lost'

export const DEFAULT_TCP: TcpConfig = { segments: 8, window: 4, rto: 6 }
export const MAX_TICKS = 80
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
  let fastRetransmittedFor = -1
  const lostAcks: number[] = []
  const received = new Set<number>()
  let inTransit: Packet[] = []
  const frames: TcpFrame[] = []

  for (let tick = 0; tick <= MAX_TICKS; tick++) {
    const events: TcpEvent[] = []
    const arrived = inTransit.filter((p) => !p.lost)
    const sent: Packet[] = []
    let timeoutCause: TimeoutCause | null = null
    const send = (p: Omit<Packet, 'lost'>, lost: boolean): void => {
      sent.push({ ...p, lost })
    }

    // 1. Получатель: принимает сегменты и отвечает кумулятивным ACK на каждый.
    for (const p of arrived.filter((x) => x.kind === 'segment').sort((a, b) => a.n - b.n)) {
      received.add(p.n)
      while (received.has(expected)) expected++
      events.push('deliver')
      const loseAck = state.dropAcks.includes(expected) && !ackLossUsed.has(expected)
      if (loseAck) {
        ackLossUsed.add(expected)
        lostAcks.push(expected)
      }
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
        // Один быстрый повтор на один base: дальнейшие дубликаты того же ACK его не повторяют.
        if (dupAcks >= DUP_ACK_THRESHOLD && fastRetransmittedFor !== base) {
          send({ kind: 'segment', n: base, retransmit: true }, false)
          events.push('fast-retransmit')
          fastRetransmittedFor = base
          timerStart = tick
        }
      }
    }

    // 3. Таймер: самый старый неподтверждённый сегмент не подтверждён за RTO тиков.
    if (base < nextSeq && tick - timerStart >= rto && !events.includes('fast-retransmit')) {
      send({ kind: 'segment', n: base, retransmit: true }, false)
      events.push('timeout')
      timeoutCause = lostAcks.some((n) => n > base) ? 'ack-lost' : 'few-dups'
      dupAcks = 0
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
    const timerLeft = base < nextSeq ? Math.max(0, rto - (tick - timerStart)) : null
    frames.push({ tick, base, nextSeq, window, expected, received: [...received].sort((a, b) => a - b), dupAcks, sent, arrived, events, rto, timerLeft, timeoutCause })
    if (done) break
    inTransit = sent
  }
  return frames
}

function plural(n: number, one: string, few: string, many: string): string {
  const m10 = n % 10
  const m100 = n % 100
  if (m10 === 1 && m100 !== 11) return one
  if (m10 >= 2 && m10 <= 4 && (m100 < 10 || m100 >= 20)) return few
  return many
}

const TIMEOUT_CAUSE_TEXT: Record<TimeoutCause, string> = {
  'few-dups': 'После потерянного сегмента ушло меньше трёх следующих (окно маленькое или это хвост потока), поэтому три дубликата ACK не набрались. ',
  'ack-lost': 'Потерян ACK, а новых сегментов, которые вызвали бы дубликаты, уже нет. ',
}

function caption(f: TcpFrame): CaptionSegment[] {
  const parts: CaptionSegment[] = [{ text: `Тик ${f.tick}. ` }]
  if (f.tick === 0) {
    parts.push(
      { text: 'Учебная модель: без SACK и ' },
      term('slow start', 'slow-start'),
      { text: `, таймер — фиксированные ${f.rto} тиков, а не формула из RFC. ` },
    )
  }
  if (f.events.includes('ack')) {
    parts.push({ text: `Пришёл ACK ${f.base}: он подтверждает всё до сегмента ${f.base - 1} включительно. ` })
  }
  if (f.events.includes('fast-retransmit')) {
    parts.push({ text: `Дубликатов ACK ${f.base} подряд уже ${f.dupAcks} (порог — ${DUP_ACK_THRESHOLD}) — отправитель не ждёт таймера: ` }, term('быстрый повтор', 'retransmission'), { text: ` сегмента ${f.base}. ` })
  } else if (f.events.includes('dup-ack')) {
    parts.push({ text: `Пришёл дубликат ACK ${f.base} (дубликатов: ${f.dupAcks}): получатель всё ещё ждёт сегмент ${f.base}. ` })
  }
  if (f.events.includes('timeout')) {
    parts.push({ text: `Подтверждения нет уже ${f.rto} ${plural(f.rto, 'тик', 'тика', 'тиков')} — повтор сегмента ${f.base} ` }, term('по таймеру', 'retransmission'), { text: '. ' })
    if (f.timeoutCause) parts.push({ text: TIMEOUT_CAUSE_TEXT[f.timeoutCause] })
  }
  const fresh = f.sent.filter((p) => p.kind === 'segment' && !p.retransmit).map((p) => p.n)
  if (fresh.length > 0) parts.push({ text: `Отправлены сегменты ${fresh.join(', ')} — столько, сколько позволяет окно (${f.window}). ` })
  if (f.events.includes('deliver')) {
    const buffered = f.received.filter((n) => n > f.expected)
    parts.push({ text: `Получатель ждёт сегмент ${f.expected} и отвечает ACK ${f.expected}. ` })
    if (buffered.length > 0) parts.push({ text: `${plural(buffered.length, 'Сегмент', 'Сегменты', 'Сегменты')} ${buffered.join(', ')} ${plural(buffered.length, 'пришёл', 'пришли', 'пришли')} не по порядку и ${plural(buffered.length, 'лежит', 'лежат', 'лежат')} в буфере. ` })
  }
  for (const p of f.sent.filter((x) => x.lost)) {
    parts.push({ text: p.kind === 'segment' ? `Сегмент ${p.n} потерян в сети. ` : `ACK ${p.n} потерян в сети. ` })
  }
  if (f.events.length === 0) {
    const full = f.nextSeq - f.base >= f.window
    parts.push({ text: `Ничего не пришло: отправитель ждёт${full ? ' — окно заполнено' : ''}. ` })
    if (f.timerLeft !== null) parts.push({ text: `До таймера осталось ${f.timerLeft} ${plural(f.timerLeft, 'тик', 'тика', 'тиков')}. ` })
  }
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
