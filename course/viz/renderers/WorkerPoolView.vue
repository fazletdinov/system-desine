<script setup lang="ts">
import { computed } from 'vue'
import { GOROUTINE_KIB, TICKS_PER_SECOND, throughputPerSecond, type WorkerPoolFrame, type WorkerPoolTick } from '../simulations/worker-pool'

const props = defineProps<{ frame: WorkerPoolFrame }>()

// Общая геометрия: viewBox 360 даёт ≥ 12 px кегля на 375 px экрана при шрифте 15.
const W = 360
const H = 96
const LEFT = 48
const RIGHT = 48
const TOP = 8
const BOTTOM = 8
const AXIS_H = 22
const FONT = 15
const plotW = W - LEFT - RIGHT
const plotH = H - TOP - BOTTOM

interface SeriesDef {
  label: string
  color: string
  dash: string
  value: (t: WorkerPoolTick) => number
  axis: 'left' | 'right'
}
interface ChartDef {
  key: string
  title: string
  summary: string
  series: SeriesDef[]
}

// «Круглый» верх шкалы: 1, 2 или 5 × 10^n — подписи оси читаются без дробей.
function niceMax(v: number): number {
  if (v <= 1) return 1
  const pow = 10 ** Math.floor(Math.log10(v))
  const m = v / pow
  return (m <= 1 ? 1 : m <= 2 ? 2 : m <= 5 ? 5 : 10) * pow
}

const last = computed(() => props.frame.ticks.at(-1) as WorkerPoolTick)
const x = (t: number): number => LEFT + (t / props.frame.seconds) * plotW
const y = (v: number, max: number): number => TOP + plotH - (v / max) * plotH

const charts = computed<ChartDef[]>(() => {
  const l = last.value
  return [
    {
      key: 'g',
      title: 'Горутины и память',
      summary: `горутин ${l.goroutines}, память ≈ ${l.memKiB} КиБ (оценка, ${GOROUTINE_KIB} КиБ на горутину)`,
      series: [{ label: 'горутины', color: 'var(--viz-service)', dash: '', value: (t) => t.goroutines, axis: 'left' }],
    },
    {
      key: 'q',
      title: 'Очередь',
      summary: `в буфере ${l.buffered}, у производителя ${l.blocked}`,
      series: [
        { label: 'в буфере канала (≤ очереди)', color: 'var(--viz-cache)', dash: '', value: (t) => t.buffered, axis: 'left' },
        { label: 'ждут у производителя', color: 'var(--viz-error)', dash: '8 4', value: (t) => t.blocked, axis: 'left' },
      ],
    },
    {
      key: 'l',
      title: 'Задержка и отказы',
      summary: `p50 ${Math.round(l.p50ms)} мс, p99 ${Math.round(l.p99ms)} мс, отказов (503) ${l.rejected}`,
      series: [
        { label: 'p50', color: 'var(--viz-client)', dash: '', value: (t) => t.p50ms, axis: 'left' },
        { label: 'p99', color: 'var(--viz-error)', dash: '8 4', value: (t) => t.p99ms, axis: 'left' },
        { label: 'отказы (справа)', color: 'var(--viz-network)', dash: '2 4', value: (t) => t.rejected, axis: 'right' },
      ],
    },
  ]
})

const maxOf = (def: ChartDef, axis: 'left' | 'right'): number => {
  const vals = def.series.filter((s) => s.axis === axis).flatMap((s) => props.frame.ticks.map(s.value))
  return niceMax(Math.max(1, ...vals))
}
// Ось X у всех трёх графиков общая: засечки через каждые 5 с.
const xTicks = computed(() => Array.from({ length: Math.floor(props.frame.seconds / 5) + 1 }, (_, i) => i * 5))
const path = (def: ChartDef, s: SeriesDef): string => {
  const max = maxOf(def, s.axis)
  return props.frame.ticks.map((t, i) => `${i === 0 ? 'M' : 'L'}${x(t.t).toFixed(1)} ${y(s.value(t), max).toFixed(1)}`).join(' ')
}
const nowX = computed(() => x(props.frame.ticks.length / TICKS_PER_SECOND))
const capacity = computed(() => Math.round(throughputPerSecond(props.frame.params)))
</script>

<template>
  <div class="wp">
    <p class="meta">Пропускная способность пула: {{ frame.params.workers }} × 1000 / {{ frame.params.taskMs }} мс = <b>{{ capacity }}</b> задач/с; поток — <b>{{ frame.params.lambda }}</b> задач/с{{ frame.params.burst ? ' (с всплеском ×5 с 5-й по 8-ю секунду)' : '' }}.</p>
    <figure v-for="(c, ci) in charts" :key="c.key" class="chart">
      <figcaption><b>{{ c.title }}</b> — {{ c.summary }}</figcaption>
      <svg :viewBox="`0 0 ${W} ${H + (ci === charts.length - 1 ? AXIS_H : 0)}`" role="img" :aria-label="`${c.title}: ${c.summary}`">
        <line v-for="tk in xTicks" :key="tk" :x1="x(tk)" :x2="x(tk)" :y1="TOP" :y2="H - BOTTOM" class="grid" />
        <line :x1="LEFT" :x2="W - RIGHT" :y1="H - BOTTOM" :y2="H - BOTTOM" class="axis" />
        <line :x1="nowX" :x2="nowX" :y1="TOP" :y2="H - BOTTOM" class="now" />
        <text :x="LEFT - 4" :y="TOP + FONT * 0.8" text-anchor="end" :font-size="FONT" class="lbl">{{ maxOf(c, 'left') }}</text>
        <text :x="LEFT - 4" :y="H - BOTTOM" text-anchor="end" :font-size="FONT" class="lbl">0</text>
        <template v-if="c.series.some((s) => s.axis === 'right')">
          <text :x="W - RIGHT + 4" :y="TOP + FONT * 0.8" :font-size="FONT" class="lbl">{{ maxOf(c, 'right') }}</text>
          <text :x="W - RIGHT + 4" :y="H - BOTTOM" :font-size="FONT" class="lbl">0</text>
        </template>
        <path v-for="s in c.series" :key="s.label" :d="path(c, s)" fill="none" :stroke="s.color" stroke-width="2.5" :stroke-dasharray="s.dash" stroke-linejoin="round" />
        <template v-if="ci === charts.length - 1">
          <text v-for="tk in xTicks" :key="`x${tk}`" :x="x(tk)" :y="H + AXIS_H - 6" text-anchor="middle" :font-size="FONT" class="lbl">{{ tk }} с</text>
        </template>
      </svg>
      <ul class="legend">
        <li v-for="s in c.series" :key="s.label">
          <svg viewBox="0 0 28 8" aria-hidden="true"><line x1="0" y1="4" x2="28" y2="4" :stroke="s.color" stroke-width="2.5" :stroke-dasharray="s.dash" /></svg>
          {{ s.label }}
        </li>
      </ul>
    </figure>
  </div>
</template>

<style scoped>
.wp { color: var(--viz-text); }
.meta { margin: 0 0 8px; font-size: 14px; color: var(--viz-muted); }
.chart { margin: 0 0 8px; }
figcaption { font-size: 14px; margin-bottom: 2px; }
svg { display: block; width: 100%; height: auto; }
.grid { stroke: var(--viz-edge); stroke-width: 1; }
.axis { stroke: var(--viz-muted); stroke-width: 1.5; }
.now { stroke: var(--viz-active); stroke-width: 2; }
.lbl { fill: var(--viz-muted); font-variant-numeric: tabular-nums; }
.legend { display: flex; flex-wrap: wrap; gap: 4px 14px; margin: 0; padding: 0; list-style: none; font-size: 13px; color: var(--viz-muted); }
.legend li { display: flex; align-items: center; gap: 6px; }
.legend svg { width: 28px; height: 8px; }
</style>
