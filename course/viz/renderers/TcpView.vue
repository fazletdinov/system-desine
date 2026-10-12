<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Packet, TcpFrame } from '../simulations/tcp'
import { useNarrow } from '../components/useNarrow'

const props = defineProps<{ state: TcpFrame; segments: number }>()

interface Layout {
  width: number
  height: number
  cell: number
  cellGap: number
  stripX: number
  stripY: number
  cellH: number
  senderX: number
  receiverX: number
  headY: number
  arrowsTop: number
  rowH: number
  slant: number
  font: number
  small: number
}

const NARROW_PX = 520

const WIDE: Layout = {
  width: 810,
  height: 450,
  cell: 80,
  cellGap: 10,
  stripX: 35,
  stripY: 12,
  cellH: 58,
  senderX: 130,
  receiverX: 680,
  headY: 104,
  arrowsTop: 160,
  rowH: 26,
  slant: 18,
  font: 20,
  small: 17,
}

const NARROW: Layout = {
  width: 400,
  height: 450,
  cell: 41,
  cellGap: 3,
  stripX: 6,
  stripY: 12,
  cellH: 62,
  senderX: 56,
  receiverX: 344,
  headY: 106,
  arrowsTop: 160,
  rowH: 26,
  slant: 18,
  font: 17,
  small: 15,
}

const svgEl = ref<SVGSVGElement>()
const narrow = useNarrow(svgEl, NARROW_PX)
const L = computed(() => (narrow.value ? NARROW : WIDE))

type CellState = 'acked' | 'flight' | 'window' | 'outside'
const CELL_GLYPH: Record<CellState, string> = { acked: '✓', flight: '→', window: '○', outside: '' }

const cells = computed(() =>
  Array.from({ length: props.segments }, (_, n) => {
    let state: CellState = 'outside'
    if (n < props.state.base) state = 'acked'
    else if (n < props.state.nextSeq) state = 'flight'
    else if (n < props.state.base + props.state.window) state = 'window'
    return { n, state, x: L.value.stripX + n * (L.value.cell + L.value.cellGap) }
  }),
)

const windowFrame = computed(() => {
  const first = props.state.base
  const last = Math.min(props.state.base + props.state.window, props.segments) - 1
  if (last < first) return null
  const pad = 3
  const x = L.value.stripX + first * (L.value.cell + L.value.cellGap) - pad
  const w = (last - first + 1) * (L.value.cell + L.value.cellGap) - L.value.cellGap + pad * 2
  return { x, y: L.value.stripY - pad, w, h: L.value.cellH + pad * 2 }
})

interface Arrow {
  key: string
  packet: Packet
  x1: number
  y1: number
  x2: number
  y2: number
  mid: { x: number; y: number }
  head: string
  label: string
  labelX: number
  labelY: number
  anchor: 'start' | 'end'
}

function arrowHead(x2: number, y2: number, x1: number, y1: number): string {
  const len = 11
  const spread = 6
  const a = Math.atan2(y2 - y1, x2 - x1)
  const bx = x2 - Math.cos(a) * len
  const by = y2 - Math.sin(a) * len
  const px = -Math.sin(a) * spread
  const py = Math.cos(a) * spread
  return `${x2},${y2} ${bx + px},${by + py} ${bx - px},${by - py}`
}

const arrows = computed<Arrow[]>(() => {
  const l = L.value
  const ordered = [...props.state.sent.filter((p) => p.kind === 'segment'), ...props.state.sent.filter((p) => p.kind === 'ack')]
  const rowH = Math.min(l.rowH, (l.height - l.arrowsTop - l.slant - 8) / Math.max(ordered.length, 1))
  return ordered.map((packet, i) => {
    const forward = packet.kind === 'segment'
    const y1 = l.arrowsTop + i * rowH
    const y2 = y1 + l.slant
    const x1 = forward ? l.senderX : l.receiverX
    const x2 = forward ? l.receiverX : l.senderX
    return {
      key: `${packet.kind}-${packet.n}-${i}`,
      packet,
      x1,
      y1,
      x2,
      y2,
      mid: { x: (x1 + x2) / 2, y: (y1 + y2) / 2 },
      head: arrowHead(x2, y2, x1, y1),
      label: forward ? `#${packet.n}${packet.retransmit ? '↻' : ''}` : `ACK ${packet.n}`,
      labelX: forward ? x1 + 12 : x1 - 12,
      labelY: y1 - 3,
      anchor: forward ? 'start' : 'end',
    }
  })
})

const buffered = computed(() => props.state.received.filter((n) => n > props.state.expected))
const bufferText = computed(() => (buffered.value.length > 0 ? `буфер: ${buffered.value.join(', ')}` : 'буфер пуст'))
const lifelineBottom = computed(() => L.value.height - 6)
</script>

<template>
  <div class="tcp">
    <svg ref="svgEl" class="tcp-svg" :viewBox="`0 0 ${L.width} ${L.height}`" role="img" :aria-label="`Схема TCP, тик ${state.tick}: полоса сегментов, отправитель, получатель и пакеты между ними`">
      <g v-for="c in cells" :key="c.n">
        <rect :x="c.x" :y="L.stripY" :width="L.cell" :height="L.cellH" rx="6" class="cell" :class="c.state" />
        <text :x="c.x + L.cell / 2" :y="L.stripY + 22" class="cell-n" :font-size="L.font">{{ c.n }}</text>
        <text :x="c.x + L.cell / 2" :y="L.stripY + 46" class="cell-glyph" :font-size="L.font">{{ CELL_GLYPH[c.state] }}</text>
      </g>
      <rect v-if="windowFrame" :x="windowFrame.x" :y="windowFrame.y" :width="windowFrame.w" :height="windowFrame.h" rx="9" class="window-frame" />

      <line :x1="L.senderX" :y1="L.headY + 10" :x2="L.senderX" :y2="lifelineBottom" class="lifeline" />
      <line :x1="L.receiverX" :y1="L.headY + 10" :x2="L.receiverX" :y2="lifelineBottom" class="lifeline" />
      <text :x="L.senderX" :y="L.headY" class="head" :font-size="L.font">Отправитель</text>
      <text :x="L.receiverX" :y="L.headY" class="head" :font-size="L.font">Получатель</text>

      <g v-for="a in arrows" :key="a.key" class="arrow" :class="[a.packet.kind, { lost: a.packet.lost, retransmit: a.packet.retransmit }]">
        <line :x1="a.x1" :y1="a.y1" :x2="a.x2" :y2="a.y2" class="shaft" />
        <polygon :points="a.head" class="tip" />
        <text :x="a.labelX" :y="a.labelY" class="plabel" :text-anchor="a.anchor" :font-size="L.small">{{ a.label }}</text>
        <g v-if="a.packet.lost" class="cross">
          <line :x1="a.mid.x - 8" :y1="a.mid.y - 8" :x2="a.mid.x + 8" :y2="a.mid.y + 8" />
          <line :x1="a.mid.x - 8" :y1="a.mid.y + 8" :x2="a.mid.x + 8" :y2="a.mid.y - 8" />
        </g>
      </g>

      <text :x="narrow ? L.width - 6 : L.receiverX" :y="L.headY + 28" class="note" :text-anchor="narrow ? 'end' : 'middle'" :font-size="L.small">{{ bufferText }}</text>
      <text :x="narrow ? 6 : L.senderX" :y="L.headY + 28" class="note" :text-anchor="narrow ? 'start' : 'middle'" :font-size="L.small">дубли ACK: {{ state.dupAcks }}</text>
    </svg>
    <ul class="legend" aria-label="Обозначения">
      <li><span class="sw acked" aria-hidden="true">✓</span> подтверждён</li>
      <li><span class="sw flight" aria-hidden="true">→</span> в полёте</li>
      <li><span class="sw window" aria-hidden="true">○</span> в окне, ещё не отправлен</li>
      <li><span class="sw outside" aria-hidden="true"></span> вне окна</li>
      <li><span class="ln seg" aria-hidden="true"></span> сегмент</li>
      <li><span class="ln ack" aria-hidden="true"></span> ACK</li>
      <li><span class="ln lost" aria-hidden="true"></span> потерян (✕)</li>
      <li><span class="ln retr" aria-hidden="true"></span> повтор (↻)</li>
    </ul>
  </div>
</template>

<style scoped>
.tcp-svg { width: 100%; height: auto; display: block; }
.cell { stroke-width: 2; }
.cell.acked { fill: color-mix(in srgb, var(--viz-response) 22%, var(--viz-node-bg)); stroke: var(--viz-response); }
.cell.flight { fill: color-mix(in srgb, var(--viz-request) 22%, var(--viz-node-bg)); stroke: var(--viz-request); }
.cell.window { fill: var(--viz-node-bg); stroke: var(--viz-request); stroke-dasharray: 5 4; }
.cell.outside { fill: var(--viz-node-bg); stroke: var(--viz-edge); }
.cell-n, .cell-glyph { fill: var(--viz-text); text-anchor: middle; }
.cell-n { font-weight: 600; font-variant-numeric: tabular-nums; }
.window-frame { fill: none; stroke: var(--viz-active); stroke-width: 4; }
.lifeline { stroke: var(--viz-edge); stroke-width: 3; }
.head { fill: var(--viz-text); font-weight: 600; text-anchor: middle; }
.note { fill: var(--viz-muted); }
.shaft { stroke-width: 3; }
.arrow.segment .shaft { stroke: var(--viz-request); }
.arrow.segment .tip { fill: var(--viz-request); }
.arrow.ack .shaft { stroke: var(--viz-response); }
.arrow.ack .tip { fill: var(--viz-response); }
.arrow.lost .shaft { stroke: var(--viz-error); }
.arrow.lost .tip { fill: var(--viz-error); }
.arrow.retransmit .shaft { stroke-dasharray: 7 5; }
.plabel { fill: var(--viz-text); font-variant-numeric: tabular-nums; }
.cross line { stroke: var(--viz-error); stroke-width: 4; stroke-linecap: round; }
.legend { list-style: none; margin: 8px 0 0; padding: 0; display: flex; flex-wrap: wrap; gap: 4px 14px; font-size: 13px; color: var(--viz-muted); }
.legend li { margin: 0; display: inline-flex; align-items: center; gap: 5px; }
.sw { display: inline-flex; align-items: center; justify-content: center; width: 20px; height: 18px; border-radius: 4px; border: 2px solid var(--viz-edge); color: var(--viz-text); font-size: 11px; }
.sw.acked { border-color: var(--viz-response); }
.sw.flight { border-color: var(--viz-request); }
.sw.window { border-color: var(--viz-request); border-style: dashed; }
.ln { display: inline-block; width: 22px; height: 0; border-top: 3px solid var(--viz-request); }
.ln.ack { border-color: var(--viz-response); }
.ln.lost { border-color: var(--viz-error); }
.ln.retr { border-top-style: dashed; }
</style>
