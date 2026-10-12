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
