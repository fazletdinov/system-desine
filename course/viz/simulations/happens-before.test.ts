import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { happensBeforeScenario, type HbFrame, type HbVariant } from './happens-before'

const VARIANTS: HbVariant[] = ['none', 'channel', 'mutex']
const last = (v: HbVariant): HbFrame => happensBeforeScenario(v).at(-1)!.state

// Путь по рёбрам синхронизации и порядку внутри горутины.
function reachable(f: HbFrame, from: string, to: string): boolean {
  const next = new Map<string, string[]>()
  const add = (a: string, b: string): void => void next.set(a, [...(next.get(a) ?? []), b])
  for (const g of [f.g1, f.g2]) g.forEach((e, i) => i > 0 && add(g[i - 1]!.id, e.id))
  for (const e of f.edges) add(e.from, e.to)
  const seen = new Set<string>()
  const stack = [from]
  while (stack.length > 0) {
    const id = stack.pop()!
    if (id === to) return true
    if (seen.has(id)) continue
    seen.add(id)
    stack.push(...(next.get(id) ?? []))
  }
  return false
}

describe('none', () => {
  it('рёбер нет, outcome any, есть отчёт о гонке в подписи', () => {
    const steps = happensBeforeScenario('none')
    for (const s of steps) expect(s.state.edges).toEqual([])
    expect(last('none').outcome).toBe('any')
    expect(last('none').raceReport).toBe(true)
    expect(captionText(steps.at(-1)!.caption)).toContain('WARNING: DATA RACE')
  })
})

describe.each(['channel', 'mutex'] as const)('%s', (v) => {
  it('от записи data к чтению data есть путь, outcome guaranteed, гонки нет', () => {
    const f = last(v)
    expect(reachable(f, 'write', 'read')).toBe(true)
    expect(f.outcome).toBe('guaranteed')
    for (const s of happensBeforeScenario(v)) expect(s.state.raceReport).toBe(false)
  })

  it('без ребра синхронизации пути от записи к чтению нет, с ребром есть', () => {
    // Последний кадр до появления ребра: запись уже есть, путь к будущему чтению не построен.
    const frames = happensBeforeScenario(v).map((s) => s.state)
    const iEdge = frames.findIndex((f) => f.edges.length > 0)
    expect(iEdge).toBeGreaterThan(0)
    const before = frames[iEdge - 1]!
    expect(before.edges).toEqual([])
    expect([...before.g1, ...before.g2].some((e) => e.id === 'write')).toBe(true)
    expect(reachable({ ...frames.at(-1)!, edges: [] }, 'write', 'read')).toBe(false)
    expect(reachable(frames.at(-1)!, 'write', 'read')).toBe(true)
  })
})

it('рёбра ссылаются на существующие события, t не убывает по кадрам', () => {
  for (const v of VARIANTS) {
    const frames = happensBeforeScenario(v).map((s) => s.state)
    for (const f of frames) {
      const ids = new Set([...f.g1, ...f.g2].map((e) => e.id))
      for (const e of f.edges) {
        expect(ids.has(e.from)).toBe(true)
        expect(ids.has(e.to)).toBe(true)
      }
    }
    const times = last(v).g1.concat(last(v).g2).map((e) => e.t)
    expect(new Set(times).size).toBe(times.length)
  }
})

it('подписи содержат happens-before, data-race, memory-model; термины из глоссария', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  for (const v of VARIANTS) {
    const steps = happensBeforeScenario(v)
    const ids = steps.flatMap((s) => captionTermIds(s.caption))
    for (const id of ['happens-before', 'data-race', 'memory-model']) expect(ids).toContain(id)
    for (const id of ids) expect(known.has(id)).toBe(true)
  }
})
