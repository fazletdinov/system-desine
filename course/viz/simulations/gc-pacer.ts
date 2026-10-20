import type { Caption, CaptionSegment } from '../core/caption'
import { term } from '../core/caption'
import type { Simulation, Step } from '../core/types'
import { MEM_LIMIT_MIB, limitedGoal, overhead } from './gomemlimit'

// Упрощённая детерминированная модель сборщика мусора Go по тикам 100 мс: живая куча, поток
// аллокаций, GOGC, GOMEMLIMIT и лимит контейнера (memory.max = MEM_LIMIT_MIB). Формулы цели
// и накладных расходов — из модели gomemlimit урока 1.17; сверху — запуск цикла по триггеру,
// доля CPU на разметку и ограничитель CPU GC (mgclimit.go). CPU считается в долях всех ядер.

export const TICK_MS = 100
export const TICKS_PER_SECOND = 1000 / TICK_MS
const DT = TICK_MS / 1000
// Окно графика: достаточно, чтобы увидеть несколько циклов и всплеск целиком.
export const WINDOW_S = 20
export const DEFAULT_GOGC = 100
// mgcpacer.go: gcBackgroundUtilization = 0.25 — фоновые воркеры разметки берут четверть CPU.
export const BACKGROUND_UTILIZATION = 0.25
// mgclimit.go: «очень консервативный предел 50 %». В рантайме это дырявое ведро ёмкостью
// 1 CPU-секунда на P, короткие всплески выше 50 % допускаются; модель упрощает его до потолка.
export const LIMITER_CAP = 0.5
// Цена разметки: CPU-секунд (в долях всех ядер) на МиБ живой кучи. Порядок величины, не замер.
export const MARK_CPU_S_PER_MIB = 0.001
// mgcpacer.go: триггер лежит между ~0,7 и ~0,95 пути от живой кучи к цели
// (minTriggerRatioNum = 45, maxTriggerRatioNum = 61 из triggerRatioDen = 64).
const MIN_TRIGGER = 45 / 64
const MAX_TRIGGER = 61 / 64
// Темп живого прогона: обычно 10 обновлений в секунду (тик за тиком); при prefers-reduced-motion —
// одно обновление в секунду сразу на 10 тиков, чтобы график не прокручивался непрерывно.
export function tickerPlan(reducedMotion: boolean): { intervalMs: number; ticksPerUpdate: number } {
  return reducedMotion ? { intervalMs: 1000, ticksPerUpdate: TICKS_PER_SECOND } : { intervalMs: TICK_MS, ticksPerUpdate: 1 }
}

export const LEAK_MIB_PER_S = 5
export const BURST_FACTOR = 3
export const BURST_S = 3
const EPS = 1e-9

export interface GcPacerParams {
  gogc: number | null // null — GOGC=off
  limitMiB: number | null // null — GOMEMLIMIT не задан
  liveMiB: number // живая куча без утечки
  allocMiBps: number
  limiter: boolean // true — ограничитель CPU GC (Go 1.19+), false — «как до 1.19»
}

export const LIMITS = {
  gogc: { min: 25, max: 800, step: 25 },
  limitMiB: { min: 64, max: 256, step: 8 },
  liveMiB: { min: 10, max: 200, step: 5 },
  allocMiBps: { min: 10, max: 400, step: 10 },
} as const

export const DEFAULT_PARAMS: GcPacerParams = { gogc: DEFAULT_GOGC, limitMiB: null, liveMiB: 60, allocMiBps: 100, limiter: true }

export interface GcTick {
  t: number // секунды от старта, конец тика
  liveMiB: number
  heapMiB: number // куча с мусором до уборки в этом тике
  goalMiB: number | null // null — цели нет (GOGC=off без лимита)
  rssMiB: number // куча + накладные, как видит cgroup; выше MEM_LIMIT_MIB не бывает
  gcCpu: number // доля всего CPU на GC: фон + помощь мутатора
  assistCpu: number // часть gcCpu сверх фоновых 25 %: помощь в разметке (mark assist)
  allocMiBps: number // фактический поток аллокаций с учётом всплеска
  marking: boolean
  cycleStart: boolean
  killed: boolean
}

export interface GcPacerState {
  params: GcPacerParams
  tick: number
  heapMiB: number
  leakedMiB: number
  leaking: boolean
  burstTicksLeft: number
  marking: boolean
  markLeftCpuS: number
  cycles: number
  ooms: number
  history: readonly GcTick[]
}

export interface GcPacerFrame {
  params: GcPacerParams
  history: readonly GcTick[]
  cycles: number
  ooms: number
  leaking: boolean
  bursting: boolean
  windowS: number
  containerMiB: number
}

export type GcPacerAction =
  | { type: 'set'; patch: Partial<GcPacerParams> }
  | { type: 'tick' }
  | { type: 'leak' }
  | { type: 'burst' }
  | { type: 'reset' }

// Цель кучи: живая × (1 + GOGC/100); при GOGC=100 это GOGC_FACTOR × живой, как в уроке 1.17. GOMEMLIMIT урезает её
// той же формулой, что в уроке 1.17: лимит минус накладные расходы.
export function heapGoal(p: GcPacerParams, live: number): number {
  const natural = p.gogc === null ? Infinity : live * (1 + p.gogc / 100)
  return p.limitMiB === null ? natural : limitedGoal(natural, p.limitMiB, live)
}

// Пейсер начинает цикл заранее: за время разметки при 25 % CPU мутатор успеет выделить
// runway, и к концу разметки куча как раз дойдёт до цели.
function trigger(live: number, goal: number, alloc: number): number {
  if (goal === Infinity) return Infinity
  if (goal <= live) return live
  const runway = alloc * (1 - BACKGROUND_UTILIZATION) * ((live * MARK_CPU_S_PER_MIB) / BACKGROUND_UTILIZATION)
  const lo = live + MIN_TRIGGER * (goal - live)
  const hi = live + MAX_TRIGGER * (goal - live)
  return Math.min(hi, Math.max(lo, goal - runway))
}

// Ограничитель работает только при заданном GOMEMLIMIT: без лимита спирали смерти быть не может.
const limiterActive = (p: GcPacerParams): boolean => p.limiter && p.limitMiB !== null

function gcShare(p: GcPacerParams, heap: number, goal: number, alloc: number, markLeft: number): number {
  const timeLeft = Math.max(0, goal - heap) / alloc
  // Куча уже у цели — мутатор помогает разметке всем своим временем.
  const need = timeLeft > EPS ? markLeft / timeLeft : 1
  const share = Math.min(1, Math.max(BACKGROUND_UTILIZATION, need))
  return limiterActive(p) ? Math.min(share, LIMITER_CAP) : share
}

const restarted = (s: GcPacerState): GcPacerState => ({
  ...s,
  heapMiB: s.params.liveMiB,
  leakedMiB: 0,
  burstTicksLeft: 0,
  marking: false,
  markLeftCpuS: 0,
})

function advance(prev: GcPacerState): GcPacerState {
  // OOM-kill: контейнер перезапускается, процесс стартует с пустой кучей и без накопленной утечки.
  const s = prev.history.at(-1)?.killed === true ? restarted(prev) : prev
  const p = s.params
  const alloc = p.allocMiBps * (s.burstTicksLeft > 0 ? BURST_FACTOR : 1)
  const leakedMiB = s.leakedMiB + (s.leaking ? LEAK_MIB_PER_S * DT : 0)
  const live = p.liveMiB + leakedMiB
  const goal = heapGoal(p, live)
  let heap = Math.max(s.heapMiB, live)
  let { marking, markLeftCpuS, cycles, ooms } = s
  let cycleStart = false
  if (!marking && heap >= trigger(live, goal, alloc)) {
    marking = true
    markLeftCpuS = live * MARK_CPU_S_PER_MIB
    cycles++
    cycleStart = true
  }
  const gcCpu = marking ? gcShare(p, heap, goal, alloc, markLeftCpuS) : 0
  // Время, отданное разметке, мутатор не работает и не выделяет память.
  heap += alloc * DT * (1 - gcCpu) + (s.leaking ? LEAK_MIB_PER_S * DT : 0)
  const peakHeap = heap
  if (marking) {
    markLeftCpuS -= gcCpu * DT
    if (markLeftCpuS <= EPS) {
      marking = false
      markLeftCpuS = 0
      heap = live // уборка освобождает весь мусор
    }
  }
  const demand = peakHeap + overhead(live)
  const killed = demand > MEM_LIMIT_MIB
  if (killed) ooms++
  const t = (s.tick + 1) / TICKS_PER_SECOND
  const tick: GcTick = {
    t,
    liveMiB: live,
    heapMiB: peakHeap,
    goalMiB: goal === Infinity ? null : goal,
    rssMiB: Math.min(demand, MEM_LIMIT_MIB),
    gcCpu,
    assistCpu: Math.max(0, gcCpu - BACKGROUND_UTILIZATION),
    allocMiBps: alloc,
    marking: gcCpu > 0,
    cycleStart,
    killed,
  }
  return {
    ...s,
    tick: s.tick + 1,
    heapMiB: heap,
    leakedMiB,
    burstTicksLeft: Math.max(0, s.burstTicksLeft - 1),
    marking,
    markLeftCpuS,
    cycles,
    ooms,
    history: [...s.history, tick].slice(-WINDOW_S * TICKS_PER_SECOND),
  }
}

const clampStep = (v: number, { min, max, step }: { min: number; max: number; step: number }): number =>
  Math.min(max, Math.max(min, Math.round(v / step) * step))

const normalize = (p: GcPacerParams): GcPacerParams => ({
  ...p,
  gogc: p.gogc === null ? null : clampStep(p.gogc, LIMITS.gogc),
  limitMiB: p.limitMiB === null ? null : clampStep(p.limitMiB, LIMITS.limitMiB),
  liveMiB: clampStep(p.liveMiB, LIMITS.liveMiB),
  allocMiBps: clampStep(p.allocMiBps, LIMITS.allocMiBps),
})

const initial = (): GcPacerState => ({
  params: DEFAULT_PARAMS,
  tick: 0,
  heapMiB: DEFAULT_PARAMS.liveMiB,
  leakedMiB: 0,
  leaking: false,
  burstTicksLeft: 0,
  marking: false,
  markLeftCpuS: 0,
  cycles: 0,
  ooms: 0,
  history: [],
})

export const gcPacerSimulation: Simulation<GcPacerState, GcPacerAction, GcPacerFrame> = {
  initial,
  apply: (s, a) => {
    switch (a.type) {
      case 'set':
        return { ...s, params: normalize({ ...s.params, ...a.patch }) }
      case 'tick':
        return advance(s)
      case 'leak':
        return { ...s, leaking: !s.leaking }
      case 'burst':
        return { ...s, burstTicksLeft: BURST_S * TICKS_PER_SECOND }
      case 'reset':
        return initial()
    }
  },
  steps: (s) => [
    {
      state: { params: s.params, history: s.history, cycles: s.cycles, ooms: s.ooms, leaking: s.leaking, bursting: s.burstTicksLeft > 0, windowS: WINDOW_S, containerMiB: MEM_LIMIT_MIB },
      caption: captionFor(s),
    } satisfies Step<GcPacerFrame>,
  ],
}

const PACER = term('пейсер', 'gc-pacer')
const GOGC = term('GOGC', 'gogc')
const GOMEMLIMIT = term('GOMEMLIMIT', 'gomemlimit')
const r = (v: number): number => Math.round(v)
const pct = (v: number): string => `${Math.round(v * 100)} %`

function situation(s: GcPacerState, now: GcTick, recentCpu: number): CaptionSegment[] {
  const p = s.params
  const second = `Секунда ${r(now.t)}: `
  if (now.killed)
    return [{ text: `${second}память процесса дошла до лимита контейнера ${MEM_LIMIT_MIB} МиБ, и ядро убило его — ` }, term('OOM-kill', 'oom-kill'), { text: '. Оркестратор перезапускает контейнер с пустой кучей.' }]
  if (p.gogc === null && p.limitMiB === null)
    return [{ text: `${second}GOGC=off и лимита нет: цели кучи не существует, сборщик не запускается ни разу, куча ${r(now.heapMiB)} МиБ растёт на весь поток аллокаций, пока контейнер не упрётся в стену.` }]
  if (recentCpu >= 0.9)
    return [{ text: `${second}живая куча ${r(now.liveMiB)} МиБ не помещается под ` }, GOMEMLIMIT, { text: ` = ${p.limitMiB} МиБ. Цель ниже живых данных, цикл начинается сразу после предыдущего, и сборщик забирает ${pct(recentCpu)} CPU: программа почти стоит. Это ` }, term('спираль смерти', 'death-spiral'), { text: '; так было бы без ограничителя из Go 1.19.' }]
  if (limiterActive(p) && now.goalMiB !== null && now.goalMiB <= now.liveMiB)
    return [{ text: `${second}живая куча ${r(now.liveMiB)} МиБ выше того, что разрешает ` }, GOMEMLIMIT, { text: ` = ${p.limitMiB} МиБ, но ограничитель CPU GC не даёт сборщику взять больше ${pct(LIMITER_CAP)}: сейчас ${pct(recentCpu)}. Куча выходит за лимит, зато программа продолжает работать. Модель упрощает дырявое ведро рантайма до жёсткого потолка.` }]
  if (p.limitMiB !== null && now.goalMiB !== null && p.gogc !== null && now.goalMiB < now.liveMiB * (1 + p.gogc / 100))
    return [{ text: `${second}естественная цель ${r(now.liveMiB * (1 + p.gogc / 100))} МиБ не помещается под ` }, GOMEMLIMIT, { text: ` = ${p.limitMiB} МиБ, поэтому цель урезана до ${r(now.goalMiB)} МиБ. Мусору меньше места, циклы идут чаще: всего ${s.cycles}.` }]
  const goal = now.goalMiB === null ? '' : `Цель ${r(now.goalMiB)} МиБ`
  const why = p.gogc === null ? ' (GOGC=off: работает только лимит)' : ` = живая ${r(now.liveMiB)} × (1 + ${p.gogc}/100)`
  const assist: CaptionSegment[] = now.assistCpu > 0 ? [{ text: ' Сейчас мутатор помогает разметке (' }, term('mark assist', 'mark-assist'), { text: `): ещё ${pct(now.assistCpu)} CPU сверх фоновых 25 %.` }] : []
  return [{ text: `${second}` }, PACER, { text: ' держит кучу у цели. ' }, GOGC, { text: `: ${goal}${why}. Циклов ${s.cycles}, CPU на GC за последнюю секунду ${pct(recentCpu)}.` }, ...assist]
}

function captionFor(s: GcPacerState): Caption {
  const now = s.history.at(-1)
  if (now === undefined)
    return [
      { text: 'Упрощённая модель: числа круглые, механизм настоящий. Нажмите «Старт». ' },
      PACER,
      { text: ' сборщика мусора ставит цель кучи: живые данные × (1 + ' },
      GOGC,
      { text: '/100) и запускает цикл заранее, чтобы разметка закончилась к цели. ' },
      GOMEMLIMIT,
      { text: ` урезает цель, а контейнер убивает процесс на ${MEM_LIMIT_MIB} МиБ.` },
    ]
  const recent = s.history.slice(-TICKS_PER_SECOND)
  const recentCpu = recent.reduce((sum, t) => sum + t.gcCpu, 0) / recent.length
  const ooms: CaptionSegment[] = s.ooms > 0 && !now.killed ? [{ text: ' Перезапусков после ' }, term('OOM-kill', 'oom-kill'), { text: `: ${s.ooms}.` }] : []
  return [...situation(s, now, recentCpu), ...ooms]
}
