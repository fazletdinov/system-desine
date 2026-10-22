<script setup lang="ts">
import { computed } from 'vue'
import type { BtFrame, BtView, Mark } from '../simulations/btree'
import { heapHeaderH, heapRowH, layoutScene, selectionLinks, tidPath, type Box } from '../simulations/btree-layout'
import { isLeaf, leafOrder, type BtNode, type HeapPage, type InnerNode, type LeafNode, type Tid } from '../simulations/btree-model'

const props = defineProps<{ frame: BtFrame; view: BtView; selected: string | null }>()
const emit = defineEmits<{ select: [id: string] }>()

const model = computed(() => props.frame.model)
const detail = computed(() => props.view === 'detail')
const scene = computed(() => layoutScene(model.value, props.view))
const box = (id: string): Box => scene.value.boxes[id] ?? { x: 0, y: 0, w: 0, h: 0 }
const links = computed(() => selectionLinks(model.value, props.selected))

const tidText = (t: Tid): string => `(${t[0]},${t[1]})`
const hasEdge = (list: readonly (readonly [string, string])[], a: string, b: string): boolean => list.some((e) => e[0] === a && e[1] === b)

// Клавиатура: у каждого кликабельного элемента фокус и Enter/Пробел, как у кнопки.
const pick = (id: string): void => emit('select', id)

function nodeMark(n: BtNode): Mark | '' {
  return props.frame.nodes[n.id] ?? ''
}
const nodeHead = (n: BtNode): string => {
  const kind = isLeaf(n) ? 'лист' : n.id === model.value.root ? 'корень' : 'внутренняя'
  return detail.value ? `${kind} · блок ${n.block}` : kind
}
const nodes = computed(() => Object.values(model.value.nodes).filter((n) => box(n.id).w > 0))

interface EdgeView { key: string; d: string; on: boolean }
const edges = computed<EdgeView[]>(() =>
  nodes.value.flatMap((n) => {
    if (isLeaf(n)) return []
    const b = box(n.id)
    return n.entries.flatMap((e, j) => {
      const c = box(e.child)
      if (!c.w) return []
      const x = b.x + ((j + 0.5) * b.w) / n.entries.length
      const y = b.y + b.h
      const d = `M${x},${y} C${x},${y + 30} ${c.x + c.w / 2},${c.y - 30} ${c.x + c.w / 2},${c.y}`
      return [{ key: `${n.id}>${e.child}`, d, on: hasEdge(props.frame.edges, n.id, e.child) }]
    })
  }),
)

// В simple рисуем только пройденные ссылки соседей: полная цепочка отвлекала бы от спуска.
const sibs = computed(() => {
  const order = leafOrder(model.value)
  return order.slice(0, -1).flatMap((id, i) => {
    const next = order[i + 1] ?? ''
    const a = box(id)
    const b = box(next)
    const on = hasEdge(props.frame.sibs, id, next)
    if (!detail.value && !on) return []
    const y = a.y + 16
    return [{ key: `${id}~${next}`, on, line: `M${a.x + a.w + 2},${y}L${b.x - 2},${y}`, hit: `M${a.x + a.w},${y}L${b.x},${y}` }]
  })
})

const metaPath = computed(() => {
  const b = box('meta')
  const r = box(model.value.root)
  return `M${b.x},${b.y + 30}L${r.x + r.w + 4},${r.y + 20}`
})

const tidPaths = computed(() => {
  const all = [
    ...props.frame.tids.map(([key, page, lp]) => ({ key, tid: [page, lp] as Tid, selected: false })),
    ...links.value.tids,
  ]
  return all.flatMap((l, i) => {
    const d = tidPath(model.value, scene.value.boxes, props.view, l)
    return d ? [{ key: `${i}:${l.key}:${l.tid.join('.')}`, d, selected: l.selected }] : []
  })
})

const entryClass = (key: number): Record<string, boolean> => ({
  hit: props.frame.hitEntries.includes(key) || props.frame.hintEntries.includes(key),
  sel: links.value.entries.has(key) || props.selected === `entry:${key}`,
})
const rowClass = (p: number, lp: number): Record<string, boolean> => ({
  hit: props.frame.hitRows.includes(`${p}:${lp}`),
  sel: links.value.rows.has(`${p}:${lp}`),
})

const pageMark = (p: HeapPage): Mark | '' => props.frame.pages[p.no] ?? ''
const rowY = (b: Box, j: number): number => b.y + heapHeaderH(detail.value) + j * heapRowH(detail.value)

// Лист в simple: записи — равные ячейки в одну строку; в detail — столбец «ключ → TID».
const cell = (b: Box, count: number, j: number): { x: number; w: number } => {
  const w = (b.w - 8) / count
  return { x: b.x + 4 + w * j, w }
}
const innerSimple = (n: InnerNode, b: Box, j: number): number => b.x + (b.w / n.entries.length) * (j + 0.5)
const leafOf = (n: BtNode): LeafNode | null => (isLeaf(n) ? n : null)
const innerOf = (n: BtNode): InnerNode | null => (isLeaf(n) ? null : n)
const sepText = (sep: number | null): string => (sep === null ? '−∞' : String(sep))
const childBlock = (id: string): number => model.value.nodes[id]?.block ?? 0
</script>

<template>
  <svg class="bt" :viewBox="`0 0 ${scene.width} ${scene.height}`" role="group" aria-label="Схема: индекс B-tree и таблица chat.members">
    <defs>
      <marker id="bt-arrow-tid" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse"><path d="M0,0L10,5L0,10z" class="head-tid" /></marker>
      <marker id="bt-arrow-sib" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse"><path d="M0,0L10,5L0,10z" class="head-sib" /></marker>
    </defs>

    <text class="lvl" x="14" y="22">корень</text>
    <text class="lvl" x="14" y="132">внутренние страницы</text>
    <text class="lvl" x="14" y="254">листья — индексные записи</text>
    <text class="lvl" x="14" y="422">таблица chat.members (heap) — строки в порядке вставки</text>
    <text class="lvl" :x="scene.width - 14" y="22" text-anchor="end">индекс members_user_id_idx</text>

    <path v-for="e in edges" :key="e.key" class="edge" :class="{ on: e.on }" :d="e.d" />

    <g v-for="s in sibs" :key="s.key" class="el" role="button" tabindex="0" aria-label="Ссылки между листьями" data-explain="sib" @click="pick('sib')" @keydown.enter.space.prevent="pick('sib')">
      <path class="sib" :class="{ on: s.on }" :d="s.line" marker-start="url(#bt-arrow-sib)" marker-end="url(#bt-arrow-sib)" />
      <path :d="s.hit" class="hit-area" />
    </g>

    <template v-if="detail">
      <g class="el" role="button" tabindex="0" aria-label="Метастраница" data-explain="meta" @click="pick('meta')" @keydown.enter.space.prevent="pick('meta')">
        <rect class="box meta" :class="[frame.nodes.meta, { sel: selected === 'meta', pulse: frame.nodes.meta === 'cur' }]" :x="box('meta').x" :y="box('meta').y" :width="box('meta').w" :height="box('meta').h" rx="9" />
        <text class="nh" :x="box('meta').x + 10" :y="box('meta').y + 19">метастраница · блок 0</text>
        <text class="t" :x="box('meta').x + 10" :y="box('meta').y + 38">корень → <tspan class="addr">блок {{ childBlock(model.root) }}</tspan></text>
      </g>
      <path class="sib" :d="metaPath" marker-end="url(#bt-arrow-sib)" />
    </template>

    <template v-for="n in nodes" :key="n.id">
      <g class="el" role="button" tabindex="0" :aria-label="`Страница индекса: ${nodeHead(n)}`" :data-explain="`node:${n.id}`" @click="pick(`node:${n.id}`)" @keydown.enter.space.prevent="pick(`node:${n.id}`)">
        <rect class="box idx" :class="[nodeMark(n), { sel: selected === `node:${n.id}`, pulse: nodeMark(n) === 'cur' }]" :x="box(n.id).x" :y="box(n.id).y" :width="box(n.id).w" :height="box(n.id).h" rx="9" />
        <text class="nh" :x="box(n.id).x + 10" :y="box(n.id).y + 18">{{ nodeHead(n) }}</text>
      </g>

      <template v-if="innerOf(n)">
        <template v-for="(e, j) in innerOf(n)!.entries" :key="e.child">
          <text v-if="detail" class="t" :x="box(n.id).x + 12" :y="box(n.id).y + 38 + j * 18">{{ sepText(e.sep) }}<tspan class="addr"> → блок {{ childBlock(e.child) }}</tspan></text>
          <text v-else class="t" text-anchor="middle" :x="innerSimple(innerOf(n)!, box(n.id), j)" :y="box(n.id).y + 38">{{ sepText(e.sep) }}</text>
        </template>
      </template>

      <template v-if="leafOf(n)">
        <g v-for="(e, j) in leafOf(n)!.entries" :key="e.key" class="el ent" :class="entryClass(e.key)" role="button" tabindex="0" :aria-label="`Индексная запись: ключ ${e.key}`" :data-explain="`entry:${e.key}`" @click="pick(`entry:${e.key}`)" @keydown.enter.space.prevent="pick(`entry:${e.key}`)">
          <template v-if="detail">
            <rect :x="box(n.id).x + 4" :y="box(n.id).y + 24 + j * 18" :width="box(n.id).w - 8" height="17" />
            <text class="t" :x="box(n.id).x + 10" :y="box(n.id).y + 37 + j * 18">{{ e.key }}<tspan class="addr"> → {{ e.tids.map(tidText).join(' ') }}</tspan></text>
          </template>
          <template v-else>
            <rect :x="cell(box(n.id), leafOf(n)!.entries.length, j).x" :y="box(n.id).y + 24" :width="cell(box(n.id), leafOf(n)!.entries.length, j).w" height="20" />
            <text class="t" text-anchor="middle" :x="cell(box(n.id), leafOf(n)!.entries.length, j).x + cell(box(n.id), leafOf(n)!.entries.length, j).w / 2" :y="box(n.id).y + 38">{{ e.key }}<tspan v-if="e.tids.length > 1" class="addr small">×{{ e.tids.length }}</tspan></text>
          </template>
        </g>
      </template>
    </template>

    <template v-for="p in model.pages" :key="p.no">
      <g class="el" role="button" tabindex="0" :aria-label="`Страница таблицы ${p.no}`" :data-explain="`page:${p.no}`" @click="pick(`page:${p.no}`)" @keydown.enter.space.prevent="pick(`page:${p.no}`)">
        <rect class="box heap" :class="[pageMark(p), { sel: selected === `page:${p.no}`, pulse: pageMark(p) === 'cur' }]" :x="box(`page:${p.no}`).x" :y="box(`page:${p.no}`).y" :width="box(`page:${p.no}`).w" :height="box(`page:${p.no}`).h" rx="9" />
        <text class="nh" :x="box(`page:${p.no}`).x + 10" :y="box(`page:${p.no}`).y + 18">стр. {{ p.no }}{{ detail ? ' · 8 КБ' : '' }}</text>
      </g>
      <template v-if="detail">
        <g class="el" role="button" tabindex="0" :aria-label="`Карта видимости страницы ${p.no}: ${p.allVisible ? 'all-visible' : 'нет отметки'}`" :data-explain="`vm:${p.no}`" @click="pick(`vm:${p.no}`)" @keydown.enter.space.prevent="pick(`vm:${p.no}`)">
          <rect class="chip" :class="[p.allVisible ? 'vis' : 'dirty', { cur: frame.vmCur === p.no, checked: frame.vmChecked[p.no] !== undefined }]" :x="box(`page:${p.no}`).x + box(`page:${p.no}`).w - 58" :y="box(`page:${p.no}`).y + 6" width="50" height="16" />
          <text class="vm" :class="p.allVisible ? 'vis' : 'dirty'" :x="box(`page:${p.no}`).x + box(`page:${p.no}`).w - 33" :y="box(`page:${p.no}`).y + 18" text-anchor="middle">VM {{ p.allVisible ? '✓' : '—' }}</text>
        </g>
        <text class="t dim small" :x="box(`page:${p.no}`).x + 10" :y="box(`page:${p.no}`).y + 36">TID   chat  user_id</text>
      </template>
      <g v-for="(r, j) in p.rows" :key="r.lp" class="el row" :class="rowClass(p.no, r.lp)" role="button" tabindex="0" :aria-label="`Строка таблицы (${p.no},${r.lp}): ${r.chat}, user_id ${r.user}`" :data-explain="`row:${p.no}:${r.lp}`" @click="pick(`row:${p.no}:${r.lp}`)" @keydown.enter.space.prevent="pick(`row:${p.no}:${r.lp}`)">
        <rect :x="box(`page:${p.no}`).x + 4" :y="rowY(box(`page:${p.no}`), j)" :width="box(`page:${p.no}`).w - 8" :height="detail ? 17 : 16" />
        <text class="t" :x="box(`page:${p.no}`).x + 10" :y="rowY(box(`page:${p.no}`), j) + 12.5"><tspan v-if="detail" class="addr">{{ tidText([p.no, r.lp]) }}  </tspan>{{ r.chat }}<tspan class="addr"> · </tspan>{{ r.user }}</text>
      </g>
    </template>

    <!-- TID поверх страниц, но не перехватывает клики: pointer-events: none -->
    <g class="tids">
      <path v-for="t in tidPaths" :key="t.key" class="tid" :class="{ sel: t.selected }" :d="t.d" marker-end="url(#bt-arrow-tid)" />
    </g>
  </svg>
</template>

<style scoped>
.bt { width: 100%; height: auto; display: block; color: var(--viz-text); }
.lvl { font: 650 10.5px var(--vp-font-family-base, system-ui); letter-spacing: 0.8px; text-transform: uppercase; fill: var(--viz-muted); }
.box { fill: var(--viz-node-bg); stroke: var(--viz-edge); stroke-width: 1.5; }
.box.heap { fill: color-mix(in srgb, var(--viz-service) 10%, var(--viz-node-bg)); }
.box.meta { stroke-dasharray: 4 3; }
.el { cursor: pointer; outline: none; }
.el:hover .box { stroke: var(--viz-muted); }
.el:focus-visible { outline: 2px solid var(--viz-client); outline-offset: 2px; }
.box.read { stroke: var(--viz-active); stroke-width: 2; }
.box.cur { stroke: var(--viz-client); stroke-width: 2.5; filter: drop-shadow(0 0 6px var(--viz-client)); }
.box.hint { stroke: var(--viz-muted); stroke-dasharray: 5 3; }
.box.sel { stroke: var(--viz-text); stroke-width: 2.5; }
.nh { font: 650 11px var(--vp-font-family-base, system-ui); fill: var(--viz-muted); }
.t { font: 500 12px var(--vp-font-family-mono, ui-monospace, monospace); fill: var(--viz-text); }
.t.dim { fill: var(--viz-muted); }
.small { font-size: 10.5px; }
.addr { fill: var(--viz-addr); }
.ent rect, .row rect { fill: transparent; rx: 4; }
.ent:hover rect, .row:hover rect { fill: color-mix(in srgb, var(--viz-text) 10%, transparent); }
.ent.hit rect, .row.hit rect { fill: var(--viz-hit-bg); }
.ent.hit .t, .row.hit .t { fill: var(--viz-hit-text); font-weight: 700; }
.ent.hit .addr, .row.hit .addr { fill: var(--viz-hit-text); }
.ent.sel rect, .row.sel rect { fill: color-mix(in srgb, var(--viz-text) 16%, transparent); stroke: var(--viz-text); stroke-width: 1; }
.edge { stroke: var(--viz-edge); stroke-width: 1.4; fill: none; }
.edge.on { stroke: var(--viz-client); stroke-width: 2.6; }
.sib { stroke: var(--viz-muted); stroke-width: 1.3; stroke-dasharray: 4 3; fill: none; }
.sib.on { stroke: var(--viz-client); stroke-width: 2.4; stroke-dasharray: none; }
.hit-area { stroke: transparent; stroke-width: 12; fill: none; }
.head-sib { fill: var(--viz-muted); }
.head-tid { fill: var(--viz-db); }
.tids { pointer-events: none; }
.tid { stroke: var(--viz-db); stroke-width: 1.8; fill: none; opacity: 0.85; }
.tid.sel { stroke: var(--viz-text); stroke-width: 2.2; opacity: 1; }
.chip { fill: var(--viz-node-bg); stroke: var(--viz-edge); rx: 5; }
.chip.cur.vis { fill: var(--viz-hit-bg); stroke: var(--viz-db); }
.chip.cur.dirty { fill: color-mix(in srgb, var(--viz-error) 25%, var(--viz-node-bg)); stroke: var(--viz-error); }
.chip.checked:not(.cur) { stroke: var(--viz-muted); }
.vm { font: 600 10px var(--vp-font-family-base, system-ui); }
.vm.vis { fill: var(--viz-db); }
.vm.dirty { fill: var(--viz-error); }
@media (prefers-reduced-motion: no-preference) {
  .pulse { animation: bt-pulse 1.4s ease-in-out infinite; }
  @keyframes bt-pulse { 50% { opacity: 0.35; } }
}
</style>
