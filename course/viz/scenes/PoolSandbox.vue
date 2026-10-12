<script setup lang="ts">
import { computed, reactive } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import PoolView from '../renderers/PoolView.vue'
import { POOL_PRESETS, poolTimeline, sanitizePool, type PoolInput, type PoolPresetId } from '../simulations/pool'

const props = withDefaults(defineProps<{ preset?: PoolPresetId }>(), { preset: 'default' })
const input = reactive<PoolInput>({ ...POOL_PRESETS[props.preset] })
const effective = computed(() => sanitizePool(input))
const result = computed(() => poolTimeline(input))

type NumericField = 'rttMs' | 'requests' | 'concurrency' | 'maxIdlePerHost'
const FIELDS: readonly { key: NumericField; id: string; label: string; unit: string; min: number; max: number; hint?: string }[] = [
  { key: 'rttMs', id: 'pool-rtt', label: 'RTT', unit: ' мс', min: 5, max: 200 },
  { key: 'requests', id: 'pool-req', label: 'Запросов', unit: '', min: 1, max: 200 },
  { key: 'concurrency', id: 'pool-conc', label: 'Параллельность (воркеров)', unit: '', min: 1, max: 50 },
  { key: 'maxIdlePerHost', id: 'pool-idle', label: 'MaxIdleConnsPerHost', unit: '', min: 0, max: 50, hint: 'в Go по умолчанию — 2' },
]

function reset(): void {
  Object.assign(input, POOL_PRESETS[props.preset])
}
</script>

<template>
  <VizFrame label="Песочница пула соединений">
    <p class="intro">Каждый воркер шлёт запросы по очереди; запрос на новом соединении стоит TCP + TLS + запрос, на готовом — один RTT. Время считается в целых RTT.</p>
    <div class="panel" role="group" aria-label="Настройки песочницы пула">
      <div v-for="f in FIELDS" :key="f.key" class="row">
        <label class="row-label" :for="f.id">{{ f.label }}: <strong>{{ effective[f.key] }}{{ f.unit }}</strong></label>
        <input :id="f.id" v-model.number="input[f.key]" type="range" :min="f.min" :max="f.max" step="1" />
        <span v-if="f.hint" class="hint">{{ f.hint }}</span>
      </div>
      <div class="row">
        <button type="button" class="toggle" :aria-pressed="input.tls" @click="input.tls = !input.tls">{{ input.tls ? '✓' : '✕' }} TLS</button>
        <button type="button" class="toggle" :aria-pressed="input.keepAlive" @click="input.keepAlive = !input.keepAlive">{{ input.keepAlive ? '✓' : '✕' }} keep-alive</button>
        <button type="button" class="reset" @click="reset">Сброс</button>
      </div>
    </div>
    <PoolView :result="result" />
    <p class="caveat">Модель упрощена: в настоящем net/http пул ещё и «перехватывает» освободившиеся соединения, а лишние фоновые подключения закрываются — см. лабораторную урока 0b.6.</p>
  </VizFrame>
</template>

<style scoped>
.intro { margin: 0 0 10px; font-size: 14px; color: var(--viz-muted); }
.panel { display: flex; flex-direction: column; gap: 6px; margin-bottom: 12px; }
.row { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; }
.row-label { font-size: 14px; color: var(--viz-text); min-width: 250px; }
.hint { font-size: 13px; color: var(--viz-muted); }
.toggle, .reset { min-width: 36px; min-height: 36px; padding: 0 12px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; cursor: pointer; }
.toggle[aria-pressed='true'] { border-color: var(--viz-request); background: color-mix(in srgb, var(--viz-request) 18%, var(--vp-c-bg)); font-weight: 700; }
.toggle:focus-visible, .reset:focus-visible, input[type='range']:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
input[type='range'] { flex: 1 1 140px; max-width: 260px; min-height: 36px; }
.caveat { margin: 10px 0 0; font-size: 13px; color: var(--viz-muted); }
</style>
