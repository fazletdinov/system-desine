<script setup lang="ts">
import { computed } from 'vue'
import { BACKGROUND_UTILIZATION, LIMITER_CAP, TICKS_PER_SECOND, type GcPacerFrame, type GcTick } from '../simulations/gc-pacer'

const props = defineProps<{ frame: GcPacerFrame }>()

// Геометрия как у WorkerPoolView: viewBox 360 даёт ≥ 12 px кегля на экране 375 px при шрифте 15.
const W = 360
const LEFT = 48
const RIGHT = 12
const TOP = 8
const BOTTOM = 8
const AXIS_H = 22
const FONT = 15
const MEM_H = 150
const CPU_H = 90
const plotW = W - LEFT - RIGHT

const last = computed<GcTick | undefined>(() => props.frame.history.at(-1))
// Ось времени скользит: последние windowS секунд.
const t0 = computed(() => Math.max(0, (last.value?.t ?? 0) - props.frame.windowS))
const x = (t: number): number => LEFT + ((t - t0.value) / props.frame.windowS) * plotW
const xTicks = computed(() => {
  const first = Math.ceil(t0.value / 5) * 5
  return Array.from({ length: Math.floor(props.frame.windowS / 5) + 1 }, (_, i) => first + i * 5).filter((t) => t <= t0.value + props.frame.windowS)
})
const barW = plotW / (props.frame.windowS * TICKS_PER_SECOND)

// Шкала памяти фиксирована: лимит контейнера всегда виден, цель выше шкалы обрезается.
const memMax = computed(() => Math.round(props.frame.containerMiB * 1.25))
const yMem = (v: number): number => TOP + (MEM_H - TOP - BOTTOM) * (1 - Math.min(v, memMax.value) / memMax.value)
const yCpu = (v: number): number => TOP + (CPU_H - TOP - BOTTOM) * (1 - v)

function path(value: (t: GcTick) => number | null): string {
  let d = ''
  let pen = 'M'
  for (const t of props.frame.history) {
    const v = value(t)
    if (v === null) {
      pen = 'M'
      continue
    }
    d += `${pen}${x(t.t).toFixed(1)} ${yMem(v).toFixed(1)} `
    pen = 'L'
  }
  return d.trim()
}

const heapPath = computed(() => path((t) => t.heapMiB))
const livePath = computed(() => path((t) => t.liveMiB))
const goalPath = computed(() => path((t) => t.goalMiB))
const goalClipped = computed(() => (last.value?.goalMiB ?? 0) > memMax.value)
const kills = computed(() => props.frame.history.filter((t) => t.killed))
const limiterOn = computed(() => props.frame.params.limiter && props.frame.params.limitMiB !== null)

const pct = (v: number): string => `${Math.round(v * 100)} %`
const mib = (v: number | null | undefined): string => (v === null || v === undefined ? '—' : `${Math.round(v)} МиБ`)
const recentCpu = computed(() => {
  const recent = props.frame.history.slice(-TICKS_PER_SECOND)
  return recent.length === 0 ? 0 : recent.reduce((s, t) => s + t.gcCpu, 0) / recent.length
})
const memSummary = computed(() =>
  last.value === undefined
    ? 'модель не запущена'
    : `живая ${mib(last.value.liveMiB)}, куча ${mib(last.value.heapMiB)}, цель ${last.value.goalMiB === null ? 'нет' : mib(last.value.goalMiB)}, память процесса ${mib(last.value.rssMiB)} из ${props.frame.containerMiB}`,
)
const cpuSummary = computed(() => `за последнюю секунду ${pct(recentCpu.value)}${limiterOn.value ? `, потолок ограничителя ${pct(LIMITER_CAP)}` : ''}`)

const MEM_SERIES = [
  { label: 'куча с мусором', cls: 'heap', dash: '' },
  { label: 'живая куча', cls: 'live', dash: '' },
  { label: 'цель GC', cls: 'goal', dash: '8 4' },
] as const
</script>

<template>
  <div class="gp">
    <dl class="counters">
      <div><dt>Циклов GC</dt><dd>{{ frame.cycles }}</dd></div>
      <div :class="{ bad: frame.ooms > 0 }"><dt>OOM-перезапусков</dt><dd>{{ frame.ooms }}</dd></div>
      <div :class="{ bad: recentCpu >= 0.9, mid: recentCpu >= 0.4 && recentCpu < 0.9 }"><dt>CPU на GC</dt><dd>{{ pct(recentCpu) }}</dd></div>
      <div><dt>Режим</dt><dd class="mode">{{ frame.leaking ? 'утечка' : '' }}{{ frame.leaking && frame.bursting ? ' + ' : '' }}{{ frame.bursting ? 'всплеск' : '' }}{{ !frame.leaking && !frame.bursting ? 'ровный' : '' }}</dd></div>
    </dl>

    <figure class="chart">
      <figcaption><b>Память, МиБ</b> — {{ memSummary }}</figcaption>
      <svg :viewBox="`0 0 ${W} ${MEM_H}`" role="img" :aria-label="`Память: ${memSummary}`">
        <line v-for="tk in xTicks" :key="tk" :x1="x(tk)" :x2="x(tk)" :y1="TOP" :y2="MEM_H - BOTTOM" class="grid" />
        <line :x1="LEFT" :x2="W - RIGHT" :y1="MEM_H - BOTTOM" :y2="MEM_H - BOTTOM" class="axis" />
        <line :x1="LEFT" :x2="W - RIGHT" :y1="yMem(frame.containerMiB)" :y2="yMem(frame.containerMiB)" class="wall" />
        <text :x="W - RIGHT - 2" :y="yMem(frame.containerMiB) - 4" text-anchor="end" :font-size="FONT - 2" class="wall-lbl">контейнер {{ frame.containerMiB }}</text>
        <template v-if="frame.params.limitMiB !== null">
          <line :x1="LEFT" :x2="W - RIGHT" :y1="yMem(frame.params.limitMiB)" :y2="yMem(frame.params.limitMiB)" class="limit" />
          <text :x="LEFT + 4" :y="yMem(frame.params.limitMiB) - 4" :font-size="FONT - 2" class="limit-lbl">GOMEMLIMIT {{ frame.params.limitMiB }}</text>
        </template>
        <line v-for="k in kills" :key="`k${k.t}`" :x1="x(k.t)" :x2="x(k.t)" :y1="TOP" :y2="MEM_H - BOTTOM" class="kill" />
        <path :d="goalPath" class="s goal" stroke-dasharray="8 4" />
        <path :d="heapPath" class="s heap" />
        <path :d="livePath" class="s live" />
        <text :x="LEFT - 4" :y="TOP + FONT * 0.8" text-anchor="end" :font-size="FONT" class="lbl">{{ memMax }}</text>
        <text :x="LEFT - 4" :y="MEM_H - BOTTOM" text-anchor="end" :font-size="FONT" class="lbl">0</text>
        <text v-if="frame.history.length === 0" :x="LEFT + plotW / 2" :y="MEM_H / 2" text-anchor="middle" :font-size="FONT" class="lbl">нажмите «Старт»</text>
      </svg>
      <ul class="legend">
        <li v-for="s in MEM_SERIES" :key="s.cls"><svg viewBox="0 0 28 8" aria-hidden="true"><line x1="0" y1="4" x2="28" y2="4" :class="['s', s.cls]" :stroke-dasharray="s.dash" /></svg>{{ s.label }}</li>
        <li><svg viewBox="0 0 28 8" aria-hidden="true"><line x1="0" y1="4" x2="28" y2="4" class="kill" /></svg>OOM-kill</li>
        <li v-if="goalClipped" class="warn">цель выше шкалы: {{ mib(last?.goalMiB) }}</li>
      </ul>
    </figure>

    <figure class="chart">
      <figcaption><b>Доля CPU на GC</b> — {{ cpuSummary }}</figcaption>
      <svg :viewBox="`0 0 ${W} ${CPU_H + AXIS_H}`" role="img" :aria-label="`CPU на GC: ${cpuSummary}`">
        <line v-for="tk in xTicks" :key="tk" :x1="x(tk)" :x2="x(tk)" :y1="TOP" :y2="CPU_H - BOTTOM" class="grid" />
        <g v-for="t in frame.history" :key="t.t">
          <rect v-if="t.gcCpu > 0" :x="x(t.t) - barW" :width="barW" :y="yCpu(Math.min(t.gcCpu, BACKGROUND_UTILIZATION))" :height="yCpu(0) - yCpu(Math.min(t.gcCpu, BACKGROUND_UTILIZATION))" class="bg-bar" />
          <rect v-if="t.assistCpu > 0" :x="x(t.t) - barW" :width="barW" :y="yCpu(t.gcCpu)" :height="yCpu(BACKGROUND_UTILIZATION) - yCpu(t.gcCpu)" class="assist-bar" />
        </g>
        <line :x1="LEFT" :x2="W - RIGHT" :y1="CPU_H - BOTTOM" :y2="CPU_H - BOTTOM" class="axis" />
        <template v-if="limiterOn">
          <line :x1="LEFT" :x2="W - RIGHT" :y1="yCpu(LIMITER_CAP)" :y2="yCpu(LIMITER_CAP)" class="cap" />
          <text :x="W - RIGHT - 2" :y="yCpu(LIMITER_CAP) - 4" text-anchor="end" :font-size="FONT - 2" class="cap-lbl">ограничитель 50 %</text>
        </template>
        <text :x="LEFT - 4" :y="TOP + FONT * 0.8" text-anchor="end" :font-size="FONT" class="lbl">100 %</text>
        <text :x="LEFT - 4" :y="CPU_H - BOTTOM" text-anchor="end" :font-size="FONT" class="lbl">0</text>
        <text v-for="tk in xTicks" :key="`x${tk}`" :x="x(tk)" :y="CPU_H + AXIS_H - 6" text-anchor="middle" :font-size="FONT" class="lbl">{{ tk }} с</text>
      </svg>
      <ul class="legend">
        <li><span class="sw bg-sw" aria-hidden="true"></span>фоновая разметка (до 25 %)</li>
        <li><span class="sw assist-sw" aria-hidden="true"></span>помощь мутатора (mark assist)</li>
      </ul>
    </figure>
  </div>
</template>

<style scoped>
.gp { color: var(--viz-text); }
.counters { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 130px), 1fr)); gap: 8px; margin: 0 0 10px; }
.counters div { min-width: 0; padding: 4px 10px; border: 2px solid var(--viz-edge); border-left-width: 8px; border-radius: 8px; }
.counters .bad { border-left-color: var(--viz-error); }
.counters .mid { border-left-color: var(--viz-active); }
dt { font-size: 12px; color: var(--viz-muted); }
dd { margin: 0; font-size: 18px; font-weight: 700; font-variant-numeric: tabular-nums; }
dd.mode { font-size: 15px; }
.chart { margin: 0 0 8px; }
figcaption { font-size: 14px; margin-bottom: 2px; }
svg { display: block; width: 100%; height: auto; }
.grid { stroke: var(--viz-edge); stroke-width: 1; }
.axis { stroke: var(--viz-muted); stroke-width: 1.5; }
.lbl { fill: var(--viz-muted); font-variant-numeric: tabular-nums; }
.s { fill: none; stroke-width: 2.5; stroke-linejoin: round; }
.s.heap { stroke: var(--viz-cache); }
.s.live { stroke: var(--viz-db); }
.s.goal { stroke: var(--viz-service); }
.wall { stroke: var(--viz-error); stroke-width: 2; }
.wall-lbl { fill: var(--viz-error); }
.limit { stroke: var(--viz-client); stroke-width: 2; stroke-dasharray: 2 4; }
.limit-lbl { fill: var(--viz-client); }
.kill { stroke: var(--viz-error); stroke-width: 3; }
.cap { stroke: var(--viz-error); stroke-width: 2; stroke-dasharray: 6 4; }
.cap-lbl { fill: var(--viz-error); }
.bg-bar { fill: var(--viz-service); }
.assist-bar { fill: var(--viz-active); }
.legend { display: flex; flex-wrap: wrap; gap: 4px 14px; margin: 0; padding: 0; list-style: none; font-size: 13px; color: var(--viz-muted); }
.legend li { display: flex; align-items: center; gap: 6px; }
.legend svg { width: 28px; height: 8px; }
.legend .warn { color: var(--viz-text); font-weight: 600; }
.sw { display: inline-block; width: 14px; height: 10px; border-radius: 2px; }
.bg-sw { background: var(--viz-service); }
.assist-sw { background: var(--viz-active); }
</style>
