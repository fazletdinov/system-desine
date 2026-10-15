import { describe, expect, it } from 'vitest'
import type { Question } from './schema'
import { mulberry32, pickSession, scoreSession } from './session'

const q = (id: string, topic: string, checklist = 4): Question => ({
  id,
  lesson: '01-go-service/01-architecture',
  topic,
  level: 'M+',
  question: id,
  answer: id,
  checklist: Array.from({ length: checklist }, (_, i) => `${id}-${i}`),
  red_flags: ['x'],
  follow_ups: [{ q: 'q', a: 'a' }],
})

const bank = [
  q('a1', 'A'), q('a2', 'A'), q('a3', 'A'),
  q('b1', 'B'), q('b2', 'B'),
  q('c1', 'C'), q('c2', 'C'),
  q('d1', 'D'), q('e1', 'E'), q('f1', 'F'),
]
const topicOf = (id: string) => bank.find((x) => x.id === id)?.topic

describe('mulberry32', () => {
  it('детерминирован и в [0, 1)', () => {
    const a = mulberry32(42), b = mulberry32(42)
    const xs = Array.from({ length: 100 }, () => a())
    expect(xs).toEqual(Array.from({ length: 100 }, () => b()))
    expect(xs.every((x) => x >= 0 && x < 1)).toBe(true)
  })
})

describe('pickSession', () => {
  it('тот же сид — та же сессия, другой — другая', () => {
    expect(pickSession(bank, 5, 7)).toEqual(pickSession(bank, 5, 7))
    expect(pickSession(bank, 5, 7).ids).not.toEqual(pickSession(bank, 5, 8).ids)
  })
  it('размер ограничен банком, вопросы не повторяются', () => {
    const s = pickSession(bank, 50, 1)
    expect(s.ids).toHaveLength(bank.length)
    expect(new Set(s.ids).size).toBe(bank.length)
  })
  it('тем хватает — все темы разные', () => {
    for (const seed of [1, 2, 3, 4, 5]) {
      const topics = pickSession(bank, 6, seed).ids.map(topicOf)
      expect(new Set(topics).size).toBe(6)
    }
  })
  it('две одинаковые темы подряд не идут, если этого можно избежать', () => {
    for (const seed of [1, 2, 3, 4, 5, 6, 7, 8]) {
      const topics = pickSession(bank, 10, seed).ids.map(topicOf)
      for (let i = 1; i < topics.length; i++) expect(topics[i]).not.toBe(topics[i - 1])
    }
  })
  it('неверный размер — ошибка', () => {
    expect(() => pickSession(bank, 0, 1)).toThrow()
  })
})

describe('scoreSession', () => {
  it('считает отмеченные пункты по темам и находит слабые', () => {
    const s = { seed: 1, ids: ['a1', 'b1', 'c1'] }
    const score = scoreSession(bank, s, {
      a1: [true, true, true, true],
      b1: [true, false, false, false],
      // c1 не отмечен вовсе — 0 из 4
    })
    expect(score.total).toEqual({ got: 5, total: 12 })
    expect(score.topics.find((t) => t.topic === 'A')).toMatchObject({ got: 4, total: 4 })
    expect(score.weak.map((t) => t.topic)).toEqual(['C', 'B'])
  })
  it('метки длиннее чеклиста не дают лишних баллов', () => {
    const score = scoreSession(bank, { seed: 1, ids: ['d1'] }, { d1: [true, true, true, true, true, true] })
    expect(score.total).toEqual({ got: 4, total: 4 })
  })
})
