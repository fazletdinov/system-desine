<script setup lang="ts">
import { computed } from 'vue'
import { GOMEMLIMIT_MIB, type MemFrame } from '../simulations/gomemlimit'

const props = defineProps<{ frames: readonly MemFrame[]; index: number; withLimit: boolean }>()

// Одинаковый масштаб в обоих вариантах: время 0-14 с, память 0-320 МиБ.
const W = 360
const H = 230
const L = 34
const R = 8
const T = 10
const B = 26
const T_MAX = 14
const Y_MAX = 320
const x = (t: number): number => L + (t / T_MAX) * (W - L - R)
const y = (mib: number): number => H - B - (mib / Y_MAX) * (H - T - B)

const reached = computed(() => props.frames.slice(0, props.index + 1))
const now = computed(() => props.frames[props.index] as MemFrame)
type Key = 'heapLiveMiB' | 'heapGoalMiB' | 'rssMiB'
const line = (key: Key): string => reached.value.map((f) => `${x(f.t)},${y(f[key])}`).join(' ')
const SERIES: readonly { key: Key; label: string; cls: string }[] = [
  { key: 'rssMiB', label: 'RSS процесса', cls: 'rss' },
  { key: 'heapGoalMiB', label: 'цель GC', cls: 'goal' },
  { key: 'heapLiveMiB', label: 'живая куча', cls: 'live' },
]
const X_TICKS = [0, 2, 4, 6, 8, 10, 12, 14]
const Y_TICKS = [0, 64, 128, 192, 256, 320]
</script>

<template>
  <div class="gm">
    <svg :viewBox="`0 0 ${W} ${H}`" role="img" :aria-label="`График памяти, ${now.t} с: живая куча ${now.heapLiveMiB}, цель GC ${now.heapGoalMiB}, RSS ${now.rssMiB} МиБ при лимите ${now.limitMiB}${now.killed ? '; процесс убит' : ''}`">
      <g class="grid">
        <line v-for="v in Y_TICKS" :key="v" :x1="L" :x2="W - R" :y1="y(v)" :y2="y(v)" />
      </g>
      <g class="axis">
        <text v-for="v in Y_TICKS" :key="`y${v}`" :x="L - 4" :y="y(v) + 4" text-anchor="end">{{ v }}</text>
        <text v-for="t in X_TICKS" :key="`x${t}`" :x="x(t)" :y="H - B + 14" text-anchor="middle">{{ t }}</text>
        <text :x="W - R" :y="H - 2" text-anchor="end">секунды</text>
        <text :x="L" :y="T - 1" text-anchor="start">МиБ</text>
      </g>
      <line class="limit" :x1="L" :x2="W - R" :y1="y(now.limitMiB)" :y2="y(now.limitMiB)" />
      <text class="limit-label" :x="L + 4" :y="y(now.limitMiB) - 4">лимит контейнера {{ now.limitMiB }}</text>
      <template v-if="withLimit">
        <line class="softlimit" :x1="L" :x2="W - R" :y1="y(GOMEMLIMIT_MIB)" :y2="y(GOMEMLIMIT_MIB)" />
        <text class="soft-label" :x="W - R - 2" :y="y(GOMEMLIMIT_MIB) + 12" text-anchor="end">GOMEMLIMIT {{ GOMEMLIMIT_MIB }}</text>
      </template>
      <polyline v-for="s in SERIES" :key="s.key" :class="['series', s.cls]" :points="line(s.key)" />
      <circle class="dot rss" :cx="x(now.t)" :cy="y(now.rssMiB)" r="4" />
      <text v-if="now.killed" class="kill" :x="W - R" :y="y(185)" text-anchor="end">✕ OOM-kill: требовалось ~{{ now.demandMiB }}</text>
    </svg>
    <ul class="legend" aria-label="Обозначения">
      <li v-for="s in SERIES" :key="s.key"><svg width="30" height="8" aria-hidden="true"><line x1="0" x2="30" y1="4" y2="4" :class="['series', s.cls]" /></svg>{{ s.label }}: <strong>{{ now[s.key] }}</strong></li>
      <li>запусков GC: <strong>{{ now.gcRuns }}</strong></li>
    </ul>
    <p class="status" :class="now.killed ? 'bad' : 'ok'">{{ now.killed ? `✕ Процесс убит: упёрся в memory.max ${now.limitMiB} МиБ (требовалось ~${now.demandMiB})` : '✓ Процесс жив' }}</p>
  </div>
</template>

<style scoped>
.gm { padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
svg { display: block; width: 100%; max-width: 640px; height: auto; margin: 0 auto; }
.grid line { stroke: var(--viz-edge); stroke-width: 1; }
.axis text { fill: var(--viz-muted); font-size: 10px; }
.series { fill: none; stroke-width: 2.5; stroke-linejoin: round; }
.series.live { stroke: var(--viz-db); }
.series.goal { stroke: var(--viz-service); stroke-dasharray: 6 3; }
.series.rss { stroke: var(--viz-client); }
.dot.rss { fill: var(--viz-client); }
.limit { stroke: var(--viz-error); stroke-width: 2; stroke-dasharray: 2 3; }
.limit-label { fill: var(--viz-text); font-size: 10px; font-weight: 700; }
.softlimit { stroke: var(--viz-cache); stroke-width: 2; stroke-dasharray: 8 2 2 2; }
.soft-label { fill: var(--viz-text); font-size: 10px; font-weight: 700; }
.kill { fill: var(--viz-text); font-size: 12px; font-weight: 700; }
.legend { display: flex; flex-wrap: wrap; gap: 4px 16px; margin: 8px 0 0; padding: 0; list-style: none; font-size: 13px; }
.legend li { display: flex; align-items: center; gap: 6px; }
.legend svg { width: 30px; flex: none; margin: 0; }
.legend .series { stroke-width: 3; }
.status { margin: 8px 0 0; padding: 4px 10px; border: 2px solid; border-radius: 8px; font-size: 14px; font-weight: 700; }
.status.ok { border-color: var(--viz-response); }
.status.bad { border-color: var(--viz-error); }
</style>
