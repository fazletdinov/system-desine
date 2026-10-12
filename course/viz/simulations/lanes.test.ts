import { describe, expect, it } from 'vitest'
import { lanesTimeline, type LanesInput } from './lanes'

const base: LanesInput = { proto: 'h2', streams: 3, packetsPerStream: 4, lostPacket: null, h1Connections: 1, rttSlots: 4 }
const done = (over: Partial<LanesInput>) => lanesTimeline({ ...base, ...over }).streamDone

describe('lanesTimeline', () => {
  it('без потерь HTTP/2 и QUIC заканчивают одинаково', () => {
    expect(done({ proto: 'quic' })).toEqual(done({ proto: 'h2' }))
  })

  // Пакет 9 — последний пакет потока 0 (пакеты чередуются: id = номер_пакета × потоки + поток).
  it('потеря пакета 9: в HTTP/2 задерживаются все потоки', () => {
    const clean = done({ proto: 'h2' })
    const lossy = done({ proto: 'h2', lostPacket: 9 })
    lossy.forEach((d, s) => expect(d).toBeGreaterThan(clean[s]!))
  })

  it('потеря пакета 9: в QUIC задерживается только поток 0', () => {
    const clean = done({ proto: 'quic' })
    const lossy = done({ proto: 'quic', lostPacket: 9 })
    expect(lossy[0]).toBeGreaterThan(clean[0]!)
    expect(lossy.slice(1)).toEqual(clean.slice(1))
  })

  it('HTTP/1.1 с одним соединением: потоки заканчиваются строго по очереди', () => {
    const d = done({ proto: 'h1', h1Connections: 1 })
    for (let s = 1; s < d.length; s++) expect(d[s]).toBeGreaterThan(d[s - 1]!)
  })

  it('у потерянного пакета время прихода — после повтора', () => {
    const r = lanesTimeline({ ...base, lostPacket: 0 })
    const p = r.packets.find((x) => x.id === 0)!
    expect(p.lost).toBe(true)
    expect(p.arrivesAt).toBe(p.sentAt + base.rttSlots + base.rttSlots / 2)
  })

  it('index — номер пакета внутри своего потока при любом протоколе', () => {
    for (const proto of ['h1', 'h2', 'quic'] as const) {
      const r = lanesTimeline({ ...base, proto })
      for (let s = 0; s < base.streams; s++) {
        const idx = r.packets.filter((p) => p.stream === s).map((p) => p.index)
        expect(idx).toEqual([0, 1, 2, 3])
      }
    }
  })

  it('HTTP/1.1 с двумя соединениями: потеря в одном не задерживает поток другого', () => {
    // Потоки 0 и 2 — на соединении 0, поток 1 — на соединении 1; пакет 1 — первый пакет потока 1.
    const clean = done({ proto: 'h1', h1Connections: 2 })
    const lossy = done({ proto: 'h1', h1Connections: 2, lostPacket: 1 })
    expect(lossy[1]).toBeGreaterThan(clean[1]!)
    expect(lossy[0]).toBe(clean[0])
    expect(lossy[2]).toBe(clean[2])
  })

  it('номер потерянного пакета вне диапазона ничего не меняет', () => {
    const clean = done({ proto: 'h2' })
    expect(done({ proto: 'h2', lostPacket: 99 })).toEqual(clean)
    expect(done({ proto: 'h2', lostPacket: -1 })).toEqual(clean)
  })
})
