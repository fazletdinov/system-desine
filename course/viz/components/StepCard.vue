<script setup lang="ts">
import type { StepCard } from '../core/types'
import CaptionText from './CaptionText.vue'

defineProps<{ card: StepCard }>()

const ROWS = [
  { key: 'what', title: 'Что происходит' },
  { key: 'uses', title: 'Какая структура' },
  { key: 'reads', title: 'Что читается' },
  { key: 'why', title: 'Почему так' },
] as const
</script>

<template>
  <dl class="step-card">
    <div v-for="row in ROWS" :key="row.key" class="cell">
      <dt class="term">{{ row.title }}</dt>
      <dd class="desc"><CaptionText :caption="card[row.key]" /></dd>
    </div>
  </dl>
</template>

<style scoped>
.step-card { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; margin: 12px 0 0; }
.cell { padding: 10px 12px; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); }
.term { margin-bottom: 4px; color: var(--vp-c-text-2); font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; }
/* Тексты карточки содержат \n и «• » — переносы должны сохраняться. */
.desc { margin: 0; line-height: 1.55; white-space: pre-line; }
</style>
