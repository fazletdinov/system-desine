<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import SupplyChainView from '../renderers/SupplyChainView.vue'
import { supplyChainSimulation, type ChainAction } from '../simulations/supply-chain'

const state = ref(supplyChainSimulation.initial())
const steps = computed(() => supplyChainSimulation.steps(state.value))

const dispatch = (action: ChainAction): void => {
  state.value = supplyChainSimulation.apply(state.value, action)
}
</script>

<template>
  <VizFrame label="Песочница: атака на цепочку поставок">
    <p class="intro">Злоумышленник перевешивает тег чужого action на свой коммит. Что выполнится в вашем workflow и что утечёт?</p>
    <div class="panel" role="radiogroup" aria-label="Закрепление">
      <button type="button" role="radio" :aria-checked="state.pin === 'tag'" :class="{ on: state.pin === 'tag' }" @click="dispatch({ type: 'pin', mode: 'tag' })">закрепление: тег</button>
      <button type="button" role="radio" :aria-checked="state.pin === 'sha'" :class="{ on: state.pin === 'sha' }" @click="dispatch({ type: 'pin', mode: 'sha' })">закрепление: SHA</button>
    </div>
    <div class="panel" role="radiogroup" aria-label="Права токена">
      <button type="button" role="radio" :aria-checked="state.permissions === 'write-all'" :class="{ on: state.permissions === 'write-all' }" @click="dispatch({ type: 'permissions', mode: 'write-all' })">permissions: write-all</button>
      <button type="button" role="radio" :aria-checked="state.permissions === 'minimal'" :class="{ on: state.permissions === 'minimal' }" @click="dispatch({ type: 'permissions', mode: 'minimal' })">permissions: минимальные</button>
    </div>
    <div class="panel" role="radiogroup" aria-label="Секрет в env">
      <button type="button" role="radio" :aria-checked="state.secretInEnv" :class="{ on: state.secretInEnv }" @click="dispatch({ type: 'secret', inEnv: true })">секрет в env: да</button>
      <button type="button" role="radio" :aria-checked="!state.secretInEnv" :class="{ on: !state.secretInEnv }" @click="dispatch({ type: 'secret', inEnv: false })">секрет в env: нет</button>
    </div>
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
