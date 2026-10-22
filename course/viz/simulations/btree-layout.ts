// Геометрия сцены B-tree: чистые функции без Vue, чтобы рендерер только рисовал.
// Размеры взяты из утверждённого прототипа (холст 1080 px по ширине).
import { isLeaf, leafOrder, rowAt, type BtModel, type InnerNode, type LeafNode, type Tid } from './btree-model'
import type { BtView } from './btree'

export interface Box { x: number; y: number; w: number; h: number }
export interface SceneLayout { boxes: Record<string, Box>; width: number; height: number }

export const SCENE_WIDTH = 1080
const ROW_Y = { root: 30, inner: 140, leaf: 262, heap: 430 }
const BOTTOM_PAD = 24

const nodeHeight = (detail: boolean, entries: number): number => (detail ? 32 + entries * 18 : 50)
export const heapHeaderH = (detail: boolean): number => (detail ? 42 : 24)
export const heapRowH = (detail: boolean): number => (detail ? 18 : 17)

function placeParent(boxes: Record<string, Box>, n: InnerNode, y: number, w: number, detail: boolean): void {
  const kids = n.entries.map((e) => boxes[e.child]).filter((b): b is Box => b !== undefined)
  const cx = kids.reduce((s, b) => s + b.x + b.w / 2, 0) / Math.max(kids.length, 1)
  boxes[n.id] = { x: cx - w / 2, y, w, h: nodeHeight(detail, n.entries.length) }
}

export function layoutScene(m: BtModel, view: BtView): SceneLayout {
  const detail = view === 'detail'
  const boxes: Record<string, Box> = {}
  const leaves = leafOrder(m)
  const slot = SCENE_WIDTH / leaves.length
  const leafW = Math.min(detail ? 184 : 156, slot - 12)
  leaves.forEach((id, i) => {
    const n = m.nodes[id] as LeafNode
    boxes[id] = { x: slot * (i + 0.5) - leafW / 2, y: ROW_Y.leaf, w: leafW, h: nodeHeight(detail, n.entries.length) }
  })
  const innerW = detail ? 160 : 150
  const inner = Object.values(m.nodes).filter((n): n is InnerNode => !isLeaf(n))
  inner.filter((n) => n.level === 1).forEach((n) => placeParent(boxes, n, ROW_Y.inner, innerW, detail))
  const root = m.nodes[m.root]
  if (root && !isLeaf(root)) placeParent(boxes, root, ROW_Y.root, innerW, detail)

  const pageSlot = SCENE_WIDTH / m.pages.length
  const pageW = Math.min(158, pageSlot - 12)
  let bottom = ROW_Y.heap
  m.pages.forEach((p, i) => {
    const h = detail ? 44 + p.rows.length * 18 : 34 + p.rows.length * 17
    boxes[`page:${p.no}`] = { x: pageSlot * (i + 0.5) - pageW / 2, y: ROW_Y.heap, w: pageW, h }
    bottom = Math.max(bottom, ROW_Y.heap + h)
  })
  boxes.meta = { x: SCENE_WIDTH - 182, y: ROW_Y.root, w: 162, h: 50 }
  return { boxes, width: SCENE_WIDTH, height: bottom + BOTTOM_PAD }
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
  const sy = detail ? lb.y + lb.h : lb.y + 44
  const ex = pb.x + pb.w / 2
  const top = pb.y - 2
  return `M${sx},${sy} C${sx},${sy + 70} ${ex},${top - 70} ${ex},${top}`
}
