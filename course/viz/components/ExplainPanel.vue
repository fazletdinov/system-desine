<script setup lang="ts">
import type { Explanation } from '../core/explain'
import CaptionText from './CaptionText.vue'

withDefaults(defineProps<{ explanation: Explanation | null; fallback?: string }>(), {
  fallback: 'Кликните на элемент схемы — здесь появится объяснение',
})
</script>

<template>
  <aside class="explain-panel" aria-live="polite">
    <p v-if="explanation === null" class="fallback">{{ fallback }}</p>
    <template v-else>
      <h4 class="title">{{ explanation.title }}</h4>
      <p class="kind">{{ explanation.kind }}</p>
      <section v-for="s in explanation.sections" :key="s.heading" class="section">
        <h5>{{ s.heading }}</h5>
        <p><CaptionText :caption="s.body" /></p>
      </section>
      <section v-if="explanation.here !== undefined" class="box here">
        <h5>Здесь</h5>
        <p><CaptionText :caption="explanation.here" /></p>
      </section>
      <section v-if="explanation.real !== undefined" class="box real">
        <h5>В настоящей системе</h5>
        <p><CaptionText :caption="explanation.real" /></p>
      </section>
    </template>
  </aside>
</template>

<style scoped>
.explain-panel { padding: 12px 14px; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); font-size: 14px; line-height: 1.55; }
.fallback { margin: 0; color: var(--vp-c-text-2); }
.title { margin: 0; font-size: 16px; }
.kind { margin: 2px 0 8px; color: var(--vp-c-text-2); font-size: 12px; }
h5 { margin: 0 0 2px; color: var(--vp-c-text-2); font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; }
p { margin: 0; white-space: pre-line; }
.section { margin-top: 10px; }
.box { margin-top: 10px; padding: 8px 10px; border: 2px solid; border-radius: 6px; }
.here { border-color: var(--viz-active); }
.real { border-color: var(--viz-cache); }
</style>
