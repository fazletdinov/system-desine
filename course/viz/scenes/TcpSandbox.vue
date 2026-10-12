<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import TcpView from '../renderers/TcpView.vue'
import { tcpSimulation, type TcpAction } from '../simulations/tcp'

const state = ref(tcpSimulation.initial())
const steps = computed(() => tcpSimulation.steps(state.value))
const segments = computed(() => state.value.config.segments)
const segmentNumbers = computed(() => Array.from({ length: segments.value }, (_, i) => i))
const ackNumbers = computed(() => Array.from({ length: segments.value }, (_, i) => i + 1))

const dispatch = (action: TcpAction): void => {
  state.value = tcpSimulation.apply(state.value, action)
}
const onWindow = (event: Event): void => {
  dispatch({ type: 'window', size: Number((event.target as HTMLInputElement).value) })
}
</script>

<template>
  <VizFrame label="Песочница TCP">
    <p class="tcp-intro">
      Учебная модель: 8 сегментов, задержка в одну сторону — 1 тик, кумулятивные ACK, повтор по таймеру через 6 тиков. SACK, slow start и настоящая формула таймера
      здесь упрощены. Каждая потеря срабатывает один раз: повтор уже доходит.
    </p>
    <div class="tcp-panel" role="group" aria-label="Настройки песочницы TCP">
      <div class="row" role="group" aria-label="Потерять сегмент">
        <span class="row-label" aria-hidden="true">Потерять сегмент</span>
        <button v-for="k in segmentNumbers" :key="`s${k}`" type="button" class="toggle" :aria-pressed="state.dropSegments.includes(k)" :aria-label="`Потерять сегмент ${k}`" @click="dispatch({ type: 'drop-segment', seq: k })">{{ k }}</button>
      </div>
      <div class="row" role="group" aria-label="Потерять ACK">
        <span class="row-label" aria-hidden="true">Потерять ACK</span>
        <button v-for="k in ackNumbers" :key="`a${k}`" type="button" class="toggle" :aria-pressed="state.dropAcks.includes(k)" :aria-label="`Потерять ACK ${k}`" @click="dispatch({ type: 'drop-ack', ack: k })">{{ k }}</button>
      </div>
      <div class="row">
        <label class="row-label" for="tcp-window">Окно: {{ state.config.window }}</label>
        <input id="tcp-window" type="range" min="1" max="8" step="1" :value="state.config.window" @input="onWindow" />
        <button type="button" class="reset" @click="dispatch({ type: 'reset' })">Сброс</button>
      </div>
    </div>
    <Player :steps="steps" label="Песочница TCP: передача 8 сегментов">
      <template #default="{ step }">
        <TcpView :state="step.state" :segments="segments" />
      </template>
    </Player>
  </VizFrame>
</template>

<style scoped>
.tcp-intro { margin: 0 0 10px; font-size: 14px; color: var(--viz-muted); }
.tcp-panel { display: flex; flex-direction: column; gap: 8px; margin-bottom: 12px; }
.row { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; }
.row-label { font-size: 14px; color: var(--viz-text); margin-right: 4px; min-width: 130px; }
.toggle, .reset { min-width: 36px; min-height: 36px; padding: 0 8px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; cursor: pointer; }
.toggle[aria-pressed='true'] { border-color: var(--viz-error); background: color-mix(in srgb, var(--viz-error) 20%, var(--vp-c-bg)); font-weight: 700; }
.toggle:focus-visible, .reset:focus-visible, input[type='range']:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
input[type='range'] { flex: 1 1 140px; max-width: 260px; min-height: 36px; }
</style>
