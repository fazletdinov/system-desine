<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import SegmentedToggle from '../components/SegmentedToggle.vue'
import SupplyChainView from '../renderers/SupplyChainView.vue'
import { supplyChainSimulation, type ChainAction } from '../simulations/supply-chain'

const state = ref(supplyChainSimulation.initial())
const steps = computed(() => supplyChainSimulation.steps(state.value))

const dispatch = (action: ChainAction): void => {
  state.value = supplyChainSimulation.apply(state.value, action)
}
const pinOptions = [
  { value: 'tag', text: 'закрепление: тег' },
  { value: 'sha', text: 'закрепление: SHA' },
] as const
const permissionOptions = [
  { value: 'write-all', text: 'permissions: write-all' },
  { value: 'minimal', text: 'permissions: минимальные' },
] as const
const secretOptions = [
  { value: true, text: 'секрет в env: да' },
  { value: false, text: 'секрет в env: нет' },
] as const
</script>

<template>
  <VizFrame label="Песочница: атака на цепочку поставок">
    <p class="intro">Злоумышленник перевешивает тег чужого action на свой коммит. Что выполнится в вашем workflow и что утечёт?</p>
    <SegmentedToggle label="Закрепление" :options="pinOptions" :model-value="state.pin" @update:model-value="(mode) => dispatch({ type: 'pin', mode })" />
    <SegmentedToggle label="Права токена" :options="permissionOptions" :model-value="state.permissions" @update:model-value="(mode) => dispatch({ type: 'permissions', mode })" />
    <SegmentedToggle label="Секрет в env" :options="secretOptions" :model-value="state.secretInEnv" @update:model-value="(inEnv) => dispatch({ type: 'secret', inEnv })" />
    <div class="panel" role="group" aria-label="Действия">
      <button type="button" @click="dispatch({ type: 'attack' })">Подменить тег</button>
      <button type="button" @click="dispatch({ type: 'run' })">Запустить workflow</button>
      <button type="button" @click="dispatch({ type: 'reset' })">Сброс</button>
    </div>
    <Player :steps="steps" label="Песочница: цепочка поставок">
      <template #default="{ step }">
        <SupplyChainView :state="step.state" />
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
