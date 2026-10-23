import { captionTermIds, type Caption } from './caption'
import type { Step } from './types'

export interface ExplainSection {
  heading: string
  body: Caption
}

// Разбор элемента схемы: что это, как устроено, что здесь, как в настоящей системе.
export interface Explanation {
  title: string
  kind: string
  sections: readonly ExplainSection[]
  here?: Caption
  real?: Caption
}

// null — по элементу объяснения нет, клик по нему ничего не открывает.
export type Explain<S> = (id: string, step: Step<S>) => Explanation | null

// Порядок = порядок показа: подпись, карточка (что → чем → как читать → зачем), вопрос → ответ.
// Нужен тестам сцен: каждый termId обязан быть в глоссарии.
export function stepTermIds<S>(step: Step<S>): string[] {
  const { card, quiz } = step
  const captions: Caption[] = [step.caption]
  if (card) captions.push(card.what, card.uses, card.reads, card.why)
  if (quiz) captions.push(quiz.question, quiz.answer)
  return captions.flatMap(captionTermIds)
}

export function explanationTermIds(e: Explanation): string[] {
  const captions: Caption[] = e.sections.map((s) => s.body)
  if (e.here !== undefined) captions.push(e.here)
  if (e.real !== undefined) captions.push(e.real)
  return captions.flatMap(captionTermIds)
}
