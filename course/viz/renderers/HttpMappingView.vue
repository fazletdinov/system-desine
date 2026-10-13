<script setup lang="ts">
import { computed } from 'vue'
import { STAGES, type HttpMappingState } from '../simulations/http-mapping'

const props = defineProps<{ state: HttpMappingState }>()

const index = computed(() => STAGES.findIndex((s) => s.id === props.state.stage))
const current = computed(() => STAGES[index.value]!)
</script>

<template>
  <div class="mapping">
    <ol class="stages" aria-label="Представления сообщения по порядку">
      <li v-for="(s, i) in STAGES" :key="s.id" class="stage" :class="{ active: i === index, done: i < index }" :aria-current="i === index ? 'step' : undefined">
        <span class="no">{{ i + 1 }}</span>
        <span class="label">{{ s.label }}</span>
      </li>
    </ol>
    <div class="card">
      <p class="card-head"><strong>{{ index + 1 }}. {{ current.label }}</strong><span class="layer">слой: {{ current.layer }}</span></p>
      <pre class="shape"><code>{{ state.shape }}</code></pre>
    </div>
  </div>
</template>

<style scoped>
.mapping { padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
.stages { list-style: none; margin: 0 0 12px; padding: 0; display: flex; flex-wrap: wrap; gap: 6px; }
.stage { margin-top: 0; display: flex; align-items: center; gap: 6px; padding: 4px 10px; border: 2px solid var(--viz-edge); border-radius: 999px; font-size: 13px; }
.stage.done { border-color: var(--viz-network); }
.stage.active { border-color: var(--viz-active); border-width: 3px; font-weight: 700; }
.no { font-variant-numeric: tabular-nums; color: var(--viz-muted); }
.stage.active .no { color: var(--viz-text); }
.card { border: 2px solid var(--viz-service); border-radius: 10px; padding: 10px 12px; }
.card-head { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 2px 12px; margin: 0 0 8px; font-size: 15px; }
.layer { font-size: 13px; color: var(--viz-muted); }
.shape { margin: 0; padding: 0; background: none; white-space: pre-wrap; overflow-wrap: anywhere; tab-size: 2; }
.shape code { font-size: 14px; line-height: 1.55; color: var(--viz-text); background: none; padding: 0; }
@media (max-width: 480px) { .shape code { font-size: 13px; } }
</style>
