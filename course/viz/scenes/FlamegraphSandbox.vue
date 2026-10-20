<script setup lang="ts">
import { computed, ref } from 'vue'
import T from '../../components/T.vue'
import VizFrame from '../components/VizFrame.vue'
import SegmentedToggle from '../components/SegmentedToggle.vue'
import FlamegraphView from '../renderers/FlamegraphView.vue'
import { captionSegments } from '../core/caption'
import { flamegraphSimulation as sim, type FlameAction, type SampleKind } from '../simulations/flamegraph'

const state = ref(sim.initial())
const step = computed(() => sim.steps(state.value)[0]!)

const dispatch = (action: FlameAction): void => {
  state.value = sim.apply(state.value, action)
}
const kindOptions: { value: SampleKind; text: string }[] = [
  { value: 'cpu', text: 'CPU' },
  { value: 'alloc_space', text: 'alloc_space — выделено' },
  { value: 'inuse_space', text: 'inuse_space — живо' },
]
const onSearch = (event: Event): void => dispatch({ type: 'search', query: (event.target as HTMLInputElement).value })
</script>

<template>
  <VizFrame label="Песочница: флейм-граф мессенджера">
    <p class="intro">
      Настоящие профили мессенджера: 20 секунд под чтением истории чата (<code>scripts/read-load.sh</code>) и наплывом регистраций (<code>scripts/overload.sh</code>), свёрнутые программой из лабы. Наведите или перейдите по блокам клавишей Tab — внизу имя и доля; щелчок или Enter увеличивает блок, Esc возвращает к корню. Стеки тоньше 0,1&nbsp;% слиты в блок «прочее» над ближайшей вызвавшей их функцией, которая сама толще порога.
    </p>
    <SegmentedToggle label="Тип выборки" :options="kindOptions" :model-value="state.kind" @update:model-value="(kind) => dispatch({ type: 'select-kind', kind })" />
    <div class="panel">
      <label class="search" for="flame-search">
        <span>Поиск функции</span>
        <input id="flame-search" type="search" :value="state.query" placeholder="например, json или argon2" autocomplete="off" spellcheck="false" @input="onSearch" />
      </label>
      <button type="button" :disabled="state.focusId === 'r'" @click="dispatch({ type: 'reset' })">К корню</button>
    </div>
    <FlamegraphView :frame="step.state" @zoom="(id) => dispatch({ type: 'zoom', id })" @reset="dispatch({ type: 'reset' })" />
    <p class="caption" aria-live="polite">
      <template v-for="(seg, i) in captionSegments(step.caption)" :key="i"><T v-if="seg.termId" :id="seg.termId">{{ seg.text }}</T><template v-else>{{ seg.text }}</template></template>
    </p>
  </VizFrame>
</template>

<style scoped>
.intro { margin: 0 0 10px; font-size: 14px; color: var(--viz-muted); }
.panel { display: flex; flex-wrap: wrap; align-items: flex-end; gap: 8px; margin-bottom: 10px; }
.search { display: flex; flex: 1 1 220px; flex-direction: column; gap: 4px; min-width: 0; font-size: 14px; color: var(--viz-text); }
input, button { min-height: 36px; padding: 0 10px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; }
input { width: 100%; box-sizing: border-box; }
button { cursor: pointer; }
button:disabled { cursor: default; opacity: 0.6; }
input:focus-visible, button:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
.caption { min-height: 6em; margin: 8px 0 0; line-height: 1.6; }
</style>
