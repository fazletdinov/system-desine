// Банк вопросов мини-интервью: YAML → проверенные типы. Ошибка называет файл и
// вопрос: банк пишут руками, и опечатку надо найти за секунды.

export type Level = 'M+' | 'S'

export interface FollowUp {
  q: string
  a: string
}

export interface Question {
  id: string
  lesson: string // путь урока без .md относительно course/modules, например 01-go-service/04-usecases
  topic: string
  level: Level
  question: string
  answer: string // Markdown
  checklist: readonly string[]
  red_flags: readonly string[]
  follow_ups: readonly FollowUp[]
}

export interface Bank {
  module: string
  title: string
  questions: readonly Question[]
}

const LIMITS = { checklist: [3, 6], red_flags: [1, 3], follow_ups: [1, 3] } as const
const ID = /^[a-z0-9]+(?:-[a-z0-9]+)*$/
const LESSON = /^[0-9]{2}[a-z0-9-]*\/[0-9]{2}-[a-z0-9-]+$/

type Obj = Record<string, unknown>
const isObj = (v: unknown): v is Obj => typeof v === 'object' && v !== null && !Array.isArray(v)

function text(o: Obj, key: string, where: string): string {
  const v = o[key]
  if (typeof v !== 'string' || v.trim() === '') throw new Error(`${where}: поле ${key} — непустая строка`)
  return v
}

function list<T>(o: Obj, key: keyof typeof LIMITS, where: string, item: (v: unknown, at: string) => T): T[] {
  const v = o[key]
  const [min, max] = LIMITS[key]
  if (!Array.isArray(v) || v.length < min || v.length > max) throw new Error(`${where}: ${key} — от ${min} до ${max} пунктов`)
  return v.map((x, i) => item(x, `${where}.${key}[${i}]`))
}

const str = (v: unknown, at: string): string => {
  if (typeof v !== 'string' || v.trim() === '') throw new Error(`${at}: непустая строка`)
  return v
}

function parseQuestion(raw: unknown, where: string): Question {
  if (!isObj(raw)) throw new Error(`${where}: вопрос — объект`)
  const id = text(raw, 'id', where)
  if (!ID.test(id)) throw new Error(`${where}: id «${id}» — латиница в kebab-case`)
  const at = `${where} (${id})`
  const lesson = text(raw, 'lesson', at)
  if (!LESSON.test(lesson)) throw new Error(`${at}: lesson «${lesson}» — вида 01-go-service/04-usecases`)
  const level = raw.level
  if (level !== 'M+' && level !== 'S') throw new Error(`${at}: level — M+ или S`)
  return {
    id,
    lesson,
    level,
    topic: text(raw, 'topic', at),
    question: text(raw, 'question', at),
    answer: text(raw, 'answer', at),
    checklist: list(raw, 'checklist', at, str),
    red_flags: list(raw, 'red_flags', at, str),
    follow_ups: list(raw, 'follow_ups', at, (v, a) => {
      if (!isObj(v)) throw new Error(`${a}: объект {q, a}`)
      return { q: text(v, 'q', a), a: text(v, 'a', a) }
    }),
  }
}

export function parseBank(raw: unknown, source: string): Bank {
  if (!isObj(raw)) throw new Error(`${source}: банк — объект`)
  const questions = raw.questions
  if (!Array.isArray(questions) || questions.length === 0) throw new Error(`${source}: questions — непустой список`)
  const parsed = questions.map((q, i) => parseQuestion(q, `${source}: questions[${i}]`))
  const seen = new Set<string>()
  for (const q of parsed) {
    if (seen.has(q.id)) throw new Error(`${source}: id «${q.id}» повторяется`)
    seen.add(q.id)
  }
  return { module: text(raw, 'module', source), title: text(raw, 'title', source), questions: parsed }
}
