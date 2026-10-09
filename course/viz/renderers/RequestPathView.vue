<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { gsap } from 'gsap'
import { NODES, type NodeId, type NodeKind, type RequestPathState } from '../simulations/request-path'
import { formatMs } from '../core/format'
import { prefersReducedMotion } from '../core/motion'

const props = defineProps<{ state: RequestPathState }>()

const W = 170
const H = 56
const POS: Record<NodeId, { x: number; y: number }> = {
  client: { x: 90, y: 230 },
  dns: { x: 90, y: 80 },
  lb: { x: 300, y: 230 },
  app: { x: 510, y: 230 },
  db: { x: 720, y: 230 },
}
const EDGES: readonly [NodeId, NodeId][] = [['client', 'dns'], ['client', 'lb'], ['lb', 'app'], ['app', 'db']]
const KIND_COLOR: Record<NodeKind, string> = {
  client: 'var(--viz-client)',
  network: 'var(--viz-network)',
  service: 'var(--viz-service)',
  db: 'var(--viz-db)',
}

const packetEl = ref<SVGCircleElement>()

function animatePacket(): void {
  const el = packetEl.value
  const packet = props.state.packet
  if (!el || !packet) return
  const from = POS[packet.from]
  const to = POS[packet.to]
  // Быстрые клики не должны накапливать анимации: прерываем предыдущую.
  gsap.killTweensOf(el)
  if (prefersReducedMotion()) {
    gsap.set(el, { attr: { cx: to.x, cy: to.y } })
    return
  }
  gsap.fromTo(el, { attr: { cx: from.x, cy: from.y } }, { attr: { cx: to.x, cy: to.y }, duration: 0.7, ease: 'power1.inOut' })
}

onMounted(animatePacket)
watch(() => props.state, () => requestAnimationFrame(animatePacket))
onBeforeUnmount(() => {
  if (packetEl.value) gsap.killTweensOf(packetEl.value)
})
</script>

<template>
  <svg class="rp" viewBox="0 0 810 300" role="img" aria-label="Схема: телефон, DNS, балансировщик, Go-сервис, PostgreSQL">
    <line v-for="[a, b] in EDGES" :key="`${a}-${b}`" :x1="POS[a].x" :y1="POS[a].y" :x2="POS[b].x" :y2="POS[b].y" class="edge" />
    <g v-for="node in NODES" :key="node.id" :transform="`translate(${POS[node.id].x - W / 2}, ${POS[node.id].y - H / 2})`">
      <rect :width="W" :height="H" rx="10" class="node" :class="{ active: state.active === node.id }" :style="{ stroke: state.active === node.id ? 'var(--viz-active)' : KIND_COLOR[node.kind] }" />
      <text :x="W / 2" :y="H / 2" class="label">{{ node.label }}</text>
    </g>
    <circle v-if="state.packet" ref="packetEl" r="11" :cx="POS[state.packet.to].x" :cy="POS[state.packet.to].y" class="packet" :class="state.packet.kind" />
    <text x="800" y="36" class="clock">⏱ {{ formatMs(state.elapsedMs) }}</text>
  </svg>
</template>

<style scoped>
.rp { width: 100%; height: auto; display: block; }
.edge { stroke: var(--viz-edge); stroke-width: 3; }
.node { fill: var(--viz-node-bg); stroke-width: 3; transition: stroke 0.2s, stroke-width 0.2s; }
.node.active { stroke-width: 6; }
.label { fill: var(--viz-text); font-size: 20px; text-anchor: middle; dominant-baseline: central; }
.packet.request { fill: var(--viz-request); }
.packet.response { fill: var(--viz-response); }
.clock { fill: var(--viz-text); font-size: 26px; font-weight: 600; text-anchor: end; font-variant-numeric: tabular-nums; }
</style>
