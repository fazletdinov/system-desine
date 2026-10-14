<script setup lang="ts">
import { computed } from 'vue'
import { duplicates, skipped, INITIAL_COUNT, type PagingState } from '../simulations/pagination'

const props = defineProps<{ state: PagingState }>()

interface Cell { seq: number; page: number | null; dup: boolean; skip: boolean; fresh: boolean; gone: boolean }

const cells = computed<Cell[]>(() => {
  const s = props.state
  const dup = new Set(duplicates(s))
  const skip = new Set(skipped(s))
  const alive = new Set(s.feed)
  // Удалённые остаются в ленте серым призраком: иначе не видно, откуда взялся сдвиг.
  const all = [...new Set([...s.feed, ...s.initial])].sort((a, b) => b - a)
  return all.map((seq) => {
    const idx = s.pages.findIndex((p) => p.includes(seq))
    return { seq, page: idx < 0 ? null : idx + 1, dup: dup.has(seq), skip: skip.has(seq), fresh: seq > INITIAL_COUNT, gone: !alive.has(seq) }
  })
})

const cursorJson = computed(() => (props.state.cursor === null ? null : JSON.stringify({ v: 1, seq: props.state.cursor })))
const cursorB64 = computed(() => (cursorJson.value === null ? '' : btoa(cursorJson.value).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')))
</script>

<template>
  <div class="pg">
    <p class="hint">История чата от новых к старым, страница — 5 сообщений. Цвет рамки — номер страницы.</p>
    <ul class="feed" aria-label="Лента сообщений">
      <li v-for="c in cells" :key="c.seq" class="cell" :class="[c.page !== null ? `p${(c.page - 1) % 4}` : 'unread', { dup: c.dup, skip: c.skip, gone: c.gone }]">
        <span class="seq">#{{ c.seq }}</span>
        <span v-if="c.page !== null" class="tag">стр. {{ c.page }}</span>
        <span v-if="c.dup" class="tag bad">дубль</span>
        <span v-if="c.skip" class="tag bad">пропуск</span>
        <span v-if="c.fresh && !c.gone" class="tag">новое</span>
        <span v-if="c.gone" class="tag">удалено</span>
      </li>
    </ul>
    <ol class="pages" aria-label="Ответы сервера">
      <li v-for="(p, i) in state.pages" :key="i">
        <b>Страница {{ i + 1 }}:</b> {{ p.length ? p.map((n) => `#${n}`).join(' ') : 'пусто, лента кончилась' }}
      </li>
    </ol>
    <p class="cur">
      <template v-if="state.mode === 'offset'">Состояние клиента: <code>offset={{ state.offset }}</code></template>
      <template v-else>
        Состояние клиента: курсор
        <template v-if="cursorJson"><code>{{ cursorJson }}</code> → <code class="b64">{{ cursorB64 }}</code></template>
        <template v-else>ещё не выдан</template>
      </template>
    </p>
  </div>
</template>

<style scoped>
.pg { container-type: inline-size; color: var(--viz-text); }
.hint { margin: 0 0 8px; font-size: 14px; color: var(--viz-muted); }
.feed { list-style: none; margin: 0 0 10px; padding: 0; display: grid; grid-template-columns: repeat(auto-fill, minmax(78px, 1fr)); gap: 6px; }
.cell { display: flex; flex-direction: column; gap: 1px; min-height: 44px; padding: 3px 6px; border: 2px solid var(--viz-edge); border-radius: 8px; background: var(--viz-node-bg); font-size: 12px; transition: border-color 0.2s; }
.seq { font-weight: 700; font-size: 14px; font-variant-numeric: tabular-nums; }
.tag { color: var(--viz-muted); }
.tag.bad { color: var(--viz-error); font-weight: 700; }
.p0 { border-color: var(--viz-client); }
.p1 { border-color: var(--viz-db); }
.p2 { border-color: var(--viz-cache); }
.p3 { border-color: var(--viz-broker); }
.cell.dup { border-color: var(--viz-error); border-width: 3px; }
.cell.skip { position: relative; border-color: var(--viz-error); border-style: dashed; }
/* Штриховка — только полоска сверху: под текстом остаётся сплошной фон. */
.cell.skip::before { content: ''; position: absolute; left: 0; right: 0; top: 0; height: 5px; background: repeating-linear-gradient(135deg, var(--viz-error) 0 3px, transparent 3px 6px); }
.cell.gone .seq { text-decoration: line-through; color: var(--viz-muted); }
.pages { margin: 0 0 8px; padding-left: 22px; font-size: 14px; }
.pages b { font-weight: 600; }
.cur { margin: 0; font-size: 14px; overflow-wrap: anywhere; }
code { font-size: 13px; padding: 0 4px; overflow-wrap: anywhere; }
@media (prefers-reduced-motion: reduce) { .cell { transition: none; } }
</style>
