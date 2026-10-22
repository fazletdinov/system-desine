<script setup lang="ts">
import { computed, useId } from 'vue'
import type { BtFrame, BtView, Mark } from '../simulations/btree'
import { CELL_H, ENTRY_TOP, FONT, LINE_H, TIDS_PER_LINE, heapHeaderH, heapRowH, layoutScene, lineOffsets, selectionLinks, tidPath, type Box } from '../simulations/btree-layout'
import { isLeaf, leafOrder, type HeapPage, type Tid } from '../simulations/btree-model'

const props = defineProps<{ frame: BtFrame; view: BtView; selected: string | null }>()
const emit = defineEmits<{ select: [id: string] }>()

// Маркеры стрелок получают уникальный суффикс: на странице может быть несколько сцен.
const uid = useId()
const tidMarker = `bt-arrow-tid-${uid}`
const sibMarker = `bt-arrow-sib-${uid}`

const model = computed(() => props.frame.model)
const detail = computed(() => props.view === 'detail')
const scene = computed(() => layoutScene(model.value, props.view))
const box = (id: string): Box => scene.value.boxes[id] ?? { x: 0, y: 0, w: 0, h: 0 }
const links = computed(() => selectionLinks(model.value, props.selected))

const tidText = (t: Tid): string => `(${t[0]},${t[1]})`
// Моноширинный знак ≈ 0,6 кегля: продолжение posting list встаёт ровно под первым TID.
const MONO_CH = FONT.mono * 0.6
function tidChunks(tids: readonly Tid[]): string[] {
  const out: string[] = []
  for (let i = 0; i < tids.length; i += TIDS_PER_LINE) out.push(tids.slice(i, i + TIDS_PER_LINE).map(tidText).join(' '))
  return out
}
const hasEdge = (list: readonly (readonly [string, string])[], a: string, b: string): boolean => list.some((e) => e[0] === a && e[1] === b)
const sepText = (sep: number | null): string => (sep === null ? '−∞' : String(sep))
const blockOf = (id: string): number => model.value.nodes[id]?.block ?? 0

// Клавиатура: у каждого кликабельного элемента фокус и Enter/Пробел, как у кнопки.
function interactive(id: string, label: string): Record<string, unknown> {
  const pick = (): void => emit('select', id)
  return {
    role: 'button',
    tabindex: 0,
    'aria-label': label,
    'data-explain': id,
    onClick: pick,
    onKeydown: (e: KeyboardEvent) => {
      if (e.key !== 'Enter' && e.key !== ' ') return
      e.preventDefault()
      pick()
    },
  }
}

interface NodeView {
  id: string
  box: Box
  head: string
  cls: (Mark | string)[]
  attrs: Record<string, unknown>
  seps: { key: string; text: string; block: number; x: number; y: number }[]
  entries: { key: number; cls: Record<string, boolean>; attrs: Record<string, unknown>; label: string; tids: string[]; contX: number; many: number; cell: { x: number; w: number }; y: number }[]
}

const nodeViews = computed<NodeView[]>(() =>
  Object.values(model.value.nodes).flatMap((n) => {
    const b = box(n.id)
    if (!b.w) return []
    const leaf = isLeaf(n)
    const kind = leaf ? 'лист' : n.id === model.value.root ? 'корень' : 'внутренняя'
    const head = detail.value ? `${kind} · блок ${n.block}` : kind
    const mark = props.frame.nodes[n.id]
    const count = n.entries.length
    const seps = leaf ? [] : n.entries.map((e, j) => ({
      key: e.child,
      text: sepText(e.sep),
      block: blockOf(e.child),
      x: detail.value ? b.x + 12 : b.x + (b.w / count) * (j + 0.5),
      y: detail.value ? b.y + ENTRY_TOP + j * LINE_H + 15 : b.y + ENTRY_TOP + 16,
    }))
    const offsets = leaf ? lineOffsets(n.entries.map((e) => e.tids.length)) : []
    const entries = !leaf ? [] : n.entries.map((e, j) => {
      const w = (b.w - 8) / count
      return {
        key: e.key,
        cls: entryClass(e.key),
        attrs: interactive(`entry:${e.key}`, `Индексная запись: ключ ${e.key}`),
        label: String(e.key),
        tids: tidChunks(e.tids),
        contX: b.x + 10 + (String(e.key).length + 3) * MONO_CH,
        many: e.tids.length,
        cell: { x: b.x + 4 + w * j, w },
        y: b.y + ENTRY_TOP + (offsets[j] ?? 0) * LINE_H,
      }
    })
    return [{
      id: n.id, box: b, head, seps, entries,
      cls: ['idx', mark ?? '', props.selected === `node:${n.id}` ? 'sel' : '', mark === 'cur' ? 'pulse' : ''],
      attrs: interactive(`node:${n.id}`, `Страница индекса: ${head}`),
    }]
  }),
)

interface EdgeView { key: string; d: string; on: boolean }
const edges = computed<EdgeView[]>(() =>
  Object.values(model.value.nodes).flatMap((n) => {
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
    const y = a.y + 18
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

// Подсветка «выбранной» записи приходит из selectionLinks (включая саму выбранную запись).
function entryClass(key: number): Record<string, boolean> {
  return {
    hit: props.frame.hitEntries.includes(key) || props.frame.hintEntries.includes(key),
    sel: links.value.entries.has(key),
  }
}

interface PageView {
  no: number
  box: Box
  title: string
  cls: (Mark | string)[]
  attrs: Record<string, unknown>
  vm: { cls: string; chipCls: Record<string, boolean>; attrs: Record<string, unknown>; text: string; x: number }
  rows: { lp: number; cls: Record<string, boolean>; attrs: Record<string, unknown>; y: number; tid: string; chat: string; user: number }[]
}

function pageView(p: HeapPage): PageView {
  const b = box(`page:${p.no}`)
  const mark = props.frame.pages[p.no]
  const vmCls = p.allVisible ? 'vis' : 'dirty'
  return {
    no: p.no,
    box: b,
    // «8 КБ» — в объяснении страницы: в заголовке рядом с чипом VM на 7 страницах ему нет места.
    title: `стр. ${p.no}`,
    cls: ['heap', mark ?? '', props.selected === `page:${p.no}` ? 'sel' : '', mark === 'cur' ? 'pulse' : ''],
    attrs: interactive(`page:${p.no}`, `Страница таблицы ${p.no}`),
    vm: {
      cls: vmCls,
      chipCls: { cur: props.frame.vmCur === p.no, checked: props.frame.vmChecked[p.no] !== undefined },
      attrs: interactive(`vm:${p.no}`, `Карта видимости страницы ${p.no}: ${p.allVisible ? 'all-visible' : 'нет отметки'}`),
      text: `VM ${p.allVisible ? '✓' : '—'}`,
      x: b.x + b.w - 52,
    },
    rows: p.rows.map((r, j) => ({
      lp: r.lp,
      cls: { hit: props.frame.hitRows.includes(`${p.no}:${r.lp}`), sel: links.value.rows.has(`${p.no}:${r.lp}`) },
      attrs: interactive(`row:${p.no}:${r.lp}`, `Строка таблицы (${p.no},${r.lp}): ${r.chat}, user_id ${r.user}`),
      y: b.y + heapHeaderH(detail.value) + j * heapRowH(),
      tid: tidText([p.no, r.lp]),
      chat: r.chat,
      user: r.user,
    })),
  }
}
const pageViews = computed(() => model.value.pages.map(pageView))
</script>

<template>
  <svg class="bt" :viewBox="`0 0 ${scene.width} ${scene.height}`" role="group" aria-label="Схема: индекс B-tree и таблица chat.members">
    <defs>
      <marker :id="tidMarker" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse"><path d="M0,0L10,5L0,10z" class="head-tid" /></marker>
      <marker :id="sibMarker" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse"><path d="M0,0L10,5L0,10z" class="head-sib" /></marker>
    </defs>

    <text class="lvl" x="14" :y="scene.rows.root - 12">корень</text>
    <text class="lvl" x="14" :y="scene.rows.inner - 10">внутренние страницы</text>
    <text class="lvl" x="14" :y="scene.rows.leaf - 10">листья — индексные записи</text>
    <text class="lvl" x="14" :y="scene.rows.heap - 10">таблица chat.members (heap) — строки в порядке вставки</text>
    <text class="lvl" :x="scene.width - 14" :y="scene.rows.root - 12" text-anchor="end">индекс members_user_id_idx</text>

    <path v-for="e in edges" :key="e.key" class="edge" :class="{ on: e.on }" :d="e.d" />

    <g v-for="s in sibs" :key="s.key" class="el" v-bind="interactive('sib', 'Ссылки между листьями')">
      <path class="sib" :class="{ on: s.on }" :d="s.line" :marker-start="`url(#${sibMarker})`" :marker-end="`url(#${sibMarker})`" />
      <path :d="s.hit" class="hit-area" />
    </g>

    <template v-if="detail">
      <g class="el" v-bind="interactive('meta', 'Метастраница')">
        <rect class="box meta" :class="[frame.nodes.meta, { sel: selected === 'meta', pulse: frame.nodes.meta === 'cur' }]" :x="box('meta').x" :y="box('meta').y" :width="box('meta').w" :height="box('meta').h" rx="9" />
        <text class="nh" :x="box('meta').x + 10" :y="box('meta').y + 20">метастраница · блок 0</text>
        <text class="t" :x="box('meta').x + 10" :y="box('meta').y + 44">корень → <tspan class="addr">блок {{ blockOf(model.root) }}</tspan></text>
      </g>
      <path class="sib" :d="metaPath" :marker-end="`url(#${sibMarker})`" />
    </template>

    <template v-for="n in nodeViews" :key="n.id">
      <g class="el" v-bind="n.attrs">
        <rect class="box" :class="n.cls" :x="n.box.x" :y="n.box.y" :width="n.box.w" :height="n.box.h" rx="9" />
        <text class="nh" :x="n.box.x + 10" :y="n.box.y + 20">{{ n.head }}</text>
      </g>

      <template v-for="s in n.seps" :key="s.key">
        <text v-if="detail" class="t" :x="s.x" :y="s.y">{{ s.text }}<tspan class="addr"> → блок {{ s.block }}</tspan></text>
        <text v-else class="t" text-anchor="middle" :x="s.x" :y="s.y">{{ s.text }}</text>
      </template>

      <g v-for="e in n.entries" :key="e.key" class="el ent" :class="e.cls" v-bind="e.attrs">
        <template v-if="detail">
          <rect :x="n.box.x + 4" :y="e.y" :width="n.box.w - 8" :height="e.tids.length * LINE_H - 2" />
          <text class="t" :x="n.box.x + 10" :y="e.y + 15">{{ e.label }}<tspan class="addr"> → {{ e.tids[0] }}</tspan><tspan v-for="(more, k) in e.tids.slice(1)" :key="k" class="addr" :x="e.contX" :dy="LINE_H">{{ more }}</tspan></text>
        </template>
        <template v-else>
          <rect :x="e.cell.x" :y="n.box.y + ENTRY_TOP" :width="e.cell.w" :height="CELL_H" />
          <text class="t" text-anchor="middle" :x="e.cell.x + e.cell.w / 2" :y="n.box.y + ENTRY_TOP + 16">{{ e.label }}<tspan v-if="e.many > 1" class="addr small">×{{ e.many }}</tspan></text>
        </template>
      </g>
    </template>

    <template v-for="p in pageViews" :key="p.no">
      <g class="el" v-bind="p.attrs">
        <rect class="box" :class="p.cls" :x="p.box.x" :y="p.box.y" :width="p.box.w" :height="p.box.h" rx="9" />
        <text class="nh" :x="p.box.x + 10" :y="p.box.y + 20">{{ p.title }}</text>
      </g>
      <template v-if="detail">
        <g class="el" v-bind="p.vm.attrs">
          <rect class="chip" :class="[p.vm.cls, p.vm.chipCls]" :x="p.vm.x" :y="p.box.y + 6" width="46" height="20" />
          <text class="vm" :class="p.vm.cls" :x="p.vm.x + 23" :y="p.box.y + 21" text-anchor="middle">{{ p.vm.text }}</text>
        </g>
        <!-- Отступ 6, а не 10: на 7 страницах подпись иначе упирается в рамку. -->
        <text class="t dim small" :x="p.box.x + 6" :y="p.box.y + 41">TID chat user_id</text>
      </template>
      <g v-for="r in p.rows" :key="r.lp" class="el row" :class="r.cls" v-bind="r.attrs">
        <rect :x="p.box.x + 4" :y="r.y" :width="p.box.w - 8" :height="heapRowH() - 1" />
        <text class="t" :x="p.box.x + 10" :y="r.y + 14.5"><tspan v-if="detail" class="addr">{{ r.tid }}</tspan><tspan :dx="detail ? 8 : 0">{{ r.chat }}</tspan><tspan class="addr"> · </tspan>{{ r.user }}</text>
      </g>
    </template>

    <!-- TID поверх страниц, но не перехватывает клики: pointer-events: none -->
    <g class="tids">
      <path v-for="t in tidPaths" :key="t.key" class="tid" :class="{ sel: t.selected }" :d="t.d" :marker-end="`url(#${tidMarker})`" />
    </g>
  </svg>
</template>

<style scoped>
.bt { width: 100%; height: auto; display: block; color: var(--viz-text); }
.lvl { font: 650 14px var(--vp-font-family-base, system-ui); letter-spacing: 0.6px; text-transform: uppercase; fill: var(--viz-muted); }
.box { fill: var(--viz-node-bg); stroke: var(--viz-edge); stroke-width: 1.5; }
/* Заливку не тонируем: на любом оттенке muted-текст заголовка падает ниже 4,5:1 в светлой теме. Куча отличается рамкой. */
.box.heap { stroke: color-mix(in srgb, var(--viz-service) 55%, var(--viz-edge)); }
.box.meta { stroke-dasharray: 4 3; }
.el { cursor: pointer; outline: none; }
.el:hover .box:not(.cur):not(.read):not(.sel) { stroke: var(--viz-muted); }
.el:focus-visible { outline: 2px solid var(--viz-client); outline-offset: 2px; }
.box.read { stroke: var(--viz-active); stroke-width: 2; }
.box.cur { stroke: var(--viz-client); stroke-width: 2.5; filter: drop-shadow(0 0 6px var(--viz-client)); }
.box.hint { stroke: var(--viz-muted); stroke-dasharray: 5 3; }
.box.sel { stroke: var(--viz-text); stroke-width: 2.5; }
.nh { font: 650 14px var(--vp-font-family-base, system-ui); fill: var(--viz-muted); }
.t { font: 500 15px var(--vp-font-family-mono, ui-monospace, monospace); fill: var(--viz-text); }
.t.dim { fill: var(--viz-muted); }
.small { font-size: 14px; }
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
.chip.cur.dirty { fill: color-mix(in srgb, var(--viz-error) 25%, var(--viz-node-bg)); }
.chip.checked:not(.cur) { stroke: var(--viz-muted); }
.vm { font: 600 14px var(--vp-font-family-base, system-ui); }
/* Подпись чипа — токены с проверенным контрастом на обычной и «активной» заливке; цвет ошибки только в рамке. */
.vm.vis { fill: var(--viz-hit-text); }
.vm.dirty { fill: var(--viz-text); }
.chip.dirty { stroke: var(--viz-error); }
@media (prefers-reduced-motion: no-preference) {
  .pulse { animation: bt-pulse 1.4s ease-in-out infinite; }
  @keyframes bt-pulse { 50% { opacity: 0.35; } }
}
</style>
