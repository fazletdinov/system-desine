<script setup lang="ts">
import { computed } from 'vue'
import { PARTS, type PartKind } from './parts'

const props = defineProps<{ kind: PartKind }>()
const label = computed(() => PARTS[props.kind])
</script>

<template>
  <!-- ignore-header: VitePress не включает метку в текст пункта оглавления. -->
  <span class="part ignore-header" :class="`part-${kind}`">{{ label }}</span>
</template>

<style scoped>
.part {
  display: inline-block;
  margin-right: 0.5em;
  padding: 0.15em 0.6em;
  border-radius: 999px;
  font-size: 0.55em;
  font-weight: 700;
  letter-spacing: 0.02em;
  vertical-align: 0.3em;
  white-space: nowrap;
}
.part-problem { color: var(--part-problem-fg); background: var(--part-problem-bg); }
.part-theory { color: var(--part-theory-fg); background: var(--part-theory-bg); }
.part-viz { color: var(--part-viz-fg); background: var(--part-viz-bg); }
.part-mistakes { color: var(--part-mistakes-fg); background: var(--part-mistakes-bg); }
.part-tradeoff { color: var(--part-tradeoff-fg); background: var(--part-tradeoff-bg); }
.part-code { color: var(--part-code-fg); background: var(--part-code-bg); }
.part-selfcheck { color: var(--part-selfcheck-fg); background: var(--part-selfcheck-bg); }
.part-sources { color: var(--part-sources-fg); background: var(--part-sources-bg); }
.part-deep { color: var(--part-deep-fg); background: var(--part-deep-bg); }
.part-practice { color: var(--part-practice-fg); background: var(--part-practice-bg); }
</style>
