<script setup lang="ts">
import { computed, ref } from 'vue'
import type { EncapsulationState, LayerId } from '../simulations/encapsulation'
import { PAYLOAD_BYTES, totalBytes } from '../simulations/encapsulation'
import { formatNumber } from '../core/format'
import { useNarrow } from '../components/useNarrow'

const props = defineProps<{ state: EncapsulationState }>()

const NARROW_PX = 520
const COLOR: Record<LayerId, string> = {
  http: 'var(--viz-service)',
  tls: 'var(--viz-broker)',
  tcp: 'var(--viz-client)',
  ip: 'var(--viz-cache)',
  eth: 'var(--viz-db)',
}

interface Geometry {
  width: number
  frameHeight: number
  stepX: number
  stepY: number
  stepBottom: number
  font: number
  totalY: number
  height: number
}

// stepY — высота полосы под подпись каждого вложенного конверта.
const WIDE: Geometry = { width: 810, frameHeight: 270, stepX: 36, stepY: 34, stepBottom: 8, font: 20, totalY: 310, height: 330 }
// Узкая раскладка: вертикальный стек — узкие отступы по бокам, подписи крупнее.
const NARROW: Geometry = { width: 400, frameHeight: 330, stepX: 14, stepY: 34, stepBottom: 8, font: 18, totalY: 370, height: 390 }

const svgEl = ref<SVGSVGElement>()
const narrow = useNarrow(svgEl, NARROW_PX)
const g = computed(() => (narrow.value ? NARROW : WIDE))

// Слои идут от внутреннего к внешнему; уровень 0 — самый внешний из имеющихся.
const frames = computed(() => {
  const layers = props.state.layers
  const geo = g.value
  return layers.map((layer, i) => {
    const level = layers.length - 1 - i
    const size = layer.headerBytes + layer.trailerBytes
    return {
      id: layer.id,
      x: level * geo.stepX,
      y: level * geo.stepY,
      w: geo.width - 2 * level * geo.stepX,
      h: geo.frameHeight - level * (geo.stepY + geo.stepBottom),
      text: layer.id === 'http' ? `${layer.label} · ${PAYLOAD_BYTES} Б` : `${layer.label} · +${size} Б`,
      active: i === props.state.active,
    }
  })
})

// Внешний конверт рисуется первым, иначе он закроет вложенные.
const framesOuterFirst = computed(() => [...frames.value].reverse())

const total = computed(() => totalBytes(props.state.layers))
const share = computed(() => Math.round((PAYLOAD_BYTES / total.value) * 100))
const totalText = computed(() => `Всего ${formatNumber(total.value)} Б, полезных ${share.value}%`)
</script>

<template>
  <svg ref="svgEl" class="enc" :viewBox="`0 0 ${g.width} ${g.height}`" role="img" aria-label="Схема: данные HTTP, завёрнутые по очереди в TLS, TCP, IP и Ethernet">
    <g v-for="f in framesOuterFirst" :key="f.id">
      <rect
        :x="f.x" :y="f.y" :width="f.w" :height="f.h" rx="10" class="frame"
        :class="{ active: f.active }"
        :style="{ '--c': COLOR[f.id] }"
      />
      <text :x="f.x + 14" :y="f.y + g.font + 4" class="label" :font-size="g.font">{{ f.text }}</text>
    </g>
    <text :x="g.width / 2" :y="g.totalY" class="total" :font-size="g.font + 2">{{ totalText }}</text>
  </svg>
</template>

<style scoped>
.enc { width: 100%; height: auto; display: block; }
.frame { stroke: var(--c); stroke-width: 3; fill: color-mix(in srgb, var(--c) 10%, var(--viz-node-bg)); }
.frame.active { stroke-width: 6; }
.label { fill: var(--viz-text); font-weight: 600; }
.total { fill: var(--viz-text); text-anchor: middle; font-weight: 600; font-variant-numeric: tabular-nums; }
</style>
