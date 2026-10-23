import { term, type Caption } from '../core/caption'
import type { Step } from '../core/types'
import { formatMs } from '../core/format'

export type NodeId = 'client' | 'dns' | 'lb' | 'app' | 'db'
export type NodeKind = 'client' | 'network' | 'service' | 'db'

export const NODES: readonly { id: NodeId; label: string; kind: NodeKind }[] = [
  { id: 'client', label: 'Телефон', kind: 'client' },
  { id: 'dns', label: 'DNS', kind: 'network' },
  { id: 'lb', label: 'Балансировщик', kind: 'network' },
  { id: 'app', label: 'Go-сервис', kind: 'service' },
  { id: 'db', label: 'PostgreSQL', kind: 'db' },
]

export interface Packet {
  from: NodeId
  to: NodeId
  kind: 'request' | 'response'
}

export interface RequestPathState {
  active: NodeId
  packet: Packet | null
  elapsedMs: number
}

// Порядки величин для мобильного интернета и сервера в соседней стране.
// Источник диапазонов — урок 0.4 и Grigorik, «High Performance Browser Networking», гл. 1.
export const LATENCY_MS = { dnsLookup: 20, rtt: 40, internalHop: 0.5, diskFsync: 2 } as const

export const TOTAL_MS = LATENCY_MS.dnsLookup + 3 * LATENCY_MS.rtt + 4 * LATENCY_MS.internalHop + LATENCY_MS.diskFsync

// M1: какой запрос идёт и почему он платит за все рукопожатия — без этого 144 мс не с чем сравнить.
// Тело совпадает с HTTP-запросом из раздела урока 0.3 «HTTP-запрос: наконец сами данные».
export const REQUEST_PATH_REQUEST = 'POST /messages {"chat_id": 42, "text": "Привет!"}'
export const REQUEST_PATH_CONTEXT = 'первый запрос после запуска приложения: соединения ещё нет.'

// M5: часы на схеме — модель, а не замер; числа берём из LATENCY_MS, чтобы подпись не разошлась с часами.
export const REQUEST_PATH_HONESTY =
  `Часы — модель: DNS ${formatMs(LATENCY_MS.dnsLookup)}, RTT ${formatMs(LATENCY_MS.rtt)} (4G, сервер в соседней стране), ` +
  `переход внутри ЦОД ${formatMs(LATENCY_MS.internalHop)}, fsync ${formatMs(LATENCY_MS.diskFsync)}. ` +
  'Это порядки величин, не замер — таблица допущений ниже схемы.'

// Номера строк в Go-обработчике из урока 0.3.
export const CODE_LINE = { entry: 6, auth: 7, execute: 38, respond: 55 } as const

interface Segment {
  active: NodeId
  packet: Packet | null
  costMs: number
  caption: Caption
  codeLine?: number
}

const req = (from: NodeId, to: NodeId): Packet => ({ from, to, kind: 'request' })
const res = (from: NodeId, to: NodeId): Packet => ({ from, to, kind: 'response' })
const half = LATENCY_MS.rtt / 2

const SEGMENTS: readonly Segment[] = [
  { active: 'client', packet: null, costMs: 0, caption: 'Вы нажали «Отправить». Чтобы доставить сообщение, телефону нужен IP-адрес сервера, а он знает только имя api.messenger.example.' },
  { active: 'dns', packet: req('client', 'dns'), costMs: LATENCY_MS.dnsLookup / 2, caption: [term('DNS', 'dns'), { text: '-запрос: «какой IP-адрес у api.messenger.example?». Если адрес уже лежит в кэше телефона (ответ DNS хранится там ' }, term('TTL', 'ttl'), { text: ' секунд), этого шага не будет.' }] },
  { active: 'client', packet: res('dns', 'client'), costMs: LATENCY_MS.dnsLookup / 2, caption: 'DNS вернул IP-адрес балансировщика. Около 20 мс ушло только на то, чтобы узнать, куда стучаться.' },
  { active: 'lb', packet: req('client', 'lb'), costMs: half, caption: [term('TCP', 'tcp'), { text: '-рукопожатие, часть 1: телефон отправляет SYN — «хочу установить соединение».' }] },
  { active: 'client', packet: res('lb', 'client'), costMs: half, caption: [{ text: 'TCP, часть 2: балансировщик отвечает SYN-ACK. Прошёл один полный ' }, term('RTT', 'rtt'), { text: ', а полезных данных ещё не отправлено ни байта.' }] },
  { active: 'lb', packet: req('client', 'lb'), costMs: half, caption: [term('TLS', 'tls'), { text: ' 1.3: телефон отправляет ACK и сразу ClientHello — поддерживаемые шифры и свою часть ключа.' }] },
  { active: 'client', packet: res('lb', 'client'), costMs: half, caption: [{ text: 'Балансировщик отвечает ServerHello и ' }, term('сертификатом', 'certificate'), { text: '. Ещё один RTT — канал зашифрован. Позади два RTT подготовки.' }] },
  { active: 'lb', packet: req('client', 'lb'), costMs: half, caption: 'Наконец сам запрос: зашифрованный POST /messages с текстом сообщения.' },
  { active: 'app', packet: req('lb', 'app'), costMs: LATENCY_MS.internalHop, codeLine: CODE_LINE.entry, caption: [{ text: 'Балансировщик расшифровал запрос (' }, term('TLS-терминация', 'tls-termination'), { text: ') и передал его одному из Go-сервисов. Внутри ' }, term('дата-центра', 'data-center'), { text: ' это доли миллисекунды.' }] },
  { active: 'app', packet: null, costMs: 0, codeLine: CODE_LINE.auth, caption: [{ text: 'Go-сервис проверяет ' }, term('токен', 'bearer-token'), { text: ' и разбирает JSON. Это микросекунды работы процессора — на фоне сети почти ноль.' }] },
  { active: 'db', packet: req('app', 'db'), costMs: LATENCY_MS.internalHop, codeLine: CODE_LINE.execute, caption: 'Сервис отправляет INSERT в PostgreSQL.' },
  { active: 'db', packet: null, costMs: LATENCY_MS.diskFsync, codeLine: CODE_LINE.execute, caption: [{ text: 'PostgreSQL записывает изменение на диск (' }, term('fsync', 'fsync'), { text: ') и только потом подтверждает. Так подтверждённое сообщение переживёт сбой питания.' }] },
  { active: 'app', packet: res('db', 'app'), costMs: LATENCY_MS.internalHop, codeLine: CODE_LINE.respond, caption: 'База подтвердила запись. Сервис формирует ответ 201 Created.' },
  { active: 'lb', packet: res('app', 'lb'), costMs: LATENCY_MS.internalHop, caption: 'Ответ 201 Created возвращается на балансировщик, тот шифрует его для телефона.' },
]

export function requestPathScenario(): Step<RequestPathState>[] {
  let elapsedMs = 0
  const steps: Step<RequestPathState>[] = SEGMENTS.map((segment) => {
    elapsedMs += segment.costMs
    return {
      state: { active: segment.active, packet: segment.packet, elapsedMs },
      caption: segment.caption,
      highlight: [segment.active],
      ...(segment.codeLine === undefined ? {} : { codeLine: segment.codeLine }),
    }
  })
  elapsedMs += half
  const network = LATENCY_MS.dnsLookup + 3 * LATENCY_MS.rtt
  steps.push({
    state: { active: 'client', packet: res('lb', 'client'), elapsedMs },
    caption: `Телефон получил ответ — галочка «отправлено». Итого ≈ ${formatMs(elapsedMs)}, из них ${formatMs(network)} — сеть до телефона, а работа сервера и базы — всего ${formatMs(elapsedMs - network)}.`,
    highlight: ['client'],
  })
  return steps
}
