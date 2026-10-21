import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import {
  HOT_WINDOW,
  KEY_KINDS,
  LEAF_FILLFACTOR,
  MAX_KEYS,
  PAGE_CAPACITY,
  keyLocalitySimulation as sim,
  treeStats,
  type KeyLocalityAction,
  type KeyLocalityState,
} from './key-locality'

const run = (...actions: KeyLocalityAction[]): KeyLocalityState => actions.reduce(sim.apply, sim.initial())
const N = 1000
const loaded = run({ type: 'insert', count: N })
const stats = (s: KeyLocalityState) => ({ bigint: treeStats(s.trees.bigint), uuidv4: treeStats(s.trees.uuidv4), uuidv7: treeStats(s.trees.uuidv7) })

describe('key-locality: монотонные ключи', () => {
  it('bigint: заполненность ≈ fillfactor, все разделения — у правого края', () => {
    const st = stats(loaded).bigint
    expect(st.keys).toBe(N)
    expect(st.splitsMiddle).toBe(0)
    expect(st.splitsRight).toBe(st.pages - 1)
    expect(st.fill).toBeGreaterThan(LEAF_FILLFACTOR - 0.03)
    expect(st.fill).toBeLessThanOrEqual(LEAF_FILLFACTOR + 0.01)
    // Все листья, кроме правого, заполнены ровно до fillfactor.
    const pages = loaded.trees.bigint.pages
    for (const p of pages.slice(0, -1)) expect(p.keys.length).toBe(PAGE_CAPACITY * LEAF_FILLFACTOR)
  })

  it('UUIDv7: шум внутри миллисекунды не ломает локальность — заполненность около fillfactor', () => {
    const st = stats(loaded).uuidv7
    expect(st.fill).toBeGreaterThan(0.85)
    expect(st.splitsMiddle).toBeLessThanOrEqual(st.splitsRight / 10)
  })

  it('монотонные ключи держат горячими одну-две страницы', () => {
    const st = stats(loaded)
    expect(st.bigint.hotPages).toBeLessThanOrEqual(2)
    expect(st.uuidv7.hotPages).toBeLessThanOrEqual(3)
  })
})

describe('key-locality: случайные ключи', () => {
  it('UUIDv4: заполненность 0,5–0,75, разделения по всему дереву', () => {
    const st = stats(loaded).uuidv4
    expect(st.fill).toBeGreaterThan(0.5)
    expect(st.fill).toBeLessThan(0.75)
    expect(st.splitsMiddle).toBeGreaterThan(st.splitsRight)
    expect(st.hotPages).toBeGreaterThan(HOT_WINDOW / 2)
  })

  it('при одном N у UUIDv4 страниц больше, чем у UUIDv7', () => {
    const st = stats(loaded)
    expect(st.uuidv4.pages).toBeGreaterThan(st.uuidv7.pages)
    expect(st.uuidv7.keys).toBe(st.uuidv4.keys)
  })

  it('ключи в листьях упорядочены, и каждый лежит не ниже границы своей страницы', () => {
    for (const kind of KEY_KINDS) {
      const pages = loaded.trees[kind].pages
      const all = pages.flatMap((p) => p.keys)
      expect(all).toEqual([...all].sort((a, b) => a - b))
      for (const p of pages) for (const k of p.keys) expect(k).toBeGreaterThanOrEqual(p.low)
      expect(all).toHaveLength(N)
    }
  })
})

describe('key-locality: CHECKPOINT и полные образы страниц', () => {
  it('после CHECKPOINT UUIDv4 пишет полный образ почти каждой страницы, bigint и UUIDv7 — единицы', () => {
    const s = run({ type: 'insert', count: N }, { type: 'checkpoint' }, { type: 'insert', count: 100 })
    const st = stats(s)
    expect(st.bigint.fpi).toBe(1)
    expect(st.uuidv7.fpi).toBeLessThanOrEqual(2)
    expect(st.uuidv4.fpi).toBeGreaterThan(20 * st.uuidv7.fpi)
  })

  it('до первого CHECKPOINT полных образов нет: все страницы созданы после него', () => {
    expect(stats(loaded).uuidv4.fpi).toBe(0)
  })
})

describe('key-locality: управление', () => {
  it('модель не растёт больше MAX_KEYS, выбор вида не меняет деревья', () => {
    const s = run({ type: 'insert', count: MAX_KEYS + 500 }, { type: 'select', kind: 'uuidv4' })
    expect(s.selected).toBe('uuidv4')
    expect(treeStats(s.trees.uuidv4).keys).toBe(MAX_KEYS)
    expect(sim.steps(s)[0]!.state.full).toBe(true)
  })

  it('модель детерминирована', () => {
    expect(run({ type: 'insert', count: 300 }).trees).toEqual(run({ type: 'insert', count: 300 }).trees)
  })

  it('сброс сохраняет выбранный вид ключа', () => {
    const s = run({ type: 'select', kind: 'uuidv7' }, { type: 'insert', count: 10 }, { type: 'reset' })
    expect(s.selected).toBe('uuidv7')
    expect(treeStats(s.trees.uuidv7).keys).toBe(0)
  })
})

describe('key-locality: подписи', () => {
  it('подпись цитирует числа лабы, термины есть в глоссарии', () => {
    const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
    const ids = new Set<string>()
    for (const kind of KEY_KINDS) {
      for (const s of [run({ type: 'select', kind }), run({ type: 'select', kind }, { type: 'insert', count: 200 }), run({ type: 'select', kind }, { type: 'checkpoint' })]) {
        const caption = sim.steps(s)[0]!.caption
        for (const id of captionTermIds(caption)) ids.add(id)
      }
    }
    for (const id of ids) expect(known.has(id), id).toBe(true)
    const text = (kind: (typeof KEY_KINDS)[number]) => captionText(sim.steps(run({ type: 'select', kind }, { type: 'insert', count: 20 }))[0]!.caption)
    expect(text('bigint')).toContain('21 960 КиБ')
    expect(text('uuidv7')).toContain('30 824 КиБ')
    expect(text('uuidv4')).toContain('38 288–38 856 КиБ')
    expect(text('uuidv4')).toContain('labs/2a/results/')
  })
})
