import { describe, expect, it } from 'vitest'
import { buildModel, leafOrder } from './btree-model'
import { layoutScene, selectionLinks, tidPath } from './btree-layout'

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
})
