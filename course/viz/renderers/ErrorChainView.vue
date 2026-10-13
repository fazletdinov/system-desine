<script setup lang="ts">
import { computed } from 'vue'
import { STATUS_CODE, type ErrorChainState } from '../simulations/error-chain'

const props = defineProps<{ state: ErrorChainState }>()

const checks = computed(() => [
  { id: 'is', call: 'errors.Is(err, domain.ErrChatNotFound)', value: props.state.isNotFound },
  { id: 'as', call: 'errors.As(err, &ve) // ve *domain.ValidationError', value: props.state.asValidation },
])

const statusText = computed(() => {
  const s = props.state.status
  if (s === null) return 'до адаптера ещё не дошли'
  return `${s} ${STATUS_CODE[s] ?? ''}`.trim()
})
</script>

<template>
  <div class="chain">
    <p class="title">Цепочка ошибки — от внешней к внутренней</p>
    <ol class="layers" aria-label="Обёртки ошибки, снаружи внутрь">
      <li v-for="(l, i) in state.layers" :key="l.where" class="layer" :style="{ marginLeft: `calc(${i} * var(--chain-indent))` }">
        <code class="where">{{ l.where }}</code>
        <span class="msg"><span class="key">Error():</span> «{{ l.message }}»</span>
        <span class="unwrap">{{ i < state.layers.length - 1 ? 'Unwrap() ↓ — внутри есть ещё ошибка' : 'конец цепочки: внутри ничего нет' }}</span>
      </li>
    </ol>
    <ul class="checks" aria-label="Проверки на границе">
      <li v-for="c in checks" :key="c.id" class="check" :class="c.value ? 'yes' : 'no'">
        <code>{{ c.call }}</code>
        <strong class="value">{{ c.value ? '✓ true' : '✗ false' }}</strong>
      </li>
    </ul>
    <p class="status" :class="{ pending: state.status === null, fail: state.status !== null && state.status >= 500 }">
      <span class="key">Ответ HTTP:</span> <strong>{{ statusText }}</strong>
    </p>
  </div>
</template>

<style scoped>
.chain { --chain-indent: 24px; padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
.title { margin: 0 0 8px; font-size: 14px; font-weight: 700; color: var(--viz-muted); }
.layers { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 8px; }
.layer { margin-top: 0; display: flex; flex-direction: column; gap: 2px; padding: 8px 12px; border: 2px solid var(--viz-service); border-radius: 10px; }
.layer:first-child { border-width: 3px; }
.where { font-size: 13px; color: var(--viz-muted); background: none; padding: 0; overflow-wrap: anywhere; }
.msg { font-size: 15px; font-weight: 600; overflow-wrap: anywhere; }
.key { font-weight: 400; color: var(--viz-muted); }
.unwrap { font-size: 13px; color: var(--viz-muted); }
.checks { list-style: none; margin: 12px 0 0; padding: 0; display: grid; gap: 8px; }
.check { margin-top: 0; display: flex; flex-wrap: wrap; justify-content: space-between; align-items: baseline; gap: 4px 12px; padding: 6px 12px; border: 2px solid var(--viz-edge); border-radius: 8px; }
.check.yes { border-color: var(--viz-response); }
.check.no { border-style: dashed; }
.check code { font-size: 13px; background: none; padding: 0; color: var(--viz-text); overflow-wrap: anywhere; }
.value { font-size: 15px; }
.check.no .value { color: var(--viz-muted); }
.status { margin: 12px 0 0; font-size: 15px; }
.status.pending strong { font-weight: 400; color: var(--viz-muted); }
.status.fail strong { color: var(--viz-error); }
@container (max-width: 520px) { .chain { --chain-indent: 12px; } }
</style>
