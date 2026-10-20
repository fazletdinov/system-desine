<script setup lang="ts">
import { computed } from 'vue'
import { DRAIN_DELAY_S, SHUTDOWN_TIMEOUT_S, HASH_CONCURRENCY, HASH_QUEUE, TIMELINE_S, type BackState, type HttpState, type LifecycleFrame } from '../simulations/app-lifecycle'

// drainStart — t первого кадра с draining, exitAt — t кадра с кодом выхода; оба считает сцена по всему сценарию.
const props = defineProps<{ state: LifecycleFrame; drainStart: number | null; exitAt: number | null }>()

const drainEnd = computed(() => (props.drainStart === null ? null : props.drainStart + DRAIN_DELAY_S))
// Выход раньше конца дренажа (второй сигнал): отметки нет, она ввела бы в заблуждение.
const showDrainMark = computed(() => drainEnd.value !== null && (props.exitAt === null || props.exitAt >= drainEnd.value))

const pct = (s: number): string => `${(s / TIMELINE_S) * 100}%`
const fmt = (s: number): string => String(s).replace('.', ',')
const HTTP_TEXT: Record<HttpState, string> = {
  serving: '● принимает запросы',
  draining: '◐ дренаж: ждёт, пока балансировщик уберёт инстанс',
  'shutting-down': '◑ Shutdown: ждёт начатые запросы',
  stopped: '✓ остановлен',
}
const BACK_TEXT: Record<BackState, string> = {
  running: '● работает',
  stopping: '◑ получил отмену, останавливается',
  stopped: '✓ остановлен',
  failed: '✕ вернул ошибку',
}
const httpClass = computed(() => (props.state.http === 'stopped' ? 'ok' : props.state.http === 'serving' ? 'mid' : 'warn'))
const backClass = (s: BackState): string => (s === 'failed' ? 'bad' : s === 'stopped' ? 'ok' : s === 'running' ? 'mid' : 'warn')
const exitClass = computed(() => (props.state.exit === 0 ? 'ok' : 'bad'))
</script>

<template>
  <div class="al">
    <p class="clock">Время: <strong>{{ fmt(state.t) }} с</strong> на шкале до {{ TIMELINE_S }} с · событие: {{ state.note }}</p>
    <div class="track" role="img" :aria-label="`Шкала 0–${TIMELINE_S} с, сейчас ${state.t} с`">
      <div class="elapsed" :style="{ width: pct(state.t) }"></div>
      <span v-if="showDrainMark" class="tick" :style="{ left: pct(drainEnd ?? 0) }" aria-hidden="true"></span>
    </div>
    <p class="scale">
      <template v-if="showDrainMark">Черта на {{ fmt(drainEnd ?? 0) }} с — конец DRAIN_DELAY ({{ DRAIN_DELAY_S }} с после начала дренажа); после неё начинается Shutdown (до {{ SHUTDOWN_TIMEOUT_S }} с).</template>
      <template v-else>Конец DRAIN_DELAY на шкале не отмечен: процесс вышел раньше, чем он наступил.</template>
    </p>

    <div class="lanes">
      <section class="lane" :class="httpClass" aria-label="HTTP-сервер">
        <h4>HTTP-сервер</h4>
        <p>{{ HTTP_TEXT[state.http] }}</p>
        <p class="sub">/readyz: {{ state.readyz === 200 ? '200 OK, готов' : '503, не готов' }}</p>
      </section>
      <section v-for="b in state.back" :key="b.name" class="lane" :class="backClass(b.state)" :aria-label="`Фоновый компонент ${b.name}`">
        <h4>Фоновый компонент</h4>
        <p>{{ b.name }}</p>
        <p class="sub">{{ BACK_TEXT[b.state] }}</p>
      </section>
      <section class="lane" aria-label="Очередь хешей Argon2id">
        <h4>Очередь хешей Argon2id</h4>
        <p>выполняются {{ state.hashQueue.running }} из {{ HASH_CONCURRENCY }}</p>
        <p class="sub">ждут {{ state.hashQueue.waiting }} из {{ HASH_QUEUE }}, отказано {{ state.hashQueue.rejected }}</p>
      </section>
    </div>

    <p class="exit" :class="state.exit === null ? '' : exitClass" role="status">
      {{ state.exit === null ? 'Процесс работает, кода выхода пока нет' : state.exit === 0 ? '✓ код выхода 0: штатная остановка' : `✕ код выхода ${state.exit}: оркестратор перезапустит инстанс` }}
    </p>
  </div>
</template>

<style scoped>
.al { padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
h4 { margin: 0; font-size: 13px; font-weight: 600; color: var(--viz-muted); }
.clock { margin: 0 0 6px; font-size: 14px; }
.track { position: relative; height: 12px; margin: 8px 0 6px; border: 2px solid var(--viz-edge); border-radius: 6px; }
.elapsed { height: 100%; background: var(--viz-request); border-radius: 4px; transition: width 0.3s; }
.tick { position: absolute; top: -4px; bottom: -4px; width: 0; border-left: 2px dashed var(--viz-muted); }
.scale { margin: 0 0 10px; font-size: 12px; color: var(--viz-muted); }
.lanes { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 170px), 1fr)); gap: 8px; }
.lane { min-width: 0; padding: 6px 10px; border: 2px solid var(--viz-edge); border-left-width: 8px; border-radius: 8px; }
.lane.ok { border-left-color: var(--viz-response); }
.lane.bad { border-left-color: var(--viz-error); }
.lane.warn, .lane.mid { border-left-color: var(--viz-active); }
.lane p { margin: 2px 0 0; font-size: 14px; font-weight: 700; overflow-wrap: anywhere; }
.lane p.sub { font-weight: 400; font-size: 13px; }
.exit { margin: 10px 0 0; padding: 8px 10px; border: 2px solid var(--viz-edge); border-left-width: 8px; border-radius: 8px; font-size: 15px; font-weight: 700; }
.exit.ok { border-left-color: var(--viz-response); }
.exit.bad { border-left-color: var(--viz-error); }
@media (prefers-reduced-motion: reduce) { .elapsed { transition: none; } }
</style>
