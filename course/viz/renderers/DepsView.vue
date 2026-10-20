<script setup lang="ts">
import { computed, ref } from 'vue'
import { BASE_IMPORTS, PACKAGES, type DepsFrame, type Edge, type Layer, type Pkg } from '../simulations/deps'
import { useNarrow } from '../components/useNarrow'

const props = defineProps<{ state: DepsFrame }>()

const NARROW_PX = 520

const svgEl = ref<SVGSVGElement>()
const narrow = useNarrow(svgEl, NARROW_PX)

const LAYER_TITLE: Record<Layer, string> = {
  root: 'корень сборки',
  adapter: 'адаптеры',
  usecase: 'use case',
  platform: 'platform',
  domain: 'домен',
  stdlib: 'стандартная библиотека',
  external: 'внешние модули',
}
const LAYER_ORDER: readonly Layer[] = ['root', 'adapter', 'usecase', 'platform', 'domain', 'stdlib', 'external']

interface Box { x: number; y: number; w: number; h: number }

// Размеры подобраны так, чтобы при ширине колонки урока (≈600 px) масштаб SVG был около 1 и текст — ≥ 13 px.
const BOX_H = 46
const ROW = 60
const TOP = 36
const BOX_W = 114
const COLUMNS = [
  { x: 2, w: BOX_W, title: 'корень' },
  { x: 164, w: BOX_W, title: 'адаптеры' },
  { x: 326, w: BOX_W, title: 'use case' },
  { x: 484, w: BOX_W, title: 'домен, внешние' },
] as const
const WIDTH = 600

// Колонка и строка каждого пакета. Строки дробные: use case стоит напротив своего домена.
const SLOT: Record<Pkg, { col: number; row: number }> = {
  'cmd/messenger': { col: 0, row: 2 },
  'user/adapters/memory': { col: 1, row: 0 },
  'user/adapters/httpapi': { col: 1, row: 1 },
  'chat/adapters/memory': { col: 1, row: 2 },
  'chat/adapters/httpapi': { col: 1, row: 3 },
  'chat/adapters/userchecker': { col: 1, row: 4 },
  'user/usecase': { col: 2, row: 0.5 },
  'chat/usecase': { col: 2, row: 3 },
  'platform/clock': { col: 2, row: 4.4 },
  'user/domain': { col: 3, row: 0.5 },
  'net/http': { col: 3, row: 2 },
  'chat/domain': { col: 3, row: 3 },
  'github.com/jackc/pgx/v5': { col: 3, row: 4.4 },
}

const boxes = computed(() => {
  const out = {} as Record<Pkg, Box>
  for (const p of PACKAGES) {
    const { col, row } = SLOT[p.id]
    const c = COLUMNS[col] as (typeof COLUMNS)[number]
    out[p.id] = { x: c.x, y: TOP + row * ROW, w: c.w, h: BOX_H }
  }
  return out
})
// Двухстрочная подпись: «chat/adapters» и «userchecker» — так полное имя пакета умещается в узкий блок.
// У пути модуля длинный хвост («jackc/pgx/v5»), поэтому его режем после хоста.
const MODULE_HOST = 'github.com/'
function splitName(id: Pkg): [string, string] {
  const i = id.startsWith(MODULE_HOST) ? MODULE_HOST.length - 1 : id.lastIndexOf('/')
  return [id.slice(0, i + 1), id.slice(i + 1)]
}
const height = TOP + 4.4 * ROW + BOX_H + 14

interface Arrow { key: string; d: string; head: string; kind: 'base' | 'root' | 'added' | 'ok' | 'bad' }

function arrowHead(x2: number, y2: number, angle: number): string {
  const len = 9
  const spread = 5
  const bx = x2 - Math.cos(angle) * len
  const by = y2 - Math.sin(angle) * len
  const px = -Math.sin(angle) * spread
  const py = Math.cos(angle) * spread
  return `${x2},${y2} ${bx + px},${by + py} ${bx - px},${by - py}`
}

function route(a: Box, b: Box): { d: string; head: string } {
  const sameColumn = a.x === b.x
  if (sameColumn) {
    const bulge = 36
    // Дуга выгибается вправо, но у правой колонки справа нет места — дуга ушла бы за
    // край viewBox и стала невидимой; тогда она выгибается влево, в промежуток между колонками.
    const right = a.x + a.w + bulge <= WIDTH
    const x1 = right ? a.x + a.w : a.x
    const cx = right ? x1 + bulge : x1 - bulge
    const y1 = a.y + a.h / 2
    const y2 = b.y + b.h / 2
    return { d: `M${x1},${y1} C${cx},${y1} ${cx},${y2} ${x1},${y2}`, head: arrowHead(x1, y2, right ? Math.PI : 0) }
  }
  const forward = b.x > a.x
  const x1 = forward ? a.x + a.w : a.x
  const x2 = forward ? b.x : b.x + b.w
  const y1 = a.y + a.h / 2
  const y2 = b.y + b.h / 2
  const dx = (x2 - x1) / 2
  return { d: `M${x1},${y1} C${x1 + dx},${y1} ${x2 - dx},${y2} ${x2},${y2}`, head: arrowHead(x2, y2, forward ? 0 : Math.PI) }
}

const isLast = (e: Edge): boolean => props.state.last !== null && e[0] === props.state.last[0] && e[1] === props.state.last[1]

const arrows = computed<Arrow[]>(() => {
  const baseCount = BASE_IMPORTS.length
  return props.state.edges.map((e, i) => {
    const added = i >= baseCount
    let kind: Arrow['kind'] = e[0] === 'cmd/messenger' ? 'root' : 'base'
    if (added) kind = 'added'
    if (added && i === props.state.edges.length - 1 && isLast(e)) kind = props.state.violation === null ? 'ok' : 'bad'
    return { key: `${e[0]}>${e[1]}#${i}`, ...route(boxes.value[e[0]], boxes.value[e[1]]), kind }
  })
})

// Рисуем нарушение и разрешённый импорт поверх остальных рёбер.
const orderedArrows = computed(() => {
  const weight = { root: 0, base: 1, added: 2, ok: 3, bad: 3 } as const
  return [...arrows.value].sort((a, b) => weight[a.kind] - weight[b.kind])
})

const layersWithPackages = computed(() =>
  LAYER_ORDER.map((layer) => ({
    layer,
    title: LAYER_TITLE[layer],
    packages: PACKAGES.filter((p) => p.layer === layer).map((p) => ({
      id: p.id,
      imports: props.state.edges.filter((e) => e[0] === p.id).map((e) => ({ to: e[1], last: isLast(e) })),
    })),
  })).filter((l) => l.packages.length > 0),
)

const verdict = computed(() => {
  if (props.state.last === null) return null
  const text = `${props.state.last[0]} → ${props.state.last[1]}`
  return props.state.violation === null ? `Добавлено: ${text} — разрешено` : `Добавлено: ${text} — нарушение: ${props.state.violation.title}`
})
</script>

<template>
  <div class="deps">
    <svg v-if="!narrow" ref="svgEl" class="deps-svg" :viewBox="`0 0 ${WIDTH} ${height}`" role="img" aria-label="Граф импортов мессенджера: пакеты по слоям и стрелки зависимостей">
      <text v-for="c in COLUMNS" :key="c.title" :x="c.x" y="20" class="col-title" font-size="15">{{ c.title }}</text>
      <path v-for="a in orderedArrows" :key="a.key" :d="a.d" class="edge" :class="a.kind" />
      <polygon v-for="a in orderedArrows" :key="`h${a.key}`" :points="a.head" class="tip" :class="a.kind" />
      <g v-for="p in PACKAGES" :key="p.id">
        <rect :x="boxes[p.id].x" :y="boxes[p.id].y" :width="boxes[p.id].w" :height="boxes[p.id].h" rx="7" class="pkg" :class="p.layer" />
        <text :x="boxes[p.id].x + boxes[p.id].w / 2" :y="boxes[p.id].y + 19" class="pkg-name" font-size="15">{{ splitName(p.id)[0] }}</text>
        <text :x="boxes[p.id].x + boxes[p.id].w / 2" :y="boxes[p.id].y + 37" class="pkg-name" font-size="15">{{ splitName(p.id)[1] }}</text>
      </g>
    </svg>
    <ul v-if="!narrow" class="sr-only" aria-label="Импорты на графе">
      <li v-for="e in state.edges" :key="`${e[0]}>${e[1]}`">{{ e[0] }} импортирует {{ e[1] }}</li>
    </ul>
    <!-- Узкий экран: тот же граф списком. Стрелки — строки «→ пакет», без горизонтальной прокрутки. -->
    <div v-else class="deps-list" role="group" aria-label="Граф импортов мессенджера списком">
      <section v-for="l in layersWithPackages" :key="l.layer" class="layer" :class="l.layer">
        <h4>{{ l.title }}</h4>
        <ul>
          <li v-for="p in l.packages" :key="p.id">
            <span class="pkg-chip" :class="l.layer">{{ p.id }}</span>
            <ul v-if="p.imports.length > 0" class="imports" aria-label="импортирует">
              <li v-for="i in p.imports" :key="i.to" :class="{ last: i.last, bad: i.last && state.violation !== null, ok: i.last && state.violation === null }">→ {{ i.to }}</li>
            </ul>
          </li>
        </ul>
      </section>
    </div>
    <p v-if="verdict" class="verdict" :class="state.violation === null ? 'ok' : 'bad'">{{ verdict }}</p>
    <ul class="legend" aria-label="Обозначения">
      <li><span class="ln base" aria-hidden="true"></span> импорт мессенджера</li>
      <li><span class="ln added" aria-hidden="true"></span> добавленный вами</li>
      <li><span class="ln ok" aria-hidden="true"></span> последний: разрешён</li>
      <li><span class="ln bad" aria-hidden="true"></span> последний: нарушение</li>
    </ul>
  </div>
</template>

<style scoped>
.deps-svg { width: 100%; height: auto; display: block; }
.col-title { fill: var(--viz-muted); }
.sr-only { position: absolute; width: 1px; height: 1px; margin: -1px; padding: 0; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; border: 0; list-style: none; }
.pkg { fill: var(--viz-node-bg); stroke-width: 2.5; }
.pkg.domain { stroke: var(--viz-db); }
.pkg.usecase { stroke: var(--viz-service); }
.pkg.adapter { stroke: var(--viz-client); }
.pkg.platform { stroke: var(--viz-cache); }
.pkg.root { stroke: var(--viz-network); }
.pkg.stdlib { stroke: var(--viz-muted); stroke-dasharray: 5 3; }
.pkg.external { stroke: var(--viz-muted); stroke-dasharray: 2 4; }
.pkg-name { fill: var(--viz-text); text-anchor: middle; }
.edge { fill: none; stroke-width: 2; }
.edge.root { stroke: var(--viz-muted); opacity: 0.3; stroke-width: 1.5; }
.edge.base { stroke: var(--viz-muted); opacity: 0.75; }
.edge.added { stroke: var(--viz-text); stroke-width: 2.5; }
.edge.ok { stroke: var(--viz-response); stroke-width: 4; }
.edge.bad { stroke: var(--viz-error); stroke-width: 4; stroke-dasharray: 8 5; }
.tip.root { fill: var(--viz-muted); opacity: 0.3; }
.tip.base { fill: var(--viz-muted); opacity: 0.75; }
.tip.added { fill: var(--viz-text); }
.tip.ok { fill: var(--viz-response); }
.tip.bad { fill: var(--viz-error); }
.deps-list { display: flex; flex-direction: column; gap: 10px; }
.layer h4 { margin: 0 0 4px; font-size: 13px; font-weight: 600; color: var(--viz-muted); }
.layer ul { list-style: none; margin: 0; padding: 0; }
.layer li { margin: 0 0 6px; }
.pkg-chip { display: inline-block; padding: 3px 8px; border: 2px solid var(--viz-edge); border-radius: 7px; background: var(--viz-node-bg); color: var(--viz-text); font-size: 14px; overflow-wrap: anywhere; }
.pkg-chip.domain { border-color: var(--viz-db); }
.pkg-chip.usecase { border-color: var(--viz-service); }
.pkg-chip.adapter { border-color: var(--viz-client); }
.pkg-chip.platform { border-color: var(--viz-cache); }
.pkg-chip.root { border-color: var(--viz-network); }
.pkg-chip.stdlib { border-style: dashed; border-color: var(--viz-muted); }
.pkg-chip.external { border-style: dotted; border-color: var(--viz-muted); }
.imports { padding-left: 16px; font-size: 13px; color: var(--viz-muted); }
.imports li { margin: 2px 0; overflow-wrap: anywhere; }
.imports li.ok { color: var(--viz-text); font-weight: 700; border-left: 4px solid var(--viz-response); padding-left: 6px; }
.imports li.bad { color: var(--viz-text); font-weight: 700; border-left: 4px solid var(--viz-error); padding-left: 6px; }
.verdict { margin: 10px 0 0; padding: 6px 10px; border-left: 4px solid var(--viz-edge); font-size: 14px; color: var(--viz-text); }
.verdict.ok { border-color: var(--viz-response); }
.verdict.bad { border-color: var(--viz-error); }
.legend { list-style: none; margin: 8px 0 0; padding: 0; display: flex; flex-wrap: wrap; gap: 4px 14px; font-size: 13px; color: var(--viz-muted); }
.legend li { margin: 0; display: inline-flex; align-items: center; gap: 5px; }
.ln { display: inline-block; width: 24px; height: 0; border-top: 3px solid var(--viz-muted); }
.ln.added { border-color: var(--viz-text); }
.ln.ok { border-color: var(--viz-response); border-top-width: 4px; }
.ln.bad { border-color: var(--viz-error); border-top-width: 4px; border-top-style: dashed; }
</style>
