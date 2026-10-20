<script setup lang="ts">
import { computed } from 'vue'
import { blackToWhite, reachable, type NodeId, type ObjId, type TricolorFrame } from '../simulations/tricolor'

const props = defineProps<{ state: TricolorFrame }>()

const W = 360
const H = 300
const R = 22
const POS: Record<NodeId, { x: number; y: number }> = {
  ROOTS: { x: 180, y: 28 },
  A: { x: 100, y: 100 },
  B: { x: 260, y: 100 },
  C: { x: 40, y: 180 },
  F: { x: 180, y: 180 },
  D: { x: 310, y: 180 },
  X: { x: 50, y: 262 },
  G: { x: 180, y: 262 },
  E: { x: 310, y: 262 },
}

// Отрезок между краями узлов, а не центрами: стрелка не прячется под кругом.
function segment(from: NodeId, to: NodeId): { x1: number; y1: number; x2: number; y2: number } {
  const a = POS[from]
  const b = POS[to]
  const len = Math.hypot(b.x - a.x, b.y - a.y)
  const ux = (b.x - a.x) / len
  const uy = (b.y - a.y) / len
  const startR = from === 'ROOTS' ? 16 : R
  return { x1: a.x + ux * startR, y1: a.y + uy * startR, x2: b.x - ux * (R + 4), y2: b.y - uy * (R + 4) }
}

const live = computed(() => reachable(props.state))
const violations = computed(() => blackToWhite(props.state))
const isViolation = (from: NodeId, to: ObjId): boolean => violations.value.some((e) => e.from === from && e.to === to)
const isNew = (from: NodeId, to: ObjId): boolean => props.state.write?.from === from && props.state.write.to === to

const edges = computed(() =>
  props.state.edges.map((e) => ({
    key: `${e.from}>${e.to}`,
    cls: isViolation(e.from, e.to) ? 'bad' : isNew(e.from, e.to) ? 'new' : 'ok',
    ...segment(e.from, e.to),
  })),
)
// Стёртый мутатором указатель показываем пунктиром в кадре, где его стёрли.
const erased = computed(() => {
  const w = props.state.write
  return w !== null && w.to === null && w.old !== null ? { key: 'erased', ...segment(w.from, w.old) } : null
})

const COLOR_TEXT = { white: 'белый', grey: 'серый', black: 'чёрный' } as const
const nodes = computed(() =>
  props.state.objects.map((o) => {
    const freedLive = o.freed && live.value.has(o.id)
    const note = freedLive ? 'освобождён живой' : o.freed ? 'освобождён' : props.state.write?.shaded.includes(o.id) ? 'затенён барьером' : ''
    return { ...o, ...POS[o.id], freedLive, note, scanning: props.state.scanning === o.id, label: `${o.id}: ${COLOR_TEXT[o.color]}${note ? `, ${note}` : ''}` }
  }),
)
const greys = computed(() => props.state.objects.filter((o) => o.color === 'grey').map((o) => o.id))
const PHASE_TEXT = { off: 'до разметки', mark: 'разметка идёт параллельно с программой', sweep: 'разметка завершена, уборка' } as const
const summary = computed(() => nodes.value.map((n) => n.label).join('; '))
</script>

<template>
  <div class="tc">
    <p class="phase">Фаза: <strong>{{ PHASE_TEXT[state.phase] }}</strong> · барьер записи: <strong :class="state.barrier ? 'ok' : 'bad'">{{ state.barrier ? 'включён' : 'выключен' }}</strong></p>
    <svg :viewBox="`0 0 ${W} ${H}`" role="img" :aria-label="`Граф объектов. ${summary}`">
      <defs>
        <marker v-for="m in ['ok', 'new', 'bad', 'gone']" :id="`tc-arrow-${m}`" :key="m" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
          <path d="M0 0 L10 5 L0 10 z" :class="`arrow-${m}`" />
        </marker>
      </defs>
      <line v-for="e in edges" :key="e.key" :x1="e.x1" :y1="e.y1" :x2="e.x2" :y2="e.y2" :class="['edge', e.cls]" :marker-end="`url(#tc-arrow-${e.cls})`" />
      <line v-if="erased" :x1="erased.x1" :y1="erased.y1" :x2="erased.x2" :y2="erased.y2" class="edge gone" marker-end="url(#tc-arrow-gone)" />
      <g>
        <rect :x="POS.ROOTS.x - 46" :y="POS.ROOTS.y - 16" width="92" height="32" rx="8" class="roots" />
        <text :x="POS.ROOTS.x" :y="POS.ROOTS.y + 5" text-anchor="middle" font-size="15" class="roots-lbl">корни</text>
      </g>
      <g v-for="n in nodes" :key="n.id">
        <circle v-if="n.scanning" :cx="n.x" :cy="n.y" :r="R + 6" class="scan" />
        <circle :cx="n.x" :cy="n.y" :r="R" :class="['node', n.color, { freed: n.freed, 'freed-live': n.freedLive }]" />
        <text :x="n.x" :y="n.y + 6" text-anchor="middle" font-size="17" :class="['id', n.color]">{{ n.freed ? '✕' : n.id }}</text>
        <text v-if="n.note" :x="n.x" :y="n.y + R + 15" text-anchor="middle" font-size="13" :class="['note', { bad: n.freedLive }]">{{ n.freed ? `${n.id} ${n.note}` : n.note }}</text>
      </g>
    </svg>
    <ul class="legend">
      <li><span class="sw white" aria-hidden="true"></span>белый — не найден</li>
      <li><span class="sw grey" aria-hidden="true"></span>серый — найден, поля не просмотрены</li>
      <li><span class="sw black" aria-hidden="true"></span>чёрный — просмотрен</li>
    </ul>
    <div class="cards">
      <div class="card"><h4>Очередь сборщика (серые)</h4><p>{{ greys.length > 0 ? greys.join(', ') : 'пусто' }}</p></div>
      <div class="card" :class="{ bad: violations.length > 0, ok: violations.length === 0 }">
        <h4>Чёрный → белый</h4>
        <p>{{ violations.length > 0 ? violations.map((e) => `${e.from} → ${e.to}`).join(', ') + ' — инвариант нарушен' : 'нет, инвариант соблюдён' }}</p>
      </div>
      <div v-if="state.write" class="card mid">
        <h4>Запись мутатора</h4>
        <p>{{ state.write.from }}.{{ state.write.field }} = {{ state.write.to ?? 'nil' }}{{ state.write.shaded.length > 0 ? ` · барьер затенил ${state.write.shaded.join(', ')}` : state.barrier ? '' : ' · барьера нет' }}</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tc { padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
.phase { margin: 0 0 6px; font-size: 14px; }
.phase .ok { color: var(--viz-response); }
.phase .bad { color: var(--viz-error); }
svg { display: block; width: 100%; max-width: 480px; height: auto; margin: 0 auto; }
.edge { stroke-width: 2.5; }
.edge.ok { stroke: var(--viz-muted); }
.edge.new { stroke: var(--viz-client); stroke-width: 3; }
.edge.bad { stroke: var(--viz-error); stroke-width: 3.5; }
.edge.gone { stroke: var(--viz-error); stroke-dasharray: 4 4; opacity: 0.7; }
.arrow-ok { fill: var(--viz-muted); }
.arrow-new { fill: var(--viz-client); }
.arrow-bad, .arrow-gone { fill: var(--viz-error); }
.roots { fill: var(--viz-node-bg); stroke: var(--viz-service); stroke-width: 2.5; }
.roots-lbl { fill: var(--viz-text); font-weight: 600; }
.node { stroke-width: 2.5; }
.node.white { fill: var(--viz-node-bg); stroke: var(--viz-muted); stroke-dasharray: 5 3; }
.node.grey { fill: var(--viz-active-line-bg); stroke: var(--viz-active); stroke-width: 3.5; }
.node.black { fill: var(--viz-text); stroke: var(--viz-text); }
.node.freed { fill: var(--viz-node-bg); stroke: var(--viz-muted); stroke-dasharray: 2 3; }
.node.freed-live { stroke: var(--viz-error); stroke-width: 4; stroke-dasharray: none; }
.scan { fill: none; stroke: var(--viz-client); stroke-width: 3; }
.id { font-weight: 700; fill: var(--viz-text); }
.id.black { fill: var(--viz-node-bg); }
.note { fill: var(--viz-muted); }
.note.bad { fill: var(--viz-error); font-weight: 700; }
.legend { display: flex; flex-wrap: wrap; gap: 4px 14px; margin: 6px 0 8px; padding: 0; list-style: none; font-size: 13px; color: var(--viz-muted); }
.legend li { display: flex; align-items: center; gap: 6px; }
.sw { display: inline-block; width: 16px; height: 16px; border-radius: 50%; border: 2px solid var(--viz-muted); }
.sw.white { background: var(--viz-node-bg); border-style: dashed; }
.sw.grey { background: var(--viz-active-line-bg); border-color: var(--viz-active); }
.sw.black { background: var(--viz-text); border-color: var(--viz-text); }
h4 { margin: 0; font-size: 13px; font-weight: 600; color: var(--viz-muted); }
.cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 160px), 1fr)); gap: 8px; }
.card { min-width: 0; padding: 6px 10px; border: 2px solid var(--viz-edge); border-left-width: 8px; border-radius: 8px; }
.card.ok { border-left-color: var(--viz-response); }
.card.bad { border-left-color: var(--viz-error); }
.card.mid { border-left-color: var(--viz-active); }
.card p { margin: 2px 0 0; font-size: 14px; font-weight: 700; overflow-wrap: anywhere; }
</style>
