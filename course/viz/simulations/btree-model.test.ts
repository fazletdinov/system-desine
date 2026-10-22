import { describe, expect, it } from 'vitest'
import {
  buildModel, cloneModel, heapInsert, isLeaf, leafFor, leafInsert, leafOrder, parentOf, pickChild, rowAt, splitLeaf,
  type InnerNode, type LeafNode,
} from './btree-model'

const inner = (m: ReturnType<typeof buildModel>, id: string) => m.nodes[id] as InnerNode
const leaf = (m: ReturnType<typeof buildModel>, id: string) => m.nodes[id] as LeafNode

describe('построение модели', () => {
  const m = buildModel()
  it('6 страниц по 4 строки, allVisible false только у страницы 5', () => {
    expect(m.pages.map((p) => p.rows.length)).toEqual([4, 4, 4, 4, 4, 4])
    expect(m.pages.map((p) => p.allVisible)).toEqual([true, true, true, true, true, false])
  })
  it('rowAt([0,2]) — вторая строка первой страницы', () => {
    expect(rowAt(m, [0, 2])).toEqual({ lp: 2, chat: 'c1', user: 15 })
    expect(rowAt(m, [9, 9])).toBeUndefined()
  })
  it('лист с ключом 42 — блок 3 с тремя TID', () => {
    const l = leafFor(m, 42)
    expect(l.block).toBe(3)
    expect(l.entries.find((e) => e.key === 42)?.tids).toEqual([[0, 1], [1, 3], [5, 3]])
  })
  it.each([
    ['b8', [null, 63]],
    ['b6', [null, 23, 42]],
    ['b7', [null, 88]],
  ])('разделители %s', (id, seps) => {
    expect(inner(m, id).entries.map((e) => e.sep)).toEqual(seps)
  })
  it('листья связаны слева направо, nextBlock = 9', () => {
    expect(leafOrder(m)).toEqual(['b1', 'b2', 'b3', 'b4', 'b5'])
    expect(m.nextBlock).toBe(9)
    expect(inner(m, 'b6').right).toBe('b7')
    expect(inner(m, 'b7').left).toBe('b6')
  })
})

describe('навигация', () => {
  const m = buildModel()
  it.each([
    ['b6', 20, 0],
    ['b6', 42, 2],
    ['b8', 77, 1],
  ])('pickChild(%s, %i) = %i', (id, key, idx) => {
    expect(pickChild(inner(m, id), key)).toBe(idx)
  })
  it('leafFor(15) — блок 1', () => {
    expect(leafFor(m, 15).block).toBe(1)
  })
  it('parentOf и isLeaf', () => {
    expect(parentOf(m, 'b3')?.id).toBe('b6')
    expect(parentOf(m, 'b8')).toBeUndefined()
    expect(isLeaf(leaf(m, 'b1'))).toBe(true)
    expect(isLeaf(inner(m, 'b8'))).toBe(false)
  })
})

describe('вставка', () => {
  it('heapInsert даёт [6,1], затем [6,2]; страница 6 не all-visible', () => {
    const m = buildModel()
    expect(heapInsert(m, 'c13', 45)).toEqual([6, 1])
    expect(heapInsert(m, 'c14', 53)).toEqual([6, 2])
    expect(m.pages[6]?.allVisible).toBe(false)
  })
  it('дубликат ключа добавляет TID в существующую запись', () => {
    const m = buildModel()
    leafInsert(leaf(m, 'b3'), 42, [6, 1])
    expect(leaf(m, 'b3').entries[0]?.tids).toHaveLength(4)
  })
  it('splitLeaf делит лист и перешивает соседей', () => {
    const m = buildModel()
    const l = leaf(m, 'b3')
    const parentBefore = structuredClone(inner(m, 'b6').entries)
    leafInsert(l, 45, [6, 1])
    leafInsert(l.entries, 53, [6, 2])
    const right = splitLeaf(m, l)
    expect(l.entries.map((e) => e.key)).toEqual([42, 45, 47])
    expect(right.id).toBe('b9')
    expect(right.entries.map((e) => e.key)).toEqual([50, 53, 56])
    expect(l.right).toBe('b9')
    expect(right.right).toBe('b4')
    expect(leaf(m, 'b4').left).toBe('b9')
    expect(m.nextBlock).toBe(10)
    expect(inner(m, 'b6').entries).toEqual(parentBefore)
    expect(parentOf(m, 'b9')).toBeUndefined()
  })
  it('пятая вставка в heap после заполнения страницы 6 открывает страницу 7', () => {
    const m = buildModel()
    const tids = [45, 53, 54, 55, 56].map((u, i) => heapInsert(m, `c${13 + i}`, u))
    expect(tids.at(-1)).toEqual([7, 1])
    expect(m.pages[6]?.rows).toHaveLength(4)
  })
  it('leafInsert нового ключа в LeafEntry[] сохраняет порядок', () => {
    const entries = leaf(buildModel(), 'b3').entries
    leafInsert(entries, 45, [6, 1])
    expect(entries.map((e) => e.key)).toEqual([42, 45, 47, 50, 56])
  })
})

describe('cloneModel', () => {
  it('копия независима от оригинала', () => {
    const m = buildModel()
    const c = cloneModel(m)
    expect(c).toEqual(m)
    heapInsert(c, 'c13', 45)
    leaf(c, 'b1').entries.length = 0
    leaf(c, 'b3').entries[0]?.tids.push([9, 9])
    c.nextBlock++
    expect(m.pages).toHaveLength(6)
    expect(leaf(m, 'b1').entries).toHaveLength(4)
    expect(leaf(m, 'b3').entries[0]?.tids).toHaveLength(3)
    expect(m.nextBlock).toBe(9)
  })
})
