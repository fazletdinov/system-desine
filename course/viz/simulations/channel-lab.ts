import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Simulation, Step } from '../core/types'

// Модель одного канала Go: буфер, очереди ждущих отправителей (sendq) и получателей (recvq),
// как в runtime/chan.go. Тексты паник — дословно из рантайма.

export type GState = 'running' | 'blocked-send' | 'blocked-recv' | 'panicked' | 'done'
export type GId = 'G1' | 'G2' | 'G3' | 'G4'
export type Cap = 0 | 1 | 3

export const PANIC_SEND_CLOSED = 'send on closed channel'
export const PANIC_CLOSE_CLOSED = 'close of closed channel'
export const PANIC_CLOSE_NIL = 'close of nil channel'

export interface Goroutine {
  id: GId
  state: GState
  holding?: number // значение, с которым отправитель ждёт в sendq
  got?: { value: number | null; ok: boolean }
}

export type LogKind = 'handoff' | 'buffered' | 'blocked' | 'woke' | 'panic' | 'zero' | 'default'

export interface ChannelLabState {
  cap: Cap
  nil: boolean
  closed: boolean
  buf: number[]
  // Индексы кольцевого буфера, как recvx/sendx в hchan: откуда читать и куда писать; всегда по модулю cap.
  recvx: number
  sendx: number
  sendq: { g: string; value: number }[]
  recvq: string[]
  gs: Goroutine[]
  log: { kind: LogKind; text: string }[]
}

export type ChannelLabAction =
  | { type: 'send'; g: string; value: number }
  | { type: 'recv'; g: string }
  | { type: 'close'; g: string }
  | { type: 'setCap'; cap: Cap }
  | { type: 'setNil'; nil: boolean }
  | { type: 'selectDefault'; g: string; op: 'send' | 'recv'; value?: number }
  | { type: 'reset' }

const GOROUTINE_IDS: readonly GId[] = ['G1', 'G2', 'G3', 'G4']

const start = (cap: Cap, nil = false): ChannelLabState => ({
  cap,
  nil,
  closed: false,
  buf: [],
  recvx: 0,
  sendx: 0,
  sendq: [],
  recvq: [],
  gs: GOROUTINE_IDS.map((id) => ({ id, state: 'running' })),
  log: [],
})

const advance = (i: number, cap: number): number => (i + 1) % cap

const withLog = (s: ChannelLabState, kind: LogKind, text: string): ChannelLabState => ({ ...s, log: [...s.log, { kind, text }] })

const patchG = (s: ChannelLabState, id: string, patch: Partial<Goroutine>): ChannelLabState => ({
  ...s,
  gs: s.gs.map((g) => (g.id === id ? { ...g, ...patch } : g)),
})

// Снимает поля ожидания: значение в руках и результат прошлого получения.
const wake = (s: ChannelLabState, id: string, got?: Goroutine['got']): ChannelLabState => ({
  ...s,
  gs: s.gs.map((g) => {
    if (g.id !== id) return g
    const { holding: _holding, got: _got, ...rest } = g
    return got ? { ...rest, state: 'running', got } : { ...rest, state: 'running' }
  }),
})

const panic = (s: ChannelLabState, id: string, text: string): ChannelLabState =>
  withLog(patchG(s, id, { state: 'panicked', holding: undefined }), 'panic', `${id}: panic: ${text}`)

function send(s: ChannelLabState, id: string, value: number): ChannelLabState {
  if (s.nil) return withLog(patchG(s, id, { state: 'blocked-send', holding: value }), 'blocked', `${id} ждёт навсегда: отправка в nil-канал никогда не готова`)
  if (s.closed) return panic(s, id, PANIC_SEND_CLOSED)
  const [receiver, ...restRecv] = s.recvq
  if (receiver !== undefined) {
    const next = wake({ ...s, recvq: restRecv }, receiver, { value, ok: true })
    return withLog(next, 'handoff', `${id} → ${receiver}: ${value} передано напрямую, минуя буфер`)
  }
  if (s.buf.length < s.cap) return withLog({ ...s, buf: [...s.buf, value], sendx: advance(s.sendx, s.cap) }, 'buffered', `${id}: ${value} лёг в буфер (${s.buf.length + 1}/${s.cap})`)
  const blocked = patchG({ ...s, sendq: [...s.sendq, { g: id, value }] }, id, { state: 'blocked-send', holding: value })
  return withLog(blocked, 'blocked', `${id} заблокирована на отправке ${value}: ${s.cap === 0 ? 'нет получателя' : 'буфер полон'}`)
}

function recv(s: ChannelLabState, id: string): ChannelLabState {
  if (s.nil) return withLog(patchG(s, id, { state: 'blocked-recv' }), 'blocked', `${id} ждёт навсегда: получение из nil-канала никогда не готово`)
  const [head, ...restBuf] = s.buf
  const [sender, ...restSend] = s.sendq
  if (head !== undefined) {
    // Буфер не пуст: получатель берёт голову, а ждущий отправитель встаёт в хвост, чтобы сохранить FIFO.
    const recvx = advance(s.recvx, s.cap)
    if (sender === undefined) return withLog(wake({ ...s, buf: restBuf, recvx }, id, { value: head, ok: true }), 'buffered', `${id} взяла ${head} из буфера`)
    const sendx = advance(s.sendx, s.cap)
    const next = wake(wake({ ...s, buf: [...restBuf, sender.value], recvx, sendx, sendq: restSend }, id, { value: head, ok: true }), sender.g)
    return withLog(next, 'woke', `${id} взяла ${head} из буфера; ${sender.g} проснулась, её ${sender.value} встало в хвост буфера`)
  }
  if (sender !== undefined) {
    const next = wake(wake({ ...s, sendq: restSend }, id, { value: sender.value, ok: true }), sender.g)
    return withLog(next, 'handoff', `${sender.g} → ${id}: ${sender.value} передано напрямую; ${sender.g} проснулась`)
  }
  if (s.closed) return withLog(wake(s, id, { value: null, ok: false }), 'zero', `${id} прочитала из закрытого канала: нулевое значение, ok=false`)
  const blocked = patchG({ ...s, recvq: [...s.recvq, id] }, id, { state: 'blocked-recv' })
  return withLog(blocked, 'blocked', `${id} заблокирована на получении: канал пуст`)
}

function close(s: ChannelLabState, id: string): ChannelLabState {
  if (s.nil) return panic(s, id, PANIC_CLOSE_NIL)
  if (s.closed) return panic(s, id, PANIC_CLOSE_CLOSED)
  let next: ChannelLabState = withLog({ ...s, closed: true, recvq: [], sendq: [] }, 'woke', `${id} закрыла канал`)
  for (const r of s.recvq) next = withLog(wake(next, r, { value: null, ok: false }), 'woke', `${r} проснулась: нулевое значение, ok=false`)
  for (const w of s.sendq) next = panic(next, w.g, PANIC_SEND_CLOSED)
  return next
}

// Готов ли канал принять или отдать значение без блокировки. Закрытый канал готов к получению всегда,
// а к отправке «готов» паникой.
function ready(s: ChannelLabState, op: 'send' | 'recv'): boolean {
  if (s.nil) return false
  if (op === 'send') return s.closed || s.recvq.length > 0 || s.buf.length < s.cap
  return s.closed || s.buf.length > 0 || s.sendq.length > 0
}

function selectDefault(s: ChannelLabState, id: string, op: 'send' | 'recv', value: number): ChannelLabState {
  if (ready(s, op)) return op === 'send' ? send(s, id, value) : recv(s, id)
  return withLog(s, 'default', `${id}: select выбрал default — ${op === 'send' ? 'отправка' : 'получение'} заблокировала бы, ветка пропущена`)
}

export const channelLab: Simulation<ChannelLabState, ChannelLabAction> = {
  initial: () => start(0),
  apply: (s, a) => {
    switch (a.type) {
      case 'reset':
        return start(s.cap)
      case 'setCap':
        return start(a.cap)
      case 'setNil':
        return start(s.cap, a.nil)
      case 'send':
      case 'recv':
      case 'close':
      case 'selectDefault': {
        const g = s.gs.find((x) => x.id === a.g)
        if (!g) return s
        if (g.state === 'blocked-send' || g.state === 'blocked-recv') return withLog(s, 'blocked', `${g.id} заблокирована и ничего не делает, пока её не разбудят`)
        if (g.state !== 'running') return withLog(s, 'panic', `${g.id} уже не работает: ${g.state === 'panicked' ? 'паника' : 'завершена'}`)
        if (a.type === 'send') return send(s, a.g, a.value)
        if (a.type === 'recv') return recv(s, a.g)
        if (a.type === 'close') return close(s, a.g)
        return selectDefault(s, a.g, a.op, a.value ?? 0)
      }
    }
  },
  steps: (s) => [{ state: s, caption: captionFor(s) } satisfies Step<ChannelLabState>],
}

function captionFor(s: ChannelLabState): Caption {
  const kind: Caption = s.nil
    ? [{ text: 'Это nil-' }, term('канал', 'channel'), { text: ': любая отправка и любое получение ждут вечно, close паникует.' }]
    : s.cap === 0
      ? [term('Небуферизованный канал', 'unbuffered-channel'), { text: ' (ёмкость 0): отправка и получение встречаются — значение передаётся напрямую.' }]
      : [term('Буферизованный канал', 'buffered-channel'), { text: ` на ${s.cap}: отправитель ждёт, только когда буфер полон, получатель — когда он пуст.` }]
  const last = s.log.at(-1)
  const tail: Caption = last
    ? [{ text: ` ${last.text}.` }]
    : [{ text: ' Нажмите действие у любой горутины. Кнопки с ' }, term('select', 'select'), { text: ' и default показывают операцию без блокировки.' }]
  return [...kind, ...tail]
}
