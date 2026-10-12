// Модель пула http.Transport: соединение после ответа возвращается в простаивающие, только если
// простаивающих меньше maxIdlePerHost (MaxIdleConnsPerHost, по умолчанию в Go — 2); иначе закрывается.
// Здесь maxIdlePerHost = 0 — «не хранить простаивающих» (в Go 0 означает значение по умолчанию, 2;
// отключают пул отрицательным значением или DisableKeepAlives), поэтому ползунок в сцене начинается с 1.
// Общее число соединений не ограничено (MaxConnsPerHost = 0). Время — в целых RTT.
export interface PoolInput {
  rttMs: number
  requests: number
  concurrency: number
  maxIdlePerHost: number
  tls: boolean
  keepAlive: boolean
}

export type PoolPresetId = 'default' | 'go-default'

export const POOL_PRESETS: Record<PoolPresetId, PoolInput> = {
  default: { rttMs: 40, requests: 20, concurrency: 4, maxIdlePerHost: 2, tls: true, keepAlive: true },
  'go-default': { rttMs: 40, requests: 100, concurrency: 10, maxIdlePerHost: 2, tls: true, keepAlive: true },
}

export type SpanKind = 'tcp' | 'tls' | 'request'

export interface Span {
  conn: number
  request: number
  kind: SpanKind
  startMs: number
  endMs: number
}

export interface PoolResult {
  spans: Span[]
  totalMs: number
  connections: number
}

const clampInt = (v: number, lo: number, hi: number): number => (Number.isFinite(v) ? Math.min(hi, Math.max(lo, Math.trunc(v))) : lo)

export function sanitizePool(i: PoolInput): PoolInput {
  const requests = clampInt(i.requests, 1, 1000)
  return {
    rttMs: clampInt(i.rttMs, 1, 1000),
    requests,
    concurrency: Math.min(clampInt(i.concurrency, 1, 100), requests),
    maxIdlePerHost: clampInt(i.maxIdlePerHost, 0, 100),
    tls: i.tls,
    keepAlive: i.keepAlive,
  }
}

interface Worker {
  freeAt: number
  conn: number | null
}

export function poolTimeline(raw: PoolInput): PoolResult {
  const i = sanitizePool(raw)
  const R = i.rttMs
  const spans: Span[] = []
  const idle: number[] = []
  const workers: Worker[] = Array.from({ length: i.concurrency }, () => ({ freeAt: 0, conn: null }))
  let connections = 0
  let next = 0

  while (next < i.requests) {
    const t = Math.min(...workers.map((w) => w.freeAt))
    const ready = workers.filter((w) => w.freeAt === t)
    // Сначала все, кто закончил в момент t, возвращают соединения — потом новые запросы их забирают.
    for (const w of ready) {
      if (w.conn === null) continue
      if (i.keepAlive && idle.length < i.maxIdlePerHost) idle.push(w.conn)
      w.conn = null
    }
    for (const w of ready) {
      if (next >= i.requests) break
      const req = next++
      let start = t
      let conn = i.keepAlive ? idle.shift() : undefined
      if (conn === undefined) {
        conn = connections++
        spans.push({ conn, request: req, kind: 'tcp', startMs: start, endMs: start + R })
        start += R
        if (i.tls) {
          spans.push({ conn, request: req, kind: 'tls', startMs: start, endMs: start + R })
          start += R
        }
      }
      spans.push({ conn, request: req, kind: 'request', startMs: start, endMs: start + R })
      w.freeAt = start + R
      w.conn = conn
    }
    // Воркеры, оставшиеся без запросов, больше не участвуют: их время — «бесконечность».
    for (const w of ready) if (w.conn === null) w.freeAt = Number.POSITIVE_INFINITY
  }
  const totalMs = spans.reduce((m, s) => Math.max(m, s.endMs), 0)
  return { spans, totalMs, connections }
}
