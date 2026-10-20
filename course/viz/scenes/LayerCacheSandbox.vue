<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import SegmentedToggle from '../components/SegmentedToggle.vue'
import LayerCacheView from '../renderers/LayerCacheView.vue'
import { layerCacheSimulation, type LayerAction } from '../simulations/layer-cache'

const state = ref(layerCacheSimulation.initial())
const steps = computed(() => layerCacheSimulation.steps(state.value))

const dispatch = (action: LayerAction): void => {
  state.value = layerCacheSimulation.apply(state.value, action)
}
const orderOptions = [
  { value: 'correct', text: 'COPY go.mod раньше' },
  { value: 'naive', text: 'COPY . . сразу' },
] as const
const dockerignoreOptions = [
  { value: true, text: '.dockerignore вкл' },
  { value: false, text: '.dockerignore выкл' },
] as const
const cacheMountOptions = [
  { value: true, text: 'cache mount вкл' },
  { value: false, text: 'cache mount выкл' },
] as const
const stageOptions = [
  { value: true, text: 'multi-stage вкл' },
  { value: false, text: 'multi-stage выкл' },
] as const
</script>

<template>
  <VizFrame label="Песочница: кэш слоёв Docker">
    <p class="intro">Соберите образ, измените файл и соберите снова: какие слои возьмутся из кэша, а какие придётся пересобрать?</p>
    <div class="panel" role="group" aria-label="Действия">
      <button type="button" @click="dispatch({ type: 'build' })">Собрать</button>
      <button type="button" @click="dispatch({ type: 'edit', file: 'source' })">Изменить main.go</button>
      <button type="button" @click="dispatch({ type: 'edit', file: 'gomod' })">Изменить go.mod</button>
      <button type="button" @click="dispatch({ type: 'edit', file: 'readme' })">Изменить README</button>
      <button type="button" @click="dispatch({ type: 'reset' })">Сброс</button>
    </div>
    <SegmentedToggle label="Порядок инструкций" :options="orderOptions" :model-value="state.order" @update:model-value="(order) => dispatch({ type: 'order', order })" />
    <SegmentedToggle label=".dockerignore" :options="dockerignoreOptions" :model-value="state.dockerignore" @update:model-value="(on) => dispatch({ type: 'dockerignore', on })" />
    <SegmentedToggle label="cache mount" :options="cacheMountOptions" :model-value="state.cacheMount" @update:model-value="(on) => dispatch({ type: 'cacheMount', on })" />
    <SegmentedToggle label="multi-stage" :options="stageOptions" :model-value="state.multiStage" @update:model-value="(multi) => dispatch({ type: 'stage', multi })" />
    <Player :steps="steps" label="Песочница: кэш слоёв">
      <template #default="{ step }">
        <LayerCacheView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>

<style scoped>
.intro { margin: 0 0 10px; font-size: 14px; color: var(--vp-c-text-2); }
.panel { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 10px; }
.panel button { min-width: 36px; min-height: 36px; padding: 0 12px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; cursor: pointer; }
.panel button.on { border-color: var(--vp-c-brand-1); font-weight: 700; }
.panel button:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
</style>
