<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import LayerCacheView from '../renderers/LayerCacheView.vue'
import { layerCacheSimulation, type LayerAction } from '../simulations/layer-cache'

const state = ref(layerCacheSimulation.initial())
const steps = computed(() => layerCacheSimulation.steps(state.value))

const dispatch = (action: LayerAction): void => {
  state.value = layerCacheSimulation.apply(state.value, action)
}
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
    <div class="panel" role="radiogroup" aria-label="Порядок инструкций">
      <button type="button" role="radio" :aria-checked="state.order === 'correct'" :class="{ on: state.order === 'correct' }" @click="dispatch({ type: 'order', order: 'correct' })">COPY go.mod раньше</button>
      <button type="button" role="radio" :aria-checked="state.order === 'naive'" :class="{ on: state.order === 'naive' }" @click="dispatch({ type: 'order', order: 'naive' })">COPY . . сразу</button>
    </div>
    <div class="panel" role="radiogroup" aria-label=".dockerignore">
      <button type="button" role="radio" :aria-checked="state.dockerignore" :class="{ on: state.dockerignore }" @click="dispatch({ type: 'dockerignore', on: true })">.dockerignore вкл</button>
      <button type="button" role="radio" :aria-checked="!state.dockerignore" :class="{ on: !state.dockerignore }" @click="dispatch({ type: 'dockerignore', on: false })">.dockerignore выкл</button>
    </div>
    <div class="panel" role="radiogroup" aria-label="cache mount">
      <button type="button" role="radio" :aria-checked="state.cacheMount" :class="{ on: state.cacheMount }" @click="dispatch({ type: 'cacheMount', on: true })">cache mount вкл</button>
      <button type="button" role="radio" :aria-checked="!state.cacheMount" :class="{ on: !state.cacheMount }" @click="dispatch({ type: 'cacheMount', on: false })">cache mount выкл</button>
    </div>
    <div class="panel" role="radiogroup" aria-label="multi-stage">
      <button type="button" role="radio" :aria-checked="state.multiStage" :class="{ on: state.multiStage }" @click="dispatch({ type: 'stage', multi: true })">multi-stage вкл</button>
      <button type="button" role="radio" :aria-checked="!state.multiStage" :class="{ on: !state.multiStage }" @click="dispatch({ type: 'stage', multi: false })">multi-stage выкл</button>
    </div>
    <Player :steps="steps" label="Песочница: кэш слоёв">
      <template #default="{ step }">
        <LayerCacheView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>

<style scoped>
.intro { margin: 0 0 10px; font-size: 14px; color: var(--viz-muted); }
.panel { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 10px; }
.panel button { min-width: 36px; min-height: 36px; padding: 0 12px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; cursor: pointer; }
.panel button.on { border-color: var(--vp-c-brand-1); font-weight: 700; }
.panel button:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
</style>
