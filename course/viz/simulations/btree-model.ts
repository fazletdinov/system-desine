// Модель индекса B-tree и таблицы (heap) PostgreSQL для сцены 2.5.
// Чистые данные и функции без Vue: сценарии строят из них шаги, рендерер рисует.
// Данные учебные: user_id — маленькие числа вместо uuid, чтобы читалось.

// Адрес строки в таблице: номер страницы и позиция на ней (с 1).
export type Tid = readonly [page: number, lp: number]
export interface HeapRow { lp: number; chat: string; user: number }
export interface HeapPage { no: number; rows: HeapRow[]; allVisible: boolean }
export interface LeafEntry { key: number; tids: Tid[] }
// sep === null — «−∞»: у первой записи страницы нет нижней границы.
export interface InnerEntry { sep: number | null; child: string }
export interface LeafNode { id: string; block: number; level: 0; entries: LeafEntry[]; left: string | null; right: string | null }
export interface InnerNode { id: string; block: number; level: number; entries: InnerEntry[]; left: string | null; right: string | null }
export type BtNode = LeafNode | InnerNode
export interface BtModel { pages: HeapPage[]; nodes: Record<string, BtNode>; root: string; nextBlock: number }

// 12 чатов по 2 участника → 24 строки chat.members: [user_a, user_b].
export const CHATS: readonly (readonly [number, number])[] = [
  [42, 15], [27, 84], [3, 63], [42, 99], [19, 50], [8, 71],
  [63, 36], [23, 88], [27, 95], [47, 31], [56, 90], [42, 77],
]
const PER_PAGE = 4
export const LEAF_CAPACITY = 5
export const BUILD_FILL = 4
// Страница, на которой после «последнего VACUUM» были изменения, поэтому она не all-visible.
const DIRTY_PAGE = 5

export function isLeaf(n: BtNode): n is LeafNode {
  return n.level === 0
}

// Общие помощники сцены: модель, сценарии, объяснения, раскладка и рендерер берут их отсюда,
// чтобы формат адресов и правило поиска ключа жили в одном месте.
export function need<T>(v: T | undefined, what: string): T {
  if (v === undefined) throw new Error(`btree: нет ${what}`)
  return v
}

const node = (m: BtModel, id: string): BtNode => need(m.nodes[id], `узла ${id}`)
export const blockOf = (m: BtModel, id: string): number => node(m, id).block
export const tidS = (t: Tid): string => `(${t[0]},${t[1]})`
export const sepS = (e: InnerEntry): string => String(e.sep ?? '−∞')
// Ключ строки таблицы «страница:позиция» — так строки помечены в кадре и в id элементов схемы (row:p:lp).
export const rowKey = (page: number, lp: number): string => `${page}:${lp}`

// Лист, где лежит запись ключа. В модели ключ живёт ровно в одном листе; в PostgreSQL длинный
// posting list может продолжиться в соседнем — тогда это вернуло бы первый из них.
export function leafOfKey(m: BtModel, key: number): { leaf: LeafNode; index: number; entry: LeafEntry } | undefined {
  for (const n of Object.values(m.nodes)) {
    if (!isLeaf(n)) continue
    const index = n.entries.findIndex((e) => e.key === key)
    const entry = n.entries[index]
    if (entry) return { leaf: n, index, entry }
  }
  return undefined
}

function minKey(m: BtModel, id: string): number {
  let n = node(m, id)
  while (!isLeaf(n)) n = node(m, need(n.entries[0], 'записи').child)
  return need(n.entries[0], 'ключа').key
}

export function buildModel(): BtModel {
  const pages: HeapPage[] = []
  let i = 0
  CHATS.forEach(([a, b], ci) => {
    for (const user of [a, b]) {
      const no = Math.floor(i / PER_PAGE)
      const page = (pages[no] ??= { no, rows: [], allVisible: no !== DIRTY_PAGE })
      page.rows.push({ lp: (i % PER_PAGE) + 1, chat: `c${ci + 1}`, user })
      i++
    }
  })

  const postings = new Map<number, Tid[]>()
  for (const p of pages) for (const r of p.rows) postings.set(r.user, [...(postings.get(r.user) ?? []), [p.no, r.lp]])
  const keys = [...postings.keys()].sort((x, y) => x - y)

  const nodes: Record<string, BtNode> = {}
  const leaves: string[] = []
  for (let j = 0; j < keys.length; j += BUILD_FILL) {
    const block = leaves.length + 1
    const id = `b${block}`
    const entries = keys.slice(j, j + BUILD_FILL).map((key) => ({ key, tids: need(postings.get(key), 'TID') }))
    nodes[id] = { id, block, level: 0, entries, left: null, right: null }
    leaves.push(id)
  }
  leaves.forEach((id, j) => {
    const n = need(nodes[id], `узла ${id}`)
    n.left = leaves[j - 1] ?? null
    n.right = leaves[j + 1] ?? null
  })

  const m: BtModel = { pages, nodes, root: 'b8', nextBlock: 9 }
  const makeInner = (block: number, level: number, kids: string[]) => {
    const id = `b${block}`
    nodes[id] = { id, block, level, entries: kids.map((child, j) => ({ sep: j ? minKey(m, child) : null, child })), left: null, right: null }
  }
  makeInner(6, 1, leaves.slice(0, 3))
  makeInner(7, 1, leaves.slice(3))
  node(m, 'b6').right = 'b7'
  node(m, 'b7').left = 'b6'
  makeInner(8, 2, ['b6', 'b7'])
  return m
}

// Индекс последней записи с sep ≤ key; у первой записи sep = −∞, она подходит всегда.
export function pickChild(n: InnerNode, key: number): number {
  let idx = 0
  n.entries.forEach((e, j) => {
    if (j > 0 && e.sep !== null && e.sep <= key) idx = j
  })
  return idx
}

export function leafFor(m: BtModel, key: number): LeafNode {
  let n = node(m, m.root)
  while (!isLeaf(n)) n = node(m, need(n.entries[pickChild(n, key)], 'ветки').child)
  return n
}

export function leafOrder(m: BtModel): string[] {
  const first = Object.values(m.nodes).find((n) => isLeaf(n) && n.left === null)
  const out: string[] = []
  let id: string | null = first?.id ?? null
  while (id) {
    out.push(id)
    id = node(m, id).right
  }
  return out
}

export function parentOf(m: BtModel, id: string): InnerNode | undefined {
  return Object.values(m.nodes).find((n): n is InnerNode => !isLeaf(n) && n.entries.some((e) => e.child === id))
}

export function rowAt(m: BtModel, t: Tid): HeapRow | undefined {
  return m.pages[t[0]]?.rows.find((r) => r.lp === t[1])
}

// Новая строка идёт в конец последней страницы; если та полна — открывается новая.
// Любая вставка снимает all-visible: до VACUUM страница снова требует проверки видимости.
export function heapInsert(m: BtModel, chat: string, user: number): Tid {
  let page = m.pages.at(-1)
  if (!page || page.rows.length >= PER_PAGE) {
    page = { no: m.pages.length, rows: [], allVisible: false }
    m.pages.push(page)
  }
  const lp = page.rows.length + 1
  page.rows.push({ lp, chat, user })
  page.allVisible = false
  return [page.no, lp]
}

export function leafInsert(leaf: LeafEntry[] | LeafNode, key: number, tid: Tid): void {
  const entries = Array.isArray(leaf) ? leaf : leaf.entries
  const existing = entries.find((e) => e.key === key)
  if (existing) {
    existing.tids.push(tid)
    return
  }
  entries.push({ key, tids: [tid] })
  entries.sort((a, b) => a.key - b.key)
}

// Правая половина уходит в новый блок. Запись в родителе не добавляем: это отдельный шаг сценария.
export function splitLeaf(m: BtModel, leaf: LeafNode): LeafNode {
  const block = m.nextBlock++
  const id = `b${block}`
  const right = leaf.entries.splice(Math.ceil(leaf.entries.length / 2))
  const created: LeafNode = { id, block, level: 0, entries: right, left: leaf.id, right: leaf.right }
  m.nodes[id] = created
  if (leaf.right) node(m, leaf.right).left = id
  leaf.right = id
  return created
}

export function cloneModel(m: BtModel): BtModel {
  return structuredClone(m)
}
