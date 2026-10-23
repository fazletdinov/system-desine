// Подпись шага: обычная строка или сегменты, часть из которых — термины глоссария.
export interface CaptionSegment {
  text: string
  termId?: string
}

export type Caption = string | readonly CaptionSegment[]

export function captionSegments(caption: Caption): readonly CaptionSegment[] {
  return typeof caption === 'string' ? [{ text: caption }] : caption
}

export function captionText(caption: Caption): string {
  return captionSegments(caption).map((s) => s.text).join('')
}

export function captionTermIds(caption: Caption): string[] {
  return captionSegments(caption).flatMap((s) => (s.termId === undefined ? [] : [s.termId]))
}

// Сегмент-термин: в плеере превращается в подсказку из глоссария.
export function term(text: string, termId: string): CaptionSegment {
  return { text, termId }
}

// Разметка термина в тексте: «[[id|текст]]». Так длинные тексты объяснений пишутся одной строкой.
const MARKER = /\[\[([a-z0-9-]+)\|([^\]]+)\]\]/g

// Разметка, не распознанная как термин (опечатка в id, нет «|»), — ошибка автора, а не текст.
export function stripTerms(src: string): string {
  const plain = src.replace(MARKER, '$2')
  if (plain.includes('[[') || plain.includes(']]')) throw new Error(`caption: битая разметка термина в «${src}»`)
  return plain
}

// «[[id|текст]]» → сегменты-термины. Термин из seen уже подчёркивали — он остаётся обычным текстом:
// так термин выделяется один раз на объяснение или на сцену, а не в каждой строке.
export function markedCaption(src: string, seen: Set<string>): Caption {
  stripTerms(src)
  const out: CaptionSegment[] = []
  let from = 0
  for (const m of src.matchAll(MARKER)) {
    const [whole, id, text] = [m[0], m[1] ?? '', m[2] ?? '']
    if (m.index > from) out.push({ text: src.slice(from, m.index) })
    if (seen.has(id)) out.push({ text })
    else {
      seen.add(id)
      out.push(term(text, id))
    }
    from = m.index + whole.length
  }
  if (from < src.length) out.push({ text: src.slice(from) })
  const only = out[0]
  return out.length === 1 && only !== undefined && only.termId === undefined ? only.text : out
}
