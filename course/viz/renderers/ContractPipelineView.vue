<script setup lang="ts">
import { computed } from 'vue'
import { STAGES, type ContractPipelineState } from '../simulations/contract-pipeline'

const props = defineProps<{ state: ContractPipelineState }>()

const index = computed(() => STAGES.findIndex((s) => s.id === props.state.stage))
const verdict = computed(() => {
  if (props.state.caught === 'compiler') return 'Поймал компилятор: go build'
  if (props.state.caught === 'contract-test') return 'Поймал контрактный тест'
  return null
})
</script>

<template>
  <div class="cp">
    <ol class="stages" aria-label="Путь от контракта до проверки">
      <li v-for="(s, i) in STAGES" :key="s.id" class="stage" :class="{ active: i === index, done: i < index }" :aria-current="i === index ? 'step' : undefined">
        <span class="no">{{ i + 1 }}</span><span>{{ s.label }}</span>
      </li>
    </ol>
    <p v-if="verdict" class="verdict" role="status">{{ verdict }}</p>
    <div v-for="(f, i) in state.files" :key="i" class="file" :class="{ caught: state.caught !== null }">
      <p class="path"><code>{{ f.path }}</code></p>
      <pre class="snippet"><code>{{ f.snippet }}</code></pre>
    </div>
  </div>
</template>

<style scoped>
.cp { padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
.stages { list-style: none; margin: 0 0 12px; padding: 0; display: flex; flex-wrap: wrap; gap: 6px; }
.stage { margin-top: 0; display: flex; align-items: center; gap: 6px; padding: 4px 10px; border: 2px solid var(--viz-edge); border-radius: 999px; font-size: 13px; }
.stage.done { border-color: var(--viz-network); }
.stage.active { border-color: var(--viz-active); border-width: 3px; font-weight: 700; }
.no { font-variant-numeric: tabular-nums; color: var(--viz-muted); }
.stage.active .no { color: var(--viz-text); }
.verdict { margin: 0 0 8px; padding: 6px 10px; border: 2px solid var(--viz-error); border-radius: 8px; font-size: 14px; font-weight: 700; color: color-mix(in srgb, var(--viz-error) 60%, var(--viz-text)); }
.file { border: 2px solid var(--viz-service); border-radius: 10px; padding: 8px 12px; margin-top: 8px; min-width: 0; }
.file.caught { border-color: var(--viz-error); }
.path { margin: 0 0 6px; font-size: 13px; color: var(--viz-muted); }
.path code { background: none; padding: 0; color: var(--viz-muted); overflow-wrap: anywhere; }
.snippet { margin: 0; padding: 0; background: none; white-space: pre-wrap; overflow-wrap: anywhere; tab-size: 2; max-width: 100%; }
.snippet code { font-size: 13px; line-height: 1.55; color: var(--viz-text); background: none; padding: 0; }
</style>
