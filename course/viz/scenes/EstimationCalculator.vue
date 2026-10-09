<script setup lang="ts">
import { computed, reactive } from 'vue'
import { DAU_SLIDER, PRESETS, dauToSlider, estimate, formatStepValue, sanitizeInput, sliderToDau, type EstimationInput, type PresetId } from '../simulations/estimation'
import { formatNumber } from '../core/format'

const props = withDefaults(defineProps<{ preset?: PresetId }>(), { preset: 'default' })
const input = reactive<EstimationInput>({ ...PRESETS[props.preset] })
const steps = computed(() => estimate(input))
// Подписи показывают значения, с которыми реально идёт расчёт: пустое поле — это 0 (или 1 для коэффициентов), а не NaN.
const effective = computed(() => sanitizeInput(input))
const dauPosition = computed({
  get: () => dauToSlider(input.dau),
  set: (pos: number) => { input.dau = sliderToDau(pos) },
})

type NumericField = Exclude<keyof EstimationInput, 'dau'>
const FIELDS: readonly { key: NumericField; label: string; min: number; max: number; step: number; hint: string }[] = [
  { key: 'messagesPerUser', label: 'Сообщений на пользователя в день', min: 1, max: 200, step: 1, hint: 'Сколько сообщений в среднем отправляет один активный пользователь.' },
  { key: 'messageBytes', label: 'Размер сообщения с метаданными, байт', min: 10, max: 5_000, step: 10, hint: 'Текст плюс id, автор, чат, время, статус.' },
  { key: 'peakFactor', label: 'Коэффициент пика', min: 1, max: 10, step: 0.5, hint: 'Во сколько раз пик выше среднего.' },
  { key: 'retentionDays', label: 'Срок хранения, дней', min: 1, max: 3_650, step: 1, hint: '365 — год, 1825 — пять лет.' },
  { key: 'replicationFactor', label: 'Коэффициент репликации', min: 1, max: 5, step: 1, hint: 'Сколько копий данных храним.' },
]

function reset(): void {
  Object.assign(input, PRESETS[props.preset])
}
</script>

<template>
  <section class="calc" aria-label="Калькулятор оценки нагрузки">
    <div class="inputs">
      <label class="field">
        <span class="name">Активных пользователей в день (DAU): <strong>{{ formatNumber(input.dau) }}</strong></span>
        <input v-model.number="dauPosition" type="range" :min="DAU_SLIDER.min" :max="DAU_SLIDER.max" :step="DAU_SLIDER.step" />
        <span class="hint">Шкала логарифмическая: от тысячи до миллиарда.</span>
      </label>
      <label v-for="f in FIELDS" :key="f.key" class="field">
        <span class="name">{{ f.label }}: <strong>{{ formatNumber(effective[f.key], 1) }}</strong></span>
        <span class="pair">
          <input v-model.number="input[f.key]" type="range" :min="f.min" :max="f.max" :step="f.step" />
          <input v-model.number="input[f.key]" type="number" :min="f.min" :max="f.max" :step="f.step" :aria-label="f.label" />
        </span>
        <span class="hint">{{ f.hint }}</span>
      </label>
      <button type="button" class="reset" @click="reset">Вернуть исходные значения</button>
    </div>
    <ol class="steps" aria-live="polite">
      <li v-for="(s, i) in steps" :key="s.id">
        <span class="no">{{ i + 1 }}</span>
        <div>
          <p class="label">{{ s.label }}: <strong>{{ formatStepValue(s) }}</strong></p>
          <p class="formula">{{ s.formula }}</p>
          <p class="expr">{{ s.expression }}</p>
        </div>
      </li>
    </ol>
  </section>
</template>

<style scoped>
.calc { display: grid; min-width: 0; gap: 20px; margin: 24px 0; padding: 16px; border: 1px solid var(--vp-c-divider); border-radius: 12px; background: var(--vp-c-bg-soft); }
@media (min-width: 960px) { .calc { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); } }
.field { display: flex; flex-direction: column; gap: 4px; margin-bottom: 14px; font-size: 14px; }
.pair { display: flex; gap: 8px; align-items: center; }
.pair input[type='range'], .field > input[type='range'] { flex: 1; min-width: 0; }
.pair input[type='number'] { flex: none; width: 96px; padding: 4px 6px; border: 1px solid var(--vp-c-divider); border-radius: 6px; background: var(--vp-c-bg); color: var(--vp-c-text-1); }
.hint { color: var(--vp-c-text-3); font-size: 12px; }
.reset { padding: 6px 12px; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font-size: 14px; cursor: pointer; }
.reset:hover { border-color: var(--vp-c-brand-1); }
.steps { list-style: none; margin: 0; padding: 0; }
.steps li { display: flex; gap: 12px; margin-bottom: 14px; }
.no { flex: none; display: grid; place-items: center; width: 28px; height: 28px; border-radius: 50%; background: var(--vp-c-brand-soft); color: var(--vp-c-brand-1); font-weight: 600; }
.steps p { margin: 0; }
.label strong { white-space: nowrap; }
.formula { color: var(--vp-c-text-2); font-size: 13px; }
.expr { font-family: var(--vp-font-family-mono); font-size: 13px; overflow-wrap: anywhere; }
</style>
