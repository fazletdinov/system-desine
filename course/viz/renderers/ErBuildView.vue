<script setup lang="ts">
import { computed } from 'vue'
import type { EntityId, ErEntity, ErFrame, ErRelation, SchemaName } from '../simulations/er-build'

const props = defineProps<{ state: ErFrame }>()

// Геометрия: на экране 375 px SVG (с выносом на поля рамки, см. стиль svg) занимает около 285 px,
// поэтому кегль 13 при viewBox шириной 300 даёт не меньше 12 px (правило README). Ячейки — две
// колонки по 116, между ними 36: хватает на «воронью лапку» с обоих концов и подпись кардинальности.
const W = 300
const H = 540
const HEAD = 22
const ROW = 17
const PAD = 6
interface Box { x: number; y: number; w: number }
const POS: Record<EntityId, Box> = {
  users: { x: 16, y: 24, w: 116 },
  credentials: { x: 168, y: 24, w: 116 },
  members: { x: 16, y: 151, w: 116 },
  chats: { x: 168, y: 151, w: 116 },
  messages: { x: 168, y: 287, w: 116 },
  idempotency_keys: { x: 16, y: 431, w: 268 },
}
const heightOf = (e: ErEntity): number => HEAD + e.attrs.length * ROW + PAD

type End = 'one' | 'zero-one' | 'zero-many'
interface Pt { x: number; y: number }
interface Geometry { points: readonly Pt[]; parentDir: Pt; childDir: Pt; label: Pt; anchor: 'start' | 'middle' | 'end' }
// parentDir/childDir — направление от рамки сущности наружу, вдоль линии.
const GEOMETRY: Record<string, Geometry> = {
  'users-credentials': { points: [{ x: 132, y: 55 }, { x: 168, y: 55 }], parentDir: { x: 1, y: 0 }, childDir: { x: -1, y: 0 }, label: { x: 150, y: 47 }, anchor: 'middle' },
  // Линия правее середины колонки: левее неё на последнем шаге стоит подпись «схема chat».
  'users-members': { points: [{ x: 112, y: 103 }, { x: 112, y: 151 }], parentDir: { x: 0, y: 1 }, childDir: { x: 0, y: -1 }, label: { x: 104, y: 124 }, anchor: 'end' },
  'chats-members': { points: [{ x: 168, y: 182 }, { x: 132, y: 182 }], parentDir: { x: -1, y: 0 }, childDir: { x: 1, y: 0 }, label: { x: 150, y: 174 }, anchor: 'middle' },
  'chats-messages': { points: [{ x: 224, y: 247 }, { x: 224, y: 287 }], parentDir: { x: 0, y: 1 }, childDir: { x: 0, y: -1 }, label: { x: 234, y: 271 }, anchor: 'start' },
  'users-messages': { points: [{ x: 16, y: 90 }, { x: 3, y: 90 }, { x: 3, y: 335 }, { x: 168, y: 335 }], parentDir: { x: -1, y: 0 }, childDir: { x: -1, y: 0 }, label: { x: 16, y: 328 }, anchor: 'start' },
}

// «Воронья лапка»: черта — «один», кружок — «может не быть», три зубца — «много».
function endMarks(p: Pt, d: Pt, end: End): { lines: string[]; circle: Pt | null } {
  const n = { x: -d.y, y: d.x }
  const at = (k: number, s = 0): Pt => ({ x: p.x + d.x * k + n.x * s, y: p.y + d.y * k + n.y * s })
  const seg = (a: Pt, b: Pt): string => `M${a.x} ${a.y}L${b.x} ${b.y}`
  const bar = (k: number): string => seg(at(k, -6), at(k, 6))
  if (end === 'one') return { lines: [bar(6), bar(10)], circle: null }
  if (end === 'zero-one') return { lines: [bar(6)], circle: at(14) }
  return { lines: [seg(at(12), at(0, -6)), seg(at(12), at(0)), seg(at(12), at(0, 6))], circle: at(16) }
}

const fresh = computed(() => new Set(props.state.fresh))
const entities = computed(() => props.state.entities.map((e) => ({ e, box: POS[e.id], h: heightOf(e) })))
const relations = computed(() =>
  props.state.relations.map((r) => {
    const g = GEOMETRY[r.id]!
    const parent = endMarks(g.points[0]!, g.parentDir, 'one')
    const child = endMarks(g.points.at(-1)!, g.childDir, r.kind === '1:1' ? 'zero-one' : 'zero-many')
    return {
      r,
      g,
      path: g.points.map((p, i) => `${i === 0 ? 'M' : 'L'}${p.x} ${p.y}`).join(''),
      marks: [...parent.lines, ...child.lines].join(''),
      circles: [parent.circle, child.circle].filter((c): c is Pt => c !== null),
      text: r.fk ? r.kind : `${r.column}, без FK`,
    }
  }),
)
const junctionOf = computed(() => new Set(props.state.manyToMany.map((m) => m.via)))

const schemas = computed(() => {
  if (!props.state.showSchemas) return []
  const groups = new Map<SchemaName, { x1: number; y1: number; x2: number; y2: number }>()
  for (const { e, box, h } of entities.value) {
    const g = groups.get(e.schema) ?? { x1: Infinity, y1: Infinity, x2: -Infinity, y2: -Infinity }
    groups.set(e.schema, { x1: Math.min(g.x1, box.x), y1: Math.min(g.y1, box.y), x2: Math.max(g.x2, box.x + box.w), y2: Math.max(g.y2, box.y + h) })
  }
  return [...groups].map(([name, g]) => ({ name, x: g.x1 - 6, y: g.y1 - 20, w: g.x2 - g.x1 + 12, h: g.y2 - g.y1 + 26 }))
})

const qualified = (e: ErEntity): string => (props.state.showSchemas ? `${e.schema}.${e.id}` : e.id)
const nameOf = (id: EntityId): string => {
  const e = props.state.entities.find((x) => x.id === id)
  return e === undefined ? id : qualified(e)
}
const describe = (r: ErRelation): string =>
  r.fk
    ? `${nameOf(r.parent)} — ${nameOf(r.child)}: ${r.kind === '1:1' ? 'один к одному' : 'один ко многим'}, внешний ключ ${r.column}`
    : `${nameOf(r.parent)} — ${nameOf(r.child)}: один ко многим по ${r.column}, без внешнего ключа`
</script>

<template>
  <div class="er">
    <p class="req"><span class="req-k">Требование</span> {{ state.requirement }} <span class="src">({{ state.source }})</span></p>

    <svg :viewBox="`0 0 ${W} ${H}`" aria-hidden="true" focusable="false">
      <g v-for="s in schemas" :key="s.name">
        <rect :x="s.x" :y="s.y" :width="s.w" :height="s.h" rx="10" class="schema" />
        <text :x="s.x + 8" :y="s.y + 14" class="schema-t">схема {{ s.name }}</text>
      </g>

      <g v-for="x in relations" :key="x.r.id" :class="['rel', { fresh: fresh.has(x.r.id), nofk: !x.r.fk }]">
        <path :d="x.path" class="line" />
        <path :d="x.marks" class="mark" />
        <circle v-for="(c, i) in x.circles" :key="i" :cx="c.x" :cy="c.y" r="4" class="opt" />
        <text :x="x.g.label.x" :y="x.g.label.y" :text-anchor="x.g.anchor" class="card">{{ x.text }}</text>
      </g>

      <g v-for="{ e, box, h } in entities" :key="e.id" :class="['ent', { fresh: fresh.has(e.id) }]">
        <rect :x="box.x" :y="box.y" :width="box.w" :height="h" rx="6" class="box" />
        <line :x1="box.x" :y1="box.y + HEAD" :x2="box.x + box.w" :y2="box.y + HEAD" class="sep" />
        <text :x="box.x + PAD" :y="box.y + 16" class="title">{{ e.id }}</text>
        <text v-if="junctionOf.has(e.id)" :x="box.x + box.w - PAD" :y="box.y + 16" text-anchor="end" class="mn">M:N</text>
        <g v-for="(a, i) in e.attrs" :key="a.name">
          <text :x="box.x + PAD" :y="box.y + HEAD + 13 + i * ROW" class="attr">{{ a.name }}</text>
          <text :x="box.x + box.w - PAD" :y="box.y + HEAD + 13 + i * ROW" text-anchor="end" class="key">{{ a.keys.join(' ') }}</text>
        </g>
      </g>
    </svg>

    <div class="sr-only">
      <p>Сущностей на диаграмме: {{ state.entities.length }}.</p>
      <ul>
        <li v-for="e in state.entities" :key="e.id">
          {{ qualified(e) }}<template v-if="fresh.has(e.id)"> (новая)</template>: {{ e.attrs.map((a) => (a.keys.length > 0 ? `${a.name} ${a.keys.join(' ')}` : a.name)).join(', ') }}
        </li>
        <li v-for="r in state.relations" :key="r.id">{{ describe(r) }}<template v-if="fresh.has(r.id)"> (выделена)</template></li>
        <li v-for="m in state.manyToMany" :key="m.via">{{ nameOf(m.a) }} и {{ nameOf(m.b) }}: многие ко многим через {{ nameOf(m.via) }}</li>
      </ul>
    </div>

    <ul class="legend">
      <li><svg viewBox="0 0 30 14" aria-hidden="true"><path d="M0 7H30M18 1V13M22 1V13" /></svg> ровно один</li>
      <li><svg viewBox="0 0 30 14" aria-hidden="true"><path d="M0 7H30M22 1V13" /><circle cx="14" cy="7" r="4" /></svg> ноль или один</li>
      <li><svg viewBox="0 0 30 14" aria-hidden="true"><path d="M0 7H30M18 7L30 1M18 7L30 13" /><circle cx="12" cy="7" r="4" /></svg> ноль или много</li>
      <li><span class="sw dash" aria-hidden="true"></span> ссылка без FOREIGN KEY</li>
      <li><span class="sw new" aria-hidden="true"></span> появилось или выделено на этом шаге</li>
      <li>PK — первичный ключ, FK — внешний, UQ — уникальный</li>
    </ul>
  </div>
</template>

<style scoped>
.er { padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
.req { margin: 0 0 8px; padding: 6px 10px; border: 2px solid var(--viz-edge); border-left: 8px solid var(--viz-client); border-radius: 8px; font-size: 14px; font-weight: 600; }
.req-k { display: block; font-size: 12px; font-weight: 400; color: var(--viz-muted); }
.src { font-weight: 400; color: var(--viz-muted); }
.er > svg { display: block; width: calc(100% + 16px); height: auto; margin: 0 -8px; }
.schema { fill: none; stroke: var(--viz-muted); stroke-width: 1.5; stroke-dasharray: 6 4; }
.schema-t { font-size: 13px; fill: var(--viz-muted); }
.box { fill: var(--viz-node-bg); stroke: var(--viz-db); stroke-width: 2; }
.ent.fresh .box { stroke: var(--viz-active); stroke-width: 4; }
.sep { stroke: var(--viz-edge); stroke-width: 1; }
.title { font-size: 14px; font-weight: 700; fill: var(--viz-text); }
.mn { font-size: 13px; font-weight: 700; fill: var(--viz-muted); }
.attr { font-size: 13px; font-family: ui-monospace, monospace; fill: var(--viz-text); }
.key { font-size: 13px; font-weight: 700; fill: var(--viz-muted); }
.line, .mark { fill: none; stroke: var(--viz-muted); stroke-width: 2; }
.opt { fill: var(--viz-node-bg); stroke: var(--viz-muted); stroke-width: 2; }
.rel.nofk .line { stroke-dasharray: 5 4; }
.rel.fresh .line, .rel.fresh .mark, .rel.fresh .opt { stroke: var(--viz-active); stroke-width: 3; }
.card { font-size: 13px; font-weight: 700; fill: var(--viz-text); }
.legend { display: flex; flex-wrap: wrap; gap: 4px 14px; margin: 8px 0 0; padding: 0; list-style: none; font-size: 13px; color: var(--viz-muted); }
.legend li { display: flex; align-items: center; gap: 6px; margin: 0; }
.legend svg { width: 30px; height: 14px; flex: none; }
.legend path { fill: none; stroke: var(--viz-muted); stroke-width: 2; }
.legend circle { fill: var(--viz-node-bg); stroke: var(--viz-muted); stroke-width: 2; }
.sw { display: inline-block; width: 26px; flex: none; }
.sw.dash { border-top: 2px dashed var(--viz-muted); }
.sw.new { height: 12px; border: 3px solid var(--viz-active); border-radius: 3px; }
.sr-only { position: absolute; width: 1px; height: 1px; margin: -1px; padding: 0; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; border: 0; }
</style>
