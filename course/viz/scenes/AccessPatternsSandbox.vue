<script setup lang="ts">
import { computed, ref } from 'vue'
import T from '../../components/T.vue'
import VizFrame from '../components/VizFrame.vue'
import SegmentedToggle from '../components/SegmentedToggle.vue'
import AccessPatternsView from '../renderers/AccessPatternsView.vue'
import { captionSegments } from '../core/caption'
import { MESSAGES_KEYS, QUERIES, accessPatternsSimulation as sim, type AccessPatternsAction, type MessagesKey } from '../simulations/access-patterns'

const state = ref(sim.initial())
const step = computed(() => sim.steps(state.value)[0]!)

const dispatch = (action: AccessPatternsAction): void => {
  state.value = sim.apply(state.value, action)
}

const queries = QUERIES.map((q) => ({ value: q.id, text: q.title }))
const keys = (Object.keys(MESSAGES_KEYS) as MessagesKey[]).map((k) => ({ value: k, text: MESSAGES_KEYS[k].title }))
const memberIndex = [
  { value: true, text: 'есть INDEX (user_id)' },
  { value: false, text: 'нет индекса по user_id' },
]
</script>

<template>
  <VizFrame label="Песочница: паттерны доступа">
    <p class="intro">Выберите запрос мессенджера и смотрите, какой ключ или индекс его обслуживает и сколько строк придётся прочитать. Меняйте ключ сообщений и индекс участников — и следите, какие запросы скатываются в полный проход. Объёмы таблиц модельные, это не замер.</p>
    <SegmentedToggle label="Запрос" :options="queries" :model-value="state.queryId" @update:model-value="(queryId) => dispatch({ type: 'query', queryId })" />
    <p class="group">Ключ <code>chat.messages</code>:</p>
    <SegmentedToggle label="Ключ chat.messages" :options="keys" :model-value="state.schema.messagesKey" @update:model-value="(key) => dispatch({ type: 'messages-key', key })" />
    <p class="group">Индекс <code>chat.members (user_id)</code>:</p>
    <SegmentedToggle label="Индекс chat.members по user_id" :options="memberIndex" :model-value="state.schema.membersUserIndex" @update:model-value="(on) => dispatch({ type: 'members-user-index', on })" />
    <div class="panel">
      <button type="button" @click="dispatch({ type: 'reset' })">Схема мессенджера</button>
    </div>
    <AccessPatternsView :frame="step.state" />
    <p class="caption" aria-live="polite">
      <template v-for="(seg, i) in captionSegments(step.caption)" :key="i"><T v-if="seg.termId" :id="seg.termId">{{ seg.text }}</T><template v-else>{{ seg.text }}</template></template>
    </p>
  </VizFrame>
</template>

<style scoped>
.intro { margin: 0 0 10px; font-size: 14px; color: var(--viz-muted); }
.group { margin: 0 0 4px; font-size: 14px; }
.panel { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 10px; }
.panel button { min-width: 36px; min-height: 36px; padding: 0 12px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; cursor: pointer; }
.panel button:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
.caption { min-height: 4.5em; margin: 8px 0 0; line-height: 1.6; }
</style>
