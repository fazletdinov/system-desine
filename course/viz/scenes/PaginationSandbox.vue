<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import SegmentedToggle from '../components/SegmentedToggle.vue'
import PaginationView from '../renderers/PaginationView.vue'
import { paginationSimulation, type PagingAction, type PagingMode } from '../simulations/pagination'

const state = ref(paginationSimulation.initial())
const steps = computed(() => paginationSimulation.steps(state.value))

const dispatch = (action: PagingAction): void => {
  state.value = paginationSimulation.apply(state.value, action)
}
const modes: { value: PagingMode; text: string }[] = [
  { value: 'offset', text: 'offset' },
  { value: 'cursor', text: 'курсор' },
]
</script>

<template>
  <VizFrame label="Песочница: пагинация">
    <p class="intro">Листайте историю чата и между страницами добавляйте или удаляйте сообщения. Сравните offset и курсор.</p>
    <SegmentedToggle label="Способ пагинации" :options="modes" :model-value="state.mode" @update:model-value="(mode) => dispatch({ type: 'mode', mode })" />
    <div class="panel" role="group" aria-label="Действия">
      <button type="button" @click="dispatch({ type: 'next' })">Следующая страница</button>
      <button type="button" @click="dispatch({ type: 'insert' })">Новое сообщение сверху</button>
      <button type="button" @click="dispatch({ type: 'delete' })">Удалить верхнее</button>
      <button type="button" @click="dispatch({ type: 'reset' })">Сброс</button>
    </div>
    <Player :steps="steps" label="Песочница: пагинация">
      <template #default="{ step }">
        <PaginationView :state="step.state" />
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
