<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import IdempotencyView from '../renderers/IdempotencyView.vue'
import { idempotencySimulation, type IdemAction, type IdemMode } from '../simulations/idempotency'

const state = ref(idempotencySimulation.initial())
const steps = computed(() => idempotencySimulation.steps(state.value))

const dispatch = (action: IdemAction): void => {
  state.value = idempotencySimulation.apply(state.value, action)
}
const modes: { id: IdemMode; label: string }[] = [
  { id: 'no-key', label: 'без ключа' },
  { id: 'key', label: 'с ключом' },
]
</script>

<template>
  <VizFrame label="Песочница: идемпотентность POST">
    <p class="intro">Создание чата: ответ потерялся, клиент повторяет запрос. Что сделает сервер без ключа и с ключом?</p>
    <div class="panel" role="radiogroup" aria-label="Режим">
      <button v-for="m in modes" :key="m.id" type="button" role="radio" :aria-checked="state.mode === m.id" :class="{ on: state.mode === m.id }" @click="dispatch({ type: 'mode', mode: m.id })">{{ m.label }}</button>
    </div>
    <div class="panel" role="group" aria-label="Действия">
      <button type="button" @click="dispatch({ type: 'send', body: 'A' })">Отправить (тело A)</button>
      <button type="button" @click="dispatch({ type: 'send', body: 'A' })">Повторить</button>
      <button type="button" @click="dispatch({ type: 'send', body: 'B' })">Повторить с телом B</button>
      <button type="button" @click="dispatch({ type: 'parallel' })">Два запроса одновременно</button>
      <button type="button" @click="dispatch({ type: 'lose' })">Потерять следующий ответ</button>
      <button type="button" @click="dispatch({ type: 'fail' })">Сбой сервера на следующем запросе</button>
      <button type="button" @click="dispatch({ type: 'reset' })">Сброс</button>
    </div>
    <Player :steps="steps" label="Песочница: идемпотентность">
      <template #default="{ step }">
        <IdempotencyView :state="step.state" />
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
