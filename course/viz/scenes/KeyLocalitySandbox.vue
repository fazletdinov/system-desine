<script setup lang="ts">
import { computed, ref } from 'vue'
import T from '../../components/T.vue'
import VizFrame from '../components/VizFrame.vue'
import SegmentedToggle from '../components/SegmentedToggle.vue'
import KeyLocalityView from '../renderers/KeyLocalityView.vue'
import { captionSegments } from '../core/caption'
import { INSERT_BATCHES, KEY_KINDS, KIND_LABEL, MAX_KEYS, PAGE_CAPACITY, keyLocalitySimulation as sim, type KeyKind, type KeyLocalityAction } from '../simulations/key-locality'

const state = ref(sim.initial())
const step = computed(() => sim.steps(state.value)[0]!)

const dispatch = (action: KeyLocalityAction): void => {
  state.value = sim.apply(state.value, action)
}

const kinds: { value: KeyKind; text: string }[] = KEY_KINDS.map((k) => ({ value: k, text: KIND_LABEL[k] }))
</script>

<template>
  <VizFrame label="Песочница: локальность ключей в B-дереве">
    <p class="intro">Модель листьев индекса первичного ключа: страница на {{ PAGE_CAPACITY }} ключей, правила разделения как в PostgreSQL. Вставки идут во все три индекса сразу — переключатель выбирает, какой рисовать. Нажмите CHECKPOINT, затем вставьте ещё, и сравните число полных образов страниц в WAL.</p>
    <SegmentedToggle label="Вид ключа" :options="kinds" :model-value="state.selected" @update:model-value="(kind) => dispatch({ type: 'select', kind })" />
    <div class="panel" role="group" aria-label="Действия">
      <button v-for="n in INSERT_BATCHES" :key="n" type="button" :disabled="step.state.full" @click="dispatch({ type: 'insert', count: n })">+{{ n }}</button>
      <button type="button" @click="dispatch({ type: 'checkpoint' })">CHECKPOINT</button>
      <button type="button" @click="dispatch({ type: 'reset' })">Сброс</button>
      <span v-if="step.state.full" class="note">предел модели — {{ MAX_KEYS }} ключей</span>
    </div>
    <KeyLocalityView :frame="step.state" />
    <p class="caption" aria-live="polite">
      <template v-for="(seg, i) in captionSegments(step.caption)" :key="i"><T v-if="seg.termId" :id="seg.termId">{{ seg.text }}</T><template v-else>{{ seg.text }}</template></template>
    </p>
  </VizFrame>
</template>

<style scoped>
.intro { margin: 0 0 10px; font-size: 14px; color: var(--viz-muted); }
.panel { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-bottom: 10px; }
.panel button { min-width: 36px; min-height: 36px; padding: 0 12px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; cursor: pointer; }
.panel button:disabled { opacity: 0.4; cursor: not-allowed; }
.panel button:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
.note { font-size: 13px; color: var(--viz-muted); }
.caption { min-height: 4.5em; margin: 8px 0 0; line-height: 1.6; }
</style>
