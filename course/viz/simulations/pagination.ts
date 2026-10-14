import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Simulation, Step } from '../core/types'

export const PAGE_SIZE = 5
export const INITIAL_COUNT = 30

export type PagingMode = 'offset' | 'cursor'

// Лента — номера сообщений (seq) от новых к старым, как история чата в мессенджере.
export interface PagingState {
  mode: PagingMode
  feed: readonly number[]
  initial: readonly number[] // лента в момент, когда начали листать
  nextSeq: number
  pages: readonly (readonly number[])[]
  offset: number // offset: сколько строк пропустить
  cursor: number | null // курсор: seq последнего показанного
  last: PagingAction['type'] | 'start'
}

export type PagingAction =
  | { type: 'next' }
  | { type: 'insert' }
  | { type: 'delete' }
  | { type: 'mode'; mode: PagingMode }
  | { type: 'reset' }

function start(mode: PagingMode): PagingState {
  const feed = Array.from({ length: INITIAL_COUNT }, (_, i) => INITIAL_COUNT - i)
  return { mode, feed, initial: feed, nextSeq: INITIAL_COUNT + 1, pages: [], offset: 0, cursor: null, last: 'start' }
}

// nextPage — то, что вернул бы сервер: offset считает строки, курсор — сравнивает ключи.
export function nextPage(s: PagingState): number[] {
  if (s.mode === 'offset') return s.feed.slice(s.offset, s.offset + PAGE_SIZE)
  const cursor = s.cursor
  const from = cursor === null ? 0 : s.feed.findIndex((seq) => seq < cursor)
  return from < 0 ? [] : s.feed.slice(from, from + PAGE_SIZE)
}

export function duplicates(s: PagingState): number[] {
  const seen = new Set<number>()
  const dup = new Set<number>()
  for (const seq of s.pages.flat()) (seen.has(seq) ? dup : seen).add(seq)
  return [...dup].sort((a, b) => b - a)
}

// Пропуск — сообщение, которое было в ленте с начала, всё ещё есть, лежит не ниже
// самого старого показанного, но не попало ни на одну страницу.
export function skipped(s: PagingState): number[] {
  const shown = new Set(s.pages.flat())
  if (shown.size === 0) return []
  const lowest = Math.min(...shown)
  const alive = new Set(s.feed)
  return s.initial.filter((seq) => seq >= lowest && !shown.has(seq) && alive.has(seq))
}

export const paginationSimulation: Simulation<PagingState, PagingAction> = {
  initial: () => start('offset'),
  apply: (s, a) => {
    switch (a.type) {
      case 'reset':
        return start(s.mode)
      case 'mode':
        return start(a.mode)
      case 'insert':
        return { ...s, feed: [s.nextSeq, ...s.feed], nextSeq: s.nextSeq + 1, last: 'insert' }
      case 'delete':
        return s.feed.length === 0 ? s : { ...s, feed: s.feed.slice(1), last: 'delete' }
      case 'next': {
        const page = nextPage(s)
        return {
          ...s,
          pages: [...s.pages, page],
          offset: s.offset + PAGE_SIZE,
          cursor: page.length > 0 ? (page.at(-1) as number) : s.cursor,
          last: 'next',
        }
      }
    }
  },
  steps: (s) => [{ state: s, caption: captionFor(s) } satisfies Step<PagingState>],
}

function captionFor(s: PagingState): Caption {
  const dup = duplicates(s)
  const skip = skipped(s)
  const how: Caption =
    s.mode === 'offset'
      ? [term('Offset-пагинация', 'offset-pagination'), { text: ': «пропусти N строк, дай 5». N считается от верха ленты, а верх двигается.' }]
      : [term('Курсор', 'cursor'), { text: ': «дай 5 старше seq последнего показанного». Ключ не зависит от того, что случилось выше.' }]
  if (s.last === 'start') return [...how, { text: ' Листайте, а между страницами добавляйте и удаляйте сообщения.' }]
  const verdict =
    dup.length + skip.length === 0
      ? 'Дублей и пропусков нет.'
      : `Дубли: ${dup.join(', ') || 'нет'}. Пропуски: ${skip.join(', ') || 'нет'}.`
  return [...how, { text: ` ${verdict} Это и есть ` }, term('дрейф страниц', 'page-drift'), { text: s.mode === 'offset' ? ' — болезнь offset.' : ' — у курсора его нет.' }]
}
