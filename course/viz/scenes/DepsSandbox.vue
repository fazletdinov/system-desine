<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import DepsView from '../renderers/DepsView.vue'
import { CANDIDATES, depsSimulation, type DepsAction } from '../simulations/deps'

const state = ref(depsSimulation.initial())
const steps = computed(() => depsSimulation.steps(state.value))
const selected = ref(0)

const dispatch = (action: DepsAction): void => {
  state.value = depsSimulation.apply(state.value, action)
}
const add = (): void => {
  const c = CANDIDATES[selected.value]
  if (c !== undefined) dispatch({ type: 'add', from: c.from, to: c.to })
}
</script>

<template>
  <VizFrame label="Песочница: граф зависимостей мессенджера">
    <p class="intro">Выберите импорт, который хочется добавить, и посмотрите, разрешит ли его правило зависимостей и как компилятор Go отнесётся к циклам.</p>
    <div class="panel">
      <label class="row-label" for="deps-candidate">Новый импорт (откуда → куда)</label>
      <select id="deps-candidate" v-model.number="selected">
        <option v-for="(c, i) in CANDIDATES" :key="`${c.from}>${c.to}`" :value="i">{{ c.from }} → {{ c.to }}</option>
      </select>
      <button type="button" @click="add">Добавить импорт</button>
      <button type="button" @click="dispatch({ type: 'reset' })">Сбросить</button>
    </div>
    <Player :steps="steps" start-at-end label="Песочница: граф зависимостей мессенджера">
      <template #default="{ step }">
        <DepsView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>

<style scoped>
.intro { margin: 0 0 10px; font-size: 14px; color: var(--viz-muted); }
.panel { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-bottom: 12px; }
.row-label { flex: 1 1 100%; font-size: 14px; color: var(--viz-text); }
select { flex: 1 1 260px; max-width: 100%; min-height: 36px; padding: 0 8px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; }
button { min-height: 36px; padding: 0 12px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; cursor: pointer; }
select:focus-visible, button:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
</style>
