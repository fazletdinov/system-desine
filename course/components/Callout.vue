<script setup lang="ts">
import { computed } from 'vue'

type Kind = 'idea' | 'mistake' | 'tradeoff' | 'source' | 'simplified'

const KINDS: Record<Kind, { icon: string; title: string }> = {
  idea: { icon: '💡', title: 'Ключевая идея' },
  mistake: { icon: '⚠️', title: 'Типичная ошибка' },
  tradeoff: { icon: '⚖️', title: 'Компромисс' },
  source: { icon: '📚', title: 'Источник' },
  simplified: { icon: '⚠️', title: 'Упрощено' },
}

const props = defineProps<{ type: Kind; title?: string }>()
const meta = computed(() => KINDS[props.type])
</script>

<template>
  <aside class="callout" :class="`callout-${type}`">
    <p class="callout-title"><span aria-hidden="true">{{ meta.icon }}</span> {{ title ?? meta.title }}</p>
    <div class="callout-body"><slot /></div>
  </aside>
</template>

<style scoped>
.callout { margin: 20px 0; padding: 14px 18px; border-left: 4px solid; border-radius: 8px; }
.callout-title { margin: 0 0 6px; font-weight: 600; }
.callout-body :deep(p:last-child) { margin-bottom: 0; }
.callout-idea { border-color: var(--vp-c-tip-1); background: var(--vp-c-tip-soft); }
.callout-mistake { border-color: var(--vp-c-danger-1); background: var(--vp-c-danger-soft); }
.callout-tradeoff { border-color: var(--vp-c-warning-1); background: var(--vp-c-warning-soft); }
.callout-source { border-color: var(--vp-c-gray-1); background: var(--vp-c-gray-soft); }
.callout-simplified { border-color: var(--vp-c-warning-1); background: var(--vp-c-warning-soft); }
</style>
