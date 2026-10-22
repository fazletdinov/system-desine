// Геометрия сцены B-tree: чистые функции без Vue, чтобы рендерер только рисовал.
// Холст 1080 единиц по ширине, как в прототипе; кегли и высоты строк увеличены под чтение во весь экран.
import { isLeaf, leafOrder, rowAt, type BtModel, type InnerNode, type LeafNode, type Tid } from './btree-model'
import type { BtView } from './btree'

export interface Box { x: number; y: number; w: number; h: number }
export interface Rows { root: number; inner: number; leaf: number; heap: number }
export interface SceneLayout { boxes: Record<string, Box>; rows: Rows; width: number; height: number }

export const SCENE_WIDTH = 1080
// Сцена смотрится во весь экран: на 1280 px холст рядом с панелью ≈ 880 px, масштаб ≈ 0,8.
// Шрифты в единицах viewBox подобраны так, чтобы на экране ключи, строки и TID были ≥ 12 px,
// а подписи — ≥ 11 px. Вся геометрия ниже рассчитана на моноширинный знак ≈ 0,6 кегля.
export const FONT = { mono: 15, label: 14 }
export const LINE_H = 21
// Posting list из трёх TID в одну строку не помещается в лист шириной 168: переносим по два.
export const TIDS_PER_LINE = 2
// Верх рамки → первая строка записей; в simple — ряд ячеек с ключами.
export const ENTRY_TOP = 28
export const CELL_H = 22
// Отступ строк: заголовок страницы и (в detail) подписи столбцов.
export const heapHeaderH = (detail: boolean): number => (detail ? 48 : 28)
export const heapRowH = (): number => 20

const ROWS: Record<BtView, Rows> = {
  simple: { root: 36, inner: 136, leaf: 240, heap: 360 },
  // Ряды разнесены под самые высокие узлы: внутренняя страница с 4 разделителями и лист из 6 строк.
  detail: { root: 36, inner: 156, leaf: 304, heap: 500 },
}
const BOTTOM_PAD = 24

export const entryLines = (tids: number): number => Math.max(1, Math.ceil(tids / TIDS_PER_LINE))
// Номер строки, с которой начинается каждая запись листа (запись с длинным posting list занимает несколько).
export function lineOffsets(tidCounts: readonly number[]): number[] {
  let line = 0
  return tidCounts.map((n) => {
    const at = line
    line += entryLines(n)
    return at
  })
}
const nodeHeight = (detail: boolean, lines: number): number => (detail ? ENTRY_TOP + 8 + lines * LINE_H : ENTRY_TOP + CELL_H + 6)

function placeParent(boxes: Record<string, Box>, n: InnerNode, y: number, w: number, detail: boolean): void {
  const kids = n.entries.map((e) => boxes[e.child]).filter((b): b is Box => b !== undefined)
  const cx = kids.reduce((s, b) => s + b.x + b.w / 2, 0) / Math.max(kids.length, 1)
  boxes[n.id] = { x: cx - w / 2, y, w, h: nodeHeight(detail, n.entries.length) }
}

export function layoutScene(m: BtModel, view: BtView): SceneLayout {
  const detail = view === 'detail'
  const rows = ROWS[view]
  const boxes: Record<string, Box> = {}
  const leaves = leafOrder(m)
  const slot = SCENE_WIDTH / leaves.length
  const leafW = Math.min(200, slot - 12)
  leaves.forEach((id, i) => {
    const n = m.nodes[id]
    if (!n || !isLeaf(n)) return
    const lines = n.entries.reduce((s, e) => s + entryLines(e.tids.length), 0)
    boxes[id] = { x: slot * (i + 0.5) - leafW / 2, y: rows.leaf, w: leafW, h: nodeHeight(detail, lines) }
  })
  const innerW = detail ? 170 : 150
  const inner = Object.values(m.nodes).filter((n): n is InnerNode => !isLeaf(n))
  inner.filter((n) => n.level === 1).forEach((n) => placeParent(boxes, n, rows.inner, innerW, detail))
  const root = m.nodes[m.root]
  if (root && !isLeaf(root)) placeParent(boxes, root, rows.root, innerW, detail)

  const pageSlot = SCENE_WIDTH / m.pages.length
  const pageW = Math.min(170, pageSlot - 8)
  let bottom = rows.heap
  m.pages.forEach((p, i) => {
    const h = heapHeaderH(detail) + p.rows.length * heapRowH() + 6
    boxes[`page:${p.no}`] = { x: pageSlot * (i + 0.5) - pageW / 2, y: rows.heap, w: pageW, h }
    bottom = Math.max(bottom, rows.heap + h)
  })
  boxes.meta = { x: SCENE_WIDTH - 220, y: rows.root, w: 200, h: 56 }
  return { boxes, rows, width: SCENE_WIDTH, height: bottom + BOTTOM_PAD }
}

export interface TidLink { key: number; tid: Tid; selected: boolean }
export interface Selection { entries: Set<number>; rows: Set<string>; tids: TidLink[] }

// Клик по записи подсвечивает её строки таблицы, клик по строке — запись индекса: связь видна в обе стороны.
export function selectionLinks(m: BtModel, selected: string | null): Selection {
  const out: Selection = { entries: new Set(), rows: new Set(), tids: [] }
  const [kind, a, b] = (selected ?? '').split(':')
  if (kind === 'entry') {
    const key = Number(a)
    const entry = Object.values(m.nodes).flatMap((n) => (isLeaf(n) ? n.entries : [])).find((e) => e.key === key)
    if (!entry) return out
    out.entries.add(key)
    for (const t of entry.tids) {
      out.rows.add(`${t[0]}:${t[1]}`)
      out.tids.push({ key, tid: t, selected: true })
    }
  }
  if (kind === 'row') {
    const row = rowAt(m, [Number(a), Number(b)])
    if (!row) return out
    out.rows.add(`${a}:${b}`)
    out.entries.add(row.user)
    out.tids.push({ key: row.user, tid: [Number(a), Number(b)], selected: true })
  }
  return out
}

export function leafOfKey(m: BtModel, key: number): { leaf: LeafNode; index: number } | undefined {
  for (const n of Object.values(m.nodes)) {
    if (!isLeaf(n)) continue
    const index = n.entries.findIndex((e) => e.key === key)
    if (index >= 0) return { leaf: n, index }
  }
  return undefined
}

// TID-стрелка идёт от низа листа (а не от записи), чтобы не пересекать текст листа.
export function tidPath(m: BtModel, boxes: Record<string, Box>, view: BtView, link: { key: number; tid: Tid }): string | null {
  const detail = view === 'detail'
  const found = leafOfKey(m, link.key)
  const page = m.pages[link.tid[0]]
  const lb = found && boxes[found.leaf.id]
  const pb = boxes[`page:${link.tid[0]}`]
  if (!found || !page || !lb || !pb) return null
  const n = found.leaf.entries.length
  const sx = detail ? lb.x + lb.w / 2 + (found.index - (n - 1) / 2) * 14 : lb.x + 4 + ((lb.w - 8) / n) * (found.index + 0.5)
  const sy = detail ? lb.y + lb.h : lb.y + ENTRY_TOP + CELL_H
  const ex = pb.x + pb.w / 2
  const top = pb.y - 2
  return `M${sx},${sy} C${sx},${sy + 70} ${ex},${top - 70} ${ex},${top}`
}
