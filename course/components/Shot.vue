<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import VizFrame from '../viz/components/VizFrame.vue'
import { layoutMarks, MARK_RADIUS_SHARE, textSegments, type ShotMark } from './shot'

const props = defineProps<{
  // Путь от корня сайта: /shots/… (файл лежит в course/public/shots/…).
  src: string
  alt: string
  // Размеры файла в пикселях: из них viewBox пометок, и страница не прыгает при загрузке.
  width: number
  height: number
  caption: string
  // Программа и версия, на которой сделан снимок: «Wireshark 4.6.6».
  version: string
  marks: ShotMark[]
}>()

const id = useId()
const frame = ref<InstanceType<typeof VizFrame>>()
// Номер, на который сейчас наведены — в списке или на снимке: подсвечиваем пару.
const active = ref<number | null>(null)

// VitePress выставляет base сайта в Vite: на GitHub Pages снимок живёт под /<repo>/.
const url = computed(() => `${import.meta.env.BASE_URL}${props.src.replace(/^\//, '')}`)
const layout = computed(() => layoutMarks(props.marks, props.width, props.height))
const notes = computed(() => [...props.marks].sort((a, b) => a.n - b.n))
const r = computed(() => props.width * MARK_RADIUS_SHARE)
const noteId = (n: number): string => `${id}-note-${n}`
</script>

<template>
  <VizFrame ref="frame" :label="`Снимок: ${caption}`">
    <figure class="shot">
      <div class="shot-canvas" title="Открыть на весь экран" @click="frame?.toggle()">
        <img :src="url" :alt="alt" :width="width" :height="height" loading="lazy" decoding="async" />
        <svg class="shot-marks" :viewBox="`0 0 ${width} ${height}`">
          <g
            v-for="m in layout"
            :key="m.n"
            class="shot-mark"
            :class="{ 'is-active': active === m.n }"
            :data-mark="m.n"
            role="img"
            :aria-label="`Пометка ${m.n}`"
            :aria-describedby="noteId(m.n)"
            @mouseenter="active = m.n"
            @mouseleave="active = null"
          >
            <template v-if="m.rect !== null">
              <rect class="shot-rect-halo" :x="m.rect.x" :y="m.rect.y" :width="m.rect.w" :height="m.rect.h" :rx="r / 4" />
              <rect class="shot-rect" :x="m.rect.x" :y="m.rect.y" :width="m.rect.w" :height="m.rect.h" :rx="r / 4" />
            </template>
            <circle class="shot-dot" :cx="m.cx" :cy="m.cy" :r="r" />
            <text class="shot-num" :x="m.cx" :y="m.cy" :font-size="r * 1.25">{{ m.n }}</text>
          </g>
        </svg>
      </div>
      <figcaption class="shot-caption">
        {{ caption }}
        <span class="shot-version">{{ version }}</span>
      </figcaption>
      <ol class="shot-notes">
        <li
          v-for="m in notes"
          :id="noteId(m.n)"
          :key="m.n"
          :value="m.n"
          :data-mark="m.n"
          :class="{ 'is-active': active === m.n }"
          @mouseenter="active = m.n"
          @mouseleave="active = null"
        >
          <span class="shot-badge" aria-hidden="true">{{ m.n }}</span>
          <span><template v-for="(s, i) in textSegments(m.text)" :key="i"><code v-if="s.code">{{ s.text }}</code><template v-else>{{ s.text }}</template></template></span>
        </li>
      </ol>
    </figure>
  </VizFrame>
</template>

<style scoped>
.shot { margin: 16px 0 24px; }
.shot-canvas {
  position: relative;
  width: fit-content;
  max-width: 100%;
  margin: 0 auto;
  border: 1px solid var(--vp-c-divider);
  border-radius: 8px;
  overflow: hidden;
  cursor: zoom-in;
  line-height: 0;
}
.shot-canvas img { display: block; max-width: 100%; height: auto; }
.shot-marks { position: absolute; inset: 0; width: 100%; height: 100%; overflow: visible; }
/* Подсветка — акцент курса. Ореол цвета фона под линией и под кружком отделяет пометку от любого снимка: светлого и тёмного. */
.shot-rect-halo { fill: none; stroke: var(--vp-c-bg); stroke-width: 4.5px; vector-effect: non-scaling-stroke; }
.shot-rect { fill: none; stroke: var(--course-accent); stroke-width: 2px; vector-effect: non-scaling-stroke; }
.shot-dot { fill: var(--course-accent); stroke: var(--vp-c-bg); stroke-width: 3px; vector-effect: non-scaling-stroke; }
.shot-num { fill: var(--vp-c-bg); font-family: var(--vp-font-family-base); font-weight: 700; text-anchor: middle; dominant-baseline: central; }
.shot-mark.is-active .shot-rect { stroke-width: 3.5px; }
.shot-mark.is-active .shot-dot { stroke-width: 5px; }
.shot-caption { margin-top: 8px; color: var(--vp-c-text-2); font-size: 14px; text-align: center; }
.shot-version { display: block; color: var(--vp-c-text-3); font-size: 12px; }
.shot-notes { margin: 12px 0 0; padding: 0; list-style: none; }
.shot-notes li { display: flex; gap: 10px; align-items: baseline; margin: 6px 0; padding: 2px 6px; border-radius: 6px; }
.shot-notes li.is-active { background: var(--course-accent-soft); }
.shot-badge {
  flex: none;
  display: inline-grid;
  place-items: center;
  width: 1.5em;
  height: 1.5em;
  border-radius: 50%;
  background: var(--course-accent);
  color: var(--vp-c-bg);
  font-size: 13px;
  font-weight: 700;
  line-height: 1;
}
</style>

<style>
/* На весь экран (класс ставит VizFrame): снимок по высоте окна, пояснения — колонкой справа.
   Отдельный блок без scoped: :global() в scoped-стилях Vue теряет вторую часть селектора. */
.viz-frame.is-fullscreen .shot { display: grid; grid-template-columns: minmax(0, 1fr) minmax(240px, 300px); gap: 8px 20px; align-items: start; margin: 0; }
.viz-frame.is-fullscreen .shot-canvas { grid-row: span 2; cursor: zoom-out; }
.viz-frame.is-fullscreen .shot-canvas img { max-height: calc(100vh - 90px); width: auto; }
/* VizFrame ограничивает высоту любого svg в полноэкранном режиме; слой пометок обязан совпадать с картинкой. */
.viz-frame.is-fullscreen .shot-marks { max-height: none; }
.viz-frame.is-fullscreen .shot-caption { margin-top: 0; text-align: left; }
.viz-frame.is-fullscreen .shot-notes { margin-top: 0; }
</style>
