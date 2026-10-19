<script setup lang="ts">
import { computed } from 'vue'
import { LOCAL_QUEUE_CAP, type GmpFrame, type GmpNote, type MState } from '../simulations/gmp-scheduler'

const props = defineProps<{ state: GmpFrame }>()

const M_TEXT: Record<MState, string> = {
  running: '● выполняет Go-код',
  spinning: '◐ ищет работу (spinning)',
  syscall: '■ заблокирован в системном вызове',
  idle: '○ спит, без P',
}
const NOTE_TEXT: Record<GmpNote, string> = {
  spawn: 'создание горутины',
  steal: 'кража работы',
  'syscall-handoff': 'передача P из-за системного вызова',
  preempt: 'вытеснение по сигналу',
  'syscall-return': 'возврат из системного вызова',
  idle: 'исходное состояние',
}
const mState = (id: string | null): MState | null => props.state.ms.find((m) => m.id === id)?.state ?? null
const columns = computed(() =>
  props.state.ps.map((p) => ({
    ...p,
    mText: p.m === null ? 'нет потока M: P свободен' : `${p.m}: ${M_TEXT[mState(p.m) ?? 'idle']}`,
    blocked: mState(p.m) === 'syscall',
  })),
)
</script>

<template>
  <div class="gmp">
    <p class="note">Событие: <strong>{{ NOTE_TEXT[state.note] }}</strong></p>
    <section class="global" aria-label="Глобальная очередь">
      <h4>Глобальная очередь</h4>
      <ul class="row"><li v-for="g in state.global" :key="g" class="g">{{ g }}</li><li v-if="state.global.length === 0" class="empty">пусто</li></ul>
    </section>
    <div class="ps">
      <section v-for="p in columns" :key="p.id" class="p" :aria-label="`Процессор ${p.id}`">
        <h4>{{ p.id }}</h4>
        <p class="m" :class="{ bad: p.blocked }">{{ p.mText }}</p>
        <div class="slot"><span class="lbl">выполняется</span><span v-if="p.running" class="g run">{{ p.running }}</span><span v-else class="empty">никто</span></div>
        <div class="slot"><span class="lbl">runnext</span><span v-if="p.runnext" class="g next">{{ p.runnext }}</span><span v-else class="empty">пусто</span></div>
        <div class="slot"><span class="lbl">очередь (до {{ LOCAL_QUEUE_CAP }})</span>
          <ul class="row"><li v-for="g in p.local" :key="g" class="g">{{ g }}</li><li v-if="p.local.length === 0" class="empty">пусто</li></ul>
        </div>
      </section>
    </div>
    <section aria-label="Потоки ОС">
      <h4>Потоки ОС (M)</h4>
      <ul class="ms"><li v-for="m in state.ms" :key="m.id" :class="m.state"><b>{{ m.id }}</b> {{ M_TEXT[m.state] }}</li></ul>
    </section>
  </div>
</template>

<style scoped>
.gmp { padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
h4 { margin: 0 0 4px; font-size: 13px; font-weight: 600; color: var(--viz-muted); }
.note { margin: 0 0 8px; font-size: 14px; }
.global { margin-bottom: 10px; padding: 6px 10px; border: 2px dashed var(--viz-edge); border-radius: 8px; }
.ps { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 200px), 1fr)); gap: 10px; margin-bottom: 10px; }
.p { min-width: 0; padding: 6px 10px; border: 2px solid var(--viz-edge); border-top: 6px solid var(--viz-service); border-radius: 8px; }
.p h4 { font-size: 15px; color: var(--viz-text); }
.m { margin: 0 0 6px; font-size: 13px; color: var(--viz-muted); overflow-wrap: anywhere; }
.m.bad { color: var(--viz-error); font-weight: 700; }
.slot { margin-top: 6px; }
.lbl { display: block; font-size: 12px; color: var(--viz-muted); }
.row { display: flex; flex-wrap: wrap; gap: 6px; margin: 2px 0 0; padding: 0; list-style: none; }
.g { display: inline-block; min-width: 2.2em; padding: 2px 8px; border: 2px solid var(--viz-edge); border-radius: 6px; font-weight: 700; font-size: 14px; text-align: center; }
.g.run { border-color: var(--viz-response); }
.g.next { border-color: var(--viz-active); background: var(--viz-active-line-bg); }
.empty { font-size: 13px; color: var(--viz-muted); }
.ms { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 200px), 1fr)); gap: 6px; margin: 0; padding: 0; list-style: none; font-size: 13px; }
.ms li { padding: 4px 8px; border: 2px solid var(--viz-edge); border-left-width: 8px; border-radius: 6px; }
.ms .running { border-left-color: var(--viz-response); }
.ms .spinning { border-left-color: var(--viz-active); }
.ms .syscall { border-left-color: var(--viz-error); }
.ms .idle { border-left-color: var(--viz-muted); }
</style>
