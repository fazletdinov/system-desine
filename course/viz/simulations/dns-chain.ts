import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Step } from '../core/types'

export type NodeId = 'stub' | 'recursive' | 'root' | 'tld' | 'auth'

export const NODES: readonly { id: NodeId; label: string }[] = [
  { id: 'stub', label: 'Телефон' },
  { id: 'recursive', label: 'Резолвер' },
  { id: 'root', label: 'Корень (.)' },
  { id: 'tld', label: '.example' },
  { id: 'auth', label: 'Авторитетный' },
]

export interface Packet {
  from: NodeId
  to: NodeId
  kind: 'query' | 'referral' | 'answer'
}

export interface DnsChainState {
  active: NodeId
  packet: Packet | null
  cache: readonly string[]
  elapsedMs: number
}

// Односторонние задержки. Половина 40-мс RTT из урока 0.3; до серверов имён резолвер ближе.
export const DNS_LATENCY_MS = { stubRecursive: 20, recursiveAuthoritative: 10 } as const
export const TTL_SECONDS = 30
export const ANSWER_IP = '172.28.0.10'
export const CACHE_ENTRY = `api.messenger.example → ${ANSWER_IP}`

// 20 (запрос) + 3 × (10 + 10) (корень, TLD, авторитетный) + 20 (ответ).
export const FIRST_ANSWER_MS = 2 * DNS_LATENCY_MS.stubRecursive + 3 * 2 * DNS_LATENCY_MS.recursiveAuthoritative
// Второй запрос: только телефон ↔ резолвер.
export const SECOND_QUERY_MS = 2 * DNS_LATENCY_MS.stubRecursive

interface Move {
  packet: Packet | null
  costMs: number
  caches?: boolean
  caption: Caption
}

const move = (from: NodeId, to: NodeId, kind: Packet['kind']): Packet => ({ from, to, kind })
const S = DNS_LATENCY_MS.stubRecursive
const A = DNS_LATENCY_MS.recursiveAuthoritative

const MOVES: readonly Move[] = [
  { packet: null, costMs: 0, caption: [{ text: 'Телефон знает только имя api.messenger.example, а ему нужен ' }, term('IP-адрес', 'ip-address'), { text: '. Свой кэш пуст, и телефон спрашивает ближайший ' }, term('резолвер', 'resolver'), { text: ' — обычно его выдаёт провайдер или роутер.' }] },
  { packet: move('stub', 'recursive', 'query'), costMs: S, caption: [term('DNS', 'dns'), { text: '-запрос телефона: «какой адрес у api.messenger.example?». До резолвера 20 мс в одну сторону. Резолвер сам обойдёт серверы имён, а телефон будет ждать.' }] },
  { packet: move('recursive', 'root', 'query'), costMs: A, caption: 'Кэш резолвера тоже пуст («холодный»). Он спрашивает один из корневых серверов имён. Ответа на вопрос корень не знает, но знает, кто отвечает за каждую зону верхнего уровня.' },
  { packet: move('root', 'recursive', 'referral'), costMs: A, caption: 'Корень отвечает не адресом, а направлением — это реферал: «про .example спросите серверы вот по таким адресам». Окончательный ответ ещё не получен.' },
  { packet: move('recursive', 'tld', 'query'), costMs: A, caption: 'Резолвер идёт к серверам зоны .example с тем же вопросом.' },
  { packet: move('tld', 'recursive', 'referral'), costMs: A, caption: 'Зона .example тоже не знает адрес, но знает, чей сервер имён отвечает за messenger.example: это ns.messenger.example. Второй реферал. Вместе с ним зона отдаёт и IP-адрес этого сервера (glue-запись): иначе получился бы тупик — чтобы узнать адрес ns.messenger.example, пришлось бы спросить самого ns.messenger.example.' },
  { packet: move('recursive', 'auth', 'query'), costMs: A, caption: 'Третий запрос — к авторитетному серверу: он хранит настоящие записи домена messenger.example.' },
  { packet: move('auth', 'recursive', 'answer'), costMs: A, caches: true, caption: [{ text: `Наконец ответ: запись A, адрес ${ANSWER_IP}, срок жизни (` }, term('TTL', 'ttl'), { text: `) ${TTL_SECONDS} секунд. Резолвер кладёт её в кэш: пока TTL не истёк, обходить серверы заново не нужно.` }] },
  { packet: move('recursive', 'stub', 'answer'), costMs: S, caption: [{ text: `Резолвер передаёт ответ телефону. С холодным кэшем это заняло ${FIRST_ANSWER_MS} мс: 20 до резолвера, три обращения к авторитетным серверам по 20 мс туда и обратно, 20 обратно. Только после этого можно начинать ` }, term('TCP', 'tcp'), { text: '-рукопожатие.' }] },
  { packet: move('stub', 'recursive', 'query'), costMs: S, caption: 'Другой телефон в той же сети (или ваш — после сброса его собственного кэша) спрашивает тот же адрес. Для резолвера это новый вопрос, но пока TTL не истёк, ответ у него уже есть.' },
  { packet: move('recursive', 'stub', 'answer'), costMs: S, caption: `Резолвер отвечает из кэша, никуда больше не ходит. Весь вопрос занял ${SECOND_QUERY_MS} мс — один обмен с резолвером.` },
  { packet: null, costMs: 0, caption: [{ text: `Итог: холодный запрос — ${FIRST_ANSWER_MS} мс, тёплый — ${SECOND_QUERY_MS} мс. Кэш живёт ${TTL_SECONDS} секунд (` }, term('TTL', 'ttl'), { text: '), потом цепочка пройдёт заново. Поэтому TTL — компромисс между скоростью и тем, как быстро новый адрес дойдёт до клиентов.' }] },
]

export function dnsChainScenario(): Step<DnsChainState>[] {
  let elapsedMs = 0
  let cache: readonly string[] = []
  return MOVES.map((m) => {
    elapsedMs += m.costMs
    if (m.caches) cache = [CACHE_ENTRY]
    const active: NodeId = m.packet === null ? 'stub' : m.packet.to
    return { state: { active, packet: m.packet, cache, elapsedMs }, caption: m.caption, highlight: [active] }
  })
}
