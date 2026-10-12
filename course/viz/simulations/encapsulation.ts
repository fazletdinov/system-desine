import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Step } from '../core/types'

export type LayerId = 'http' | 'tls' | 'tcp' | 'ip' | 'eth'

export interface Layer {
  id: LayerId
  label: string
  headerBytes: number
  trailerBytes: number
}

export interface EncapsulationState {
  layers: readonly Layer[]
  active: number
}

export const PAYLOAD_BYTES = 200

// TLS: 5 байт заголовка записи + 16 байт тега + 1 байт типа содержимого (TLS 1.3).
// TCP: 20 байт заголовка + 12 байт опций (метки времени, SACK, выравнивание) — типично для Linux.
// Ethernet: 14 байт заголовка + 4 байта контрольной суммы кадра (FCS).
const TLS: Layer = { id: 'tls', label: 'TLS', headerBytes: 5, trailerBytes: 16 + 1 }
const TCP: Layer = { id: 'tcp', label: 'TCP', headerBytes: 20 + 12, trailerBytes: 0 }
const IP: Layer = { id: 'ip', label: 'IP', headerBytes: 20, trailerBytes: 0 }
const ETH: Layer = { id: 'eth', label: 'Ethernet', headerBytes: 14, trailerBytes: 4 }
const HTTP: Layer = { id: 'http', label: 'HTTP-тело', headerBytes: 0, trailerBytes: 0 }

const overhead = (l: Layer): number => l.headerBytes + l.trailerBytes

export const HEADER_BYTES = { tls: overhead(TLS), tcp: overhead(TCP), ip: overhead(IP), eth: overhead(ETH) } as const

export const TOTAL_BYTES = PAYLOAD_BYTES + HEADER_BYTES.tls + HEADER_BYTES.tcp + HEADER_BYTES.ip + HEADER_BYTES.eth

export const PAYLOAD_SHARE_PERCENT = Math.round((PAYLOAD_BYTES / TOTAL_BYTES) * 100)

// Слои перечислены от внутреннего (HTTP) к внешнему.
export function totalBytes(layers: readonly Layer[]): number {
  return PAYLOAD_BYTES + layers.reduce((sum, l) => sum + overhead(l), 0)
}

const CAPTIONS: readonly Caption[] = [
  [{ text: 'Всё начинается с полезных данных: тело ' }, term('HTTP', 'http'), { text: `-запроса «Отправить сообщение» занимает ${PAYLOAD_BYTES} байт. Пока это просто текст.` }],
  [term('TLS', 'tls'), { text: ` шифрует тело и добавляет к нему служебные байты: заголовок записи, тег проверки целостности и тип содержимого — всего ${HEADER_BYTES.tls}. Итого ${PAYLOAD_BYTES + HEADER_BYTES.tls} байт.` }],
  [term('TCP', 'tcp'), { text: ` кладёт данные в сегмент: порты, номера байтов и окно занимают ${HEADER_BYTES.tcp} байта вместе с опциями. Итого ${PAYLOAD_BYTES + HEADER_BYTES.tls + HEADER_BYTES.tcp} байт.` }],
  [{ text: 'Сегмент становится IP-пакетом: ' }, term('IP-адреса', 'ip-address'), { text: ` отправителя и получателя, ещё ${HEADER_BYTES.ip} байт. Итого ${PAYLOAD_BYTES + HEADER_BYTES.tls + HEADER_BYTES.tcp + HEADER_BYTES.ip} байт.` }],
  `Последний конверт — кадр Ethernet: MAC-адреса соседних устройств и контрольная сумма, ещё ${HEADER_BYTES.eth} байт. Кадр готов уходить в провод или в Wi-Fi.`,
  [{ text: `Итого ${TOTAL_BYTES} байт ради ${PAYLOAD_BYTES} байт полезных данных: ${PAYLOAD_SHARE_PERCENT}% — смысл, остальное — служебные конверты. Каждый уровень сделал свою работу, не зная про соседей: это ` }, term('инкапсуляция', 'encapsulation'), { text: '. Чем меньше сообщение, тем тяжелее конверты.' }],
]

export function encapsulationScenario(): Step<EncapsulationState>[] {
  const stack = [HTTP, TLS, TCP, IP, ETH]
  return CAPTIONS.map((caption, i) => {
    const count = Math.min(i + 1, stack.length)
    return {
      state: { layers: stack.slice(0, count), active: count - 1 },
      caption,
      highlight: [stack[count - 1]!.id],
    }
  })
}
