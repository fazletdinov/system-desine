<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Lane, UdpTcpState } from '../simulations/udp-vs-tcp'
import { MESSAGES } from '../simulations/udp-vs-tcp'
import { useNarrow } from '../components/useNarrow'

const props = defineProps<{ state: UdpTcpState }>()

const NARROW_PX = 520

type CellKind = 'delivered' | 'buffered' | 'lost' | 'flight' | 'idle'

interface Layout {
  width: number
  height: number
  cols: number
  cell: { w: number; h: number; gap: number }
  left: number
  laneHeight: number
  font: number
  small: number
}

const WIDE: Layout = { width: 810, height: 340, cols: 6, cell: { w: 100, h: 70, gap: 12 }, left: 90, laneHeight: 150, font: 22, small: 16 }
// Узкая раскладка: по три сообщения в ряд, подписи не мельче 12 px на 375.
const NARROW: Layout = { width: 400, height: 560, cols: 3, cell: { w: 114, h: 64, gap: 12 }, left: 10, laneHeight: 250, font: 22, small: 18 }

const svgEl = ref<SVGSVGElement>()
const narrow = useNarrow(svgEl, NARROW_PX)
const layout = computed(() => (narrow.value ? NARROW : WIDE))

const CELL_TEXT: Record<CellKind, string> = { delivered: 'отдано', buffered: 'ждёт', lost: 'потеряно', flight: 'в пути', idle: '—' }
const CELL_MARK: Record<CellKind, string> = { delivered: '✓', buffered: '⏳', lost: '✗', flight: '', idle: '' }

function kindOf(lane: Lane, message: number): CellKind {
  if (lane.delivered.includes(message)) return 'delivered'
  if (lane.buffered.includes(message)) return 'buffered'
  if (lane.lost.includes(message)) return 'lost'
  return message - 1 <= props.state.tick ? 'flight' : 'idle'
}

const lanes = computed(() => {
  const l = layout.value
  return ([
    { key: 'tcp', title: 'TCP', lane: props.state.tcp },
    { key: 'udp', title: 'UDP', lane: props.state.udp },
  ] as const).map((entry, laneIndex) => {
    const top = laneIndex * l.laneHeight
    const cells = Array.from({ length: MESSAGES }, (_, i) => {
      const message = i + 1
      const col = i % l.cols
      const row = Math.floor(i / l.cols)
      const kind = kindOf(entry.lane, message)
      return {
        message,
        kind,
        x: l.left + col * (l.cell.w + l.cell.gap),
        y: top + 40 + row * (l.cell.h + l.cell.gap),
        text: `${CELL_MARK[kind]} ${CELL_TEXT[kind]}`.trim(),
      }
    })
    const rows = Math.ceil(MESSAGES / l.cols)
    const received = entry.lane.delivered.length > 0 ? entry.lane.delivered.join(', ') : 'ничего'
    return { ...entry, top, cells, summaryY: top + 40 + rows * (l.cell.h + l.cell.gap) + 18, received }
  })
})
</script>

<template>
  <svg ref="svgEl" class="ut" :viewBox="`0 0 ${layout.width} ${layout.height}`" role="img" aria-label="Схема: шесть сообщений по TCP и по UDP, третье потеряно">
    <g v-for="lane in lanes" :key="lane.key">
      <text :x="layout.left" :y="lane.top + 26" class="title" :font-size="layout.font">{{ lane.title }}</text>
      <g v-for="cell in lane.cells" :key="cell.message">
        <rect :x="cell.x" :y="cell.y" :width="layout.cell.w" :height="layout.cell.h" rx="8" class="cell" :class="cell.kind" />
        <text :x="cell.x + layout.cell.w / 2" :y="cell.y + 24" class="num" :font-size="layout.font">{{ cell.message }}</text>
        <text :x="cell.x + layout.cell.w / 2" :y="cell.y + layout.cell.h - 12" class="state" :font-size="layout.small">{{ cell.text }}</text>
      </g>
      <text :x="layout.left" :y="lane.summaryY" class="summary" :font-size="layout.small + 1">Приложение получило: {{ lane.received }}</text>
    </g>
    <text :x="layout.width - 10" :y="layout.height - 10" class="tick" :font-size="layout.small + 2">тик {{ state.tick }}</text>
  </svg>
</template>

<style scoped>
.ut { width: 100%; height: auto; display: block; }
.title { fill: var(--viz-text); font-weight: 700; }
.cell { fill: var(--viz-node-bg); stroke-width: 3; transition: stroke 0.2s; }
.cell.idle { stroke: var(--viz-edge); stroke-dasharray: 6 4; }
.cell.flight { stroke: var(--viz-request); }
.cell.delivered { stroke: var(--viz-response); }
.cell.buffered { stroke: var(--viz-cache); }
.cell.lost { stroke: var(--viz-error); stroke-width: 5; }
.num { fill: var(--viz-text); text-anchor: middle; font-weight: 700; }
.state { fill: var(--viz-text); text-anchor: middle; }
.summary { fill: var(--viz-text); }
.tick { fill: var(--viz-muted); text-anchor: end; font-variant-numeric: tabular-nums; }
@media (prefers-reduced-motion: reduce) { .cell { transition: none; } }
</style>
