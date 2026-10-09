<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from 'vue'

const props = defineProps<{ line?: number }>()
const root = ref<HTMLElement>()

function highlight(): void {
  const lines = root.value?.querySelectorAll<HTMLElement>('span.line') ?? []
  lines.forEach((el, i) => el.classList.toggle('viz-active-line', i + 1 === props.line))
}

onMounted(highlight)
watch(() => props.line, () => nextTick(highlight))
</script>

<template>
  <div ref="root" class="viz-code-panel"><slot /></div>
</template>

<style scoped>
.viz-code-panel :deep(div[class*='language-']) { margin: 0; }
.viz-code-panel :deep(span.line) { display: inline-block; width: 100%; transition: background-color 0.2s; }
.viz-code-panel :deep(span.line.viz-active-line) { background: var(--viz-active-line-bg); box-shadow: inset 3px 0 0 var(--viz-active); }
</style>
