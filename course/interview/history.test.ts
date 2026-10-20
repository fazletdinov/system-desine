import { describe, expect, it } from 'vitest'
import { appendHistory, HISTORY_LIMIT, lessonLabel, parseHistory, parseSeed, type HistoryEntry } from './history'

const entry = (seed: number): HistoryEntry => ({ at: '2026-10-15T10:00:00.000Z', seed, ratio: 0.5, weak: ['Тесты'] })

describe('parseSeed', () => {
  it.each([
    ['0', 0],
    ['42', 42],
    [' 7 ', 7],
    ['4294967295', 4294967295],
  ])('принимает %j', (text, seed) => {
    expect(parseSeed(text)).toBe(seed)
  })
  it.each(['', 'abc', '-1', '1.5', '4294967296', '1e3'])('отклоняет %j', (text) => {
    expect(parseSeed(text)).toBeUndefined()
  })
})

describe('lessonLabel', () => {
  it.each([
    ['01-go-service/04-usecases', '1.4'],
    ['01-go-service/19-delivery', '1.19'],
    ['00b-network/06-http1-pools', '0b.6'],
    ['01-5-go-concurrency/07-netpoller', '1.5.7'],
    ['01-5-go-concurrency/10-profiling', '1.5.10'],
  ])('%s → %s', (lesson, label) => {
    expect(lessonLabel(lesson)).toBe(label)
  })
})

describe('parseHistory', () => {
  it('читает корректный список', () => {
    expect(parseHistory(JSON.stringify([entry(1), entry(2)]))).toEqual([entry(1), entry(2)])
  })
  it('пропускает испорченные записи и отбрасывает мусор целиком', () => {
    expect(parseHistory(JSON.stringify([entry(1), { seed: 'x' }, null]))).toEqual([entry(1)])
    expect(parseHistory('{не json')).toEqual([])
    expect(parseHistory(JSON.stringify({ a: 1 }))).toEqual([])
    expect(parseHistory(null)).toEqual([])
  })
})

describe('appendHistory', () => {
  it('новая запись первой, длина ограничена', () => {
    const many = Array.from({ length: HISTORY_LIMIT }, (_, i) => entry(i))
    const next = appendHistory(many, entry(99))
    expect(next).toHaveLength(HISTORY_LIMIT)
    expect(next[0]?.seed).toBe(99)
    expect(next.at(-1)?.seed).toBe(HISTORY_LIMIT - 2)
  })
})
