<script setup lang="ts">
import { computed, ref } from 'vue'
import T from '../../components/T.vue'
import VizFrame from '../components/VizFrame.vue'
import SegmentedToggle from '../components/SegmentedToggle.vue'
import NormalFormsView from '../renderers/NormalFormsView.vue'
import { captionSegments } from '../core/caption'
import { normalFormsSimulation as sim, type NormalFormsAction, type Schema } from '../simulations/normal-forms'

const state = ref(sim.initial())
const step = computed(() => sim.steps(state.value)[0]!)

const dispatch = (action: NormalFormsAction): void => {
  state.value = sim.apply(state.value, action)
}

const schemas: { value: Schema; text: string }[] = [
  { value: 'flat', text: 'одна большая таблица' },
  { value: 'nf3', text: '3НФ' },
]
const authors = computed(() => state.value.users.map((u) => ({ value: u.id, text: u.name })))
</script>

<template>
  <VizFrame label="Песочница: нормальные формы и аномалии">
    <p class="intro">Одни и те же сообщения, пользователи и чаты — в одной большой таблице и в 3НФ. Каждое действие выполняется в обеих схемах сразу: переключайтесь и сравнивайте, сколько строк оно тронуло и где сломало данные.</p>
    <SegmentedToggle label="Схема" :options="schemas" :model-value="state.schema" @update:model-value="(schema) => dispatch({ type: 'schema', schema })" />
    <SegmentedToggle label="Автор" :options="authors" :model-value="state.authorId" @update:model-value="(authorId) => dispatch({ type: 'author', authorId })" />
    <div class="panel" role="group" aria-label="Действия">
      <button type="button" @click="dispatch({ type: 'message' })">Новое сообщение</button>
      <button type="button" @click="dispatch({ type: 'register' })">Новый пользователь без сообщений</button>
      <button type="button" @click="dispatch({ type: 'rename' })">Переименовать автора</button>
      <button type="button" @click="dispatch({ type: 'delete-last' })">Удалить последнее сообщение автора</button>
      <button type="button" @click="dispatch({ type: 'reset' })">Сброс</button>
    </div>
    <label class="check"><input type="checkbox" :checked="state.partial" @change="dispatch({ type: 'partial', partial: ($event.target as HTMLInputElement).checked })" /> переименовывать только одну строку (как код, который правит то, что показывает)</label>
    <NormalFormsView :frame="step.state" />
    <p class="caption" aria-live="polite">
      <template v-for="(seg, i) in captionSegments(step.caption)" :key="i"><T v-if="seg.termId" :id="seg.termId">{{ seg.text }}</T><template v-else>{{ seg.text }}</template></template>
    </p>
  </VizFrame>
</template>

<style scoped>
.intro { margin: 0 0 10px; font-size: 14px; color: var(--viz-muted); }
.panel { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 10px; }
.panel button { min-width: 36px; min-height: 36px; padding: 0 12px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; cursor: pointer; }
.panel button:focus-visible, input:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
.check { display: flex; align-items: flex-start; gap: 8px; min-height: 36px; margin-bottom: 10px; font-size: 14px; }
.check input { flex: none; width: 20px; height: 20px; margin: 0; }
.caption { min-height: 4.5em; margin: 8px 0 0; line-height: 1.6; }
</style>
