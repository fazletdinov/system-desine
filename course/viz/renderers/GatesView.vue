<script setup lang="ts">
import { computed } from 'vue'
import { GATES, type GatesState } from '../simulations/validation-gates'

const props = defineProps<{ state: GatesState }>()

type GateStatus = 'passed' | 'here-pass' | 'here-reject' | 'ahead' | 'unreached'

const STATUS_TEXT: Record<GateStatus, string> = {
  passed: '✓ пропустил',
  'here-pass': '✓ пропустил — ввод здесь',
  'here-reject': '✗ отказ — ввод здесь',
  ahead: '… впереди',
  unreached: '— не дошёл',
}

const current = computed(() => GATES.findIndex((g) => g.id === props.state.gate))

const gates = computed(() =>
  GATES.map((g, i) => {
    let status: GateStatus
    if (i < current.value) status = 'passed'
    else if (i === current.value) status = props.state.verdict === 'pass' ? 'here-pass' : 'here-reject'
    else status = props.state.verdict === 'pass' ? 'ahead' : 'unreached'
    return { ...g, status, text: STATUS_TEXT[status] }
  }),
)

const verdictText = computed(() => {
  const label = GATES[current.value]?.label ?? ''
  if (props.state.verdict === 'reject') return `Отклонён на воротах «${label}»`
  return props.state.gate === 'stored' ? 'Прошёл все ворота и сохранён' : `Прошёл ворота «${label}», идёт дальше`
})
</script>

<template>
  <div class="gates">
    <p class="input"><span class="key">Ввод</span><code>{{ state.input }}</code></p>
    <ol class="line" aria-label="Ворота валидации по порядку">
      <li v-for="g in gates" :key="g.id" class="gate" :class="g.status" :aria-current="g.id === state.gate ? 'step' : undefined">
        <span class="name">{{ g.label }}</span>
        <code class="check">{{ g.check }}</code>
        <span class="status">{{ g.text }}</span>
      </li>
    </ol>
    <p class="verdict" :class="state.verdict">
      <strong>{{ state.verdict === 'reject' ? '✗' : '✓' }} {{ verdictText }}</strong>
      <span v-if="state.reason" class="reason">{{ state.reason }}</span>
    </p>
  </div>
</template>

<style scoped>
.gates { container-type: inline-size; padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
.input { display: flex; flex-wrap: wrap; align-items: baseline; gap: 4px 10px; margin: 0 0 12px; }
.key { font-size: 13px; font-weight: 700; color: var(--viz-muted); text-transform: uppercase; letter-spacing: 0.04em; }
.input code { white-space: pre-wrap; font-size: 15px; color: var(--viz-text); background: none; padding: 0; overflow-wrap: anywhere; }
.line { list-style: none; margin: 0; padding: 0; display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 20px; }
.gate { position: relative; margin: 0; display: flex; flex-direction: column; gap: 4px; padding: 8px 10px; border: 2px solid var(--viz-edge); border-radius: 10px; background: var(--viz-node-bg); }
.gate:not(:last-child)::after { content: '→'; position: absolute; right: -10px; top: 50%; transform: translate(50%, -50%); font-size: 14px; line-height: 1; color: var(--viz-muted); }
.gate.passed, .gate.here-pass { border-color: var(--viz-response); }
.gate.here-pass { border-width: 4px; }
.gate.here-reject { border-color: var(--viz-error); border-width: 4px; }
.gate.ahead, .gate.unreached { border-style: dashed; }
.name { font-size: 14px; font-weight: 700; }
.check { font-size: 13px; color: var(--viz-muted); background: none; padding: 0; overflow-wrap: anywhere; }
.status { font-size: 13px; font-weight: 600; }
.gate.ahead .status, .gate.unreached .status { font-weight: 400; color: var(--viz-muted); }
.verdict { display: flex; flex-direction: column; gap: 4px; margin: 12px 0 0; font-size: 15px; }
.verdict.reject strong { color: var(--viz-error); }
.reason { font-family: var(--vp-font-family-mono); font-size: 13px; overflow-wrap: anywhere; }
/* В колонке урока (~560 px) пять ворот в ряд не помещаются — ставим их друг под другом. */
@container (max-width: 640px) {
  .line { grid-template-columns: 1fr; gap: 14px; }
  .gate { display: grid; grid-template-columns: minmax(9.5em, auto) minmax(0, 1fr) auto; align-items: baseline; column-gap: 12px; }
  .gate:not(:last-child)::after { content: '↓'; right: auto; left: 24px; top: 100%; bottom: auto; line-height: 14px; transform: translate(-50%, 2px); }
}
@container (max-width: 420px) {
  .gate { grid-template-columns: auto minmax(0, 1fr); }
  .gate .status { grid-column: 1 / -1; }
}
</style>
