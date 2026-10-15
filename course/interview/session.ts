import type { Question } from './schema'

export interface Session {
  seed: number
  ids: readonly string[]
}

// Отметки самооценки: по вопросу — флажки пунктов чеклиста.
export type Marks = Readonly<Record<string, readonly boolean[]>>

export interface TopicScore {
  topic: string
  lessons: readonly string[]
  got: number
  total: number
}

export interface Score {
  total: { got: number; total: number }
  topics: readonly TopicScore[]
  weak: readonly TopicScore[]
}

// Тема «слабая», если отмечено меньше этой доли пунктов чеклиста.
const WEAK_RATIO = 0.6

// mulberry32 — маленький детерминированный генератор: сессию можно повторить по сиду,
// а тесты не зависят от Math.random.
export function mulberry32(seed: number): () => number {
  let a = seed >>> 0
  return () => {
    a = (a + 0x6d2b79f5) >>> 0
    let t = a
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

function shuffle<T>(xs: readonly T[], rnd: () => number): T[] {
  const out = [...xs]
  for (let i = out.length - 1; i > 0; i--) {
    const j = Math.floor(rnd() * (i + 1))
    ;[out[i], out[j]] = [out[j] as T, out[i] as T]
  }
  return out
}

// pickSession выбирает вопросы так, чтобы покрыть как можно больше тем и не ставить
// одну тему два раза подряд: на настоящем собеседовании темы тоже чередуются.
export function pickSession(questions: readonly Question[], size: number, seed: number): Session {
  if (!Number.isInteger(size) || size < 1) throw new Error(`размер сессии — целое ≥ 1, получено ${size}`)
  const pool = shuffle(questions, mulberry32(seed))
  const used = new Set<string>()
  const ids: string[] = []
  let last: string | undefined
  const left = (topic: string) => pool.filter((q) => q.topic === topic && !ids.includes(q.id)).length
  while (ids.length < Math.min(size, questions.length)) {
    const free = pool.filter((q) => !ids.includes(q.id))
    // Сначала — новая тема (в случайном порядке пула).
    const fresh = free.find((q) => !used.has(q.topic) && q.topic !== last)
    // Новых тем нет — тема, где осталось больше всего вопросов: иначе в конце
    // останутся вопросы одной темы и пойдут подряд (жадный алгоритм перестановки).
    const busiest = free
      .filter((q) => q.topic !== last)
      .reduce<Question | undefined>((best, q) => (best === undefined || left(q.topic) > left(best.topic) ? q : best), undefined)
    const next = fresh ?? busiest ?? free[0]
    if (next === undefined) break
    ids.push(next.id)
    used.add(next.topic)
    last = next.topic
  }
  return { seed, ids }
}

export function scoreSession(questions: readonly Question[], session: Session, marks: Marks): Score {
  const byTopic = new Map<string, { lessons: Set<string>; got: number; total: number }>()
  for (const id of session.ids) {
    const q = questions.find((x) => x.id === id)
    if (q === undefined) continue
    const got = (marks[id] ?? []).slice(0, q.checklist.length).filter(Boolean).length
    const t = byTopic.get(q.topic) ?? { lessons: new Set<string>(), got: 0, total: 0 }
    t.lessons.add(q.lesson)
    t.got += got
    t.total += q.checklist.length
    byTopic.set(q.topic, t)
  }
  const topics = [...byTopic].map(([topic, t]) => ({ topic, lessons: [...t.lessons], got: t.got, total: t.total }))
  const ratio = (t: TopicScore) => (t.total === 0 ? 1 : t.got / t.total)
  const weak = topics.filter((t) => ratio(t) < WEAK_RATIO).sort((a, b) => ratio(a) - ratio(b))
  const total = topics.reduce((acc, t) => ({ got: acc.got + t.got, total: acc.total + t.total }), { got: 0, total: 0 })
  return { total, topics, weak }
}
