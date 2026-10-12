<script setup lang="ts">
withDefaults(defineProps<{ tone?: 'num' | 'brand' | 'accent' }>(), { tone: 'num' })
</script>

<template>
  <span class="n" :class="`n-${tone}`"><slot /></span>
</template>

<style scoped>
.n {
  padding: 0 0.3em;
  border-radius: 5px;
  font-family: var(--vp-font-family-mono);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.n-num { color: var(--course-num-fg); background: var(--course-num-bg); }
.n-brand { color: var(--vp-c-brand-1); background: var(--vp-c-brand-soft); }
.n-accent { color: var(--course-accent); background: var(--course-accent-soft); }
</style>
