<script setup lang="ts">
import { computed, ref } from 'vue'
import { LATENCIES, LATENCY_SOURCE, barWidth, compare, humanSeconds, type LatencyKind } from '../simulations/latency'
import { formatNs, formatSeconds } from '../core/format'

type Mode = 'real' | 'human'
const mode = ref<Mode>('real')
const baselineId = ref('ram')
const baseline = computed(() => LATENCIES.find((l) => l.id === baselineId.value) ?? LATENCIES[0]!)

const KIND_COLOR: Record<LatencyKind, string> = {
  cpu: 'var(--viz-service)',
  memory: 'var(--viz-cache)',
  disk: 'var(--viz-db)',
  network: 'var(--viz-network)',
}
const KIND_LABEL: Record<LatencyKind, string> = { cpu: 'процессор', memory: 'память', disk: 'диск', network: 'сеть' }

const value = (ns: number): string => (mode.value === 'real' ? formatNs(ns) : formatSeconds(humanSeconds(ns)))
</script>

<template>
  <section class="latency" aria-label="Шкала задержек">
    <div class="toolbar">
      <div class="modes" role="group" aria-label="Масштаб">
        <button type="button" :aria-pressed="mode === 'real'" :class="{ on: mode === 'real' }" @click="mode = 'real'">Реальное время</button>
        <button type="button" :aria-pressed="mode === 'human'" :class="{ on: mode === 'human' }" @click="mode = 'human'">Человеческий масштаб: 1 нс = 1 с</button>
      </div>
      <label class="baseline">
        Сравнивать с:
        <select v-model="baselineId">
          <option v-for="l in LATENCIES" :key="l.id" :value="l.id">{{ l.label }}</option>
        </select>
      </label>
    </div>
    <ol class="rows">
      <li v-for="l in LATENCIES" :key="l.id" :class="{ base: l.id === baselineId }">
        <div class="row-head">
          <span class="label">{{ l.label }}</span>
          <strong class="value">{{ value(l.ns) }}</strong>
        </div>
        <div class="track" aria-hidden="true">
          <div class="bar" :style="{ width: `${barWidth(l.ns) * 100}%`, background: KIND_COLOR[l.kind] }" />
        </div>
        <span class="meta">{{ KIND_LABEL[l.kind] }} · {{ compare(l, baseline) }}</span>
      </li>
    </ol>
    <p class="note">
      Длина полос — в логарифмической шкале: каждые ~12% ширины означают ×10. Значения — порядки величин, источник:
      <a :href="LATENCY_SOURCE.url" target="_blank" rel="noopener">{{ LATENCY_SOURCE.title }}</a>.
    </p>
  </section>
</template>

<style scoped>
.latency { margin: 24px 0; padding: 16px; border: 1px solid var(--vp-c-divider); border-radius: 12px; background: var(--vp-c-bg-soft); }
.toolbar { display: flex; flex-wrap: wrap; gap: 12px; justify-content: space-between; align-items: center; }
.modes { display: flex; flex-wrap: wrap; gap: 6px; }
.modes button { padding: 6px 12px; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); font-size: 14px; }
.modes button.on { border-color: var(--vp-c-brand-1); color: var(--vp-c-brand-1); background: var(--vp-c-brand-soft); }
/* Подписи вариантов длинные: без min-width/max-width select растягивает страницу на узком экране. */
.baseline { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; min-width: 0; max-width: 100%; font-size: 14px; }
.baseline select { min-width: 0; max-width: 100%; padding: 4px 6px; border: 1px solid var(--vp-c-divider); border-radius: 6px; background: var(--vp-c-bg); }
.rows { list-style: none; padding: 0; margin: 16px 0 0; }
.rows li { margin: 0 0 14px; padding: 6px 8px; border-radius: 8px; }
.rows li.base { background: var(--viz-active-line-bg); }
.row-head { display: flex; justify-content: space-between; gap: 12px; font-size: 14px; }
.value { font-variant-numeric: tabular-nums; white-space: nowrap; }
.track { height: 10px; margin: 6px 0 4px; border-radius: 5px; background: var(--vp-c-divider); }
.bar { height: 100%; border-radius: 5px; transition: width 0.3s; }
.meta { color: var(--vp-c-text-2); font-size: 12px; }
.note { color: var(--vp-c-text-2); font-size: 13px; }
@media (prefers-reduced-motion: reduce) { .bar { transition: none; } }
</style>
