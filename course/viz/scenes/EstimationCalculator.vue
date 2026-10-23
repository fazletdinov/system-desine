<script setup lang="ts">
import VizFrame from '../components/VizFrame.vue'
import ExplainAside from '../components/ExplainAside.vue'
import CaptionText from '../components/CaptionText.vue'
import StepQuiz from '../components/StepQuiz.vue'
import { computed, reactive, ref } from 'vue'
import { captionText } from '../core/caption'
import { formatNumber } from '../core/format'
import { explainIdFromEvent } from '../core/selection'
import {
  DAU_SLIDER,
  ESTIMATION_HONESTY,
  ESTIMATION_QUIZ,
  INPUT_FIELDS,
  NUMERIC_RANGE,
  PRESETS,
  dauToSlider,
  estimate,
  estimationContext,
  formatStepValue,
  sanitizeInput,
  sliderToDau,
  type EstimationInput,
  type PresetId,
} from '../simulations/estimation'
import { ESTIMATION_SIMPLIFICATIONS, explainEstimation } from '../simulations/estimation-explain'

const props = withDefaults(defineProps<{ preset?: PresetId }>(), { preset: 'default' })
const input = reactive<EstimationInput>({ ...PRESETS[props.preset] })
const steps = computed(() => estimate(input))
// Подписи показывают значения, с которыми реально идёт расчёт: пустое поле — это 0 (или 1 для коэффициентов), а не NaN.
const effective = computed(() => sanitizeInput(input))
const dauPosition = computed({
  get: () => dauToSlider(input.dau),
  set: (pos: number) => { input.dau = sliderToDau(pos) },
})
const context = computed(() => estimationContext(props.preset))

const selected = ref<string | null>(null)
// computed от выбора и входов: двинули ползунок — числа в панели пересчитались вместе с шагами.
const explanation = computed(() => (selected.value === null ? null : explainEstimation(selected.value, input)))

const pick = (e: Event): void => {
  const id = explainIdFromEvent(e)
  if (id !== null) selected.value = id
}
// Enter и пробел — только на самой карточке шага: на ссылке-термине внутри Enter должен открыть глоссарий.
const onKey = (e: KeyboardEvent): void => {
  if ((e.key !== 'Enter' && e.key !== ' ') || e.target !== e.currentTarget) return
  e.preventDefault()
  pick(e)
}

function reset(): void {
  Object.assign(input, PRESETS[props.preset])
}
</script>

<template>
  <VizFrame label="Калькулятор оценки нагрузки">
    <section class="calc" aria-label="Калькулятор оценки нагрузки">
      <p class="context"><CaptionText :caption="context" /></p>
      <p class="honesty"><CaptionText :caption="ESTIMATION_HONESTY" /></p>
      <ExplainAside :explanation="explanation" :simplifications="ESTIMATION_SIMPLIFICATIONS" fallback="Нажмите на вход или результат, чтобы увидеть объяснение">
        <div class="grid">
          <!-- Вход объясняется при любом касании: клик по подписи, ползунку или полю, фокус с клавиатуры. -->
          <div class="inputs">
            <div
              v-for="f in INPUT_FIELDS"
              :key="f.key"
              class="field"
              :class="{ picked: selected === `in:${f.key}` }"
              :data-explain="`in:${f.key}`"
              @click="pick"
              @focusin="pick"
            >
              <span class="name"><CaptionText :caption="f.label" />: <strong class="num">{{ formatNumber(effective[f.key], f.key === 'dau' ? 0 : 1) }}</strong></span>
              <input
                v-if="f.key === 'dau'"
                v-model.number="dauPosition"
                type="range"
                :min="DAU_SLIDER.min"
                :max="DAU_SLIDER.max"
                :step="DAU_SLIDER.step"
                :aria-label="captionText(f.label)"
              />
              <span v-else class="pair">
                <input v-model.number="input[f.key]" type="range" v-bind="NUMERIC_RANGE[f.key]" :aria-label="captionText(f.label)" />
                <input v-model.number="input[f.key]" type="number" v-bind="NUMERIC_RANGE[f.key]" :aria-label="`${captionText(f.label)}, число`" />
              </span>
              <span class="hint">{{ f.hint }}</span>
            </div>
            <button type="button" class="reset" @click="reset">Вернуть исходные значения</button>
          </div>
          <ol class="steps">
            <li v-for="(s, i) in steps" :key="s.id">
              <div
                class="step"
                :class="{ picked: selected === `step:${s.id}` }"
                role="button"
                tabindex="0"
                :aria-label="`Шаг ${i + 1}, ${captionText(s.label)}: объяснение`"
                :aria-pressed="selected === `step:${s.id}`"
                :data-explain="`step:${s.id}`"
                @click="pick"
                @keydown="onKey"
              >
                <span class="no" aria-hidden="true">{{ i + 1 }}</span>
                <div class="body">
                  <p class="label"><CaptionText :caption="s.label" />: <strong class="num">{{ formatStepValue(s) }}</strong></p>
                  <p class="formula">{{ s.formula }}</p>
                  <p class="expr">{{ s.expression }}</p>
                </div>
              </div>
            </li>
          </ol>
        </div>
      </ExplainAside>
      <StepQuiz :quiz="ESTIMATION_QUIZ" />
    </section>
  </VizFrame>
</template>

<style scoped>
/* Именованный контейнер: две колонки зависят от ширины самой сцены (колонка урока ≈ 690 px — одна, полный экран — две), а не окна. */
.calc { container: estimation / inline-size; min-width: 0; margin: 24px 0; padding: 16px; border: 1px solid var(--vp-c-divider); border-radius: 12px; background: var(--vp-c-bg-soft); color: var(--viz-text); }
.context { margin: 0 0 4px; font-size: 14px; line-height: 1.5; }
.honesty { margin: 0 0 12px; color: var(--viz-muted); font-size: 13px; }
.grid { display: grid; gap: 12px 20px; min-width: 0; }
@container estimation (min-width: 900px) { .grid { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); } }
.inputs, .steps { min-width: 0; }
/* Рамка той же толщины у всех: выбор меняет только цвет и стиль, раскладка не прыгает. */
.field, .step { border: 2px solid transparent; border-radius: 8px; cursor: pointer; }
.field:hover, .step:hover { border-color: var(--vp-c-divider); }
.field.picked, .step.picked { border-style: dashed; border-color: var(--viz-text); }
.field { display: flex; flex-direction: column; gap: 4px; margin-bottom: 6px; padding: 4px 6px; font-size: 14px; }
.pair { display: flex; gap: 8px; align-items: center; }
.pair input[type='range'], .field > input[type='range'] { flex: 1; min-width: 0; }
.pair input[type='number'] { flex: none; width: 96px; padding: 4px 6px; border: 1px solid var(--vp-c-divider); border-radius: 6px; background: var(--vp-c-bg); color: var(--viz-text); }
.field input:focus-visible { outline: 2px solid var(--viz-text); outline-offset: 2px; }
.hint { color: var(--viz-muted); font-size: 12px; }
.reset { margin: 4px 0 0 6px; padding: 6px 12px; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--viz-text); font-size: 14px; cursor: pointer; }
.reset:hover { border-color: var(--vp-c-brand-1); }
.reset:focus-visible { outline: 2px solid var(--viz-text); outline-offset: 2px; }
.steps { list-style: none; margin: 0; padding: 0; }
.steps li { margin: 0 0 6px; }
.step { display: flex; gap: 12px; padding: 4px 6px; }
.step:focus-visible { outline: 2px solid var(--viz-text); outline-offset: 2px; }
.body { min-width: 0; }
/* Номер — текст на нейтральном фоне с рамкой: синий на синеватом читался плохо (инвентаризация 1.4, M7). */
.no { flex: none; display: grid; place-items: center; width: 28px; height: 28px; border: 1px solid var(--vp-c-divider); border-radius: 50%; background: var(--vp-c-bg); color: var(--viz-text); font-weight: 600; }
.steps p { margin: 0; }
/* Число целиком: разряды разделены неразрывным пробелом, nowrap не даёт рвать «4 000 000 000» и «28 МБ/с». */
.num { white-space: nowrap; font-variant-numeric: tabular-nums; }
.formula { color: var(--viz-muted); font-size: 13px; }
/* Перенос только по обычным пробелам вокруг «×», «÷», «=»; break-word рвёт слово, лишь если оно шире строки. */
.expr { font-family: var(--vp-font-family-mono); font-size: 13px; overflow-wrap: break-word; }
</style>
