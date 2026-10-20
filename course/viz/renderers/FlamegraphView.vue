<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { prefersReducedMotion } from '../core/motion'
import {
  fitLabel,
  formatShare,
  formatValue,
  isFlameNavKey,
  neighbour,
  project,
  shareOfFocus,
  shareOfTotal,
  windowOf,
  type FlameFrame,
  type FlameNode,
  type FrameKind,
  type ViewWindow,
} from '../simulations/flamegraph'

const props = defineProps<{ frame: FlameFrame }>()
const emit = defineEmits<{ zoom: [id: string]; reset: [] }>()

// Геометрия в настоящих пикселях: viewBox равен ширине контейнера, поэтому кегль 12 —
// это 12 px на любом экране, а подпись обрезается по реальной ширине блока.
const ROW_H = 20
const FONT = 12
const CHAR_PX = 7.4 // ширина знака моноширинного шрифта при кегле 12 (0,6 em) с запасом
const MIN_PX = 1.5 // уже — блок не рисуется: его не увидеть и в него не попасть
const ZOOM_MS = 300

const KIND_TEXT: Record<FrameKind, string> = {
  messenger: 'код мессенджера',
  stdlib: 'стандартная библиотека',
  runtime: 'рантайм Go',
  external: 'сторонний модуль',
  other: 'слитые мелкие стеки',
  root: 'корень профиля',
}
const LEGEND: readonly FrameKind[] = ['messenger', 'stdlib', 'runtime', 'external', 'other']

const box = ref<HTMLDivElement>()
const svg = ref<SVGSVGElement>()
const width = ref(640) // SSR и первый кадр; на клиенте — ширина контейнера
const view = ref<ViewWindow>(windowOf(props.frame.focus))
const activeId = ref<string | null>(null)
// Блок с tabindex=0 — единственная остановка Tab в графе (roving tabindex): иначе сотни
// блоков профиля превращают Tab в сотни нажатий. По остальным блокам ходят стрелками.
const cursorId = ref<string>(props.frame.focus.id)

const tree = computed(() => props.frame.tree)
const searching = computed(() => props.frame.matches.query !== '')
const rects = computed(() =>
  project(tree.value, view.value, MIN_PX / width.value).map((r) => ({
    ...r,
    px: r.x * width.value,
    wpx: r.w * width.value,
  })),
)
const maxDepth = computed(() => rects.value.reduce((m, r) => Math.max(m, r.node.depth), 0))
const height = computed(() => (maxDepth.value + 1) * ROW_H)
// Флейм-граф растёт вверх: корень — нижняя строка, вызванные функции — над вызвавшей.
const yOf = (n: FlameNode): number => (maxDepth.value - n.depth) * ROW_H
// Курсор мог уйти из окна (зум, смена типа выборки) — тогда остановкой Tab служит увеличенный блок.
const tabStopId = computed(() => (rects.value.some((r) => r.node.id === cursorId.value) ? cursorId.value : props.frame.focus.id))

const active = computed(() => (activeId.value === null ? undefined : tree.value.byId.get(activeId.value)) ?? props.frame.focus)
const describe = (n: FlameNode): string => {
  const total = formatShare(shareOfTotal(tree.value, n))
  return n === tree.value.root ? `${n.name}: ${formatValue(n.value, tree.value.unit)}` : `${n.name}: ${total} профиля, ${formatValue(n.value, tree.value.unit)}`
}

let raf = 0
let observer: ResizeObserver | undefined
const stopAnim = (): void => {
  if (raf !== 0) cancelAnimationFrame(raf)
  raf = 0
}
// Увеличение — плавный переход окна от прежнего узла к новому; при reduced motion — сразу.
const animateTo = (target: ViewWindow): void => {
  stopAnim()
  if (prefersReducedMotion()) {
    view.value = target
    return
  }
  const from = { ...view.value }
  const t0 = performance.now()
  const tick = (now: number): void => {
    // Метка кадра rAF может быть раньше t0 из performance.now(): без нижней границы p < 0
    // выносит окно за край увеличенного узла, на узком узле оно выворачивается (end < start),
    // и на один кадр граф пустеет — все блоки пересоздаются, а фокус уходит в body.
    const p = Math.min(1, Math.max(0, (now - t0) / ZOOM_MS))
    const e = 1 - (1 - p) ** 3
    view.value = { start: from.start + (target.start - from.start) * e, end: from.end + (target.end - from.end) * e }
    raf = p < 1 ? requestAnimationFrame(tick) : 0
    // Высота графа меняется вместе с окном — после перехода снова прижимаемся к корню.
    if (p >= 1) void scrollToRoot()
  }
  raf = requestAnimationFrame(tick)
}

const scrollToRoot = async (): Promise<void> => {
  await nextTick()
  if (box.value) box.value.scrollTop = box.value.scrollHeight
}

watch(
  () => [props.frame.tree, props.frame.focus] as const,
  ([t, f], [prevTree]) => {
    activeId.value = null
    cursorId.value = f.id
    if (t !== prevTree) {
      // Другой тип выборки — другой масштаб: переход между ними ничего не объясняет.
      stopAnim()
      view.value = windowOf(f)
      void scrollToRoot()
      return
    }
    animateTo(windowOf(f))
    // Увеличенный блок — у нижнего края: под ним только предки, над ним его поддерево.
    void scrollToRoot()
  },
)

onMounted(() => {
  const el = box.value
  if (!el) return
  observer = new ResizeObserver(([entry]) => {
    if (entry && entry.contentRect.width > 0) width.value = entry.contentRect.width
  })
  observer.observe(el)
  void scrollToRoot()
})
onBeforeUnmount(() => {
  stopAnim()
  observer?.disconnect()
})

const zoom = (n: FlameNode): void => {
  if (n !== props.frame.focus) emit('zoom', n.id)
}
const onFocus = (id: string): void => {
  activeId.value = id
  cursorId.value = id
}
const focusBlock = async (id: string): Promise<void> => {
  cursorId.value = id
  await nextTick()
  svg.value?.querySelector<SVGGElement>(`[data-id="${id}"]`)?.focus()
}
// Стрелки — по видимым блокам, Home — к корню. Esc в графе — к корню; если увеличения нет,
// Esc уходит дальше (VizFrame закрывает оверлей).
const onKeydown = async (event: KeyboardEvent): Promise<void> => {
  if (isFlameNavKey(event.key)) {
    const to = neighbour(rects.value, cursorId.value, event.key)
    event.preventDefault() // стрелки не прокручивают страницу, даже на краю графа
    if (to !== undefined) await focusBlock(to.id)
    return
  }
  if (event.key !== 'Escape' || props.frame.focus === tree.value.root) return
  event.stopPropagation()
  emit('reset')
  await nextTick()
  await focusBlock(tree.value.root.id)
}
</script>

<template>
  <div class="flame">
    <div ref="box" class="graph">
      <svg
        ref="svg"
        :viewBox="`0 0 ${width} ${height}`"
        :width="width"
        :height="height"
        role="group"
        aria-label="Флейм-граф: стрелки — по блокам, Enter — увеличить блок, Esc — к корню"
        @keydown="onKeydown"
      >
        <g
          v-for="r in rects"
          :key="r.node.id"
          class="node"
          :class="[`k-${r.node.kind}`, { hit: frame.matches.ids.has(r.node.id), dim: searching && !frame.matches.ids.has(r.node.id), focus: r.node === frame.focus }]"
          :data-id="r.node.id"
          :transform="`translate(${r.px.toFixed(2)},${yOf(r.node)})`"
          :tabindex="r.node.id === tabStopId ? 0 : -1"
          role="button"
          :aria-label="describe(r.node)"
          @click="zoom(r.node)"
          @keydown.enter.prevent="zoom(r.node)"
          @keydown.space.prevent="zoom(r.node)"
          @mouseenter="activeId = r.node.id"
          @mouseleave="activeId = null"
          @focus="onFocus(r.node.id)"
          @blur="activeId = null"
        >
          <title>{{ describe(r.node) }}</title>
          <rect :width="Math.max(r.wpx - 1, 0.5)" :height="ROW_H - 1" rx="2" />
          <text x="4" :y="ROW_H / 2" dominant-baseline="central" :font-size="FONT">{{ fitLabel(r.node.name, r.wpx, CHAR_PX) }}</text>
        </g>
      </svg>
    </div>
    <!-- Без aria-live: подсказка меняется на каждом наведении мыши; программе чтения экрана
         то же имя и долю сообщает aria-label блока при фокусе. -->
    <div class="tip">
      <p class="tip-name"><span class="swatch" :class="`k-${active.kind}`" aria-hidden="true"></span><code>{{ active.name }}</code></p>
      <p class="tip-meta">
        {{ KIND_TEXT[active.kind] }} · {{ formatShare(shareOfTotal(tree, active)) }} всего профиля ({{ formatValue(active.value, tree.unit) }})
        <template v-if="frame.focus !== tree.root"> · {{ formatShare(shareOfFocus(active, frame.focus)) }} увеличенного блока</template>
        · в самой функции {{ formatShare(active.self / tree.root.value) }}
      </p>
    </div>
    <ul class="legend" aria-label="Цвета блоков">
      <li v-for="k in LEGEND" :key="k"><span class="swatch" :class="`k-${k}`" aria-hidden="true"></span>{{ KIND_TEXT[k] }}</li>
      <li><span class="swatch hit-swatch" aria-hidden="true"></span>совпадение поиска</li>
    </ul>
  </div>
</template>

<style scoped>
.flame { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
.graph { max-height: 32rem; overflow-x: hidden; overflow-y: auto; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg-soft); }
svg { display: block; font-family: var(--vp-font-family-mono); }
.node { cursor: pointer; }
.node text { fill: var(--viz-text); pointer-events: none; }
.node:focus { outline: none; }
/* Цвет — чей код: мессенджер, стандартная библиотека, рантайм, сторонний модуль (легенда). */
.k-messenger { --c: var(--viz-service); }
.k-stdlib { --c: var(--viz-client); }
.k-runtime { --c: var(--viz-network); }
.k-external { --c: var(--viz-cache); }
.node rect, .swatch { fill: color-mix(in srgb, var(--c) 35%, var(--viz-node-bg)); stroke: var(--c); stroke-width: 1; background: color-mix(in srgb, var(--c) 35%, var(--viz-node-bg)); border-color: var(--c); }
.node.k-other rect, .swatch.k-other { fill: var(--viz-node-bg); stroke: var(--viz-muted); stroke-dasharray: 4 3; background: var(--viz-node-bg); border-color: var(--viz-muted); border-style: dashed; }
.node.k-root rect, .swatch.k-root { fill: var(--viz-node-bg); stroke: var(--viz-muted); background: var(--viz-node-bg); border-color: var(--viz-muted); }
/* Состояния — после цветов, чтобы перекрыть рамку. Текст не бледнеет: контраст подписи сохраняется. */
.node.focus text { font-weight: 700; }
.node.dim rect { fill-opacity: 0.35; }
.node.hit rect { stroke: var(--viz-text); stroke-width: 2.5; }
.node.hit text { font-weight: 700; }
.node:focus-visible rect { stroke: var(--vp-c-brand-1); stroke-width: 3; }
.tip { min-height: 4.2em; padding: 6px 10px; border-left: 4px solid var(--viz-edge); background: var(--viz-node-bg); }
.tip p { margin: 0; line-height: 1.5; font-size: 14px; color: var(--viz-text); }
.tip-name { display: flex; align-items: baseline; gap: 6px; overflow-wrap: anywhere; }
.tip-name code { font-size: 13px; }
.tip .tip-meta { color: var(--viz-muted); }
.legend { display: flex; flex-wrap: wrap; gap: 4px 14px; margin: 0; padding: 0; list-style: none; font-size: 13px; color: var(--viz-text); }
.legend li { display: flex; align-items: center; gap: 6px; }
.swatch { display: inline-block; flex: none; width: 14px; height: 12px; border-width: 1px; border-style: solid; border-radius: 2px; }
.swatch.hit-swatch { border: 2.5px solid var(--viz-text); background: transparent; }
</style>
