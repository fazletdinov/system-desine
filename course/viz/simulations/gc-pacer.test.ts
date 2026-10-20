import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import {
  BURST_FACTOR,
  BURST_S,
  DEFAULT_PARAMS,
  LIMITER_CAP,
  LIMITS,
  TICKS_PER_SECOND,
  WINDOW_S,
  gcPacerSimulation as sim,
  heapGoal,
  tickerPlan,
  TICK_MS,
  type GcPacerAction,
  type GcPacerParams,
  type GcPacerState,
  type GcTick,
} from './gc-pacer'
import { GOGC_FACTOR, MEM_LIMIT_MIB } from './gomemlimit'

const base: GcPacerParams = { gogc: 100, limitMiB: null, liveMiB: 100, allocMiBps: 100, limiter: true }

const start = (patch: Partial<GcPacerParams> = {}): GcPacerState => sim.apply(sim.initial(), { type: 'set', patch: { ...base, ...patch } })
const run = (s: GcPacerState, seconds: number): GcPacerState => {
  let cur = s
  for (let i = 0; i < seconds * TICKS_PER_SECOND; i++) cur = sim.apply(cur, { type: 'tick' })
  return cur
}
const mean = (ticks: readonly GcTick[], f: (t: GcTick) => number): number => ticks.reduce((sum, t) => sum + f(t), 0) / ticks.length

describe('gc-pacer: модель', () => {
  it('goal is 2x live at GOGC=100 without limit', () => {
    expect(heapGoal(base, 100)).toBe(100 * GOGC_FACTOR)
    const s = run(start(), 1)
    expect(s.history.at(-1)!.goalMiB).toBe(200)
  })

  it('GOGC масштабирует цель: GOGC=50 — 1,5× живой, GOGC=off без лимита — цели нет', () => {
    expect(heapGoal({ ...base, gogc: 50 }, 100)).toBe(150)
    expect(heapGoal({ ...base, gogc: null }, 100)).toBe(Infinity)
  })

  it('limit below goal raises GC frequency', () => {
    const free = run(start(), 10)
    const limited = run(start({ limitMiB: 150 }), 10)
    expect(limited.cycles).toBeGreaterThan(free.cycles)
    expect(free.cycles).toBeGreaterThan(0)
    expect(limited.ooms).toBe(0)
  })

  it('limiter caps GC CPU near 50%: живая куча выше лимита', () => {
    const s = run(start({ liveMiB: 150, limitMiB: 100, limiter: true }), 10)
    expect(s.history.every((t) => t.gcCpu <= 0.55)).toBe(true)
    expect(mean(s.history, (t) => t.gcCpu)).toBeGreaterThanOrEqual(0.4)
    expect(LIMITER_CAP).toBe(0.5)
  })

  it('без ограничителя (как до 1.19) доля CPU GC растёт к 1,0 — спираль смерти', () => {
    const s = run(start({ liveMiB: 150, limitMiB: 100, limiter: false }), 10)
    expect(mean(s.history.slice(-TICKS_PER_SECOND), (t) => t.gcCpu)).toBeGreaterThanOrEqual(0.95)
  })

  it('ограничитель работает только при заданном GOMEMLIMIT: без лимита переключатель ни на что не влияет', () => {
    const a = run(start({ limiter: true }), 5)
    const b = run(start({ limiter: false }), 5)
    expect(a.history).toEqual(b.history)
  })

  it('GOGC=off without limit never collects: куча растёт до OOM контейнера', () => {
    const s = run(start({ gogc: null, liveMiB: 50 }), 10)
    expect(s.cycles).toBe(0)
    expect(s.ooms).toBeGreaterThanOrEqual(1)
    const killed = s.history.filter((t) => t.killed)
    expect(killed.length).toBeGreaterThan(0)
    expect(killed.every((t) => t.rssMiB === MEM_LIMIT_MIB)).toBe(true)
    expect(Math.max(...s.history.map((t) => t.rssMiB))).toBe(MEM_LIMIT_MIB)
  })

  it('GOGC=off with limit collects only near limit', () => {
    const limitMiB = 128
    const s = run(start({ gogc: null, limitMiB, liveMiB: 50 }), 10)
    expect(s.cycles).toBeGreaterThan(0)
    expect(s.ooms).toBe(0)
    const starts = s.history.filter((t) => t.cycleStart)
    expect(starts.length).toBeGreaterThan(0)
    expect(starts.every((t) => t.heapMiB >= 0.75 * limitMiB)).toBe(true)
  })

  it('leak action grows live heap', () => {
    const before = start({ liveMiB: 40 })
    const leaking = run(sim.apply(before, { type: 'leak' }), 5)
    expect(leaking.leaking).toBe(true)
    expect(leaking.history.at(-1)!.liveMiB).toBeGreaterThan(before.params.liveMiB + 20)
    // Повторное нажатие останавливает рост.
    const stopped = sim.apply(leaking, { type: 'leak' })
    const after = run(stopped, 2)
    expect(after.history.at(-1)!.liveMiB).toBe(leaking.history.at(-1)!.liveMiB)
    expect(after.ooms).toBe(0)
  })

  it('burst action raises alloc rate for 3 s', () => {
    const s = run(sim.apply(start(), { type: 'burst' }), 5)
    const rates = s.history.map((t) => t.allocMiBps)
    const raised = rates.filter((r) => r === base.allocMiBps * BURST_FACTOR)
    expect(raised).toHaveLength(BURST_S * TICKS_PER_SECOND)
    expect(rates.slice(0, BURST_S * TICKS_PER_SECOND).every((r) => r === base.allocMiBps * BURST_FACTOR)).toBe(true)
    expect(rates.at(-1)).toBe(base.allocMiBps)
  })

  it('доля CPU GC — фоновые 25 % плюс помощь в разметке, не больше 1', () => {
    const s = run(start({ allocMiBps: 400 }), 10)
    for (const t of s.history) {
      expect(t.gcCpu).toBeGreaterThanOrEqual(0)
      expect(t.gcCpu).toBeLessThanOrEqual(1)
      expect(t.assistCpu).toBeLessThanOrEqual(t.gcCpu)
      if (t.marking) expect(t.gcCpu).toBeGreaterThanOrEqual(0.25)
    }
  })

  it('после OOM процесс перезапускается: утечка начинается заново', () => {
    const s = run(sim.apply(start({ liveMiB: 120 }), { type: 'leak' }), 20)
    expect(s.ooms).toBeGreaterThanOrEqual(1)
    const i = s.history.findIndex((t) => t.killed)
    expect(s.history[i + 1]!.liveMiB).toBeLessThan(s.history[i]!.liveMiB)
  })
})

describe('gc-pacer: песочница', () => {
  it('initial — параметры по умолчанию, история пуста, модель детерминирована', () => {
    const s = sim.initial()
    expect(s.params).toEqual(DEFAULT_PARAMS)
    expect(s.history).toHaveLength(0)
    expect(run(start(), 3)).toEqual(run(start(), 3))
  })

  it('темп прогона: обычно тик за тиком, при reduced motion — секунда модели за одно обновление', () => {
    expect(tickerPlan(false)).toEqual({ intervalMs: TICK_MS, ticksPerUpdate: 1 })
    const reduced = tickerPlan(true)
    expect(reduced.ticksPerUpdate).toBe(TICKS_PER_SECOND)
    // Модель идёт в реальном времени в обоих режимах.
    expect(reduced.intervalMs).toBe(reduced.ticksPerUpdate * TICK_MS)
  })

  it('история ограничена окном графика', () => {
    const s = run(start(), WINDOW_S + 5)
    expect(s.history).toHaveLength(WINDOW_S * TICKS_PER_SECOND)
    expect(s.history.at(-1)!.t).toBeCloseTo(WINDOW_S + 5)
  })

  it('set ограничивает значения диапазонами, GOGC и лимит — с шагом', () => {
    const s = sim.apply(sim.initial(), { type: 'set', patch: { gogc: 9999, limitMiB: 10, liveMiB: 0, allocMiBps: 1e6 } })
    expect(s.params.gogc).toBe(LIMITS.gogc.max)
    expect(s.params.limitMiB).toBe(LIMITS.limitMiB.min)
    expect(s.params.liveMiB).toBe(LIMITS.liveMiB.min)
    expect(s.params.allocMiBps).toBe(LIMITS.allocMiBps.max)
    expect(sim.apply(s, { type: 'set', patch: { gogc: 130 } }).params.gogc).toBe(125)
  })

  it('reset возвращает начальное состояние', () => {
    const actions: GcPacerAction[] = [{ type: 'leak' }, { type: 'burst' }, { type: 'tick' }]
    const s = actions.reduce(sim.apply, start({ limitMiB: 100 }))
    expect(sim.apply(s, { type: 'reset' })).toEqual(sim.initial())
  })

  it('steps — один кадр с подписью; термины из глоссария', () => {
    const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
    const states = [
      sim.initial(),
      run(start(), 3),
      run(start({ limitMiB: 150 }), 3),
      run(start({ liveMiB: 150, limitMiB: 100 }), 3),
      run(start({ liveMiB: 150, limitMiB: 100, limiter: false }), 3),
      run(start({ gogc: null, liveMiB: 50 }), 3),
      run(sim.apply(start(), { type: 'leak' }), 1),
    ]
    const ids = new Set<string>()
    for (const s of states) {
      const steps = sim.steps(s)
      expect(steps).toHaveLength(1)
      expect(captionText(steps[0]!.caption).trim().length).toBeGreaterThan(20)
      for (const id of captionTermIds(steps[0]!.caption)) ids.add(id)
    }
    for (const id of ['gc-pacer', 'gogc', 'gomemlimit', 'death-spiral', 'oom-kill']) expect(ids).toContain(id)
    for (const id of ids) expect(known.has(id)).toBe(true)
  })
})
