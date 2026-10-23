<script setup lang="ts">
import VizFrame from '../components/VizFrame.vue'
import ExplainAside from '../components/ExplainAside.vue'
import SegmentedToggle from '../components/SegmentedToggle.vue'
import CaptionText from '../components/CaptionText.vue'
import StepQuiz from '../components/StepQuiz.vue'
import { computed, ref } from 'vue'
import { captionText } from '../core/caption'
import { keepDigitGroups } from '../core/format'
import { isExplainActivationKey, pickExplainId } from '../core/selection'
import {
  KIND_LABEL,
  LATENCIES,
  LATENCY_CONTEXT,
  LATENCY_NOTE,
  LATENCY_QUIZ,
  LATENCY_SOURCE,
  MODE_LABEL,
  RANGE_LINKS,
  barWidth,
  baselineOf,
  compare,
  latencyRange,
  latencyValue,
  type LatencyItem,
  type LatencyKind,
  type LatencyMode,
} from '../simulations/latency'
import { LATENCY_SIMPLIFICATIONS, explainLatency } from '../simulations/latency-explain'

const mode = ref<LatencyMode>('real')
const baselineId = ref('ram')
const baseline = computed(() => baselineOf(baselineId.value))
const selected = ref<string | null>(null)
// computed, а не вызов в шаблоне: объяснение пересобирается только при смене строки, масштаба или точки отсчёта.
const explanation = computed(() => (selected.value === null ? null : explainLatency(selected.value, { mode: mode.value, baselineId: baselineId.value })))

const MODES = (['real', 'human'] as const).map((value) => ({ value, text: MODE_LABEL[value] }))

const KIND_COLOR: Record<LatencyKind, string> = {
  cpu: 'var(--viz-service)',
  memory: 'var(--viz-cache)',
  disk: 'var(--viz-db)',
  network: 'var(--viz-network)',
}

// Вторая строка под полосой: откуда главное число, число модели (если главное — замер) и диапазон.
// «реально» — только у диапазона с документом; без него — «оценка».
const realLine = (l: LatencyItem): string => {
  const parts = [l.typical.source]
  if (l.author) parts.push(`модель Scott ${latencyValue(l.model.ns, mode.value)}`)
  if (l.range) parts.push(`${l.range.estimate ? 'оценка' : 'реально'} ${latencyRange(l.range, mode.value)}`)
  return parts.join(' · ')
}

const pick = (e: Event): void => { selected.value = pickExplainId(e, selected.value) }
const onKey = (e: KeyboardEvent): void => {
  if (!isExplainActivationKey(e)) return
  // Пробел иначе прокрутил бы страницу.
  e.preventDefault()
  pick(e)
}
</script>

<template>
  <VizFrame label="Шкала задержек">
    <section class="latency" aria-label="Шкала задержек">
      <p class="context"><CaptionText :caption="LATENCY_CONTEXT" /></p>
      <ExplainAside :explanation="explanation" :simplifications="LATENCY_SIMPLIFICATIONS" fallback="Нажмите на строку шкалы, чтобы увидеть объяснение">
        <div class="toolbar">
          <SegmentedToggle v-model="mode" label="Масштаб" :options="MODES" />
          <label class="baseline">
            Сравнивать с:
            <select v-model="baselineId">
              <option v-for="l in LATENCIES" :key="l.id" :value="l.id">{{ captionText(l.label) }}</option>
            </select>
          </label>
        </div>
        <ol class="rows">
          <li v-for="l in LATENCIES" :key="l.id">
            <div
              class="row"
              :class="{ base: l.id === baselineId, picked: selected === `row:${l.id}` }"
              role="button"
              tabindex="0"
              :aria-label="`${captionText(l.label)}: объяснение`"
              :aria-pressed="selected === `row:${l.id}`"
              :data-explain="`row:${l.id}`"
              @click="pick"
              @keydown="onKey"
            >
              <div class="row-head">
                <span class="label"><CaptionText :caption="l.label" /></span>
                <strong class="value">{{ latencyValue(l.typical.ns, mode) }}</strong>
              </div>
              <div class="track" aria-hidden="true">
                <div class="bar" :style="{ width: `${barWidth(l.typical.ns) * 100}%`, background: KIND_COLOR[l.kind] }" />
              </div>
              <div class="meta">
                <span>{{ KIND_LABEL[l.kind] }} · {{ compare(l, baseline) }}</span>
                <span>{{ keepDigitGroups(realLine(l)) }}</span>
              </div>
              <span class="meta messenger">В мессенджере: {{ keepDigitGroups(l.messenger) }}</span>
            </div>
          </li>
        </ol>
        <p class="note">
          <CaptionText :caption="LATENCY_NOTE" /> «У автора» — лаба 0.4 (labs/0-4/results/). Модель:
          <a :href="LATENCY_SOURCE.url" target="_blank" rel="noopener">{{ keepDigitGroups(LATENCY_SOURCE.title) }}</a>.
        </p>
        <details class="sources">
          <summary>Источники диапазонов</summary>
          <ul>
            <li v-for="k in RANGE_LINKS" :key="k.url"><a :href="k.url" target="_blank" rel="noopener">{{ keepDigitGroups(k.title) }}</a></li>
          </ul>
        </details>
      </ExplainAside>
      <StepQuiz :quiz="LATENCY_QUIZ" />
    </section>
  </VizFrame>
</template>

<style scoped>
.latency { margin: 24px 0; padding: 16px; border: 1px solid var(--vp-c-divider); border-radius: 12px; background: var(--vp-c-bg-soft); color: var(--viz-text); }
.context { margin: 0 0 12px; font-size: 14px; line-height: 1.5; }
.toolbar { display: flex; flex-wrap: wrap; gap: 4px 12px; justify-content: space-between; align-items: center; }
/* Подписи вариантов длинные: без min-width/max-width select растягивает страницу на узком экране. */
.baseline { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; min-width: 0; max-width: 100%; margin-bottom: 10px; font-size: 14px; }
.baseline select { min-width: 0; max-width: 100%; padding: 4px 6px; border: 1px solid var(--vp-c-divider); border-radius: 6px; background: var(--vp-c-bg); color: var(--viz-text); }
.rows { list-style: none; padding: 0; margin: 8px 0 0; }
.rows li { margin: 0 0 6px; }
/* Рамка той же толщины у всех строк: точка отсчёта и выбранная строка меняют только цвет, без сдвига раскладки. */
.row { padding: 6px 8px; border: 2px solid transparent; border-radius: 8px; cursor: pointer; }
.row:hover { border-color: var(--vp-c-divider); }
.row:focus-visible { outline: 2px solid var(--viz-text); outline-offset: 2px; }
/* Точка отсчёта — рамкой, а не заливкой: на заливке --viz-muted в светлой теме даёт 4,33:1 (инвентаризация, M7).
   Цвет рамки — --viz-active-border: у --viz-active на светлом фоне ~2:1, а рамке нужно ≥ 3:1. */
.row.base { border-color: var(--viz-active-border); }
.row.picked { border-style: dashed; border-color: var(--viz-text); }
.row.base.picked { border-style: solid; border-color: var(--viz-active-border); box-shadow: inset 0 0 0 1px var(--viz-text); }
.row-head { display: flex; justify-content: space-between; gap: 12px; font-size: 14px; }
.value { font-variant-numeric: tabular-nums; white-space: nowrap; }
.track { height: 10px; margin: 6px 0 4px; border-radius: 5px; background: var(--vp-c-divider); }
.bar { height: 100%; border-radius: 5px; transition: width 0.3s; }
.meta { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 0 12px; color: var(--viz-muted); font-size: 12px; line-height: 1.5; }
.messenger { display: block; }
.note { margin: 8px 0 0; color: var(--viz-muted); font-size: 13px; }
/* Ссылка — цветом основного текста с подчёркиванием: синий текст на тёмно-синем фоне плохо читается (§4.4). */
.note a, .sources a { color: var(--viz-text); text-decoration: underline; }
.sources { margin: 6px 0 0; color: var(--viz-muted); font-size: 13px; }
.sources summary { cursor: pointer; }
.sources summary:focus-visible { outline: 2px solid var(--viz-text); outline-offset: 2px; }
.sources ul { margin: 6px 0 0; padding-left: 18px; }
.sources li { overflow-wrap: anywhere; }
@media (prefers-reduced-motion: reduce) { .bar { transition: none; } }
</style>
