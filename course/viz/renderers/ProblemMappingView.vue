<script setup lang="ts">
import { computed } from 'vue'
import { PROBLEM_SOURCES, SOURCE_LABELS, type ProblemMappingState } from '../simulations/problem-mapping'

const props = defineProps<{ state: ProblemMappingState }>()

const rows = computed(() => (props.state.problem === null ? [] : Object.entries(props.state.problem)))
const isError = (v: unknown) => typeof v === 'number' && v >= 400
const show = (v: unknown) => (typeof v === 'string' ? v : JSON.stringify(v))
</script>

<template>
  <div class="pm">
    <ol class="sources" aria-label="Источники ошибок">
      <li v-for="s in PROBLEM_SOURCES" :key="s" class="src" :class="{ active: s === state.source }" :aria-current="s === state.source ? 'step' : undefined">{{ SOURCE_LABELS[s] }}</li>
    </ol>
    <div class="err">
      <p class="label">Что пришло изнутри</p>
      <code class="err-text">{{ state.error }}</code>
    </div>
    <div class="out" :class="{ empty: state.problem === null }">
      <p class="label">Что получит клиент</p>
      <template v-if="state.problem !== null">
        <ul v-if="state.headers.length > 0" class="headers"><li v-for="h in state.headers" :key="h"><code>{{ h }}</code></li></ul>
        <dl class="fields">
          <div v-for="[k, v] in rows" :key="k" class="row" :class="{ hl: state.highlightFields.includes(k) }">
            <dt>{{ k }}</dt>
            <dd :class="{ bad: k === 'status' && isError(v) }"><code>{{ show(v) }}</code></dd>
          </div>
        </dl>
      </template>
      <p v-else class="none">ошибка ещё не превращена в ответ</p>
    </div>
  </div>
</template>

<style scoped>
.pm { padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
.sources { list-style: none; margin: 0 0 12px; padding: 0; display: flex; flex-wrap: wrap; gap: 6px; }
.src { margin-top: 0; padding: 4px 10px; border: 2px solid var(--viz-edge); border-radius: 999px; font-size: 13px; }
.src.active { border-color: var(--viz-active); border-width: 3px; font-weight: 700; }
.label { margin: 0 0 6px; font-size: 13px; color: var(--viz-muted); }
.err { border: 2px solid var(--viz-error); border-radius: 10px; padding: 8px 12px; margin-bottom: 8px; }
.err-text { display: block; background: none; padding: 0; font-size: 13px; line-height: 1.5; color: var(--viz-text); overflow-wrap: anywhere; white-space: pre-wrap; }
.out { border: 2px solid var(--viz-response); border-radius: 10px; padding: 8px 12px; }
.out.empty { border-style: dashed; border-color: var(--viz-edge); }
.none { margin: 0; font-size: 13px; font-style: italic; color: var(--viz-muted); }
.headers { margin: 0 0 6px; padding: 0; list-style: none; }
.headers li { margin-top: 0; }
.headers code { background: none; padding: 0; font-size: 13px; color: var(--viz-text); overflow-wrap: anywhere; }
.fields { margin: 0; }
.row { display: grid; grid-template-columns: minmax(84px, 28%) minmax(0, 1fr); gap: 2px 10px; padding: 3px 6px; border-radius: 6px; border-left: 4px solid transparent; }
.row.hl { border-left-color: var(--viz-active); background: var(--viz-active-line-bg); }
.row dt { font-size: 13px; font-weight: 600; margin: 0; overflow-wrap: anywhere; }
.row dd { margin: 0; min-width: 0; }
.row code { background: none; padding: 0; font-size: 13px; color: var(--viz-text); overflow-wrap: anywhere; white-space: pre-wrap; }
.row dd.bad code { font-weight: 700; color: color-mix(in srgb, var(--viz-error) 60%, var(--viz-text)); }
</style>
