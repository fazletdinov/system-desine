import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Simulation, Step } from '../core/types'

// Дискретная детерминированная модель по тикам 100 мс. Случайности нет: задачи приходят
// ровными порциями (дробный остаток переносится на следующий тик), поэтому один и тот же
// набор параметров всегда даёт один и тот же график.

export type PoolMode = 'unbounded' | 'pool' | 'semaphore'

export interface WorkerPoolParams {
  mode: PoolMode
  lambda: number // задач/с
  workers: number
  queue: number
  taskMs: number
  burst: boolean // ×5 на 3 с с t=5 с
}

export interface WorkerPoolTick {
  t: number // секунды от старта, конец тика
  goroutines: number
  memKiB: number // оценка: goroutines × GOROUTINE_KIB
  queued: number // buffered + blocked
  buffered: number // pool: в буфере канала (≤ queue); semaphore: ждут места; unbounded: ждут ядро
  blocked: number // pool: заблокированный производитель держит задачи у себя, канал полон
  done: number // накопленно
  rejected: number // накопленно
  p50ms: number
  p99ms: number
}

export const TICK_MS = 100
export const TICKS_PER_SECOND = 1000 / TICK_MS
export const SECONDS = 20
// Стартовый стек горутины — 2 КиБ, но с учётом кучи, стека вызовов и структур задачи берём
// 8 КиБ: это порядок величины для оценки, а не измерение.
export const GOROUTINE_KIB = 8
export const BURST_FROM_S = 5
export const BURST_LENGTH_S = 3
export const BURST_FACTOR = 5
// Окно, по которому считаются перцентили: достаточно длинное, чтобы набрать выборку, и короткое,
// чтобы график отражал текущее состояние, а не историю.
const LATENCY_WINDOW_TICKS = 2 * TICKS_PER_SECOND
const EPS = 1e-9

export const LIMITS = {
  lambda: { min: 10, max: 500 },
  workers: { min: 1, max: 16 },
  queue: { min: 0, max: 64 },
  taskMs: { min: 10, max: 500 },
} as const

export const DEFAULT_PARAMS: WorkerPoolParams = { mode: 'unbounded', lambda: 100, workers: 4, queue: 8, taskMs: 100, burst: false }

interface Job {
  arrivedMs: number
  remainingMs: number
}

interface Worker {
  job: Job | null
}

// Номер по ближайшему рангу: p99 из 100 значений — 99-е по возрастанию.
function percentile(sorted: readonly number[], p: number): number {
  if (sorted.length === 0) return 0
  return sorted[Math.max(0, Math.ceil((p / 100) * sorted.length) - 1)] as number
}

function arrivalRate(p: WorkerPoolParams, tMs: number): number {
  const inBurst = p.burst && tMs >= BURST_FROM_S * 1000 && tMs < (BURST_FROM_S + BURST_LENGTH_S) * 1000
  return p.lambda * (inBurst ? BURST_FACTOR : 1)
}

export function simulateWorkerPool(p: WorkerPoolParams, seconds: number): WorkerPoolTick[] {
  const ticks: WorkerPoolTick[] = []
  const active: Job[] = [] // unbounded: каждая задача — своя горутина
  const workers: Worker[] = Array.from({ length: p.workers }, () => ({ job: null })) // pool и semaphore
  const waiting: Job[] = [] // pool: буфер канала ёмкости queue; semaphore: ждущие горутины
  const backlog: Job[] = [] // pool: задачи, которые производитель не смог отправить в полный канал
  const window: { tick: number; latency: number }[] = []
  let carry = 0
  let done = 0
  let rejected = 0
  let p50 = 0
  let p99 = 0

  const finish = (job: Job, atMs: number, tick: number): void => {
    done++
    window.push({ tick, latency: atMs - job.arrivedMs })
  }

  // Производитель, заблокированный на полном канале, дописывает задачи в буфер по мере освобождения мест.
  const refill = (): void => {
    while (backlog.length > 0 && waiting.length < p.queue) waiting.push(backlog.shift() as Job)
  }

  // Воркер берёт из буфера; при ёмкости 0 (рандеву) — прямо у заблокированного производителя.
  const pull = (): Job | null => {
    const job = waiting.shift() ?? backlog.shift() ?? null
    refill()
    return job
  }

  for (let tick = 0; tick < seconds * TICKS_PER_SECOND; tick++) {
    const t0 = tick * TICK_MS
    carry += (arrivalRate(p, t0) * TICK_MS) / 1000
    const arrivals = Math.floor(carry + EPS)
    carry -= arrivals

    // Освободившиеся воркеры сначала разбирают очередь: иначе новая задача обгонит ждущие (нарушит FIFO).
    for (const w of workers) {
      if (w.job !== null) continue
      w.job = pull()
    }

    for (let i = 0; i < arrivals; i++) {
      const job: Job = { arrivedMs: t0, remainingMs: p.taskMs }
      if (p.mode === 'unbounded') {
        active.push(job)
        continue
      }
      const idle = workers.find((w) => w.job === null)
      if (idle) idle.job = job
      // Канал-буфер ёмкости queue. Когда он полон: pool — производитель блокируется и держит задачу у себя,
      // semaphore — сразу отказ.
      else if (waiting.length < p.queue) waiting.push(job)
      else if (p.mode === 'pool') backlog.push(job)
      else rejected++
    }

    if (p.mode === 'unbounded') {
      // CPU — workers ядер: при active > workers время задачи растёт пропорционально (processor sharing).
      const speed = Math.min(1, p.workers / Math.max(1, active.length))
      for (let i = active.length - 1; i >= 0; i--) {
        const job = active[i] as Job
        const before = job.remainingMs
        job.remainingMs -= TICK_MS * speed
        if (job.remainingMs > EPS) continue
        finish(job, t0 + before / speed, tick)
        active.splice(i, 1)
      }
    } else {
      for (const w of workers) {
        let used = 0
        while (used < TICK_MS) {
          if (w.job === null) {
            const next = pull()
            if (!next) break
            w.job = next
          }
          const step = Math.min(w.job.remainingMs, TICK_MS - used)
          w.job.remainingMs -= step
          used += step
          if (w.job.remainingMs > EPS) continue
          finish(w.job, t0 + used, tick)
          w.job = null
        }
      }
    }

    while (window[0] && window[0].tick <= tick - LATENCY_WINDOW_TICKS) window.shift()
    // Нет завершений в окне — оставляем прошлые значения: графику нечего показывать, а ноль соврал бы.
    if (window.length > 0) {
      const sorted = window.map((x) => x.latency).sort((a, b) => a - b)
      p50 = percentile(sorted, 50)
      p99 = percentile(sorted, 99)
    }

    const busy = workers.filter((w) => w.job !== null).length
    // pool: воркеры + одна горутина-производитель. unbounded: все активные задачи. semaphore: каждая принятая задача.
    const goroutines = p.mode === 'unbounded' ? active.length : p.mode === 'pool' ? p.workers + 1 : busy + waiting.length
    // unbounded очереди нет: «в очереди» — готовые горутины, которым не хватило ядра.
    const buffered = p.mode === 'unbounded' ? Math.max(0, active.length - p.workers) : waiting.length
    const blocked = backlog.length
    ticks.push({ t: (tick + 1) / TICKS_PER_SECOND, goroutines, memKiB: goroutines * GOROUTINE_KIB, queued: buffered + blocked, buffered, blocked, done, rejected, p50ms: p50, p99ms: p99 })
  }
  return ticks
}

// --- Песочница ---

export interface WorkerPoolState {
  params: WorkerPoolParams
  ticks: readonly WorkerPoolTick[]
}

export interface WorkerPoolFrame {
  params: WorkerPoolParams
  ticks: readonly WorkerPoolTick[] // тики до текущей секунды включительно
  seconds: number // длина полного прогона: ось времени одинакова на всех кадрах
}

export type WorkerPoolAction = { type: 'set'; patch: Partial<WorkerPoolParams> } | { type: 'run' } | { type: 'reset' }

const clamp = (v: number, { min, max }: { min: number; max: number }): number => Math.min(max, Math.max(min, Math.round(v)))

const normalize = (p: WorkerPoolParams): WorkerPoolParams => ({
  ...p,
  lambda: clamp(p.lambda, LIMITS.lambda),
  workers: clamp(p.workers, LIMITS.workers),
  queue: clamp(p.queue, LIMITS.queue),
  taskMs: clamp(p.taskMs, LIMITS.taskMs),
})

const compute = (params: WorkerPoolParams): WorkerPoolState => ({ params, ticks: simulateWorkerPool(params, SECONDS) })

export const workerPoolSimulation: Simulation<WorkerPoolState, WorkerPoolAction, WorkerPoolFrame> = {
  initial: () => compute(DEFAULT_PARAMS),
  apply: (s, a) => {
    switch (a.type) {
      case 'set':
        return compute(normalize({ ...s.params, ...a.patch }))
      case 'run':
        return compute(s.params)
      case 'reset':
        return compute(DEFAULT_PARAMS)
    }
  },
  steps: (s) =>
    Array.from({ length: SECONDS }, (_, i): Step<WorkerPoolFrame> => {
      const ticks = s.ticks.slice(0, (i + 1) * TICKS_PER_SECOND)
      return { state: { params: s.params, ticks, seconds: SECONDS }, caption: captionFor(s.params, ticks) }
    }),
}

export const throughputPerSecond = (p: WorkerPoolParams): number => (p.workers * 1000) / p.taskMs

function captionFor(p: WorkerPoolParams, ticks: readonly WorkerPoolTick[]): Caption {
  const now = ticks.at(-1) as WorkerPoolTick
  const load = `${p.lambda} задач/с против пропускной способности ${Math.round(throughputPerSecond(p))} задач/с`
  const second = `Секунда ${Math.round(now.t)}: `
  if (p.mode === 'unbounded')
    return [{ text: `${second}по горутине на задачу, ${load}. Живых ` }, term('горутин', 'goroutine'), { text: ` ${now.goroutines}, оценка памяти ${now.memKiB} КиБ; предела нет.` }]
  if (p.mode === 'pool')
    return [term('Пул', 'worker-pool'), { text: ` из ${p.workers} воркеров, ${load}. В буфере канала ${now.buffered} из ${p.queue}, у производителя ждёт ${now.blocked}, p99 ${Math.round(now.p99ms)} мс: когда буфер полон, производитель блокируется — это ` }, term('backpressure', 'backpressure'), { text: '.' }]
  return [term('Семафор', 'semaphore'), { text: ` на ${p.workers} места и очередь ${p.queue}, ${load}. Отказов (503) ${now.rejected}, p99 ${Math.round(now.p99ms)} мс.` }]
}
