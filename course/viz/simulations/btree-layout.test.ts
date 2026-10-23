import { describe, expect, it } from 'vitest'
import { buildModel, leafFor, leafOrder } from './btree-model'
import { btreeScenario } from './btree'
import { CELL_H, ENTRY_TOP, layoutScene, lineOffsets, selectionLinks, tidPath } from './btree-layout'

describe('btree-layout', () => {
  const m = buildModel()

  it('даёт рамку каждому узлу, странице и метастранице', () => {
    const { boxes } = layoutScene(m, 'detail')
    for (const id of Object.keys(m.nodes)) expect(boxes[id]).toBeDefined()
    for (const p of m.pages) expect(boxes[`page:${p.no}`]).toBeDefined()
    expect(boxes.meta).toBeDefined()
  })

  it('листья идут слева направо в порядке цепочки соседей', () => {
    const { boxes } = layoutScene(m, 'simple')
    const xs = leafOrder(m).map((id) => boxes[id]?.x ?? -1)
    expect([...xs].sort((a, b) => a - b)).toEqual(xs)
  })

  it('клик по записи подсвечивает все её строки, клик по строке — запись', () => {
    const byEntry = selectionLinks(m, 'entry:42')
    expect(byEntry.rows.size).toBe(3)
    const [p, lp] = [...byEntry.rows][0]!.split(':')
    expect([...selectionLinks(m, `row:${p}:${lp}`).entries]).toEqual([42])
  })

  it('TID-путь строится только для существующего ключа', () => {
    const { boxes } = layoutScene(m, 'detail')
    expect(tidPath(m, boxes, 'detail', { key: 42, tid: [0, 1] })).toMatch(/^M/)
    expect(tidPath(m, boxes, 'detail', { key: 1000, tid: [0, 1] })).toBeNull()
  })

  it('в detail TID начинается у нижней кромки листа, в simple — под строкой записей', () => {
    const leaf = leafFor(m, 42)
    const start = (view: 'simple' | 'detail'): number => {
      const { boxes } = layoutScene(m, view)
      const d = tidPath(m, boxes, view, { key: 42, tid: [0, 1] }) ?? ''
      return Number(/^M[\d.-]+,([\d.-]+)/.exec(d)?.[1])
    }
    const detailBox = layoutScene(m, 'detail').boxes[leaf.id]!
    const simpleBox = layoutScene(m, 'simple').boxes[leaf.id]!
    expect(start('detail')).toBe(detailBox.y + detailBox.h)
    expect(start('simple')).toBe(simpleBox.y + ENTRY_TOP + CELL_H)
    expect(start('detail')).not.toBe(start('simple'))
  })

  it('posting list переносится по два TID: запись с тремя TID занимает две строки', () => {
    expect(lineOffsets([1, 3, 1, 2])).toEqual([0, 1, 3, 4])
  })

  it('лист с posting list выше листа того же числа записей без него', () => {
    const { boxes } = layoutScene(m, 'detail')
    const withDup = leafFor(m, 42)
    const plain = leafOrder(m).map((id) => m.nodes[id]!).find((n) => n.id !== withDup.id && n.entries.length === withDup.entries.length)!
    expect(boxes[withDup.id]!.h).toBeGreaterThan(boxes[plain.id]!.h)
  })

  // Самые высокие узлы — во вставке: лист из 5 записей до сплита и внутренняя страница с 4 разделителями после.
  it('ряды не налезают друг на друга ни в одном виде и кадре вставки', () => {
    for (const model of [m, ...btreeScenario('ins').map((s) => s.state.model)])
      for (const view of ['simple', 'detail'] as const) {
        const { boxes, rows } = layoutScene(model, view)
        const bottomOf = (pick: (n: (typeof model.nodes)[string]) => boolean): number =>
          Math.max(...Object.values(model.nodes).filter(pick).map((n) => boxes[n.id]!.y + boxes[n.id]!.h))
        // 24 — место под подпись следующего ряда.
        expect(bottomOf((n) => n.id === model.root) + 24).toBeLessThanOrEqual(rows.inner)
        expect(bottomOf((n) => n.level === 1) + 24).toBeLessThanOrEqual(rows.leaf)
        expect(bottomOf((n) => n.level === 0) + 24).toBeLessThanOrEqual(rows.heap)
      }
  })

  it('без выбора и для неизвестных id связей нет', () => {
    for (const sel of [null, 'entry:1000', 'row:99:9', 'node:b1', 'page:0']) {
      const s = selectionLinks(m, sel)
      expect([s.entries.size, s.rows.size, s.tids.length]).toEqual([0, 0, 0])
    }
  })
})
