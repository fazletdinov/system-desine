<script setup lang="ts" generic="S">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { Step } from '../core/types'
import CaptionText from './CaptionText.vue'
import StepCard from './StepCard.vue'
import StepQuiz from './StepQuiz.vue'
import { SPEEDS, initPlayer, keyToAction, playerReducer, stepDelayMs, type PlayerAction } from '../core/player'

const props = defineProps<{ steps: readonly Step<S>[]; label: string; startAtEnd?: boolean }>()

const emit = defineEmits<{ step: [index: number] }>()

const state = ref(initPlayer(props.steps.length))
const dispatch = (action: PlayerAction): void => {
  state.value = playerReducer(state.value, action)
}
const step = computed(() => props.steps[state.value.index] as Step<S>)
const isFirst = computed(() => state.value.index === 0)
const isLast = computed(() => state.value.index === state.value.total - 1)

// startAtEnd: песочница, где каждое действие добавляет шаг, показывает его результат, а не начало.
// stepsGeneration сбрасывает вопрос шага: после замены шагов индекс может остаться тем же.
const stepsGeneration = ref(0)
watch(() => props.steps, (steps) => {
  stepsGeneration.value++
  dispatch({ type: 'reset', total: steps.length })
  if (props.startAtEnd) dispatch({ type: 'last' })
  // Индекс мог не измениться (0 → 0), а шаги — другие: потребитель должен узнать об этом.
  emit('step', state.value.index)
})
onMounted(() => emit('step', state.value.index))

// Сцены со второй колонкой (панель объяснения) следят за шагом снаружи.
watch(() => state.value.index, (index) => emit('step', index))

let timer: ReturnType<typeof setTimeout> | undefined
watch(
  () => [state.value.playing, state.value.index, state.value.speed] as const,
  ([playing, , speed]) => {
    clearTimeout(timer)
    if (playing) timer = setTimeout(() => dispatch({ type: 'tick' }), stepDelayMs(speed))
  },
)
onBeforeUnmount(() => clearTimeout(timer))

function onKey(event: KeyboardEvent): void {
  const action = keyToAction(event.key, event.target instanceof HTMLElement ? event.target : null, event)
  if (action === null) return
  event.preventDefault()
  dispatch(action)
}

function onSeek(event: Event): void {
  dispatch({ type: 'seek', index: Number((event.target as HTMLInputElement).value) - 1 })
}

function onSpeed(event: Event): void {
  const speed = SPEEDS.find((s) => s === Number((event.target as HTMLSelectElement).value))
  if (speed !== undefined) dispatch({ type: 'speed', speed })
}
</script>

<template>
  <figure class="viz-player" tabindex="0" :aria-label="label" @keydown="onKey">
    <div class="viz-stage" :class="{ 'has-code': $slots.code, 'has-side': $slots.side }">
      <div class="viz-canvas"><slot :step="step" :index="state.index" /></div>
      <div v-if="$slots.side" class="viz-side"><slot name="side" :step="step" :index="state.index" /></div>
      <div v-if="$slots.code" class="viz-code"><slot name="code" :line="step.codeLine" /></div>
    </div>
    <figcaption class="viz-caption" aria-live="polite">
      <span class="viz-step-no">Шаг {{ state.index + 1 }} из {{ state.total }}</span>
      <CaptionText :caption="step.caption" />
    </figcaption>
    <StepCard v-if="step.card" :card="step.card" />
    <StepQuiz v-if="step.quiz" :key="`${stepsGeneration}-${state.index}`" :quiz="step.quiz" />
    <div class="viz-controls" role="group" aria-label="Управление визуализацией">
      <button type="button" :disabled="isFirst" aria-label="В начало" @click="dispatch({ type: 'first' })">⏮</button>
      <button type="button" :disabled="isFirst" aria-label="Шаг назад" @click="dispatch({ type: 'prev' })">◀</button>
      <button type="button" class="viz-play" :aria-label="state.playing ? 'Пауза' : 'Воспроизвести'" @click="dispatch({ type: 'toggle' })">
        {{ state.playing ? '⏸' : '▶' }}
      </button>
      <button type="button" :disabled="isLast" aria-label="Шаг вперёд" @click="dispatch({ type: 'next' })">▶▏</button>
      <button type="button" :disabled="isLast" aria-label="В конец" @click="dispatch({ type: 'last' })">⏭</button>
      <input class="viz-progress" type="range" min="1" :max="state.total" :value="state.index + 1" aria-label="Перейти к шагу" @input="onSeek" />
      <label class="viz-speed">
        Скорость
        <select :value="state.speed" @change="onSpeed">
          <option v-for="s in SPEEDS" :key="s" :value="s">{{ String(s).replace('.', ',') }}×</option>
        </select>
      </label>
    </div>
    <p class="viz-hint">Клавиши: ← → — шаги, пробел — пуск и пауза, Home / End — в начало и в конец.</p>
  </figure>
</template>

<style scoped>
.viz-player { container-type: inline-size; margin: 24px 0; padding: 16px; border: 1px solid var(--vp-c-divider); border-radius: 12px; background: var(--vp-c-bg-soft); }
.viz-player:focus-visible { outline: 2px solid var(--vp-c-brand-1); outline-offset: 2px; }
.viz-stage { display: grid; gap: 16px; }
@container (min-width: 900px) { .viz-stage.has-code { grid-template-columns: 3fr 2fr; } }
/* Ширина контейнера, а не окна: плеер бывает и в узкой колонке урока. */
@container (min-width: 900px) { .viz-stage.has-side { grid-template-columns: minmax(0, 1fr) 340px; align-items: start; } }
.viz-stage.has-side .viz-code { grid-column: 1 / -1; }
.viz-canvas, .viz-code, .viz-side { min-width: 0; }
.viz-caption { min-height: 4.5em; margin-top: 12px; line-height: 1.6; }
.viz-step-no { display: block; color: var(--vp-c-text-2); font-size: 13px; }
.viz-controls { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-top: 12px; }
.viz-controls button { min-width: 40px; min-height: 40px; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); }
.viz-controls button:disabled { opacity: 0.4; cursor: not-allowed; }
.viz-controls .viz-play { border-color: var(--vp-c-brand-1); color: var(--vp-c-brand-1); }
.viz-progress { flex: 1 1 140px; min-width: 0; }
.viz-speed { display: flex; align-items: center; gap: 6px; font-size: 14px; }
.viz-speed select { padding: 4px 6px; border: 1px solid var(--vp-c-divider); border-radius: 6px; background: var(--vp-c-bg); }
.viz-hint { margin: 8px 0 0; color: var(--vp-c-text-3); font-size: 12px; }
</style>
