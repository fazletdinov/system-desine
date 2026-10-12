// Учебная модель: отправитель шлёт по одному пакету за слот в каждом соединении; задержка в одну
// сторону — rttSlots / 2; потерянный пакет обнаруживается через RTT и приходит ещё через полпути.
// HTTP/2: одно TCP-соединение, потоки чередуются, TCP отдаёт данные приложению строго по порядку.
// QUIC: то же чередование, но порядок соблюдается только внутри потока.
// HTTP/1.1: потоки распределены по h1Connections соединениям и идут в каждом по очереди.
// Управления перегрузкой нет — это упрощение, названное в уроке.
export type Proto = 'h1' | 'h2' | 'quic'

export interface LanesInput {
  proto: Proto
  streams: number
  packetsPerStream: number
  lostPacket: number | null
  h1Connections: number
  rttSlots: number
}

export interface LanePacket {
  id: number
  stream: number
  index: number // номер пакета внутри своего потока: по нему «тот же» пакет находится в разных протоколах
  conn: number
  sentAt: number
  arrivesAt: number
  deliveredAt: number
  lost: boolean
}

export interface LanesResult {
  packets: LanePacket[]
  streamDone: number[]
}

export function lanesTimeline(i: LanesInput): LanesResult {
  const half = i.rttSlots / 2
  const order: { stream: number; index: number; conn: number; sentAt: number }[] = []
  if (i.proto === 'h1') {
    const k = Math.max(1, i.h1Connections)
    const slot = Array.from({ length: k }, () => 0)
    for (let s = 0; s < i.streams; s++) {
      const conn = s % k
      for (let p = 0; p < i.packetsPerStream; p++) order.push({ stream: s, index: p, conn, sentAt: slot[conn]!++ })
    }
    order.sort((a, b) => a.sentAt - b.sentAt || a.conn - b.conn)
  } else {
    let t = 0
    for (let p = 0; p < i.packetsPerStream; p++) for (let s = 0; s < i.streams; s++) order.push({ stream: s, index: p, conn: 0, sentAt: t++ })
  }

  const packets: LanePacket[] = order.map((o, id) => {
    const lost = id === i.lostPacket
    const arrivesAt = lost ? o.sentAt + i.rttSlots + half : o.sentAt + half
    return { id, ...o, arrivesAt, deliveredAt: arrivesAt, lost }
  })

  // Порядок доставки приложению: по соединению (TCP) или по потоку (QUIC).
  const lastByKey = new Map<string, number>()
  for (const p of [...packets].sort((a, b) => a.sentAt - b.sentAt || a.conn - b.conn)) {
    const key = i.proto === 'quic' ? `s${p.stream}` : `c${p.conn}`
    p.deliveredAt = Math.max(p.arrivesAt, lastByKey.get(key) ?? 0)
    lastByKey.set(key, p.deliveredAt)
  }

  const streamDone = Array.from({ length: i.streams }, (_, s) =>
    Math.max(...packets.filter((p) => p.stream === s).map((p) => p.deliveredAt)),
  )
  return { packets, streamDone }
}
