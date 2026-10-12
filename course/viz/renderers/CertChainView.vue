<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Cert, CertChainState, CertId, CertStatus } from '../simulations/cert-chain'
import { useNarrow } from '../components/useNarrow'

const props = defineProps<{ state: CertChainState }>()

const NARROW_PX = 520

interface Layout {
  width: number
  height: number
  card: { w: number; h: number }
  font: number
  small: number
  origin: (index: number) => { x: number; y: number }
}

// Порядок карточек — от корня к листу: так читается направление доверия.
const WIDE: Layout = {
  width: 810,
  height: 170,
  card: { w: 250, h: 150 },
  font: 18,
  small: 16,
  origin: (i) => ({ x: i * 280, y: 10 }),
}

const NARROW: Layout = {
  width: 400,
  height: 520,
  card: { w: 380, h: 140 },
  font: 19,
  small: 18,
  origin: (i) => ({ x: 10, y: 10 + i * 180 }),
}

const ROLE: Record<CertId, string> = { root: 'Корневой', intermediate: 'Промежуточный', leaf: 'Лист' }
const STATUS_TEXT: Record<CertStatus, string> = { pending: '○ ждёт проверки', ok: '✓ проверен', fail: '✗ не прошёл' }

const svgEl = ref<SVGSVGElement>()
const narrow = useNarrow(svgEl, NARROW_PX)
const layout = computed(() => (narrow.value ? NARROW : WIDE))

const statusText = (cert: Cert): string => (cert.id === 'root' && cert.status === 'fail' ? '✗ нет в хранилище ОС' : STATUS_TEXT[cert.status])

const arrows = computed(() => {
  const { card, origin } = layout.value
  return props.state.certs.slice(1).map((_, i) => {
    const a = origin(i)
    const b = origin(i + 1)
    return narrow.value
      ? { x1: a.x + card.w / 2, y1: a.y + card.h, x2: b.x + card.w / 2, y2: b.y }
      : { x1: a.x + card.w, y1: a.y + card.h / 2, x2: b.x, y2: b.y + card.h / 2 }
  })
})
</script>

<template>
  <div class="cc">
    <svg ref="svgEl" :viewBox="`0 0 ${layout.width} ${layout.height}`" role="img" aria-label="Схема: корневой, промежуточный и листовой сертификаты и результат их проверки">
      <defs>
        <marker id="cc-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto">
          <path d="M0,0 L10,5 L0,10 z" class="arrow-head" />
        </marker>
      </defs>
      <line v-for="(a, i) in arrows" :key="i" v-bind="a" class="arrow" marker-end="url(#cc-arrow)" />
      <g v-for="(cert, i) in state.certs" :key="cert.id" :transform="`translate(${layout.origin(i).x}, ${layout.origin(i).y})`">
        <rect :width="layout.card.w" :height="layout.card.h" rx="10" class="card" :class="cert.status" />
        <text x="12" :y="layout.font + 8" class="role" :font-size="layout.font">{{ ROLE[cert.id] }}</text>
        <text x="12" :y="layout.font + 8 + layout.font + 8" class="subject" :font-size="layout.font">{{ cert.subject }}</text>
        <text x="12" :y="layout.font + 8 + 2 * (layout.font + 8)" class="issuer" :font-size="layout.small">выдан: {{ cert.id === 'root' ? 'сам себе' : cert.issuer }}</text>
        <text x="12" :y="layout.font + 8 + 3 * (layout.font + 8) + 6" class="status" :font-size="layout.font">{{ statusText(cert) }}</text>
      </g>
    </svg>
    <p class="check">{{ state.check }}</p>
  </div>
</template>

<style scoped>
.cc svg { width: 100%; height: auto; display: block; }
.card { fill: var(--viz-node-bg); stroke-width: 3; transition: stroke 0.2s; }
.card.pending { stroke: var(--viz-edge); stroke-dasharray: 8 5; }
.card.ok { stroke: var(--viz-response); }
.card.fail { stroke: var(--viz-error); stroke-width: 5; }
.role { fill: var(--viz-muted); font-weight: 600; }
.subject { fill: var(--viz-text); font-weight: 600; }
.issuer { fill: var(--viz-muted); }
.status { fill: var(--viz-text); font-weight: 600; }
.arrow { stroke: var(--viz-muted); stroke-width: 3; }
.arrow-head { fill: var(--viz-muted); }
.check { margin: 0.5rem 0 0; font-size: 0.95rem; font-weight: 600; color: var(--vp-c-text-1); }
@media (prefers-reduced-motion: reduce) { .card { transition: none; } }
</style>
