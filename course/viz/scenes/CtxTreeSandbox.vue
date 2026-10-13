<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import CtxTreeView from '../renderers/CtxTreeView.vue'
import { ctxTreeSimulation, type CtxAction } from '../simulations/ctx-tree'

const state = ref(ctxTreeSimulation.initial())
const steps = computed(() => ctxTreeSimulation.steps(state.value))

const dispatch = (action: CtxAction): void => {
  state.value = ctxTreeSimulation.apply(state.value, action)
}
</script>

<template>
  <VizFrame label="Песочница: дерево контекстов">
    <p class="intro">
      Сценарий запроса «отправить сообщение»: таймаут запроса — 1 с, проверка участников — 100 мс, запись в аудит отвязана от отмены. Отменяйте узлы и крутите время.
    </p>
    <div class="panel" role="group" aria-label="Управление временем">
      <button type="button" @click="dispatch({ type: 'advance', ms: 50 })">+50 мс</button>
      <button type="button" @click="dispatch({ type: 'advance', ms: 500 })">+500 мс</button>
      <button type="button" @click="dispatch({ type: 'reset' })">Сброс</button>
    </div>
    <Player :steps="steps" label="Песочница: дерево контекстов Go">
      <template #default="{ step }">
        <CtxTreeView :state="step.state" @cancel="dispatch({ type: 'cancel', id: $event })" />
      </template>
    </Player>
  </VizFrame>
</template>

<style scoped>
.intro { margin: 0 0 10px; font-size: 14px; color: var(--viz-muted); }
.panel { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 12px; }
.panel button { min-width: 36px; min-height: 36px; padding: 0 12px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; cursor: pointer; }
.panel button:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
</style>
