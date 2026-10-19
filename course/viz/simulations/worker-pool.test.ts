import { describe, expect, it } from 'vitest'
import { simulateWorkerPool, workerPoolSimulation as sim, SECONDS, type WorkerPoolParams } from './worker-pool'

const base: WorkerPoolParams = { mode: 'pool', lambda: 100, workers: 4, queue: 8, taskMs: 100, burst: false }
// пропускная способность base: 4 × 1000 / 100 = 40 задач/с
const at = (ticks: ReturnType<typeof simulateWorkerPool>, second: number) => ticks[second * 10 - 1]!

describe('simulateWorkerPool', () => {
  it('отдаёт 10 тиков на секунду и детерминирован', () => {
    const a = simulateWorkerPool(base, 20)
    expect(a).toHaveLength(200)
    expect(a.at(-1)!.t).toBe(20)
    expect(simulateWorkerPool(base, 20)).toEqual(a)
  })

  it('unbounded: при перегрузке горутины растут без предела (к концу ≥ 3× от 2-й секунды)', () => {
    const t = simulateWorkerPool({ ...base, mode: 'unbounded' }, 20)
    expect(t.at(-1)!.goroutines).toBeGreaterThanOrEqual(3 * at(t, 2).goroutines)
    expect(t.at(-1)!.memKiB).toBe(t.at(-1)!.goroutines * 8)
  })

  it('pool: горутин не больше workers + 1, p99 растёт, отказов нет', () => {
    const t = simulateWorkerPool({ ...base, mode: 'pool' }, 20)
    expect(t.every((x) => x.goroutines <= base.workers + 1)).toBe(true)
    expect(t.at(-1)!.p99ms).toBeGreaterThan(at(t, 5).p99ms)
    expect(t.at(-1)!.rejected).toBe(0)
  })

  it('pool: буфер канала не больше queue, остальное ждёт у производителя; ползунок меняет картину', () => {
    const q8 = simulateWorkerPool({ ...base, mode: 'pool', queue: 8 }, 20)
    const q32 = simulateWorkerPool({ ...base, mode: 'pool', queue: 32 }, 20)
    expect(q8.every((x) => x.buffered <= 8)).toBe(true)
    expect(q32.every((x) => x.buffered <= 32)).toBe(true)
    expect(q8.every((x) => x.queued === x.buffered + x.blocked)).toBe(true)
    expect(at(q8, 20).buffered).toBe(8)
    expect(at(q32, 20).buffered).toBe(32)
    expect(at(q8, 20).blocked).toBeGreaterThan(at(q32, 20).blocked)
  })

  it('semaphore: p99 ограничена (queue / workers + 1) × taskMs × 1.1, отказы есть', () => {
    const t = simulateWorkerPool({ ...base, mode: 'semaphore' }, 20)
    const bound = (base.queue / base.workers + 1) * base.taskMs * 1.1
    expect(Math.max(...t.map((x) => x.p99ms))).toBeLessThanOrEqual(bound)
    expect(t.at(-1)!.rejected).toBeGreaterThan(0)
  })

  it('при 50% пропускной способности все режимы дают близкую p99 и не отказывают', () => {
    const p99 = (['unbounded', 'pool', 'semaphore'] as const).map((mode) => {
      const t = simulateWorkerPool({ ...base, mode, lambda: 20 }, 20)
      expect(t.at(-1)!.rejected).toBe(0)
      return t.at(-1)!.p99ms
    })
    const [min, max] = [Math.min(...p99), Math.max(...p99)]
    expect(min).toBeGreaterThan(0)
    expect(max / min).toBeLessThanOrEqual(1.2)
  })

  it('burst: отказы semaphore появляются только в окне всплеска ±1 с', () => {
    const t = simulateWorkerPool({ ...base, mode: 'semaphore', lambda: 24, burst: true }, 20)
    const rejectedAt = t.filter((x, i) => x.rejected > (t[i - 1]?.rejected ?? 0)).map((x) => x.t)
    expect(rejectedAt.length).toBeGreaterThan(0)
    expect(rejectedAt.every((s) => s >= 4 && s <= 9)).toBe(true)
  })

  it('semaphore с очередью 0 отказывает сразу, когда все воркеры заняты', () => {
    const t = simulateWorkerPool({ ...base, mode: 'semaphore', queue: 0 }, 5)
    expect(t.at(-1)!.rejected).toBeGreaterThan(0)
    expect(t.every((x) => x.queued === 0)).toBe(true)
  })
})

describe('workerPoolSimulation', () => {
  it('initial считает 20 секунд, кадры — по одному на секунду', () => {
    const s = sim.initial()
    expect(s.ticks).toHaveLength(SECONDS * 10)
    const steps = sim.steps(s)
    expect(steps).toHaveLength(SECONDS)
    expect(steps[4]!.state.ticks.at(-1)!.t).toBe(5)
  })

  it('set меняет параметры и пересчитывает, значения ограничены диапазоном', () => {
    const s = sim.apply(sim.initial(), { type: 'set', patch: { workers: 99, lambda: 1 } })
    expect(s.params.workers).toBe(16)
    expect(s.params.lambda).toBe(10)
  })

  it('run пересчитывает без смены параметров, reset возвращает начальные', () => {
    const s = sim.apply(sim.initial(), { type: 'set', patch: { mode: 'unbounded' } })
    expect(sim.apply(s, { type: 'run' })).toEqual(s)
    expect(sim.apply(s, { type: 'reset' })).toEqual(sim.initial())
  })
})
