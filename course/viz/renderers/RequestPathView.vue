<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { gsap } from 'gsap'
import { NODES, type NodeId, type NodeKind, type RequestPathState } from '../simulations/request-path'
import { formatNumber } from '../core/format'
import { prefersReducedMotion } from '../core/motion'

const props = defineProps<{ state: RequestPathState }>()

interface Point {
  x: number
  y: number
}

interface Layout {
  width: number
  height: number
  label: number
  clock: Point
  pos: Record<NodeId, Point>
}

const W = 170
const H = 56
const R = 11
// Уже этой ширины горизонтальная схема сжимается так, что подписи не прочитать.
const NARROW_PX = 520

const WIDE: Layout = {
  width: 810,
  height: 300,
  label: 20,
  clock: { x: 800, y: 36 },
  pos: {
    client: { x: 90, y: 230 },
    dns: { x: 90, y: 80 },
    lb: { x: 300, y: 230 },
    app: { x: 510, y: 230 },
    db: { x: 720, y: 230 },
  },
}

// Вертикальная схема для телефона: тот же граф, узлы друг над другом.
const NARROW: Layout = {
  width: 400,
  height: 520,
  label: 22,
  clock: { x: 390, y: 36 },
  pos: {
    client: { x: 100, y: 470 },
    dns: { x: 300, y: 470 },
    lb: { x: 100, y: 340 },
    app: { x: 100, y: 210 },
    db: { x: 100, y: 80 },
  },
}

const EDGES: readonly [NodeId, NodeId][] = [['client', 'dns'], ['client', 'lb'], ['lb', 'app'], ['app', 'db']]
const KIND_COLOR: Record<NodeKind, string> = {
  client: 'var(--viz-client)',
  network: 'var(--viz-network)',
  service: 'var(--viz-service)',
  db: 'var(--viz-db)',
}

// SSR всегда рисует широкую схему; узкая включается только на клиенте после монтирования.
const narrow = ref(false)
const layout = computed(() => (narrow.value ? NARROW : WIDE))
const pos = (id: NodeId): Point => layout.value.pos[id]

// Точка у края узла `at`, обращённого к узлу `toward`: пакет не закрывает подпись.
function edgePoint(at: NodeId, toward: NodeId): Point {
  const a = pos(at)
  const b = pos(toward)
  if (a.y === b.y) return { x: a.x + Math.sign(b.x - a.x) * (W / 2 + R), y: a.y }
  return { x: a.x, y: a.y + Math.sign(b.y - a.y) * (H / 2 + R) }
}

const packetEnd = computed(() => {
  const p = props.state.packet
  return p ? edgePoint(p.to, p.from) : null
})

const clockText = computed(() => `⏱ ${formatNumber(props.state.elapsedMs, 1)}\u00A0мс`)

const svgEl = ref<SVGSVGElement>()
const packetEl = ref<SVGCircleElement>()

function animatePacket(): void {
  const el = packetEl.value
  const packet = props.state.packet
  if (!el || !packet) return
  // Старт из центра источника: между краями соседних узлов слишком короткий путь, полёт не заметен.
  const from = pos(packet.from)
  const to = edgePoint(packet.to, packet.from)
  // Быстрые клики не должны накапливать анимации: прерываем предыдущую.
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

let observer: ResizeObserver | undefined

onMounted(() => {
  const container = svgEl.value?.parentElement
  if (container) {
    observer = new ResizeObserver(([entry]) => {
      if (entry) narrow.value = entry.contentRect.width < NARROW_PX
    })
    observer.observe(container)
  }
  animatePacket()
})
watch(() => props.state, animatePacket, { flush: 'post' })
// При смене раскладки старые координаты пакета теряют смысл: ставим его сразу в конечную точку.
watch(narrow, snapPacket, { flush: 'post' })
onBeforeUnmount(() => {
  observer?.disconnect()
  if (packetEl.value) gsap.killTweensOf(packetEl.value)
})
</script>

<template>
  <svg
    ref="svgEl"
    class="rp"
    :viewBox="`0 0 ${layout.width} ${layout.height}`"
    role="img"
    aria-label="Схема: телефон, DNS, балансировщик, Go-сервис, PostgreSQL"
  >
    <line v-for="[a, b] in EDGES" :key="`${a}-${b}`" :x1="pos(a).x" :y1="pos(a).y" :x2="pos(b).x" :y2="pos(b).y" class="edge" />
    <g v-for="node in NODES" :key="node.id" :transform="`translate(${pos(node.id).x - W / 2}, ${pos(node.id).y - H / 2})`">
      <rect :width="W" :height="H" rx="10" class="node" :class="{ active: state.active === node.id }" :style="{ stroke: state.active === node.id ? 'var(--viz-active)' : KIND_COLOR[node.kind] }" />
      <text :x="W / 2" :y="H / 2" class="label" :font-size="layout.label">{{ node.label }}</text>
    </g>
    <circle v-if="state.packet && packetEnd" ref="packetEl" :r="R" :cx="packetEnd.x" :cy="packetEnd.y" class="packet" :class="state.packet.kind" />
    <text :x="layout.clock.x" :y="layout.clock.y" class="clock">{{ clockText }}</text>
  </svg>
</template>

<style scoped>
.rp { width: 100%; height: auto; display: block; }
.edge { stroke: var(--viz-edge); stroke-width: 3; }
.node { fill: var(--viz-node-bg); stroke-width: 3; transition: stroke 0.2s, stroke-width 0.2s; }
.node.active { stroke-width: 6; }
.label { fill: var(--viz-text); text-anchor: middle; dominant-baseline: central; }
.packet.request { fill: var(--viz-request); }
.packet.response { fill: var(--viz-response); }
@media (prefers-reduced-motion: reduce) { .node { transition: none; } }
.clock { fill: var(--viz-text); font-size: 26px; font-weight: 600; text-anchor: end; font-variant-numeric: tabular-nums; }
</style>
