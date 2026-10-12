<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { gsap } from 'gsap'
import { NODES, TTL_SECONDS, type DnsChainState, type NodeId, type Packet } from '../simulations/dns-chain'
import { formatNumber } from '../core/format'
import { prefersReducedMotion } from '../core/motion'
import { useNarrow } from '../components/useNarrow'

const props = defineProps<{ state: DnsChainState }>()

interface Point {
  x: number
  y: number
}

interface Layout {
  width: number
  height: number
  label: number
  node: { w: number; h: number }
  clock: Point
  clockFont: number
  cache: { x: number; y: number; w: number; h: number }
  pos: Record<NodeId, Point>
}

const R = 11
const NARROW_PX = 520

const WIDE: Layout = {
  width: 810,
  height: 340,
  label: 20,
  node: { w: 160, h: 60 },
  clock: { x: 800, y: 36 },
  clockFont: 26,
  cache: { x: 10, y: 230, w: 450, h: 100 },
  pos: {
    stub: { x: 90, y: 130 },
    recursive: { x: 330, y: 130 },
    root: { x: 560, y: 50 },
    tld: { x: 560, y: 160 },
    auth: { x: 560, y: 270 },
  },
}

// Вертикальная схема: клиент и резолвер слева, три сервера имён справа.
const NARROW: Layout = {
  width: 400,
  height: 600,
  label: 20,
  node: { w: 170, h: 56 },
  clock: { x: 390, y: 32 },
  clockFont: 24,
  cache: { x: 10, y: 440, w: 380, h: 140 },
  pos: {
    stub: { x: 100, y: 120 },
    recursive: { x: 100, y: 260 },
    root: { x: 300, y: 110 },
    tld: { x: 300, y: 240 },
    auth: { x: 300, y: 370 },
  },
}

const EDGES: readonly [NodeId, NodeId][] = [['stub', 'recursive'], ['recursive', 'root'], ['recursive', 'tld'], ['recursive', 'auth']]
const KIND_LABEL: Record<Packet['kind'], string> = { query: 'запрос', referral: 'реферал', answer: 'ответ' }

const svgEl = ref<SVGSVGElement>()
const packetEl = ref<SVGCircleElement>()
const narrow = useNarrow(svgEl, NARROW_PX)
const layout = computed(() => (narrow.value ? NARROW : WIDE))
const pos = (id: NodeId): Point => layout.value.pos[id]

// Точка на границе прямоугольника узла `at` по направлению к `toward`, плюс радиус пакета:
// пакет останавливается у края и не закрывает подпись.
function edgePoint(at: NodeId, toward: NodeId): Point {
  const a = pos(at)
  const b = pos(toward)
  const dx = b.x - a.x
  const dy = b.y - a.y
  const { w, h } = layout.value.node
  const t = Math.min(dx === 0 ? Infinity : w / 2 / Math.abs(dx), dy === 0 ? Infinity : h / 2 / Math.abs(dy))
  const len = Math.hypot(dx, dy)
  const reach = t * len + R
  return { x: a.x + (dx / len) * reach, y: a.y + (dy / len) * reach }
}

const packetEnd = computed(() => {
  const p = props.state.packet
  return p ? edgePoint(p.to, p.from) : null
})

const clockText = computed(() => `⏱ ${formatNumber(props.state.elapsedMs, 0)} мс`)
const kindText = computed(() => (props.state.packet ? KIND_LABEL[props.state.packet.kind] : ''))

function animatePacket(): void {
  const el = packetEl.value
  const packet = props.state.packet
  if (!el || !packet) return
  const from = pos(packet.from)
  const to = edgePoint(packet.to, packet.from)
  gsap.killTweensOf(el)
  if (prefersReducedMotion()) {
    gsap.set(el, { attr: { cx: to.x, cy: to.y } })
    return
  }
  gsap.fromTo(el, { attr: { cx: from.x, cy: from.y } }, { attr: { cx: to.x, cy: to.y }, duration: 0.7, ease: 'power1.inOut' })
}

function snapPacket(): void {
  const el = packetEl.value
  const end = packetEnd.value
  if (!el || !end) return
  gsap.killTweensOf(el)
  gsap.set(el, { attr: { cx: end.x, cy: end.y } })
}

onMounted(animatePacket)
watch(() => props.state, animatePacket, { flush: 'post' })
// При смене раскладки старые координаты пакета теряют смысл.
watch(narrow, snapPacket, { flush: 'post' })
onBeforeUnmount(() => {
  if (packetEl.value) gsap.killTweensOf(packetEl.value)
})
</script>

<template>
  <svg ref="svgEl" class="dns" :viewBox="`0 0 ${layout.width} ${layout.height}`" role="img" aria-label="Схема: телефон, резолвер, корневой сервер, сервер зоны .example и авторитетный сервер имён">
    <line v-for="[a, b] in EDGES" :key="`${a}-${b}`" :x1="pos(a).x" :y1="pos(a).y" :x2="pos(b).x" :y2="pos(b).y" class="edge" />
    <g v-for="node in NODES" :key="node.id" :transform="`translate(${pos(node.id).x - layout.node.w / 2}, ${pos(node.id).y - layout.node.h / 2})`">
      <rect :width="layout.node.w" :height="layout.node.h" rx="10" class="node" :class="{ active: state.active === node.id }" />
      <text :x="layout.node.w / 2" :y="layout.node.h / 2" class="label" :font-size="layout.label">{{ node.label }}</text>
    </g>
    <g>
      <rect :x="layout.cache.x" :y="layout.cache.y" :width="layout.cache.w" :height="layout.cache.h" rx="10" class="cache" />
      <text :x="layout.cache.x + 14" :y="layout.cache.y + 30" class="cache-title" :font-size="layout.label">Кэш резолвера</text>
      <template v-if="state.cache.length > 0">
        <text v-for="(entry, i) in state.cache" :key="entry" :x="layout.cache.x + 14" :y="layout.cache.y + 62 + i * 30" class="cache-entry" :font-size="layout.label - 2">{{ entry }}</text>
        <text :x="layout.cache.x + 14" :y="layout.cache.y + 62 + state.cache.length * 30" class="cache-ttl" :font-size="layout.label - 2">TTL {{ TTL_SECONDS }}&nbsp;с</text>
      </template>
      <text v-else :x="layout.cache.x + 14" :y="layout.cache.y + 62" class="cache-ttl" :font-size="layout.label - 2">пусто</text>
    </g>
    <circle v-if="state.packet && packetEnd" ref="packetEl" :r="R" :cx="packetEnd.x" :cy="packetEnd.y" class="packet" :class="state.packet.kind" />
    <text :x="layout.clock.x" :y="layout.clock.y" class="clock" :font-size="layout.clockFont">{{ clockText }}</text>
    <text v-if="kindText" :x="layout.clock.x" :y="layout.clock.y + 28" class="kind" :font-size="layout.label - 2">{{ kindText }}</text>
  </svg>
</template>

<style scoped>
.dns { width: 100%; height: auto; display: block; }
.edge { stroke: var(--viz-edge); stroke-width: 3; }
.node { fill: var(--viz-node-bg); stroke: var(--viz-network); stroke-width: 3; transition: stroke 0.2s, stroke-width 0.2s; }
.node.active { stroke: var(--viz-active); stroke-width: 6; }
.label { fill: var(--viz-text); text-anchor: middle; dominant-baseline: central; }
.cache { fill: color-mix(in srgb, var(--viz-cache) 10%, var(--viz-node-bg)); stroke: var(--viz-cache); stroke-width: 3; }
.cache-title { fill: var(--viz-text); font-weight: 600; }
.cache-entry { fill: var(--viz-text); }
.cache-ttl { fill: var(--viz-muted); }
.packet.query { fill: var(--viz-request); }
.packet.referral { fill: var(--viz-cache); }
.packet.answer { fill: var(--viz-response); }
.clock { fill: var(--viz-text); font-weight: 600; text-anchor: end; font-variant-numeric: tabular-nums; }
.kind { fill: var(--viz-muted); text-anchor: end; }
@media (prefers-reduced-motion: reduce) { .node { transition: none; } }
</style>
