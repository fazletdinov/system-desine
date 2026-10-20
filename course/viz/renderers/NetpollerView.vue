<script setup lang="ts">
import { computed } from 'vue'
import { gStates, type GState, type NetpollerFrame } from '../simulations/netpoller'

const props = defineProps<{ state: NetpollerFrame }>()

const G_TEXT: Record<GState, string> = {
  running: '● выполняется на M',
  runnable: '◐ готова, в очереди P',
  waiting: '◌ запаркована на pollDesc',
  done: '✓ завершилась',
}
const M_TEXT = {
  go: 'выполняет Go-код',
  syscall: 'в системном вызове',
  findRunnable: 'findRunnable: ищет работу',
} as const

const fmt = (ms: number): string => String(Math.round(ms * 100) / 100).replace('.', ',')
const gs = computed(() => props.state.gs.map((g) => ({ id: g, state: gStates(props.state, g)[0] ?? 'done' })))
const mLine = computed(() => (props.state.m.g === null ? M_TEXT[props.state.m.where] : `${props.state.m.g} — ${M_TEXT[props.state.m.where]}`))
const result = computed(() => {
  const r = props.state.result
  if (r === null) return null
  return r.ok ? { ok: true, text: `n = ${r.n}, err = nil` } : { ok: false, text: `n = 0, err: i/o timeout (${r.err})` }
})
</script>

<template>
  <div class="np">
    <p class="clock">Время: <strong>t = {{ fmt(state.t) }} мс</strong></p>
    <p class="sys"><span class="lbl">последний системный вызов</span><code>{{ state.syscall ?? '—' }}</code></p>
    <div class="grid">
      <section class="card sched" aria-label="Поток и процессор">
        <h4>Поток M с процессором P</h4>
        <p class="m" :class="{ idle: state.m.g === null }">{{ mLine }}</p>
        <span class="lbl">локальная очередь P</span>
        <ul class="row"><li v-for="g in state.runq" :key="g" class="g">{{ g }}</li><li v-if="state.runq.length === 0" class="empty">пусто</li></ul>
      </section>
      <section class="card" aria-label="Горутины">
        <h4>Горутины</h4>
        <ul class="gs">
          <li v-for="g in gs" :key="g.id" :class="g.state"><b>{{ g.id }}</b> {{ G_TEXT[g.state] }}</li>
        </ul>
      </section>
      <section class="card poll" aria-label="pollDesc сокета">
        <h4>pollDesc сокета (рантайм)</h4>
        <p>rg: <b>{{ state.pollDesc.rg ?? 'никого' }}</b></p>
        <p v-if="state.timer" :class="{ fired: state.timer.fired }">таймер дедлайна: {{ fmt(state.timer.atMs) }} мс — <b>{{ state.timer.fired ? 'сработал' : 'ждёт' }}</b></p>
      </section>
      <section class="card kernel" aria-label="Ядро: сокет и epoll">
        <h4>Ядро: сокет и epoll</h4>
        <p>буфер сокета: <b>{{ state.socketBytes }} байт</b></p>
        <p :class="{ ready: state.epoll.ready }">список готовых epoll: <b>{{ state.epoll.ready ? 'сокет готов к чтению' : 'пусто' }}</b></p>
      </section>
    </div>
    <p v-if="result" class="result" :class="result.ok ? 'ok' : 'bad'" role="status">conn.Read вернул: {{ result.text }}</p>
  </div>
</template>

<style scoped>
.np { padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
h4 { margin: 0 0 4px; font-size: 13px; font-weight: 600; color: var(--viz-muted); }
.clock { margin: 0 0 6px; font-size: 14px; }
.sys { margin: 0 0 10px; font-size: 14px; }
.sys code { display: block; margin-top: 2px; padding: 4px 8px; border: 2px solid var(--viz-edge); border-radius: 6px; font-size: 13px; overflow-wrap: anywhere; }
.lbl { display: block; font-size: 12px; color: var(--viz-muted); }
.grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 200px), 1fr)); gap: 10px; }
.card { min-width: 0; padding: 6px 10px; border: 2px solid var(--viz-edge); border-top-width: 6px; border-radius: 8px; font-size: 14px; }
.card p { margin: 2px 0; overflow-wrap: anywhere; }
.sched { border-top-color: var(--viz-service); }
.poll { border-top-color: var(--viz-client); }
.kernel { border-top-color: var(--viz-network); }
.m { font-weight: 700; }
.m.idle { color: var(--viz-muted); }
.row { display: flex; flex-wrap: wrap; gap: 6px; margin: 2px 0 0; padding: 0; list-style: none; }
.g { display: inline-block; min-width: 2.2em; padding: 2px 8px; border: 2px solid var(--viz-edge); border-radius: 6px; font-weight: 700; text-align: center; }
.empty { font-size: 13px; color: var(--viz-muted); }
.gs { display: grid; gap: 6px; margin: 0; padding: 0; list-style: none; }
.gs li { padding: 4px 8px; border: 2px solid var(--viz-edge); border-left-width: 8px; border-radius: 6px; }
.gs .running { border-left-color: var(--viz-response); }
.gs .runnable { border-left-color: var(--viz-active); }
.gs .waiting { border-left-color: var(--viz-client); }
.gs .done { border-left-color: var(--viz-muted); color: var(--viz-muted); }
.ready b { color: var(--viz-response); }
.fired b { color: var(--viz-error); }
.result { margin: 10px 0 0; padding: 6px 10px; border: 2px solid var(--viz-edge); border-left-width: 8px; border-radius: 8px; font-weight: 700; }
.result.ok { border-left-color: var(--viz-response); }
.result.bad { border-left-color: var(--viz-error); }
</style>
