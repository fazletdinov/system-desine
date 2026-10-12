<script setup lang="ts">
import { computed } from 'vue'
import { useData } from 'vitepress'
import { lessonKicker } from './lesson-kicker'

const { page } = useData()
const kicker = computed(() => lessonKicker(page.value.relativePath))
</script>

<template>
  <p v-if="kicker" class="lesson-kicker">{{ kicker }}</p>
</template>

<style scoped>
.lesson-kicker {
  margin: 0 0 8px;
  color: var(--vp-c-brand-1);
  font-family: var(--vp-font-family-mono);
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
</style>
