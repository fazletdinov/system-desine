<script setup lang="ts">
import { ref } from 'vue'

defineProps<{ question: string }>()
const open = ref(false)
</script>

<template>
  <div class="self-check">
    <p class="question"><strong>Вопрос.</strong> {{ question }}</p>
    <button type="button" class="toggle" :aria-expanded="open" @click="open = !open">
      {{ open ? 'Скрыть ответ' : 'Сначала ответьте сами, затем откройте ответ' }}
    </button>
    <div v-show="open" class="answer"><slot /></div>
  </div>
</template>

<style scoped>
.self-check { margin: 16px 0; padding: 14px 18px; border: 1px solid var(--vp-c-divider); border-radius: 8px; }
.question { margin: 0 0 10px; }
.toggle { padding: 6px 12px; border: 1px solid var(--vp-c-brand-1); border-radius: 6px; color: var(--vp-c-brand-1); font-size: 14px; }
.toggle:hover { background: var(--vp-c-brand-soft); }
.answer { margin-top: 12px; }
.answer :deep(p:last-child) { margin-bottom: 0; }
</style>
