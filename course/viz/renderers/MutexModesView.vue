<script setup lang="ts">
import { computed } from 'vue'
import { STARVATION_MS, type MutexFrame } from '../simulations/mutex-modes'

const props = defineProps<{ state: MutexFrame }>()

const T_MAX = 2.5
const pct = (ms: number): string => `${(ms / T_MAX) * 100}%`
const fmt = (ms: number): string => String(ms).replace('.', ',')
const starving = computed(() => props.state.mode === 'starving')
const waiters = computed(() => props.state.waiters.map((w) => ({ ...w, waited: Math.round((props.state.t - w.since) * 10) / 10 })))
const HANDOFF_TEXT = {
  barging: 'barging: новичок на CPU перехватил замок',
  direct: 'прямая передача: замок отдан первому в очереди',
} as const
</script>

<template>
  <div class="mm">
    <p class="mode" :class="starving ? 'bad' : 'ok'" role="status">{{ starving ? '◆ режим голодания (starvation)' : '◇ нормальный режим' }}</p>
    <p class="clock">Время: <strong>{{ fmt(state.t) }} мс</strong></p>
    <div class="track" role="img" :aria-label="`Шкала 0–${T_MAX} мс, сейчас ${state.t} мс, порог ожидания ${STARVATION_MS} мс`">
      <div class="elapsed" :style="{ width: pct(state.t) }"></div>
      <div v-for="w in waiters" :key="w.g" class="wait" :style="{ left: pct(w.since), width: pct(state.t - w.since) }" :class="{ over: w.waited > STARVATION_MS }"></div>
    </div>
    <p class="scale">Полоса под шкалой — ожидание горутины; она краснеет, когда ожидание превысило порог {{ STARVATION_MS }} мс.</p>
    <div class="cards">
      <div class="card"><h4>Владелец замка</h4><p>{{ state.owner ?? 'замок свободен' }}</p></div>
      <div class="card"><h4>Приходит на CPU</h4><p>{{ state.arriving ?? 'никого' }}</p></div>
      <div class="card" :class="{ ok: state.handoff === 'direct', mid: state.handoff === 'barging' }"><h4>Передача</h4><p>{{ state.handoff ? HANDOFF_TEXT[state.handoff] : 'нет передачи' }}</p></div>
    </div>
    <section class="queue" aria-label="Очередь ожидающих">
      <h4>Очередь ожидающих (голова слева)</h4>
      <ol class="q">
        <li v-for="w in waiters" :key="w.g" :class="{ over: w.waited > STARVATION_MS }"><b>{{ w.g }}</b> ждёт {{ fmt(w.waited) }} мс<span v-if="w.waited > STARVATION_MS"> — больше порога</span></li>
        <li v-if="waiters.length === 0" class="empty">пусто</li>
      </ol>
    </section>
  </div>
</template>

<style scoped>
.mm { padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
h4 { margin: 0; font-size: 13px; font-weight: 600; color: var(--viz-muted); }
.mode { margin: 0 0 6px; padding: 6px 10px; border: 2px solid var(--viz-edge); border-left-width: 8px; border-radius: 8px; font-size: 18px; font-weight: 700; }
.mode.ok { border-left-color: var(--viz-response); }
.mode.bad { border-left-color: var(--viz-error); }
.clock { margin: 0 0 6px; font-size: 14px; }
.track { position: relative; height: 12px; margin: 0 0 6px; border: 2px solid var(--viz-edge); border-radius: 6px; }
.elapsed { height: 100%; background: var(--viz-request); border-radius: 4px; transition: width 0.3s; }
.wait { position: absolute; top: 14px; height: 8px; border-radius: 4px; background: var(--viz-active); }
.wait.over { background: var(--viz-error); }
.scale { margin: 14px 0 8px; font-size: 12px; color: var(--viz-muted); }
.cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 150px), 1fr)); gap: 8px; }
.card { min-width: 0; padding: 6px 10px; border: 2px solid var(--viz-edge); border-left-width: 8px; border-radius: 8px; }
.card.ok { border-left-color: var(--viz-response); }
.card.mid { border-left-color: var(--viz-active); }
.card p { margin: 2px 0 0; font-size: 14px; font-weight: 700; overflow-wrap: anywhere; }
.queue { margin-top: 10px; }
.q { display: flex; flex-wrap: wrap; gap: 6px; margin: 4px 0 0; padding: 0; list-style: none; }
.q li { padding: 4px 10px; border: 2px solid var(--viz-edge); border-radius: 6px; font-size: 13px; }
.q li.over { border-color: var(--viz-error); font-weight: 700; }
.empty { color: var(--viz-muted); }
@media (prefers-reduced-motion: reduce) { .elapsed { transition: none; } }
</style>
