import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import rawRecords from '../data/escape-lab.json'
import {
  CASES,
  decisiveLines,
  escapeLabSimulation as sim,
  recordFor,
  verdictOf,
  type CompilerRecord,
  type EscapeLabAction,
} from './escape-lab'

const records = rawRecords as CompilerRecord[]
const casesDir = fileURLToPath(new URL('../../../labs/1-5/alloc/escape/cases/', import.meta.url))
const keyOf = (c: string, v: string): string => `${c}/${v}`
const catalogKeys = CASES.flatMap((c) => c.variants.map((v) => keyOf(c.id, v.id)))
const apply = (actions: EscapeLabAction[]) => actions.reduce(sim.apply, sim.initial())

describe('escape-lab: каталог и JSON компилятора', () => {
  it('ключи каталога совпадают с ключами JSON, без лишних и без потерянных', () => {
    const jsonKeys = records.map((r) => keyOf(r.case, r.variant))
    expect([...catalogKeys].sort()).toEqual([...jsonKeys].sort())
    expect(new Set(catalogKeys).size).toBe(catalogKeys.length)
  })

  it.each(CASES.flatMap((c) => c.variants.map((v) => [keyOf(c.id, v.id), c.id, v.id] as const)))(
    '%s: комбинация переключателя находит запись в JSON',
    (_key, caseId, variantId) => {
      const s = apply([{ type: 'select-case', caseId }, { type: 'select-variant', variantId }])
      expect(sim.steps(s)[0]!.state.record).toBe(recordFor(caseId, variantId))
    },
  )

  it.each(CASES.map((c) => [c.id, c] as const))('%s: есть вариант с кучей и вариант без неё', (_id, c) => {
    const where = c.variants.map((v) => recordFor(c.id, v.id).where)
    expect(where).toContain('heap')
    expect(where).toContain('stack')
  })

  it('у варианта heap в строке компилятора «escapes to heap» или «moved to heap»', () => {
    for (const r of records.filter((x) => x.where === 'heap')) {
      expect(r.compiler, keyOf(r.case, r.variant)).toMatch(/escapes to heap|moved to heap/)
    }
  })

  it('решающая строка: heap — escapes/moved, stack — does not escape', () => {
    for (const r of records) {
      const lines = decisiveLines(r)
      expect(lines.length, keyOf(r.case, r.variant)).toBeGreaterThan(0)
      const re = r.where === 'heap' ? /escapes to heap|moved to heap/ : /does not escape/
      for (const l of lines) expect(l).toMatch(re)
    }
  })

  it('исходник варианта дословно присутствует в cases/*.go', () => {
    for (const c of CASES) {
      for (const v of c.variants) {
        const file = readFileSync(casesDir + recordFor(c.id, v.id).file, 'utf8')
        expect(v.source.length).toBeGreaterThan(0)
        for (const fragment of v.source) expect(file, keyOf(c.id, v.id)).toContain(fragment)
      }
    }
  })

  it('функция варианта — именно та, чей ключ escape: стоит над ней в .go', () => {
    for (const c of CASES) {
      for (const v of c.variants) {
        const file = readFileSync(casesDir + recordFor(c.id, v.id).file, 'utf8')
        const head = v.source[v.source.length - 1]!.split('\n')[0]!
        const at = file.indexOf(head)
        expect(file.lastIndexOf(`// escape:${keyOf(c.id, v.id)}`, at), keyOf(c.id, v.id)).toBeGreaterThan(-1)
      }
    }
  })
})

describe('escape-lab: вердикт и оговорки', () => {
  it('slice/var и elems/values — третье состояние, не «просто стек»', () => {
    expect(verdictOf('slice', 'var')).toBe('stack-limited')
    expect(verdictOf('elems', 'values')).toBe('stack-limited')
    expect(verdictOf('slice', 'const')).toBe('stack')
    expect(verdictOf('slice', 'returned')).toBe('heap')
  })

  it('оговорка только у вариантов, где компилятор сказал «стек»', () => {
    for (const c of CASES) {
      for (const v of c.variants) {
        if (v.caveat !== undefined) expect(recordFor(c.id, v.id).where).toBe('stack')
      }
    }
    expect(CASES.flatMap((c) => c.variants).filter((v) => v.caveat !== undefined)).toHaveLength(2)
  })

  it('slice/var: оговорка про 32 байта, elems/values — про n ≤ 8', () => {
    const frame = (c: string, v: string) => sim.steps(apply([{ type: 'select-case', caseId: c }, { type: 'select-variant', variantId: v }]))[0]!.state
    expect(frame('slice', 'var').caveat!.note).toContain('32')
    expect(frame('slice', 'var').caveat!.note).toContain('1.25')
    expect(frame('elems', 'values').caveat!.note).toContain('n ≤ 8')
  })

  it('large: пояснения называют оба предела (R5)', () => {
    const text = CASES.find((c) => c.id === 'large')!.variants.map((v) => captionText(v.rule)).join(' ')
    expect(text).toContain('MaxImplicitStackVarSize')
    expect(text).toContain('64 КиБ')
    expect(text).toContain('MaxStackVarSize')
    expect(text).toContain('128 КиБ')
    expect(text).toMatch(/make.*new.*&T\{\}/)
  })
})

describe('escape-lab: песочница', () => {
  it('начальное состояние — первый вариант первого случая', () => {
    const s = sim.initial()
    expect(s.caseId).toBe(CASES[0]!.id)
    expect(s.variantId).toBe(CASES[0]!.variants[0]!.id)
  })

  it('смена случая сбрасывает вариант на первый; чужой вариант и случай игнорируются', () => {
    const s = apply([{ type: 'select-case', caseId: 'iface' }, { type: 'select-variant', variantId: 'any' }])
    expect(s.variantId).toBe('any')
    expect(apply([{ type: 'select-case', caseId: 'slice' }]).variantId).toBe('const')
    expect(sim.apply(s, { type: 'select-variant', variantId: 'нет такого' })).toBe(s)
    expect(sim.apply(s, { type: 'select-case', caseId: 'нет такого' })).toBe(s)
    expect(sim.apply(s, { type: 'reset' })).toEqual(sim.initial())
  })

  it('кадр: исходник, строка компилятора и вердикт берутся из каталога и JSON', () => {
    const f = sim.steps(apply([{ type: 'select-case', caseId: 'return' }, { type: 'select-variant', variantId: 'pointer' }]))[0]!.state
    expect(f.verdict).toBe('heap')
    expect(f.compiler).toContain('new(point) escapes to heap')
    expect(f.source).toContain('return p')
  })

  it('подпись каждого варианта непустая, термины — из глоссария', () => {
    const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
    for (const c of CASES) {
      for (const v of c.variants) {
        const step = sim.steps({ caseId: c.id, variantId: v.id })[0]!
        expect(captionText(step.caption).length).toBeGreaterThan(20)
        for (const id of captionTermIds(step.caption)) expect(known.has(id), `${c.id}/${v.id}: ${id}`).toBe(true)
      }
    }
  })
})
