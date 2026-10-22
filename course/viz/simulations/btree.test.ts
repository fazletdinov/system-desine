import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionSegments, captionText, type Caption } from '../core/caption'
import { stepTermIds } from '../core/explain'
import type { InnerEntry, LeafEntry } from './btree-model'
import { BT_SCENARIOS, btreeScenario, type BtFrame, type BtScenarioId } from './btree'

const IDS: BtScenarioId[] = ['intro', 'one', 'dup', 'range', 'ios', 'seq', 'ins']
const frames = (id: BtScenarioId): BtFrame[] => btreeScenario(id).map((s) => s.state)
const last = (id: BtScenarioId): BtFrame => frames(id).at(-1)!

it('вкладки идут в порядке прототипа', () => {
  expect(BT_SCENARIOS.map((s) => s.id)).toEqual(IDS)
  expect(BT_SCENARIOS.map((s) => s.view)).toEqual(['simple', 'detail', 'detail', 'detail', 'detail', 'simple', 'detail'])
})

describe('числа в последнем шаге сценария', () => {
  it('one: 7 шагов, 3 страницы индекса, 1 обращение к таблице, результат c1', () => {
    expect(btreeScenario('one')).toHaveLength(7)
    const c = last('one').counters
    expect([c.index, c.heapAccess]).toEqual([3, 1])
    expect(last('one').result).toEqual(['c1'])
  })
  it('dup: 3 страницы индекса, 3 обращения к таблице', () => {
    const c = last('dup').counters
    expect([c.index, c.heapAccess]).toEqual([3, 3])
    expect(last('dup').result).toEqual(['c1', 'c4', 'c12'])
  })
  it('range: 10 строк, 5 страниц индекса, 9 обращений к 6 страницам таблицы', () => {
    const f = last('range')
    expect(f.result).toHaveLength(10)
    expect([f.counters.index, f.counters.heapAccess, f.counters.heapPages.length]).toEqual([5, 9, 6])
  })
  it('ios: 3 страницы индекса, один Heap Fetch, count = 3', () => {
    const f = last('ios')
    expect([f.counters.index, f.counters.heapFetches, f.result.length]).toEqual([3, 1, 3])
  })
  it('seq: 6 страниц таблицы, индекс не читается', () => {
    const f = last('seq')
    expect([f.counters.heapAccess, f.counters.index]).toEqual([6, 0])
    expect(f.result).toEqual(['c1', 'c4', 'c12'])
  })
})

describe('вставка и сплит', () => {
  const steps = frames('ins')
  const leaves = (f: BtFrame) => Object.values(f.model.nodes).filter((n) => n.level === 0)

  it('в конце новый лист b9 с ключами 50, 53, 56 и разделитель 50 в b6', () => {
    const m = steps.at(-1)!.model
    expect((m.nodes.b9!.entries as LeafEntry[]).map((e) => e.key)).toEqual([50, 53, 56])
    expect(m.nodes.b6!.entries).toContainEqual({ sep: 50, child: 'b9' })
  })
  it('кадр сплита: лист уже разделён, а в родителе записи 50 ещё нет', () => {
    const i = btreeScenario('ins').findIndex((s) => /сплит/.test(s.state.title))
    expect(i).toBeGreaterThan(0)
    const m = steps[i]!.model
    expect(m.nodes.b9).toBeDefined()
    expect((m.nodes.b6!.entries as InnerEntry[]).some((e) => e.sep === 50)).toBe(false)
    expect((steps[i + 1]!.model.nodes.b6!.entries as InnerEntry[]).filter((e) => e.sep === 50 && e.child === 'b9')).toHaveLength(1)
  })
  it('в кадре шага 1 пять листьев, снимки соседних шагов не делят объекты', () => {
    expect(leaves(steps[0]!)).toHaveLength(5)
    expect(leaves(steps.at(-1)!)).toHaveLength(6)
    steps.forEach((f, i) => i > 0 && expect(f.model).not.toBe(steps[i - 1]!.model))
    expect(steps[0]!.model.pages).toHaveLength(7)
  })
})

describe('каждый шаг', () => {
  it.each(IDS)('%s: есть карточка, фокус и счётчик вопросов', (id) => {
    const steps = btreeScenario(id)
    expect(steps.length).toBeGreaterThan(0)
    for (const s of steps) {
      expect(s.card).toBeDefined()
      expect(s.state.focus.length).toBeGreaterThan(0)
      expect(s.state.title.length).toBeGreaterThan(0)
    }
    const quizzes = { intro: 2, one: 2, dup: 1, range: 1, ios: 1, seq: 1, ins: 1 }
    expect(steps.filter((s) => s.quiz !== undefined)).toHaveLength(quizzes[id])
  })

  it('все термины из подписей, карточек и вопросов есть в глоссарии', () => {
    const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
    for (const id of IDS) for (const s of btreeScenario(id)) for (const t of stepTermIds(s)) expect(known.has(t)).toBe(true)
  })

  it('термин в сценарии размечен один раз — при первом появлении', () => {
    for (const id of IDS) {
      const ids = btreeScenario(id).flatMap(stepTermIds)
      expect(new Set(ids).size).toBe(ids.length)
    }
  })

  it('разметка терминов не просачивается в текст', () => {
    for (const id of IDS)
      for (const s of btreeScenario(id)) {
        const texts = [s.state.title, s.state.learn, ...Object.values(s.state.notes).map(captionText), ...stepCaptions(s).map(captionText)]
        for (const t of texts) expect(t).not.toMatch(/\[\[|\]\]/)
      }
  })

  // Читатель открывает вкладки независимо: первое упоминание каждого термина в сценарии должно быть подсказкой.
  const KEYWORDS: [id: string, re: RegExp][] = [
    ['tid', /\bTID\b/],
    ['heap-table', /\b[Hh]eap(?! (Scan|Fetches))/],
    ['db-page', /(?<!мета)страниц/i],
    ['seq-scan', /Seq Scan/],
    ['bitmap-scan', /Bitmap/],
    ['b-tree', /B-tree|B-дерев/],
    ['query-planner', /планировщик/i],
    ['index-only-scan', /Index Only Scan/],
    ['index', /индексн\p{L}* запис/iu],
    ['metapage', /метастраниц/i],
    ['posting-list', /posting list/i],
    ['visibility-map', /карт\p{L}* видимости|visibility map/iu],
    ['vacuum', /VACUUM/],
    ['mvcc', /MVCC/],
    ['fillfactor', /fillfactor/i],
    ['free-space-map', /FSM|карт\p{L}* свободного места/iu],
    ['b-tree-page-split', /сплит/i],
  ]
  it.each(IDS)('%s: первое упоминание каждого термина — подсказка', (id) => {
    const segs = btreeScenario(id).flatMap((s) => stepCaptions(s).flatMap((c) => [...captionSegments(c)]))
    const misses = KEYWORDS.flatMap(([termId, re]) => {
      const first = segs.find((seg) => re.test(seg.text))
      return first && first.termId !== termId ? [`${termId}: «${first.text.slice(0, 90)}»`] : []
    })
    expect(misses).toEqual([])
  })
})

// Подписи шага в порядке показа, без общей подписи: в ней терминов нет намеренно.
function stepCaptions(s: ReturnType<typeof btreeScenario>[number]): Caption[] {
  const { card, quiz } = s
  return [...(card ? [card.what, card.uses, card.reads, card.why] : []), ...(quiz ? [quiz.question, quiz.answer] : [])]
}
