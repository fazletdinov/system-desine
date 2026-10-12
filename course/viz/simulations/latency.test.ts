import { describe, expect, it } from 'vitest'
import { LATENCIES, MIN_BAR, barWidth, compare, humanSeconds } from './latency'

const byId = (id: string) => LATENCIES.find((l) => l.id === id)!

describe('LATENCIES', () => {
  it('отсортированы по возрастанию, id уникальны', () => {
    const ns = LATENCIES.map((l) => l.ns)
    expect(ns).toEqual([...ns].sort((a, b) => a - b))
    expect(new Set(LATENCIES.map((l) => l.id)).size).toBe(LATENCIES.length)
  })
})

describe('barWidth', () => {
  it('минимум для самого быстрого, 1 для самого медленного, монотонно растёт', () => {
    const widths = LATENCIES.map((l) => barWidth(l.ns))
    expect(widths[0]).toBe(MIN_BAR)
    expect(widths.at(-1)).toBe(1)
    for (let i = 1; i < widths.length; i++) expect(widths[i]!).toBeGreaterThan(widths[i - 1]!)
  })
})

describe('humanSeconds', () => {
  it('1 нс превращается в 1 с', () => {
    expect(humanSeconds(1)).toBe(1)
    expect(humanSeconds(150_000_000)).toBe(150_000_000)
  })
})

describe('compare', () => {
  it.each([
    ['ssd-random', 'ram', 'в 160 раз медленнее'],
    ['l1', 'ram', 'в 100 раз быстрее'],
    ['ram', 'ram', 'это и есть точка отсчёта'],
    ['intercontinental', 'dc-rtt', 'в 300 раз медленнее'],
    // «раза» после 2–4 и дробей, но «раз» после 12–14 и остальных
    ['hdd-seek', 'dc-rtt', 'в 4 раза медленнее'],
    ['hdd-1mb', 'dc-rtt', 'в 1,7 раза медленнее'],
    ['l2', 'ram', 'в 25 раз быстрее'],
    ['ram-1mb', 'hdd-seek', 'в 667 раз быстрее'],
    ['ssd-random', 'ssd-1mb', 'в 3,1 раза быстрее'],
  ])('%s относительно %s → %s', (a, b, expected) => {
    expect(compare(byId(a), byId(b))).toBe(expected)
  })
})
