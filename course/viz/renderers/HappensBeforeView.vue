<script setup lang="ts">
import { computed } from 'vue'
import type { HbEvent, HbFrame } from '../simulations/happens-before'

const props = defineProps<{ state: HbFrame }>()

// Геометрия: viewBox 360 — подписи ≥ 12 px на экране 375 px.
const W = 360
const ROW = 46
const TOP = 34
const COL = [90, 270] as const
const BOX_W = 150
const BOX_H = 30

const rows = computed(() => {
  const all = [...props.state.g1, ...props.state.g2].map((e) => e.t).sort((a, b) => a - b)
  return new Map(all.map((t, i) => [t, i]))
})
interface Placed { e: HbEvent; x: number; y: number }
const place = (list: HbEvent[], col: 0 | 1): Placed[] => list.map((e) => ({ e, x: COL[col], y: TOP + (rows.value.get(e.t) ?? 0) * ROW }))
const g1 = computed(() => place(props.state.g1, 0))
const g2 = computed(() => place(props.state.g2, 1))
const byId = computed(() => new Map([...g1.value, ...g2.value].map((p) => [p.e.id, p])))
const height = computed(() => TOP + Math.max(rows.value.size, 4) * ROW)
const edges = computed(() =>
  props.state.edges.flatMap((e) => {
    const a = byId.value.get(e.from)
    const b = byId.value.get(e.to)
    return a && b ? [{ key: `${e.from}-${e.to}`, x1: a.x + BOX_W / 2, y1: a.y, x2: b.x - BOX_W / 2, y2: b.y }] : []
  }),
)
const programOrder = (list: Placed[]) => list.slice(1).map((p, i) => ({ key: p.e.id, x: p.x, y1: list[i]!.y + BOX_H / 2, y2: p.y - BOX_H / 2 }))
const order1 = computed(() => programOrder(g1.value))
const order2 = computed(() => programOrder(g2.value))

const badge = computed(() => {
  if (props.state.raceReport) return { cls: 'bad', text: '✕ гонка данных: WARNING: DATA RACE — результат не определён' }
  if (props.state.outcome === 'guaranteed') return { cls: 'ok', text: '✓ гонки нет: чтение гарантированно видит запись' }
  return { cls: 'mid', text: '… цепочка строится' }
})
const outcomeText = computed(() => (props.state.outcome === 'any' ? 'любой: 42, 0 или рваное значение' : props.state.outcome === 'guaranteed' ? 'гарантирован: 42' : 'пока не определён'))
</script>

<template>
  <div class="hb">
    <p class="badge" :class="badge.cls" role="status">{{ badge.text }}</p>
    <svg :viewBox="`0 0 ${W} ${height}`" role="img" :aria-label="`Две вертикальные шкалы времени: G1 и G2, рёбер синхронизации ${edges.length}`">
      <defs>
        <marker id="hb-arrow-order" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0 L10 5 L0 10 z" class="head-order" /></marker>
        <marker id="hb-arrow-edge" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0 L10 5 L0 10 z" class="head-edge" /></marker>
      </defs>
      <text :x="COL[0]" y="16" text-anchor="middle" class="title">G1</text>
      <text :x="COL[1]" y="16" text-anchor="middle" class="title">G2</text>
      <line v-for="(c, i) in COL" :key="i" :x1="c" y1="24" :x2="c" :y2="height - 6" class="axis" />
      <line v-for="o in order1" :key="o.key" :x1="o.x" :y1="o.y1" :x2="o.x" :y2="o.y2" class="order" marker-end="url(#hb-arrow-order)" />
      <line v-for="o in order2" :key="o.key" :x1="o.x" :y1="o.y1" :x2="o.x" :y2="o.y2" class="order" marker-end="url(#hb-arrow-order)" />
      <line v-for="e in edges" :key="e.key" :x1="e.x1" :y1="e.y1" :x2="e.x2" :y2="e.y2" class="edge" marker-end="url(#hb-arrow-edge)" />
      <g v-for="p in [...g1, ...g2]" :key="p.e.id">
        <rect :x="p.x - BOX_W / 2" :y="p.y - BOX_H / 2" :width="BOX_W" :height="BOX_H" rx="6" class="ev" />
        <text :x="p.x" :y="p.y + 5" text-anchor="middle" class="op">{{ p.e.op }}</text>
      </g>
    </svg>
    <ul class="legend">
      <li><span class="sw order-sw" aria-hidden="true"></span> порядок внутри одной горутины</li>
      <li><span class="sw edge-sw" aria-hidden="true"></span> синхронизация между горутинами</li>
      <li>Результат чтения data: <strong>{{ outcomeText }}</strong></li>
    </ul>
  </div>
</template>

<style scoped>
.hb { padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
.badge { margin: 0 0 8px; padding: 6px 10px; border: 2px solid var(--viz-edge); border-left-width: 8px; border-radius: 8px; font-size: 14px; font-weight: 700; }
.badge.ok { border-left-color: var(--viz-response); }
.badge.bad { border-left-color: var(--viz-error); }
.badge.mid { border-left-color: var(--viz-active); }
svg { display: block; width: 100%; height: auto; }
.title { font-size: 15px; font-weight: 700; fill: var(--viz-text); }
.axis { stroke: var(--viz-edge); stroke-width: 2; stroke-dasharray: 4 4; }
.order { stroke: var(--viz-muted); stroke-width: 2; }
.edge { stroke: var(--viz-response); stroke-width: 3; }
.head-order { fill: var(--viz-muted); }
.head-edge { fill: var(--viz-response); }
.ev { fill: var(--viz-node-bg); stroke: var(--viz-service); stroke-width: 2; }
.op { font-size: 12px; font-family: ui-monospace, monospace; fill: var(--viz-text); }
.legend { margin: 8px 0 0; padding: 0; list-style: none; font-size: 13px; color: var(--viz-muted); }
.legend li { margin-top: 2px; }
.sw { display: inline-block; width: 26px; margin-right: 6px; vertical-align: middle; border-top: 2px solid var(--viz-muted); }
.edge-sw { border-top: 3px solid var(--viz-response); }
</style>
