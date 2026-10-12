<script setup lang="ts">
defineProps<{ title: string }>()
</script>

<template>
  <!-- Нативный details: доступен с клавиатуры, а поиск по странице раскрывает его сам. -->
  <details class="deep">
    <summary><span aria-hidden="true">🔬</span> Под капотом: {{ title }}</summary>
    <div class="deep-body"><slot /></div>
  </details>
</template>

<style scoped>
.deep {
  margin: 20px 0;
  border-left: 4px solid var(--vp-c-brand-1);
  border-radius: 8px;
  background: var(--course-deep-bg);
}
.deep summary {
  padding: 10px 14px;
  color: var(--part-deep-fg);
  font-weight: 600;
  cursor: pointer;
}
.deep summary:focus-visible { outline: 2px solid var(--vp-c-brand-1); outline-offset: 2px; }
.deep-body { padding: 0 16px 12px; }
.deep-body :deep(p:last-child) { margin-bottom: 0; }
</style>
