import { existsSync, readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { parse } from 'yaml'
import { parseBank } from './schema'

const COURSE = fileURLToPath(new URL('..', import.meta.url))
const INTERVIEW = join(COURSE, 'interview')

// Что банк обязан покрыть: каталог модуля и число уроков до мини-интервью.
// perLesson — сколько вопросов допустимо на один урок, если план модуля это задаёт.
interface Coverage {
  dir: string
  lessons: number
  perLesson?: readonly [number, number]
}

const COVERAGE: Record<string, Coverage> = {
  'module-01.yaml': { dir: '01-go-service', lessons: 19 },
  'module-01-5.yaml': { dir: '01-5-go-concurrency', lessons: 10, perLesson: [2, 4] },
}

const bankFiles = readdirSync(INTERVIEW).filter((f) => f.endsWith('.yaml'))

const lessonsBefore = (dir: string, count: number): string[] =>
  readdirSync(join(COURSE, 'modules', dir))
    .filter((f) => /^\d{2}-.+\.md$/.test(f) && Number(f.slice(0, 2)) <= count)
    .map((f) => `${dir}/${f.replace(/\.md$/, '')}`)

it('у каждого банка есть правило покрытия', () => {
  expect(bankFiles.filter((f) => COVERAGE[f] === undefined)).toEqual([])
})

describe.each(bankFiles)('банк %s', (file) => {
  const bank = parseBank(parse(readFileSync(join(INTERVIEW, file), 'utf8')), file)
  const coverage = COVERAGE[file]

  it('каждый вопрос ссылается на существующий урок', () => {
    for (const q of bank.questions) expect(existsSync(join(COURSE, 'modules', `${q.lesson}.md`)), q.id).toBe(true)
  })
  it('у каждого урока до мини-интервью есть вопросы', () => {
    if (coverage === undefined) throw new Error(`${file}: нет правила покрытия`)
    const lessons = lessonsBefore(coverage.dir, coverage.lessons)
    expect(lessons).toHaveLength(coverage.lessons)
    const covered = new Set(bank.questions.map((q) => q.lesson))
    expect(lessons.filter((l) => !covered.has(l))).toEqual([])
    expect([...covered].filter((l) => !lessons.includes(l))).toEqual([])
  })
  it('вопросов на урок — в пределах плана модуля', () => {
    if (coverage?.perLesson === undefined) return
    const [min, max] = coverage.perLesson
    const perLesson = new Map<string, number>()
    for (const q of bank.questions) perLesson.set(q.lesson, (perLesson.get(q.lesson) ?? 0) + 1)
    for (const [lesson, n] of perLesson) {
      expect(n, lesson).toBeGreaterThanOrEqual(min)
      expect(n, lesson).toBeLessThanOrEqual(max)
    }
  })
  it('размер и баланс уровней', () => {
    expect(bank.questions.length).toBeGreaterThanOrEqual(22)
    expect(bank.questions.length).toBeLessThanOrEqual(30)
    const senior = bank.questions.filter((q) => q.level === 'S').length
    expect(senior / bank.questions.length).toBeGreaterThan(0.3)
    expect(senior / bank.questions.length).toBeLessThan(0.7)
  })
})
