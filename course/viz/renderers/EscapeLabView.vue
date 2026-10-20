<script setup lang="ts">
import { computed } from 'vue'
import type { EscapeFrame, Verdict } from '../simulations/escape-lab'

const props = defineProps<{ frame: EscapeFrame }>()

const BADGE: Record<Verdict, { icon: string; text: string }> = {
  stack: { icon: '✓', text: 'стек' },
  heap: { icon: '▲', text: 'куча' },
  'stack-limited': { icon: '◐', text: '' },
}
const badge = computed(() => {
  const b = BADGE[props.frame.verdict]
  return { icon: b.icon, text: props.frame.caveat?.badge ?? b.text }
})
const compilerLines = computed(() => props.frame.compiler.split('\n').map((text) => ({ text, decisive: props.frame.decisive.includes(text) })))
const onStack = computed(() => props.frame.verdict !== 'heap')
const onHeap = computed(() => props.frame.verdict !== 'stack')
</script>

<template>
  <div class="escape" :data-verdict="frame.verdict">
    <div class="verdict">
      <span class="badge" :class="`is-${frame.verdict}`">
        <span aria-hidden="true">{{ badge.icon }}</span> {{ badge.text }}
      </span>
      <span class="case">{{ frame.caseTitle }}</span>
    </div>
    <div class="zones" aria-hidden="true">
      <div class="zone stack" :class="{ on: onStack, partial: frame.verdict === 'stack-limited' }">
        <span class="zone-title">Стек (кадр функции)</span>
        <span v-if="onStack" class="chip">{{ frame.verdict === 'stack-limited' ? 'пока помещается' : 'объект' }}</span>
      </div>
      <div class="zone heap" :class="{ on: onHeap, partial: frame.verdict === 'stack-limited' }">
        <span class="zone-title">Куча (убирает GC)</span>
        <span v-if="onHeap" class="chip">{{ frame.verdict === 'stack-limited' ? 'если не поместилось' : 'объект' }}</span>
      </div>
    </div>
    <p v-if="frame.caveat" class="caveat"><b>Оговорка.</b> {{ frame.caveat.note }}</p>
    <pre class="code" tabindex="0" :aria-label="`Код варианта, ${frame.caseTitle}`"><code>{{ frame.source }}</code></pre>
    <p class="label">Вывод компилятора (<code>go build -gcflags=-m=1</code>):</p>
    <pre class="compiler" tabindex="0" aria-label="Вывод компилятора"><code><template v-for="(l, i) in compilerLines" :key="i"><span :class="{ decisive: l.decisive }">{{ l.text }}</span>{{ '\n' }}</template></code></pre>
  </div>
</template>

<style scoped>
.escape { display: flex; flex-direction: column; gap: 10px; min-width: 0; }
.verdict { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 12px; }
.case { font-size: 14px; color: var(--viz-muted); }
.badge { display: inline-flex; gap: 6px; align-items: center; padding: 4px 12px; border: 3px solid; border-radius: 8px; font-weight: 700; font-size: 16px; color: var(--viz-text); background: var(--viz-node-bg); }
.badge.is-stack { border-color: var(--viz-db); }
.badge.is-heap { border-color: var(--viz-cache); }
.badge.is-stack-limited { border-style: dashed; border-color: var(--viz-cache); border-left-color: var(--viz-db); border-bottom-color: var(--viz-db); }
.zones { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.zone { display: flex; flex-direction: column; gap: 6px; min-height: 64px; padding: 8px 10px; border: 2px dashed var(--viz-edge); border-radius: 8px; color: var(--viz-muted); font-size: 14px; }
.zone.on { border-style: solid; color: var(--viz-text); }
.zone.stack.on { border-color: var(--viz-db); }
.zone.heap.on { border-color: var(--viz-cache); }
.zone.partial.on { border-style: dashed; }
.zone-title { font-weight: 600; }
.chip { align-self: flex-start; padding: 2px 8px; border-radius: 6px; background: var(--viz-node-bg); border: 1px solid currentColor; font-size: 13px; }
.caveat { margin: 0; padding: 8px 10px; border-left: 4px solid var(--viz-cache); background: var(--viz-active-line-bg); font-size: 14px; line-height: 1.5; color: var(--viz-text); }
.label { margin: 0; font-size: 14px; color: var(--viz-muted); }
.code, .compiler { margin: 0; padding: 10px 12px; overflow-x: auto; max-width: 100%; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg-soft); font-family: var(--vp-font-family-mono); font-size: 13px; line-height: 1.5; color: var(--viz-text); tab-size: 4; }
.compiler span { display: inline-block; }
.compiler .decisive { font-weight: 700; background: var(--viz-active-line-bg); }
.code:focus-visible, .compiler:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
@media (max-width: 480px) { .zones { grid-template-columns: 1fr; } }
</style>
