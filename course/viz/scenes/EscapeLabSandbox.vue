<script setup lang="ts">
import { computed, ref } from 'vue'
import T from '../../components/T.vue'
import VizFrame from '../components/VizFrame.vue'
import SegmentedToggle from '../components/SegmentedToggle.vue'
import EscapeLabView from '../renderers/EscapeLabView.vue'
import { captionSegments } from '../core/caption'
import { CASES, escapeLabSimulation as sim, type EscapeLabAction } from '../simulations/escape-lab'

const state = ref(sim.initial())
const step = computed(() => sim.steps(state.value)[0]!)
const current = computed(() => CASES.find((c) => c.id === state.value.caseId)!)
const options = computed(() => current.value.variants.map((v) => ({ value: v.id, text: v.toggleText })))

const dispatch = (action: EscapeLabAction): void => {
  state.value = sim.apply(state.value, action)
}
const onCase = (event: Event): void => dispatch({ type: 'select-case', caseId: (event.target as HTMLSelectElement).value })
</script>

<template>
  <VizFrame label="Песочница: куда компилятор кладёт объект">
    <p class="intro">Выберите пример и переключите одну деталь в коде. Плашка, код и строки компилятора — настоящий вывод <code>go build -gcflags=-m=1</code> для этих функций, снятый программой из лабы.</p>
    <div class="panel">
      <label class="row-label" for="escape-case">Пример</label>
      <select id="escape-case" :value="state.caseId" @change="onCase">
        <option v-for="c in CASES" :key="c.id" :value="c.id">{{ c.title }}</option>
      </select>
    </div>
    <p class="row-label">{{ current.toggleLabel }}</p>
    <SegmentedToggle :label="current.toggleLabel" :options="options" :model-value="state.variantId" @update:model-value="(variantId) => dispatch({ type: 'select-variant', variantId })" />
    <EscapeLabView :frame="step.state" />
    <p class="caption" aria-live="polite">
      <template v-for="(seg, i) in captionSegments(step.caption)" :key="i"><T v-if="seg.termId" :id="seg.termId">{{ seg.text }}</T><template v-else>{{ seg.text }}</template></template>
    </p>
  </VizFrame>
</template>

<style scoped>
.intro { margin: 0 0 10px; font-size: 14px; color: var(--viz-muted); }
.panel { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-bottom: 10px; }
.row-label { flex: 1 1 100%; margin: 0 0 4px; font-size: 14px; color: var(--viz-text); }
select { flex: 1 1 260px; max-width: 100%; min-height: 36px; padding: 0 8px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; }
select:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
.caption { min-height: 4.5em; margin: 8px 0 0; line-height: 1.6; }
</style>
