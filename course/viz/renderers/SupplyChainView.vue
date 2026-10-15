<script setup lang="ts">
import type { ChainState } from '../simulations/supply-chain'

defineProps<{ state: ChainState }>()

// Выражение GitHub Actions; в шаблоне Vue его скобки пришлось бы экранировать.
const SECRET_EXPR = '${' + '{ secrets.DEPLOY_KEY }}'
</script>

<template>
  <div class="sc">
    <section class="block" aria-label="Workflow">
      <h4>workflow</h4>
      <ul class="yaml">
        <li v-for="a in state.actions" :key="a.name" :class="{ hijacked: state.attacked && state.pin === 'tag' && a.name === 'changed-files' }">
          <code>uses: {{ a.ref }}</code>
          <span v-if="state.attacked && state.pin === 'tag' && a.name === 'changed-files'" class="tag">тег перевешен</span>
        </li>
        <li><code>permissions: {{ state.permissions === 'write-all' ? 'write-all' : 'contents: read' }}</code></li>
        <li v-if="state.secretInEnv"><code>env: DEPLOY_KEY: {{ SECRET_EXPR }}</code></li>
      </ul>
    </section>
    <section class="block" aria-label="Что выполнилось">
      <h4>что выполнилось</h4>
      <p v-if="state.result === null" class="meta">Workflow ещё не запускали.</p>
      <ul v-else class="chips">
        <li v-for="e in state.result.executed" :key="e.name" :class="{ bad: e.malicious }">
          <b>{{ e.name }}</b>
          <span>{{ e.malicious ? 'код злоумышленника' : 'исходный код' }}</span>
        </li>
      </ul>
    </section>
    <section class="block" aria-label="Что утекло">
      <h4>что утекло</h4>
      <p v-if="state.result === null" class="meta">Запустите workflow.</p>
      <p v-else-if="state.result.leaked.length === 0" class="ok">ничего</p>
      <ul v-else class="leaks">
        <li v-for="x in state.result.leaked" :key="x"><code>{{ x }}</code></li>
      </ul>
    </section>
  </div>
</template>

<style scoped>
.sc { container-type: inline-size; display: flex; flex-direction: column; gap: 8px; color: var(--viz-text); }
.block { padding: 8px 10px; border: 2px solid var(--viz-edge); border-radius: 10px; background: var(--viz-node-bg); min-width: 0; }
h4 { margin: 0 0 6px; font-size: 15px; }
.meta { margin: 2px 0; font-size: 13px; color: var(--viz-muted); }
.ok { margin: 2px 0; font-weight: 700; }
ul { margin: 0; padding: 0; list-style: none; display: flex; flex-direction: column; gap: 4px; }
.yaml li { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 8px; padding: 3px 6px; border-left: 4px solid var(--viz-edge); border-radius: 4px; transition: border-color 0.2s, background-color 0.2s; }
.yaml li.hijacked { border-left-color: var(--viz-error); background: var(--viz-active-line-bg); }
code { font-size: 12px; overflow-wrap: anywhere; }
.tag { padding: 1px 6px; border: 2px solid var(--viz-error); border-radius: 6px; font-size: 11px; font-weight: 700; }
.chips { flex-direction: row; flex-wrap: wrap; gap: 8px; }
.chips li { display: flex; flex-direction: column; padding: 4px 10px; border: 2px solid var(--viz-response); border-radius: 8px; font-size: 13px; }
.chips li.bad { border-color: var(--viz-error); border-style: dashed; }
.chips li.bad span { font-weight: 700; }
.leaks li { padding: 3px 6px; border-left: 4px solid var(--viz-error); border-radius: 4px; }
@media (prefers-reduced-motion: reduce) { .yaml li { transition: none; } }
</style>
