import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Step } from '../core/types'

export interface Lane {
  delivered: readonly number[]
  buffered: readonly number[]
  lost: readonly number[]
}

export interface UdpTcpState {
  tick: number
  tcp: Lane
  udp: Lane
}

// Модель: сообщение i уходит на тике i − 1 и идёт 1 тик. Потеря сообщения 3 обнаруживается
// только по таймеру RTO, поэтому повтор TCP уходит на тике 2 + RTO и доходит на следующем.
export const MESSAGES = 6
export const LOST_MESSAGE = 3
export const RTO_TICKS = 4
export const TCP_DONE_TICK = LOST_MESSAGE - 1 + RTO_TICKS + 1
export const UDP_DONE_TICK = MESSAGES
const RETRANSMIT_TICK = TCP_DONE_TICK - 1

const range = (from: number, to: number): number[] => Array.from({ length: Math.max(0, to - from + 1) }, (_, i) => from + i)
const arrived = (tick: number): number[] => range(1, Math.min(tick, MESSAGES)).filter((m) => m !== LOST_MESSAGE)

function udpLane(tick: number): Lane {
  return { delivered: arrived(tick), buffered: [], lost: tick >= LOST_MESSAGE ? [LOST_MESSAGE] : [] }
}

function tcpLane(tick: number): Lane {
  if (tick >= TCP_DONE_TICK) return { delivered: range(1, MESSAGES), buffered: [], lost: [] }
  const got = arrived(tick)
  return {
    delivered: got.filter((m) => m < LOST_MESSAGE),
    buffered: got.filter((m) => m > LOST_MESSAGE),
    lost: tick >= LOST_MESSAGE && tick < RETRANSMIT_TICK ? [LOST_MESSAGE] : [],
  }
}

const CAPTIONS: readonly Caption[] = [
  [{ text: 'Одно и то же отправляем двумя способами: ' }, term('TCP', 'tcp'), { text: ' и ' }, term('UDP', 'udp'), { text: `. Шесть сообщений, по одному на тик; путь в одну сторону занимает один тик. Сообщение ${LOST_MESSAGE} сеть потеряет.` }],
  'Сообщение 1 дошло по обеим дорожкам и сразу отдано приложению.',
  'Сообщение 2 дошло и отдано. Пока всё одинаково.',
  `Сообщение ${LOST_MESSAGE} потеряно по пути. Ни один из протоколов пока об этом не знает: ждут следующих.`,
  [{ text: 'Дошло сообщение 4. UDP отдаёт его приложению сразу. TCP получил его, но порядок нарушен: приложению он отдаст только после сообщения 3, поэтому 4 лежит в буфере. Это ' }, term('блокировка очереди', 'head-of-line-blocking'), { text: '.' }],
  'Дошло сообщение 5. UDP отдал его сразу, TCP снова держит в буфере: приложение ничего нового не видит.',
  [{ text: `Дошло сообщение 6. UDP закончил: получено 1, 2, 4, 5, 6, а сообщение ${LOST_MESSAGE} потеряно навсегда, и протокол не будет его искать. У TCP сработал таймер (RTO = ${RTO_TICKS} тика): отправитель делает ` }, term('повторную передачу', 'retransmission'), { text: ' сообщения 3.' }],
  [{ text: `Повтор дошёл. Теперь TCP отдаёт приложению сразу 3, 4, 5, 6 по порядку. Итог: TCP доставил всё, но на ${TCP_DONE_TICK - UDP_DONE_TICK} тик позже. UDP быстрее, но дыру в данных закрывать должно само приложение. В реальной сети RTO обычно намного больше одного тика, и разница заметнее.` }],
]

export function udpVsTcpScenario(): Step<UdpTcpState>[] {
  return CAPTIONS.map((caption, tick) => ({
    state: { tick, tcp: tcpLane(tick), udp: udpLane(tick) },
    caption,
  }))
}
