<script setup lang="ts">
import { computed } from 'vue'
import { cpuShare, type CgroupFrame } from '../simulations/cgroup-limits'

const props = defineProps<{ state: CgroupFrame }>()

const ms = (us: number): number => us / 1000
const memPct = computed(() => (props.state.kind === 'memory' ? (props.state.usageMiB / props.state.maxMiB) * 100 : 0))
const share = computed(() => (props.state.kind === 'cpu' ? Math.round(cpuShare(props.state) * 100) : 0))
</script>

<template>
  <div class="cg">
    <template v-if="state.kind === 'memory'">
      <p class="head">
        <code>memory.current</code> <strong>{{ state.usageMiB }} МиБ</strong> из <code>memory.max</code> <strong>{{ state.maxMiB }} МиБ</strong>
      </p>
      <div class="track" role="img" :aria-label="`Занято ${state.usageMiB} из ${state.maxMiB} МиБ`">
        <div class="fill" :class="{ hot: state.usageMiB >= 60 }" :style="{ width: `${memPct}%` }"></div>
      </div>
      <p class="badges">
        <span v-if="state.oomKilled" class="badge bad">✕ Процесс убит, код выхода 137</span>
        <span v-else-if="state.events.oom > 0" class="badge bad">⚠ Лимит достигнут</span>
        <span v-else-if="state.reclaimed" class="badge warn">⚠ Ядро пытается освободить кэш страниц</span>
        <span v-else class="badge ok">✓ Ниже лимита</span>
      </p>
      <dl class="events" aria-label="memory.events">
        <div><dt>oom</dt><dd>{{ state.events.oom }}</dd></div>
        <div><dt>oom_kill</dt><dd>{{ state.events.oom_kill }}</dd></div>
      </dl>
    </template>
    <template v-else>
      <p class="head"><code>cpu.max</code> = <strong>{{ state.quotaUs }} {{ state.periodUs }}</strong>: {{ ms(state.quotaUs) }} мс на каждые {{ ms(state.periodUs) }} мс</p>
      <ol class="periods" aria-label="Периоды планировщика">
        <li v-for="(p, i) in state.periods" :key="i">
          <span class="no">Период {{ i + 1 }}</span>
          <div class="bar" role="img" :aria-label="`Работал ${ms(p.used)} мс, ожидал ${ms(state.periodUs - p.used)} мс`">
            <div class="used" :style="{ width: `${(p.used / state.periodUs) * 100}%` }">работает {{ ms(p.used) }} мс</div>
            <div class="wait">{{ p.throttled ? 'ждёт' : '' }} {{ ms(state.periodUs - p.used) }} мс</div>
          </div>
        </li>
      </ol>
      <p class="badges"><span class="badge warn">⏸ Throttled: {{ state.periods.filter((p) => p.throttled).length }} из {{ state.periods.length }} периодов</span></p>
      <pre v-if="state.stat" class="stat" aria-label="cpu.stat">nr_periods {{ state.stat.nr_periods }}
nr_throttled {{ state.stat.nr_throttled }}
throttled_usec {{ state.stat.throttled_usec }}</pre>
      <p v-if="state.stat" class="share">Итоговая доля CPU: <strong>{{ share }} %</strong> при спросе 100 %</p>
    </template>
  </div>
</template>

<style scoped>
.cg { padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
.head { margin: 0 0 8px; font-size: 14px; overflow-wrap: anywhere; }
.head code, .events dt { background: none; padding: 0; color: var(--viz-text); }
.track { height: 24px; border: 2px solid var(--viz-edge); border-radius: 8px; overflow: hidden; }
.fill { height: 100%; background: var(--viz-service); transition: width 0.3s; }
.fill.hot { background: var(--viz-error); }
.badges { margin: 10px 0 0; }
.badge { display: inline-block; padding: 3px 10px; border: 2px solid; border-radius: 8px; font-size: 14px; font-weight: 700; background: var(--viz-node-bg); color: var(--viz-text); }
.badge.ok { border-color: var(--viz-response); }
.badge.warn { border-color: var(--viz-active); }
.badge.bad { border-color: var(--viz-error); }
.events { display: flex; gap: 16px; margin: 10px 0 0; font-size: 14px; }
.events > div { display: flex; gap: 6px; }
.events dd { margin: 0; font-weight: 700; font-variant-numeric: tabular-nums; }
.periods { margin: 0; padding: 0; list-style: none; }
.periods li { margin-top: 6px; }
.no { font-size: 13px; color: var(--viz-muted); }
.bar { display: flex; height: 28px; border: 2px solid var(--viz-edge); border-radius: 6px; overflow: hidden; font-size: 12px; line-height: 24px; }
.used { background: color-mix(in srgb, var(--viz-response) 35%, var(--viz-node-bg)); padding: 0 6px; white-space: nowrap; overflow: hidden; border-right: 2px solid var(--viz-response); }
.wait { flex: 1; padding: 0 6px; white-space: nowrap; overflow: hidden; background: repeating-linear-gradient(135deg, transparent 0 6px, color-mix(in srgb, var(--viz-active) 30%, transparent) 6px 8px); }
.stat { margin: 10px 0 0; padding: 8px 12px; border: 2px solid var(--viz-edge); border-radius: 8px; background: var(--viz-node-bg); color: var(--viz-text); font-size: 13px; }
.share { margin: 8px 0 0; font-size: 14px; }
@media (prefers-reduced-motion: reduce) { .fill { transition: none; } }
</style>
