import { describe, expect, it } from 'vitest'
import { POOL_PRESETS, poolTimeline, sanitizePool, type PoolInput, type Span } from './pool'

const base: PoolInput = { rttMs: 40, requests: 5, concurrency: 1, maxIdlePerHost: 2, tls: true, keepAlive: true }

describe('poolTimeline', () => {
  it.each<[string, Partial<PoolInput>, number, number]>([
    ['keep-alive, последовательно: одно соединение, (2 + n) RTT', {}, 1, 280],
    ['без keep-alive: каждый запрос — TCP + TLS + запрос', { keepAlive: false }, 5, 600],
    ['без TLS: (1 + n) RTT', { tls: false }, 1, 240],
    ['10 воркеров, пул 10, 100 запросов: 10 соединений, 12 RTT', { requests: 100, concurrency: 10, maxIdlePerHost: 10 }, 10, 480],
  ])('%s', (_name, over, conns, total) => {
    const r = poolTimeline({ ...base, ...over })
    expect(r.connections).toBe(conns)
    expect(r.totalMs).toBe(total)
  })

  it('пул 2 при 10 воркерах открывает больше соединений и работает дольше, чем пул 10', () => {
    const small = poolTimeline({ ...base, requests: 100, concurrency: 10, maxIdlePerHost: 2 })
    const big = poolTimeline({ ...base, requests: 100, concurrency: 10, maxIdlePerHost: 10 })
    expect(small.connections).toBeGreaterThan(big.connections)
    expect(small.totalMs).toBeGreaterThan(big.totalMs)
  })

  it('спаны не пересекаются внутри одного соединения', () => {
    const r = poolTimeline({ ...base, requests: 30, concurrency: 4, maxIdlePerHost: 2 })
    const byConn = new Map<number, Span[]>()
    for (const s of r.spans) byConn.set(s.conn, [...(byConn.get(s.conn) ?? []), s])
    for (const spans of byConn.values()) {
      const sorted = [...spans].sort((a, b) => a.startMs - b.startMs)
      for (let i = 1; i < sorted.length; i++) expect(sorted[i]!.startMs).toBeGreaterThanOrEqual(sorted[i - 1]!.endMs)
    }
  })
})

describe('POOL_PRESETS', () => {
  it('go-default: пул 2 при 10 воркерах дает заметно больше 10 соединений', () => {
    expect(POOL_PRESETS['go-default']).toMatchObject({ requests: 100, concurrency: 10, maxIdlePerHost: 2, tls: true, keepAlive: true })
    expect(poolTimeline(POOL_PRESETS['go-default']).connections).toBeGreaterThan(10)
  })
})

describe('sanitizePool', () => {
  it('приводит мусор к допустимому', () => {
    expect(sanitizePool({ rttMs: -1, requests: 0, concurrency: 0, maxIdlePerHost: -3, tls: true, keepAlive: true })).toEqual({
      rttMs: 1, requests: 1, concurrency: 1, maxIdlePerHost: 0, tls: true, keepAlive: true,
    })
    expect(sanitizePool({ ...base, concurrency: 50, requests: 5 }).concurrency).toBe(5)
  })
})
