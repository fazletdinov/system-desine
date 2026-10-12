<script setup lang="ts">
import { computed, ref } from 'vue'
import type { PoolResult, Span, SpanKind } from '../simulations/pool'
import { formatMs } from '../core/format'
import { useNarrow } from '../components/useNarrow'

const props = defineProps<{ result: PoolResult }>()

const MAX_ROWS = 40
const NARROW_PX = 520
const LABEL_MIN_PX = 24

interface Layout {
  width: number
  labelW: number
  rowH: number
  barH: number
  axisH: number
  font: number
}

const WIDE: Layout = { width: 810, labelW: 76, rowH: 22, barH: 16, axisH: 26, font: 14 }
const NARROW: Layout = { width: 360, labelW: 38, rowH: 18, barH: 13, axisH: 24, font: 11 }

const svgEl = ref<SVGSVGElement>()
const narrow = useNarrow(svgEl, NARROW_PX)
const L = computed(() => (narrow.value ? NARROW : WIDE))

const rows = computed(() => Math.min(props.result.connections, MAX_ROWS))
const hidden = computed(() => Math.max(0, props.result.connections - MAX_ROWS))
const plotW = computed(() => L.value.width - L.value.labelW - 8)
const height = computed(() => L.value.axisH + rows.value * L.value.rowH + 6)
const scale = computed(() => plotW.value / Math.max(1, props.result.totalMs))
const x = (ms: number): number => L.value.labelW + ms * scale.value

const KIND_TEXT: Record<SpanKind, string> = { tcp: 'TCP', tls: 'TLS', request: '' }

interface Bar {
  key: string
  kind: SpanKind
  x: number
  y: number
  w: number
  label: string
}

const bars = computed<Bar[]>(() =>
  props.result.spans
    .filter((s: Span) => s.conn < MAX_ROWS)
    .map((s, i) => {
      const w = (s.endMs - s.startMs) * scale.value
      // Подпись — только если отрезок достаточно широк в реальных пикселях (viewBox ≈ пиксели).
      const text = s.kind === 'request' ? `${s.request + 1}` : KIND_TEXT[s.kind]
      return { key: `${i}`, kind: s.kind, x: x(s.startMs), y: L.value.axisH + s.conn * L.value.rowH + (L.value.rowH - L.value.barH) / 2, w, label: w >= LABEL_MIN_PX ? text : '' }
    }),
)

// Шаг шкалы — 1, 2 или 5 × 10^k, чтобы меток было не больше ~6.
const ticks = computed<number[]>(() => {
  const raw = props.result.totalMs / 6
  const mag = 10 ** Math.floor(Math.log10(Math.max(raw, 1)))
  const step = [1, 2, 5, 10].map((m) => m * mag).find((s) => s >= raw) ?? mag * 10
  const out: number[] = []
  for (let t = 0; t <= props.result.totalMs; t += step) out.push(t)
  return out
})

const rowLabels = computed(() => Array.from({ length: rows.value }, (_, c) => ({ c, y: L.value.axisH + c * L.value.rowH + L.value.rowH / 2 })))
const tcpCount = computed(() => props.result.spans.filter((s) => s.kind === 'tcp').length)
const tlsCount = computed(() => props.result.spans.filter((s) => s.kind === 'tls').length)
const handshakes = computed(() => (tlsCount.value > 0 ? `${tcpCount.value} TCP + ${tlsCount.value} TLS` : `${tcpCount.value} TCP`))
</script>

<template>
  <div class="pool">
    <svg ref="svgEl" class="pool-svg" :viewBox="`0 0 ${L.width} ${height}`" role="img" :aria-label="`Диаграмма Ганта: ${result.connections} соединений, общее время ${formatMs(result.totalMs)}`">
      <g v-for="t in ticks" :key="t">
        <line :x1="x(t)" :y1="L.axisH - 4" :x2="x(t)" :y2="height" class="grid" />
        <text :x="x(t)" :y="L.axisH - 9" class="tick" :font-size="L.font" text-anchor="middle">{{ t }}</text>
      </g>
      <text :x="L.width - 4" :y="12" class="tick" :font-size="L.font" text-anchor="end">мс</text>
      <text v-for="r in rowLabels" :key="r.c" :x="L.labelW - 6" :y="r.y" class="row-label" :font-size="L.font" text-anchor="end" dominant-baseline="central">{{ narrow ? r.c + 1 : `соед. ${r.c + 1}` }}</text>
      <g v-for="b in bars" :key="b.key">
        <rect :x="b.x" :y="b.y" :width="Math.max(b.w - 1, 1)" :height="L.barH" rx="3" class="bar" :class="b.kind" />
        <text v-if="b.label" :x="b.x + b.w / 2" :y="b.y + L.barH / 2" class="bar-text" :font-size="L.font - 2" text-anchor="middle" dominant-baseline="central">{{ b.label }}</text>
      </g>
    </svg>
    <p v-if="hidden > 0" class="more">и ещё {{ hidden }} {{ hidden % 10 === 1 && hidden % 100 !== 11 ? 'соединение' : [2, 3, 4].includes(hidden % 10) && ![12, 13, 14].includes(hidden % 100) ? 'соединения' : 'соединений' }}</p>
    <ul class="legend" aria-label="Обозначения">
      <li><span class="sw tcp" aria-hidden="true">TCP</span> установка соединения</li>
      <li><span class="sw tls" aria-hidden="true">TLS</span> рукопожатие TLS</li>
      <li><span class="sw request" aria-hidden="true">№</span> запрос и ответ (номер запроса)</li>
    </ul>
    <dl class="totals">
      <div><dt>Общее время</dt><dd>{{ formatMs(result.totalMs) }}</dd></div>
      <div><dt>Соединений</dt><dd>{{ result.connections }}</dd></div>
      <div><dt>Рукопожатий</dt><dd>{{ handshakes }}</dd></div>
    </dl>
  </div>
</template>

<style scoped>
.pool-svg { width: 100%; height: auto; display: block; }
.grid { stroke: var(--viz-edge); stroke-width: 1; }
.tick, .row-label { fill: var(--viz-muted); font-variant-numeric: tabular-nums; }
.bar { stroke-width: 1.5; }
.bar.tcp { fill: color-mix(in srgb, var(--viz-network) 30%, var(--viz-node-bg)); stroke: var(--viz-network); }
.bar.tls { fill: color-mix(in srgb, var(--viz-service) 30%, var(--viz-node-bg)); stroke: var(--viz-service); }
.bar.request { fill: color-mix(in srgb, var(--viz-request) 30%, var(--viz-node-bg)); stroke: var(--viz-request); }
.bar-text { fill: var(--viz-text); font-weight: 600; font-variant-numeric: tabular-nums; }
.more { margin: 4px 0 0; font-size: 13px; color: var(--viz-muted); }
.legend { list-style: none; margin: 8px 0 0; padding: 0; display: flex; flex-wrap: wrap; gap: 4px 14px; font-size: 13px; color: var(--viz-muted); }
.legend li { margin: 0; display: inline-flex; align-items: center; gap: 5px; }
.sw { display: inline-flex; align-items: center; justify-content: center; min-width: 30px; height: 18px; padding: 0 3px; border-radius: 4px; border: 2px solid; color: var(--viz-text); font-size: 11px; font-weight: 600; }
.sw.tcp { border-color: var(--viz-network); background: color-mix(in srgb, var(--viz-network) 30%, var(--viz-node-bg)); }
.sw.tls { border-color: var(--viz-service); background: color-mix(in srgb, var(--viz-service) 30%, var(--viz-node-bg)); }
.sw.request { border-color: var(--viz-request); background: color-mix(in srgb, var(--viz-request) 30%, var(--viz-node-bg)); }
.totals { display: flex; flex-wrap: wrap; gap: 6px 24px; margin: 10px 0 0; }
.totals div { display: flex; gap: 6px; align-items: baseline; }
.totals dt { font-size: 13px; color: var(--viz-muted); }
.totals dt::after { content: ':'; }
.totals dd { margin: 0; font-weight: 700; color: var(--viz-text); font-variant-numeric: tabular-nums; }
</style>
