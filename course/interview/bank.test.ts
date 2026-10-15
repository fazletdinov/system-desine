import { existsSync, readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { parse } from 'yaml'
import { parseBank } from './schema'

const COURSE = fileURLToPath(new URL('..', import.meta.url))
const bank = parseBank(parse(readFileSync(join(COURSE, 'interview', 'module-01.yaml'), 'utf8')), 'module-01.yaml')

describe('банк вопросов модуля 1', () => {
  it('каждый вопрос ссылается на существующий урок', () => {
    for (const q of bank.questions) expect(existsSync(join(COURSE, 'modules', `${q.lesson}.md`)), q.id).toBe(true)
  })
  it('у каждого урока 1.1–1.19 есть хотя бы один вопрос', () => {
    const lessons = readdirSync(join(COURSE, 'modules', '01-go-service'))
      .filter((f) => /^(0[1-9]|1[0-9])-.+\.md$/.test(f))
      .map((f) => `01-go-service/${f.replace(/\.md$/, '')}`)
    expect(lessons).toHaveLength(19)
    const covered = new Set(bank.questions.map((q) => q.lesson))
    expect(lessons.filter((l) => !covered.has(l))).toEqual([])
  })
  it('размер и баланс уровней', () => {
    expect(bank.questions.length).toBeGreaterThanOrEqual(22)
    expect(bank.questions.length).toBeLessThanOrEqual(30)
    const senior = bank.questions.filter((q) => q.level === 'S').length
    expect(senior / bank.questions.length).toBeGreaterThan(0.3)
    expect(senior / bank.questions.length).toBeLessThan(0.7)
  })
})
