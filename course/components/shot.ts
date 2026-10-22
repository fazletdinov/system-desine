// Пометка на снимке экрана. Координаты — в процентах от размеров картинки,
// чтобы пометки не зависели от того, в каком масштабе снимок показан.
// С w и h пометка — рамка с левым верхним углом в (x, y), а номер стоит рядом
// с рамкой снаружи (см. layoutMarks). Без рамки (x, y) — центр кружка с номером.
export interface ShotMark {
  n: number
  x: number
  y: number
  w?: number
  h?: number
  // Где поставить номер относительно рамки, если автоматический выбор закрывает
  // нужный текст: в плотной таблице соседние строки лучше не трогать, и номер
  // уходит выше или ниже — в пустую область.
  at?: MarkSide
  // Пояснение под снимком. Текст в `обратных кавычках` выводится как код.
  text: string
}

export const MARK_SIDES = ['right', 'left', 'above', 'below'] as const
export type MarkSide = (typeof MARK_SIDES)[number]

export interface MarkLayout {
  n: number
  cx: number
  cy: number
  rect: { x: number; y: number; w: number; h: number } | null
}

export interface TextSegment {
  text: string
  code: boolean
}

// Радиус кружка — доля ширины снимка: на экране он одного размера при любом
// разрешении исходника и растёт вместе со снимком в полноэкранном режиме.
export const MARK_RADIUS_SHARE = 0.015

const inRange = (v: number): boolean => Number.isFinite(v) && v >= 0 && v <= 100

// Ошибки разметки, которые иначе видны только глазами: номер пропущен или
// повторён, пометка за краем снимка, пояснения нет.
export function markProblems(marks: readonly ShotMark[]): string[] {
  const problems: string[] = []
  const numbers = marks.map((m) => m.n)
  const sorted = [...numbers].sort((a, b) => a - b)
  if (sorted.some((n, i) => n !== i + 1)) problems.push(`номера пометок должны идти 1…${marks.length} без пропусков и повторов: ${numbers.join(', ')}`)
  for (const m of marks) {
    if (!inRange(m.x) || !inRange(m.y)) problems.push(`пометка ${m.n}: (x, y) вне снимка`)
    if ((m.w === undefined) !== (m.h === undefined)) problems.push(`пометка ${m.n}: у рамки нужны и w, и h`)
    if (m.w !== undefined && m.h !== undefined && (!(m.w > 0 && m.h > 0) || !inRange(m.x + m.w) || !inRange(m.y + m.h))) problems.push(`пометка ${m.n}: рамка выходит за снимок`)
    if (m.at !== undefined && m.w === undefined) problems.push(`пометка ${m.n}: сторона номера (at) задаётся только у рамки`)
    if (m.at !== undefined && !MARK_SIDES.includes(m.at)) problems.push(`пометка ${m.n}: неизвестная сторона номера «${m.at}»`)
    if (m.text.trim() === '') problems.push(`пометка ${m.n}: нет пояснения`)
    if (m.text.split('`').length % 2 === 0) problems.push(`пометка ${m.n}: непарная обратная кавычка`)
  }
  return problems
}

const clamp = (v: number, lo: number, hi: number): number => Math.min(Math.max(v, lo), hi)

// Пометки в пикселях снимка (под viewBox SVG), по возрастанию номера.
// Рамка чуть шире заданной области, чтобы линия не легла на текст. Номер ставим
// снаружи рамки, по центру её высоты: справа, если там есть место, иначе слева,
// иначе — на левом верхнем углу. Так кружок не закрывает то, на что указывает.
// Поле at задаёт сторону явно: above и below ставят номер по центру ширины рамки.
export function layoutMarks(marks: readonly ShotMark[], width: number, height: number): MarkLayout[] {
  const r = width * MARK_RADIUS_SHARE
  const pad = r * 0.15
  return [...marks]
    .sort((a, b) => a.n - b.n)
    .map((m) => {
      const x = (m.x / 100) * width
      const y = (m.y / 100) * height
      if (m.w === undefined || m.h === undefined) return { n: m.n, cx: clamp(x, r, width - r), cy: clamp(y, r, height - r), rect: null }
      const x0 = Math.max(x - pad, 0)
      const y0 = Math.max(y - pad, 0)
      const x1 = Math.min(x + (m.w / 100) * width + pad, width)
      const y1 = Math.min(y + (m.h / 100) * height + pad, height)
      const rect = { x: x0, y: y0, w: x1 - x0, h: y1 - y0 }
      const [cx, cy] = markCenter(rect, m.at, r, width)
      return { n: m.n, cx: clamp(cx, r, width - r), cy: clamp(cy, r, height - r), rect }
    })
}

function markCenter(rect: { x: number; y: number; w: number; h: number }, at: MarkSide | undefined, r: number, width: number): [number, number] {
  const gap = r * 1.2
  const right = rect.x + rect.w + gap
  const left = rect.x - gap
  const mid = rect.y + rect.h / 2
  const center = rect.x + rect.w / 2
  switch (at) {
    case 'right':
      return [right, mid]
    case 'left':
      return [left, mid]
    case 'above':
      return [center, rect.y - gap]
    case 'below':
      return [center, rect.y + rect.h + gap]
  }
  if (right + r <= width) return [right, mid]
  if (left - r >= 0) return [left, mid]
  return [rect.x, rect.y]
}

// Код в пояснении — в обратных кавычках, как в Markdown урока.
export function textSegments(text: string): TextSegment[] {
  return text
    .split('`')
    .map((part, i) => ({ text: part, code: i % 2 === 1 }))
    .filter((s) => s.text !== '')
}
