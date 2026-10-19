<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import SegmentedToggle from '../components/SegmentedToggle.vue'
import WorkerPoolView from '../renderers/WorkerPoolView.vue'
import { LIMITS, workerPoolSimulation, type PoolMode, type WorkerPoolAction, type WorkerPoolParams } from '../simulations/worker-pool'

const state = ref(workerPoolSimulation.initial())
const steps = computed(() => workerPoolSimulation.steps(state.value))

const dispatch = (action: WorkerPoolAction): void => {
  state.value = workerPoolSimulation.apply(state.value, action)
}
const set = (patch: Partial<WorkerPoolParams>): void => dispatch({ type: 'set', patch })
const onRange = (key: 'lambda' | 'workers' | 'queue' | 'taskMs', event: Event): void => {
  set({ [key]: Number((event.target as HTMLInputElement).value) })
}

const modes: { value: PoolMode; text: string }[] = [
  { value: 'unbounded', text: 'горутина на задачу' },
  { value: 'pool', text: 'пул воркеров' },
  { value: 'semaphore', text: 'семафор с отказом' },
]
const sliders = [
  { key: 'lambda', label: 'Поток λ, задач/с', unit: '' },
  { key: 'workers', label: 'Воркеров', unit: '' },
  { key: 'queue', label: 'Буфер очереди (ёмкость канала)', unit: '' },
  { key: 'taskMs', label: 'Время задачи, мс', unit: '' },
] as const
</script>

<template>
  <VizFrame label="Песочница: пул воркеров">
    <p class="intro">Поток задач сильнее, чем умеют обработать воркеры. Сравните три способа: горутина на каждую задачу, пул с буферным каналом (когда он полон, производитель блокируется) и семафор, который отказывает сверх очереди. Память — оценка, а не измерение.</p>
    <SegmentedToggle label="Режим" :options="modes" :model-value="state.params.mode" @update:model-value="(mode) => set({ mode })" />
    <div class="sliders">
      <label v-for="s in sliders" :key="s.key" class="slider">
        <span>{{ s.label }}: <b>{{ state.params[s.key] }}</b></span>
        <input type="range" :min="LIMITS[s.key].min" :max="LIMITS[s.key].max" step="1" :value="state.params[s.key]" @input="onRange(s.key, $event)" />
      </label>
    </div>
    <div class="panel">
      <label class="check"><input type="checkbox" :checked="state.params.burst" @change="set({ burst: ($event.target as HTMLInputElement).checked })" /> всплеск ×5 на 3 с (с 5-й секунды)</label>
      <button type="button" @click="dispatch({ type: 'run' })">Пересчитать</button>
      <button type="button" @click="dispatch({ type: 'reset' })">Сброс</button>
    </div>
    <Player :steps="steps" label="Песочница: пул воркеров, 20 секунд">
      <template #default="{ step }">
        <WorkerPoolView :frame="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>

<style scoped>
.intro { margin: 0 0 10px; font-size: 14px; color: var(--viz-muted); }
.sliders { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 8px 16px; margin-bottom: 10px; }
.slider { display: flex; flex-direction: column; gap: 2px; font-size: 14px; }
.slider input { width: 100%; min-height: 36px; }
.panel { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-bottom: 10px; }
.check { display: flex; align-items: center; gap: 6px; min-height: 36px; font-size: 14px; }
.check input { width: 20px; height: 20px; }
.panel button { min-width: 36px; min-height: 36px; padding: 0 12px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; cursor: pointer; }
.panel button:focus-visible, input:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
</style>
