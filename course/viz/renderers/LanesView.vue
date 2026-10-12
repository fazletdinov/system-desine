<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import type { LanePacket, LanesResult, Proto } from '../simulations/lanes'
import { formatNumber } from '../core/format'
import { useNarrow } from '../components/useNarrow'

export interface Lane {
  proto: Proto
  title: string
  result: LanesResult
  /** Готовность потоков того же протокола без потерь — чтобы показать, насколько потеря задержала. */
  baseline: number[]
  connections: number
}

const props = defineProps<{ lanes: Lane[]; streams: number }>()
const emit = defineEmits<{ pick: [stream: number, index: number] }>()

const NARROW_PX = 520
const STREAM_VARS = ['--viz-client', '--viz-cache', '--viz-db'] as const
const hatchId = `hatch-${useId()}`

interface Layout {
  width: number
  labelW: number
  rowH: number
  axisH: number
  font: number
  maxBox: number
}

const WIDE: Layout = { width: 810, labelW: 54, rowH: 52, axisH: 22, font: 14, maxBox: 30 }
const NARROW: Layout = { width: 360, labelW: 40, rowH: 44, axisH: 20, font: 11, maxBox: 22 }

const svgEl = ref<SVGSVGElement>()
const narrow = useNarrow(svgEl, NARROW_PX)
const L = computed(() => (narrow.value ? NARROW : WIDE))

// Общая шкала времени для всех дорожек — чтобы протоколы можно было сравнивать по длине.
const horizon = computed(() => Math.ceil(Math.max(1, ...props.lanes.flatMap((l) => l.result.packets.map((p) => Math.max(p.arrivesAt, p.deliveredAt)))) + 1))
const unit = computed(() => (L.value.width - L.value.labelW - 8) / horizon.value)
const box = computed(() => Math.min(L.value.maxBox, unit.value - 2))
const x = (slot: number): number => L.value.labelW + slot * unit.value

const ticks = computed(() => {
  const step = horizon.value > 20 ? 5 : horizon.value > 10 ? 2 : 1
  return Array.from({ length: Math.floor(horizon.value / step) + 1 }, (_, i) => i * step)
})

const streamColor = (s: number): string => `var(${STREAM_VARS[s % STREAM_VARS.length]})`
const rowsOf = (lane: Lane): number => (lane.proto === 'h1' ? lane.connections : 1)
const heightOf = (lane: Lane): number => L.value.axisH + rowsOf(lane) * L.value.rowH + 4
const rowLabel = (lane: Lane, c: number): string => (lane.proto === 'quic' ? 'QUIC' : lane.proto === 'h1' && lane.connections > 1 ? `TCP ${c + 1}` : 'TCP')
const waits = (p: LanePacket): boolean => p.deliveredAt > p.arrivesAt + 1e-9
const boxY = (p: LanePacket): number => L.value.axisH + p.conn * L.value.rowH + 4

// Стрелка повтора: вниз из квадрата, вправо на RTT с половиной, вверх к месту, где пакет «долетел».
function retryPath(p: LanePacket): string {
  const x1 = x(p.sentAt) + box.value / 2
  const x2 = x(p.arrivesAt) + box.value / 2
  const y0 = boxY(p) + box.value
  const y1 = y0 + 12
  return `M ${x1} ${y0} L ${x1} ${y1} L ${x2} ${y1} L ${x2} ${y0 + 3}`
}

const fmt = (v: number): string => formatNumber(v, 1)
const delta = (lane: Lane, s: number): string => {
  const d = (lane.result.streamDone[s] ?? 0) - (lane.baseline[s] ?? 0)
  return d > 1e-9 ? ` (+${fmt(d)})` : ''
}
</script>

<template>
  <div class="lanes">
    <ul class="legend" aria-label="Потоки и обозначения">
      <li v-for="s in streams" :key="s"><span class="sw" :style="{ '--c': streamColor(s - 1) }" aria-hidden="true">{{ s }}</span> поток {{ s }}</li>
      <li><span class="sw lost" aria-hidden="true">✕</span> потерян, повтор — стрелкой</li>
      <li><span class="sw hatch" aria-hidden="true"></span> дошёл, но ждёт потерянный пакет</li>
    </ul>
    <section v-for="lane in lanes" :key="lane.proto" class="lane" :aria-label="lane.title">
      <h4 class="lane-title">{{ lane.title }}</h4>
      <svg :ref="(el) => { if (lane.proto === lanes[0]?.proto) svgEl = el as SVGSVGElement }" class="lane-svg" :viewBox="`0 0 ${L.width} ${heightOf(lane)}`" role="img" :aria-label="`${lane.title}: дорожка из ${lane.result.packets.length} пакетов по слотам времени`">
        <defs>
          <pattern :id="hatchId" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
            <line x1="0" y1="0" x2="0" y2="6" class="hatch-line" />
          </pattern>
        </defs>
        <g v-for="t in ticks" :key="t">
          <line :x1="x(t)" :y1="L.axisH - 3" :x2="x(t)" :y2="heightOf(lane)" class="grid" />
          <text :x="x(t) + box / 2" :y="L.axisH - 7" class="tick" :font-size="L.font" text-anchor="middle">{{ t }}</text>
        </g>
        <text v-for="c in rowsOf(lane)" :key="`l${c}`" :x="L.labelW - 6" :y="L.axisH + (c - 1) * L.rowH + 4 + box / 2" class="row-label" :font-size="L.font" text-anchor="end" dominant-baseline="central">{{ rowLabel(lane, c - 1) }}</text>
        <g v-for="p in lane.result.packets" :key="p.id" class="pkt" @click="emit('pick', p.stream, p.index)">
          <rect :x="x(p.sentAt)" :y="boxY(p)" :width="box" :height="box" rx="4" class="box" :style="{ '--c': streamColor(p.stream) }" />
          <rect v-if="waits(p)" :x="x(p.sentAt)" :y="boxY(p)" :width="box" :height="box" rx="4" :fill="`url(#${hatchId})`" class="hatch" />
          <text :x="x(p.sentAt) + box / 2" :y="boxY(p) + box / 2" class="pkt-n" :font-size="L.font" text-anchor="middle" dominant-baseline="central">{{ p.stream + 1 }}</text>
          <g v-if="p.lost" class="cross">
            <line :x1="x(p.sentAt) + 3" :y1="boxY(p) + 3" :x2="x(p.sentAt) + box - 3" :y2="boxY(p) + box - 3" />
            <line :x1="x(p.sentAt) + 3" :y1="boxY(p) + box - 3" :x2="x(p.sentAt) + box - 3" :y2="boxY(p) + 3" />
          </g>
        </g>
        <g v-for="p in lane.result.packets.filter((q) => q.lost)" :key="`r${p.id}`" class="retry">
          <path :d="retryPath(p)" />
          <polygon :points="`${x(p.arrivesAt) + box / 2},${boxY(p) + box + 1} ${x(p.arrivesAt) + box / 2 - 5},${boxY(p) + box + 9} ${x(p.arrivesAt) + box / 2 + 5},${boxY(p) + box + 9}`" />
        </g>
      </svg>
      <p class="axis-note">слоты времени; пакеты нарисованы там, где отправлены</p>
      <ul class="done" :aria-label="`Время готовности потоков, ${lane.title}`">
        <li v-for="(d, s) in lane.result.streamDone" :key="s">
          <span class="sw" :style="{ '--c': streamColor(s) }" aria-hidden="true">{{ s + 1 }}</span>
          поток {{ s + 1 }} готов в слоте <strong>{{ fmt(d) }}</strong><span class="late">{{ delta(lane, s) }}</span>
        </li>
      </ul>
    </section>
  </div>
</template>

<style scoped>
.lane { margin: 10px 0 0; padding-top: 8px; border-top: 1px solid var(--viz-edge); }
.lane-title { margin: 0 0 4px; font-size: 15px; color: var(--viz-text); }
.lane-svg { width: 100%; height: auto; display: block; }
.grid { stroke: var(--viz-edge); stroke-width: 1; }
.tick, .row-label { fill: var(--viz-muted); font-variant-numeric: tabular-nums; }
.pkt { cursor: pointer; }
.box { fill: color-mix(in srgb, var(--c) 28%, var(--viz-node-bg)); stroke: var(--c); stroke-width: 2; }
.hatch-line { stroke: var(--viz-text); stroke-width: 1.5; opacity: 0.55; }
.pkt-n { fill: var(--viz-text); font-weight: 700; pointer-events: none; }
.cross line { stroke: var(--viz-error); stroke-width: 3; stroke-linecap: round; }
.retry path { fill: none; stroke: var(--viz-error); stroke-width: 2.5; stroke-dasharray: 5 3; }
.retry polygon { fill: var(--viz-error); }
.axis-note { margin: 0 0 4px; font-size: 12px; color: var(--viz-muted); }
.legend, .done { list-style: none; margin: 8px 0 0; padding: 0; display: flex; flex-wrap: wrap; gap: 4px 14px; font-size: 13px; color: var(--viz-muted); }
.done { color: var(--viz-text); }
.legend li, .done li { margin: 0; display: inline-flex; align-items: center; gap: 5px; }
.late { color: var(--viz-error); font-weight: 600; margin-left: -3px; }
.sw { display: inline-flex; align-items: center; justify-content: center; width: 20px; height: 18px; border-radius: 4px; border: 2px solid var(--c, var(--viz-edge)); background: color-mix(in srgb, var(--c, transparent) 28%, var(--viz-node-bg)); color: var(--viz-text); font-size: 11px; font-weight: 700; }
.sw.lost { --c: var(--viz-error); color: var(--viz-error); }
.sw.hatch { background: repeating-linear-gradient(45deg, var(--viz-muted) 0 1.5px, transparent 1.5px 5px); }
</style>
