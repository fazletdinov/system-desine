// Мелкая логика симулятора, вынесенная из компонента, чтобы её можно было
// проверить без браузера: сид, подпись урока, история прошлых сессий.

export interface HistoryEntry {
  at: string // ISO 8601
  seed: number
  ratio: number // доля отмеченных пунктов, 0…1
  weak: readonly string[] // слабые темы
}

// Сколько прошлых сессий помнить: больше на экране уже не читается.
export const HISTORY_LIMIT = 10
// Версия в ключе: при смене формата старые записи просто не прочитаются.
export const historyKey = (module: string): string => `interview:v1:${module}`

const MAX_SEED = 0xffffffff

// Сид — целое без знака в 32 бита: столько принимает mulberry32.
export function parseSeed(text: string): number | undefined {
  const t = text.trim()
  if (!/^\d{1,10}$/.test(t)) return undefined
  const n = Number(t)
  return n <= MAX_SEED ? n : undefined
}

export function randomSeed(): number {
  return crypto.getRandomValues(new Uint32Array(1))[0] ?? 0
}

// «01-go-service/04-usecases» → «1.4», «00b-network/06-http1-pools» → «0b.6».
export function lessonLabel(lesson: string): string {
  const [dir = '', file = ''] = lesson.split('/')
  const mod = /^(\d{2})([a-z]?)/.exec(dir)
  const num = /^(\d{2})/.exec(file)
  if (mod === null || num === null) return lesson
  return `${Number(mod[1])}${mod[2] ?? ''}.${Number(num[1])}`
}

function isEntry(v: unknown): v is HistoryEntry {
  if (typeof v !== 'object' || v === null) return false
  const e = v as Record<string, unknown>
  return (
    typeof e.at === 'string' &&
    typeof e.seed === 'number' &&
    typeof e.ratio === 'number' &&
    Array.isArray(e.weak) &&
    e.weak.every((t) => typeof t === 'string')
  )
}

// Хранилище пишет и читает не только наш код: испорченная запись не должна ронять компонент.
export function parseHistory(raw: string | null): HistoryEntry[] {
  if (raw === null) return []
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return []
  }
  return Array.isArray(parsed) ? parsed.filter(isEntry).slice(0, HISTORY_LIMIT) : []
}

export function appendHistory(list: readonly HistoryEntry[], entry: HistoryEntry): HistoryEntry[] {
  return [entry, ...list].slice(0, HISTORY_LIMIT)
}
