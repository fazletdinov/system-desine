<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import LanesView, { type Lane } from '../renderers/LanesView.vue'
import { lanesTimeline, type LanesInput, type Proto } from '../simulations/lanes'

const STREAMS = 3
const PACKETS_PER_STREAM = 4
const TITLES: Record<Proto, string> = { h1: 'HTTP/1.1', h2: 'HTTP/2 поверх TCP', quic: 'HTTP/3 поверх QUIC' }
const ALL: readonly Proto[] = ['h1', 'h2', 'quic']

const props = withDefaults(defineProps<{ protocols?: string }>(), { protocols: 'h1,h2' })
const shown = computed<Proto[]>(() => {
  const wanted = props.protocols.split(',').map((s) => s.trim())
  const list = ALL.filter((p) => wanted.includes(p))
  return list.length > 0 ? list : ['h1', 'h2']
})

interface Loss {
  stream: number
  index: number
}
// По умолчанию теряем последний пакет потока 1: эффект виден сразу (ранняя потеря при малом RTT «рассасывается»).
const loss = ref<Loss | null>({ stream: 0, index: PACKETS_PER_STREAM - 1 })
const rttSlots = ref(4)
const h1Connections = ref(1)

const base = (proto: Proto): LanesInput => ({
  proto,
  streams: STREAMS,
  packetsPerStream: PACKETS_PER_STREAM,
  lostPacket: null,
  h1Connections: h1Connections.value,
  rttSlots: rttSlots.value,
})

// Одна и та же потеря (поток, номер внутри потока) в разных протоколах — это разные id пакетов.
function lostId(input: LanesInput): number | null {
  if (loss.value === null) return null
  const { stream, index } = loss.value
  return lanesTimeline(input).packets.find((p) => p.stream === stream && p.index === index)?.id ?? null
}

const lanes = computed<Lane[]>(() =>
  shown.value.map((proto) => {
    const clean = base(proto)
    return {
      proto,
      title: proto === 'h1' ? `${TITLES.h1}, соединений: ${h1Connections.value}` : TITLES[proto],
      result: lanesTimeline({ ...clean, lostPacket: lostId(clean) }),
      baseline: lanesTimeline(clean).streamDone,
      connections: proto === 'h1' ? h1Connections.value : 1,
    }
  }),
)

function pick(stream: number, index: number): void {
  const same = loss.value?.stream === stream && loss.value.index === index
  loss.value = same ? null : { stream, index }
}
const isLost = (stream: number, index: number): boolean => loss.value?.stream === stream && loss.value.index === index
const indices = Array.from({ length: PACKETS_PER_STREAM }, (_, i) => i)
const streamsList = Array.from({ length: STREAMS }, (_, i) => i)
</script>

<template>
  <VizFrame label="Песочница: HTTP/1.1, HTTP/2 и QUIC при потере пакета">
    <p class="intro">
      Учебная модель: {{ STREAMS }} потока по {{ PACKETS_PER_STREAM }} пакета, пакет в слот, задержка в одну сторону — половина RTT. Потерянный пакет обнаруживается через RTT. Управления перегрузкой нет. HTTP/1.1 с k соединениями шлёт k пакетов за слот (полосы больше), а один и тот же пакет в разных протоколах уходит в разные слоты — поэтому сравнивайте каждый протокол с его собственным запуском без потери (число «+N» у потока).
      Нажмите на квадрат или кнопку, чтобы потерять пакет; повторное нажатие снимает потерю. Потеря одного из первых пакетов при малом RTT может «рассосаться» к концу — потеряйте пакет из последних, и эффект станет заметнее.
    </p>
    <div class="panel" role="group" aria-label="Настройки песочницы">
      <div v-for="s in streamsList" :key="s" class="row" role="group" :aria-label="`Потерять пакет потока ${s + 1}`">
        <span class="row-label" aria-hidden="true">Потерять, поток {{ s + 1 }}</span>
        <button v-for="i in indices" :key="i" type="button" class="toggle" :aria-pressed="isLost(s, i)" :aria-label="`Потерять пакет ${i + 1} потока ${s + 1}`" @click="pick(s, i)">{{ isLost(s, i) ? '✕ ' : '' }}{{ i + 1 }}</button>
      </div>
      <div class="row">
        <label class="row-label" for="lanes-rtt">RTT: {{ rttSlots }} слот.</label>
        <input id="lanes-rtt" v-model.number="rttSlots" type="range" min="2" max="8" step="1" />
      </div>
      <div v-if="shown.includes('h1')" class="row">
        <label class="row-label" for="lanes-h1">Соединений HTTP/1.1: {{ h1Connections }}</label>
        <input id="lanes-h1" v-model.number="h1Connections" type="range" min="1" :max="STREAMS" step="1" />
      </div>
    </div>
    <LanesView :lanes="lanes" :streams="STREAMS" @pick="pick" />
  </VizFrame>
</template>

<style scoped>
.intro { margin: 0 0 10px; font-size: 14px; color: var(--viz-muted); }
.panel { display: flex; flex-direction: column; gap: 6px; margin-bottom: 8px; }
.row { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; }
.row-label { font-size: 14px; color: var(--viz-text); margin-right: 4px; min-width: 150px; }
.toggle { min-width: 36px; min-height: 36px; padding: 0 8px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; cursor: pointer; }
.toggle[aria-pressed='true'] { border-color: var(--viz-error); background: color-mix(in srgb, var(--viz-error) 20%, var(--vp-c-bg)); font-weight: 700; }
.toggle:focus-visible, input[type='range']:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
input[type='range'] { flex: 1 1 140px; max-width: 260px; min-height: 36px; }
</style>
