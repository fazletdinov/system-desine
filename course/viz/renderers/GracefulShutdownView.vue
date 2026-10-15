<script setup lang="ts">
import { computed } from 'vue'
import { DRAIN_DELAY_S, DOCKER_STOP_TIMEOUT_S, LB_REMOVED_S, TIMELINE_S, type ShutdownFrame, type ShutdownProcess } from '../simulations/graceful-shutdown'

const props = defineProps<{ state: ShutdownFrame }>()

const pct = (s: number): string => `${(s / TIMELINE_S) * 100}%`
const wrapper = computed(() => props.state.signalAt === 'pid1-sh')
const done = computed(() => props.state.process === 'exited' || props.state.process === 'killed')

// Отметки на шкале: номер, время, подпись, достигнута ли.
const marks = computed(() => {
  const s = props.state
  const list: { n: number; at: number; text: string; reached: boolean }[] = [{ n: 1, at: 0, text: wrapper.value ? 'SIGTERM пришёл в sh (PID 1)' : 'SIGTERM пришёл приложению (PID 1)', reached: true }]
  if (!wrapper.value) {
    list.push({ n: 2, at: LB_REMOVED_S, text: 'балансировщик убрал инстанс', reached: !s.inLb })
    list.push({ n: 3, at: DRAIN_DELAY_S, text: 'конец дренажа, Shutdown', reached: s.process === 'shutting-down' || s.process === 'exited' })
    list.push({ n: 4, at: 6, text: 'выход, код 0', reached: s.process === 'exited' })
  } else {
    list.push({ n: 2, at: DOCKER_STOP_TIMEOUT_S, text: 'SIGKILL, код 137', reached: s.process === 'killed' })
  }
  return list
})

const PROCESS_TEXT: Record<ShutdownProcess, string> = {
  running: '● работает как обычно',
  draining: '◐ дренаж: ждёт, пока балансировщик уберёт его',
  'shutting-down': '◑ останавливается: ждёт начатые запросы',
  exited: '✓ вышел штатно',
  killed: '✕ убит SIGKILL',
}
const NEW_TEXT = { served: 'обслуживаются', refused: 'отклоняются', none: 'не приходят' } as const
const procClass = computed(() => (props.state.process === 'killed' ? 'bad' : props.state.process === 'exited' ? 'ok' : 'mid'))
</script>

<template>
  <div class="gs">
    <p class="clock">Время от SIGTERM: <strong>{{ String(state.t).replace('.', ',') }} с</strong> на шкале до {{ TIMELINE_S }} с</p>
    <div class="track" role="img" :aria-label="`Шкала 0–${TIMELINE_S} с, сейчас ${state.t} с`">
      <div class="elapsed" :style="{ width: pct(state.t) }"></div>
      <span class="tick" :style="{ left: pct(DOCKER_STOP_TIMEOUT_S) }" aria-hidden="true"></span>
      <span v-for="(m, i) in marks" :key="m.n" class="mark" :class="{ reached: m.reached, lower: i % 2 === 1 }" :style="{ left: pct(m.at) }" aria-hidden="true">{{ m.n }}</span>
    </div>
    <p class="scale">Деления шкалы: 10 с — умолчание docker stop; 30 с — stop_grace_period в нашем compose (в Kubernetes — terminationGracePeriodSeconds).</p>
    <ol class="legend" aria-label="Отметки на шкале">
      <li v-for="m in marks" :key="m.n" :class="{ reached: m.reached }"><span class="n">{{ m.n }}</span>{{ m.reached ? '✓' : '○' }} {{ m.at }} с — {{ m.text }}</li>
    </ol>

    <div class="cards">
      <div class="card" :class="procClass"><h4>Процесс</h4><p>{{ PROCESS_TEXT[state.process] }}</p><p v-if="state.exitCode !== null" class="sub">код выхода {{ state.exitCode }}</p></div>
      <div class="card"><h4>/readyz</h4><p>{{ state.readyz === null ? 'не отвечает' : state.readyz === 200 ? '200 OK: готов' : '503: не готов' }}</p></div>
      <div class="card"><h4>Балансировщик</h4><p>{{ state.inLb ? 'инстанс в ротации' : 'инстанс убран' }}</p><p class="sub">новые запросы: {{ NEW_TEXT[state.newRequests] }}</p></div>
      <div class="card"><h4>Начатые запросы</h4><p>{{ done && state.process === 'killed' ? `оборвано ${state.inflight}` : state.inflight }}</p></div>
    </div>
  </div>
</template>

<style scoped>
.gs { padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
.clock { margin: 0 0 6px; font-size: 14px; }
.track { position: relative; height: 12px; margin: 22px 11px 34px; border: 2px solid var(--viz-edge); border-radius: 6px; }
.elapsed { height: 100%; background: var(--viz-request); border-radius: 4px; transition: width 0.3s; }
.mark { position: absolute; top: -22px; display: grid; place-items: center; width: 20px; height: 20px; margin-left: -10px; border: 2px solid var(--viz-muted); border-radius: 50%; background: var(--viz-node-bg); font-size: 11px; font-weight: 700; }
.mark.lower { top: 14px; }
.mark.reached { border-color: var(--viz-active); background: var(--viz-active-line-bg); }
.tick { position: absolute; top: -4px; bottom: -4px; width: 0; border-left: 2px dashed var(--viz-muted); }
.scale { margin: 0 0 6px; font-size: 12px; color: var(--viz-muted); }
.legend { margin: 0 0 12px; padding: 0; list-style: none; font-size: 13px; color: var(--viz-muted); }
.legend li { margin-top: 2px; }
.legend li.reached { color: var(--viz-text); }
.n { display: inline-block; width: 1.4em; font-weight: 700; }
.cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 150px), 1fr)); gap: 8px; }
.card { border: 2px solid var(--viz-edge); border-left-width: 8px; border-radius: 8px; padding: 6px 10px; min-width: 0; }
.card.ok { border-left-color: var(--viz-response); }
.card.bad { border-left-color: var(--viz-error); }
.card.mid { border-left-color: var(--viz-active); }
.card h4 { margin: 0; font-size: 13px; font-weight: 600; color: var(--viz-muted); }
.card p { margin: 2px 0 0; font-size: 14px; font-weight: 700; overflow-wrap: anywhere; }
.card p.sub { font-weight: 400; font-size: 13px; }
@media (prefers-reduced-motion: reduce) { .elapsed { transition: none; } }
</style>
