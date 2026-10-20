<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import T from '../../components/T.vue'
import VizFrame from '../components/VizFrame.vue'
import SegmentedToggle from '../components/SegmentedToggle.vue'
import GcPacerView from '../renderers/GcPacerView.vue'
import { captionSegments } from '../core/caption'
import { prefersReducedMotion } from '../core/motion'
import { BURST_FACTOR, BURST_S, DEFAULT_GOGC, LIMITS, TICKS_PER_SECOND, gcPacerSimulation as sim, tickerPlan, type GcPacerAction, type GcPacerParams } from '../simulations/gc-pacer'

const state = ref(sim.initial())
const step = computed(() => sim.steps(state.value)[0]!)
const running = ref(false)
// Последние значения ползунков: переключатель «off» и «лимит не задан» их не теряет.
const lastGogc = ref(DEFAULT_GOGC)
const lastLimit = ref(192)

const dispatch = (action: GcPacerAction): void => {
  state.value = sim.apply(state.value, action)
}
const set = (patch: Partial<GcPacerParams>): void => dispatch({ type: 'set', patch })
const num = (event: Event): number => Number((event.target as HTMLInputElement).value)
const checked = (event: Event): boolean => (event.target as HTMLInputElement).checked

let timer: ReturnType<typeof setInterval> | undefined
const stop = (): void => {
  clearInterval(timer)
  timer = undefined
  running.value = false
}
const advance = (ticks: number): void => {
  for (let i = 0; i < ticks; i++) dispatch({ type: 'tick' })
}
// Модель идёт в реальном времени; при prefers-reduced-motion — крупными шагами по секунде.
const toggleRun = (): void => {
  if (running.value) return stop()
  running.value = true
  const plan = tickerPlan(prefersReducedMotion())
  timer = setInterval(() => advance(plan.ticksPerUpdate), plan.intervalMs)
}
const stepSecond = (): void => advance(TICKS_PER_SECOND)
const reset = (): void => {
  stop()
  lastGogc.value = DEFAULT_GOGC
  lastLimit.value = 192
  dispatch({ type: 'reset' })
}
onBeforeUnmount(stop)

const onGogc = (event: Event): void => {
  lastGogc.value = num(event)
  set({ gogc: lastGogc.value })
}
const onGogcOff = (event: Event): void => set({ gogc: checked(event) ? null : lastGogc.value })
const onLimit = (event: Event): void => {
  lastLimit.value = num(event)
  set({ limitMiB: lastLimit.value })
}
const onLimitOn = (event: Event): void => set({ limitMiB: checked(event) ? lastLimit.value : null })

const limiterOptions = [
  { value: true, text: 'ограничитель CPU GC (1.19+)' },
  { value: false, text: 'без ограничителя (как до 1.19)' },
]
const p = computed(() => state.value.params)
</script>

<template>
  <VizFrame label="Песочница: пейсер сборщика мусора">
    <p class="intro">Упрощённая модель Go-процесса в контейнере на 256 МиБ. Двигайте ползунки на ходу, включайте утечку и всплеск аллокаций и смотрите, как сборщик держит кучу у цели и сколько процессора за это берёт.</p>
    <div class="sliders">
      <div class="slider">
        <label for="gcp-gogc">GOGC: <b>{{ p.gogc === null ? 'off' : p.gogc }}</b></label>
        <input id="gcp-gogc" type="range" :min="LIMITS.gogc.min" :max="LIMITS.gogc.max" :step="LIMITS.gogc.step" :value="p.gogc ?? lastGogc" :disabled="p.gogc === null" @input="onGogc" />
        <label class="check"><input type="checkbox" :checked="p.gogc === null" @change="onGogcOff" /> GOGC=off</label>
      </div>
      <div class="slider">
        <label for="gcp-limit">GOMEMLIMIT: <b>{{ p.limitMiB === null ? 'не задан' : `${p.limitMiB} МиБ` }}</b></label>
        <input id="gcp-limit" type="range" :min="LIMITS.limitMiB.min" :max="LIMITS.limitMiB.max" :step="LIMITS.limitMiB.step" :value="p.limitMiB ?? lastLimit" :disabled="p.limitMiB === null" @input="onLimit" />
        <label class="check"><input type="checkbox" :checked="p.limitMiB !== null" @change="onLimitOn" /> задать лимит</label>
      </div>
      <div class="slider">
        <label for="gcp-live">Живая куча: <b>{{ p.liveMiB }} МиБ</b>{{ state.leakedMiB > 0 ? ` + утечка ${Math.round(state.leakedMiB)}` : '' }}</label>
        <input id="gcp-live" type="range" :min="LIMITS.liveMiB.min" :max="LIMITS.liveMiB.max" :step="LIMITS.liveMiB.step" :value="p.liveMiB" @input="set({ liveMiB: num($event) })" />
      </div>
      <div class="slider">
        <label for="gcp-alloc">Аллокации: <b>{{ p.allocMiBps }} МиБ/с</b></label>
        <input id="gcp-alloc" type="range" :min="LIMITS.allocMiBps.min" :max="LIMITS.allocMiBps.max" :step="LIMITS.allocMiBps.step" :value="p.allocMiBps" @input="set({ allocMiBps: num($event) })" />
      </div>
    </div>
    <SegmentedToggle label="Ограничитель CPU GC" :options="limiterOptions" :model-value="p.limiter" @update:model-value="(limiter) => set({ limiter })" />
    <div class="panel">
      <button type="button" class="primary" :aria-pressed="running" @click="toggleRun">{{ running ? '⏸ Пауза' : '▶ Старт' }}</button>
      <button type="button" :disabled="running" @click="stepSecond">+1 с</button>
      <button type="button" :aria-pressed="state.leaking" @click="dispatch({ type: 'leak' })">{{ state.leaking ? 'Остановить утечку' : 'Утечка' }}</button>
      <button type="button" @click="dispatch({ type: 'burst' })">Всплеск ×{{ BURST_FACTOR }} на {{ BURST_S }} с</button>
      <button type="button" @click="reset">Сброс</button>
    </div>
    <GcPacerView :frame="step.state" />
    <p class="caption" :aria-live="running ? 'off' : 'polite'">
      <template v-for="(seg, i) in captionSegments(step.caption)" :key="i"><T v-if="seg.termId" :id="seg.termId">{{ seg.text }}</T><template v-else>{{ seg.text }}</template></template>
    </p>
  </VizFrame>
</template>

<style scoped>
.intro { margin: 0 0 10px; font-size: 14px; color: var(--viz-muted); }
.sliders { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 200px), 1fr)); gap: 8px 16px; margin-bottom: 10px; }
.slider { display: flex; flex-direction: column; gap: 2px; font-size: 14px; min-width: 0; }
.slider input[type='range'] { width: 100%; min-height: 36px; }
.check { display: flex; align-items: center; gap: 6px; min-height: 36px; }
.check input { width: 20px; height: 20px; }
.panel { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-bottom: 10px; }
.panel button { min-width: 36px; min-height: 36px; padding: 0 12px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; cursor: pointer; }
.panel button[aria-pressed='true'] { border-color: var(--vp-c-brand-1); font-weight: 700; }
.panel button.primary { border-color: var(--vp-c-brand-1); color: var(--vp-c-brand-1); }
.panel button:disabled { opacity: 0.4; cursor: not-allowed; }
.panel button:focus-visible, input:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
.caption { min-height: 4.5em; margin: 8px 0 0; line-height: 1.6; }
</style>
