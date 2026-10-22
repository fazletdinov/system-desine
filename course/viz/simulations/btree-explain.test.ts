import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionText } from '../core/caption'
import { explanationTermIds, type Explanation } from '../core/explain'
import { btreeScenario, type BtScenarioId } from './btree'
import { BT_SIMPLIFICATIONS, explainBtree } from './btree-explain'

const IDS: BtScenarioId[] = ['intro', 'one', 'dup', 'range', 'ios', 'seq', 'ins']

const allText = (e: Explanation): string =>
  [e.title, e.kind, ...e.sections.map((s) => captionText(s.body)), ...(e.here ? [captionText(e.here)] : []), ...(e.real ? [captionText(e.real)] : [])].join('\n')
const section = (e: Explanation, heading: string): string => {
  const s = e.sections.find((x) => x.heading === heading)
  if (!s) throw new Error(`нет раздела ${heading}`)
  return captionText(s.body)
}

describe('объяснения элементов сцены B-tree', () => {
  it('корень на шаге, где в фокусе корень: заголовок и пример на числах', () => {
    const steps = btreeScenario('one')
    const step = steps.find((s) => s.state.focus === 'node:b8')!
    expect(steps.indexOf(step)).toBe(2)
    const e = explainBtree('node:b8', step)!
    expect(e.title).toBe('Корень')
    expect(section(e, 'Пример на числах')).toMatch(/блок 6/)
    expect(section(e, 'Пример на числах')).toMatch(/блок 7/)
    expect(e.here).toBeDefined()
    expect(e.real).toBeDefined()
  })

  it('запись 42 называет все три адреса', () => {
    const e = explainBtree('entry:42', btreeScenario('dup')[0]!)!
    const text = allText(e)
    for (const tid of ['(0,1)', '(1,3)', '(5,3)']) expect(text).toContain(tid)
  })

  it('строка 0:2 — (c1, 15)', () => {
    const e = explainBtree('row:0:2', btreeScenario('one')[0]!)!
    expect(e.title).toContain('(c1, 15)')
  })

  it('узла b9 до сплита нет, после — лист с блоком 9', () => {
    const steps = btreeScenario('ins')
    expect(explainBtree('node:b9', steps[0]!)).toBeNull()
    const e = explainBtree('node:b9', steps.at(-1)!)!
    expect(e.title).toBe('Листовая страница')
    expect(allText(e)).toMatch(/блок 9/)
  })

  it('неизвестный id и несуществующие элементы дают null', () => {
    const step = btreeScenario('one')[0]!
    for (const id of ['foo', 'node:zz', 'entry:999', 'row:9:9', 'page:99', 'vm:99', 'entry:x']) expect(explainBtree(id, step)).toBeNull()
  })

  it('служебные элементы объяснены', () => {
    const step = btreeScenario('one')[0]!
    for (const id of ['meta', 'sib', 'tid', 'table', 'plan', 'page:0', 'vm:5']) expect(explainBtree(id, step)).not.toBeNull()
  })

  it('страница 5 без отметки all-visible, остальные — с ней', () => {
    const step = btreeScenario('ios')[0]!
    expect(section(explainBtree('page:5', step)!, 'Карта видимости')).toMatch(/отметки нет/)
    expect(section(explainBtree('page:0', step)!, 'Карта видимости')).toMatch(/all-visible/)
  })

  it('«Что в модели упрощено» содержит данные, размеры, план', () => {
    expect(BT_SIMPLIFICATIONS.sections.map((s) => s.heading)).toEqual(
      expect.arrayContaining(['Данные придуманы', 'Размеры', 'Логика ≠ файл', 'Опущено', 'Видимость', 'План']),
    )
  })

  it('все термины по всем элементам всех кадров есть в глоссарии, разметка не просачивается', () => {
    const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
    const check = (e: Explanation | null) => {
      if (!e) return
      for (const t of explanationTermIds(e)) expect(known.has(t)).toBe(true)
      expect(allText(e)).not.toMatch(/\[\[|\]\]|<\/?(code|br|dl|dd|dt)/)
    }
    check(BT_SIMPLIFICATIONS)
    for (const id of IDS)
      for (const step of btreeScenario(id)) {
        const m = step.state.model
        const ids = [
          'meta', 'sib', 'tid', 'table', 'plan',
          ...Object.keys(m.nodes).map((n) => `node:${n}`),
          ...Object.values(m.nodes).flatMap((n) => (n.level === 0 ? n.entries.map((e) => `entry:${(e as { key: number }).key}`) : [])),
          ...m.pages.flatMap((p) => [`page:${p.no}`, `vm:${p.no}`, ...p.rows.map((r) => `row:${p.no}:${r.lp}`)]),
        ]
        for (const eid of ids) {
          const e = explainBtree(eid, step)
          expect(e, `${id}/${eid}`).not.toBeNull()
          check(e)
        }
      }
  })
})
