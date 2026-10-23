<script setup lang="ts">
import { ref, watch } from 'vue'
import type { Explanation } from '../core/explain'
import ExplainPanel from './ExplainPanel.vue'

const props = defineProps<{
  explanation: Explanation | null
  simplifications?: Explanation
  fallback?: string
}>()

const showSimplifications = ref(false)

// Выбор элемента заменяет «Что упрощено»: панель одна, показывает последнее действие.
watch(() => props.explanation, (e) => { if (e !== null) showSimplifications.value = false })
</script>

<template>
  <div class="explain-aside">
    <div class="aside-grid">
    <div class="aside-main"><slot /></div>
    <div class="aside-panel">
      <ExplainPanel :explanation="showSimplifications && simplifications ? simplifications : explanation" :fallback="fallback" />
      <button v-if="simplifications" type="button" class="simplify" :aria-pressed="showSimplifications" @click="showSimplifications = !showSimplifications">
        Что в модели упрощено
      </button>
    </div>
    </div>
  </div>
</template>

<style scoped>
.explain-aside { container-type: inline-size; }
.aside-grid { display: grid; gap: 16px; }
/* Ширина контейнера, а не окна: песочница бывает и в узкой колонке урока. Сам контейнер сетку не стилизует, поэтому она внутри. */
@container (min-width: 900px) { .aside-grid { grid-template-columns: minmax(0, 1fr) 340px; align-items: start; } }
.aside-main, .aside-panel { min-width: 0; }
.simplify { margin-top: 8px; min-height: 40px; padding: 0 12px; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); cursor: pointer; }
.simplify[aria-pressed='true'] { border-color: var(--viz-active); }
.simplify:focus-visible { outline: 2px solid var(--viz-active); outline-offset: 2px; }
</style>
