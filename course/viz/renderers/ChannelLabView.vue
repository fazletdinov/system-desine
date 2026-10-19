<script setup lang="ts">
import { computed } from 'vue'
import type { ChannelLabState, GState } from '../simulations/channel-lab'

const props = defineProps<{ state: ChannelLabState }>()

const STATE_TEXT: Record<GState, string> = {
  running: 'работает',
  'blocked-send': 'ждёт отправки',
  'blocked-recv': 'ждёт получения',
  panicked: 'паника',
  done: 'завершена',
}

const LOG_LINES = 6
const recent = computed(() => props.state.log.slice(-LOG_LINES).reverse())
// Ячейки кольца: у канала с cap=0 буфера нет — рисуем одну щель «рандеву».
const cells = computed(() => Array.from({ length: props.state.cap }, (_, i) => props.state.buf[i] ?? null))
const gotText = (got: { value: number | null; ok: boolean }): string => `получено ${got.value ?? '0 (ноль)'}, ok=${got.ok}`
</script>

<template>
  <div class="lab">
    <div class="flow">
      <section class="queue" aria-label="Очередь ждущих отправителей">
        <h4>sendq</h4>
        <ul>
          <li v-for="w in state.sendq" :key="w.g">{{ w.g }} → {{ w.value }}</li>
          <li v-if="state.sendq.length === 0" class="empty">пусто</li>
        </ul>
      </section>
      <section class="chan" :class="{ closed: state.closed, nil: state.nil }" aria-label="Канал">
        <h4>
          {{ state.nil ? 'nil-канал' : `канал, ёмкость ${state.cap}` }}
          <span v-if="state.closed" class="tag">закрыт</span>
        </h4>
        <p v-if="state.nil" class="meta">Канала нет: ни отправка, ни получение никогда не станут готовыми.</p>
        <div v-else-if="state.cap === 0" class="ring" role="img" aria-label="Буфера нет, значение передаётся напрямую">
          <div class="cell rendezvous">рандеву</div>
        </div>
        <div v-else class="ring" role="img" :aria-label="`Буфер: занято ${state.buf.length} из ${state.cap}`">
          <div v-for="(c, i) in cells" :key="i" class="cell" :class="{ full: c !== null }">{{ c ?? '·' }}</div>
        </div>
        <p v-if="!state.nil && state.cap > 0" class="meta">занято {{ state.buf.length }} из {{ state.cap }}</p>
      </section>
      <section class="queue" aria-label="Очередь ждущих получателей">
        <h4>recvq</h4>
        <ul>
          <li v-for="g in state.recvq" :key="g">{{ g }}</li>
          <li v-if="state.recvq.length === 0" class="empty">пусто</li>
        </ul>
      </section>
    </div>
    <ul class="gs" aria-label="Горутины">
      <li v-for="g in state.gs" :key="g.id" class="g" :class="g.state">
        <b>{{ g.id }}</b>
        <span class="status">{{ STATE_TEXT[g.state] }}</span>
        <span v-if="g.holding !== undefined" class="meta">держит {{ g.holding }}</span>
        <span v-if="g.got" class="meta">{{ gotText(g.got) }}</span>
        <span v-if="state.nil && (g.state === 'blocked-send' || g.state === 'blocked-recv')" class="meta">навсегда</span>
      </li>
    </ul>
    <ol class="log" aria-label="Журнал событий">
      <li v-for="(l, i) in recent" :key="state.log.length - i" :class="l.kind">{{ l.text }}</li>
      <li v-if="recent.length === 0" class="empty">Событий пока нет.</li>
    </ol>
  </div>
</template>

<style scoped>
.lab { container-type: inline-size; color: var(--viz-text); }
.flow { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 2fr) minmax(0, 1fr); gap: 8px; margin-bottom: 10px; }
section { padding: 8px 10px; border: 2px solid var(--viz-edge); border-radius: 10px; background: var(--viz-node-bg); min-width: 0; }
h4 { margin: 0 0 6px; font-size: 15px; }
.chan { border-color: var(--viz-broker); }
.chan.closed { border-style: dashed; border-color: var(--viz-error); }
.chan.nil { border-style: dotted; border-color: var(--viz-network); }
.tag { margin-left: 6px; padding: 1px 6px; border: 1px solid var(--viz-error); border-radius: 6px; font-size: 12px; color: var(--viz-error); }
.ring { display: flex; flex-wrap: wrap; gap: 6px; }
.cell { min-width: 44px; min-height: 40px; display: grid; place-items: center; border: 2px solid var(--viz-edge); border-radius: 8px; font-size: 18px; font-variant-numeric: tabular-nums; color: var(--viz-muted); }
.cell.full { border-color: var(--viz-broker); color: var(--viz-text); font-weight: 700; background: color-mix(in srgb, var(--viz-broker) 18%, var(--viz-node-bg)); }
.cell.rendezvous { font-size: 14px; border-style: dashed; padding: 0 10px; }
ul, ol { margin: 0; padding: 0; list-style: none; }
.queue li { font-size: 14px; padding: 2px 0; font-variant-numeric: tabular-nums; }
.empty, .meta { font-size: 13px; color: var(--viz-muted); }
.meta { margin: 4px 0 0; }
.gs { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 8px; margin-bottom: 10px; }
.g { display: flex; flex-direction: column; gap: 2px; padding: 8px 10px; border: 2px solid var(--viz-edge); border-left-width: 8px; border-radius: 10px; background: var(--viz-node-bg); min-width: 0; font-size: 14px; }
.g .status { font-weight: 700; }
.g.running { border-color: var(--viz-response); }
.g.blocked-send, .g.blocked-recv { border-color: var(--viz-cache); }
.g.panicked { border-color: var(--viz-error); }
.g.done { border-color: var(--viz-network); }
.g .meta { margin: 0; overflow-wrap: anywhere; }
.log { display: flex; flex-direction: column; gap: 2px; font-size: 14px; }
.log li { padding-left: 8px; border-left: 4px solid var(--viz-edge); overflow-wrap: anywhere; }
.log li.panic { border-left-color: var(--viz-error); font-weight: 700; }
.log li.blocked { border-left-color: var(--viz-cache); }
.log li.handoff, .log li.buffered, .log li.woke { border-left-color: var(--viz-response); }
.log li.zero, .log li.default { border-left-color: var(--viz-network); }
@container (max-width: 520px) {
  .flow { grid-template-columns: 1fr; }
  .gs { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
</style>
