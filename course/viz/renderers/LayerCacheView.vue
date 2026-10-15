<script setup lang="ts">
import { computed } from 'vue'
import type { FileId, Layer, LayerState } from '../simulations/layer-cache'

const props = defineProps<{ state: LayerState }>()

const FILES: { id: FileId; name: string }[] = [
  { id: 'gomod', name: 'go.mod' },
  { id: 'source', name: 'main.go' },
  { id: 'readme', name: 'README.md' },
]

const seconds = computed(() => Math.round(props.state.lastBuild.reduce((sum, l) => sum + l.seconds, 0)))
const buildStage = computed(() => props.state.lastBuild.filter((l) => l.stage === 'build'))
const runtimeStage = computed(() => props.state.lastBuild.filter((l) => l.stage === 'runtime'))
const built = computed(() => props.state.lastBuild.length > 0)
// «Изменён с прошлой сборки»: у файла есть правка, которой не было при последней сборке.
const lastBuiltVersions = computed(() => props.state.builtVersions)
const dirty = (id: FileId): boolean => props.state.versions[id] !== lastBuiltVersions.value[id]

const fmt = (l: Layer): string => (l.seconds === 0 ? '0 с' : `${l.seconds < 1 ? l.seconds.toFixed(1) : Math.round(l.seconds)} с`)
</script>

<template>
  <div class="lc">
    <div class="totals" aria-label="Итоги сборки">
      <p class="big">время сборки <b>{{ built ? `~${seconds} с` : '—' }}</b></p>
      <p class="big">размер образа <b>{{ built ? `${state.imageMB} МБ` : '—' }}</b></p>
    </div>
    <div class="cols">
      <section class="col" aria-label="Стадия сборки">
        <h4>{{ state.multiStage ? 'стадия сборки (build)' : 'единственная стадия' }}</h4>
        <ol class="layers">
          <li v-for="l in buildStage" :key="l.id" :class="l.status">
            <code>{{ l.instruction }}</code>
            <span class="badge" :class="l.status">{{ l.status === 'cached' ? 'CACHED' : 'BUILT' }}</span>
            <span class="sec">{{ fmt(l) }}</span>
          </li>
          <li v-if="!built" class="meta">Образ ещё не собирали.</li>
        </ol>
      </section>
      <section class="col" aria-label="Итоговый образ">
        <h4>итоговый образ</h4>
        <ol v-if="runtimeStage.length > 0" class="layers">
          <li v-for="l in runtimeStage" :key="l.id" :class="l.status">
            <code>{{ l.instruction }}</code>
            <span class="badge" :class="l.status">{{ l.status === 'cached' ? 'CACHED' : 'BUILT' }}</span>
            <span class="sec">{{ fmt(l) }}</span>
          </li>
        </ol>
        <p v-else-if="built" class="meta">Без multi-stage в образ попадает вся стадия сборки: компилятор, исходники, кэши.</p>
        <p v-else class="meta">Образ ещё не собирали.</p>
      </section>
    </div>
    <ul class="files" aria-label="Файлы проекта">
      <li v-for="f in FILES" :key="f.id" :class="{ dirty: dirty(f.id) }">
        <code>{{ f.name }}</code>
        <span class="rev">правок: {{ state.versions[f.id] }}</span>
        <span v-if="dirty(f.id)" class="chg">изменён с прошлой сборки</span>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.lc { container-type: inline-size; color: var(--viz-text); }
.totals { display: flex; flex-wrap: wrap; gap: 8px 24px; margin-bottom: 10px; }
.big { margin: 0; font-size: 14px; color: var(--viz-muted); }
.big b { margin-left: 4px; font-size: 26px; font-variant-numeric: tabular-nums; color: var(--viz-text); }
.cols { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; margin-bottom: 10px; }
.col { padding: 8px 10px; border: 2px solid var(--viz-edge); border-radius: 10px; background: var(--viz-node-bg); min-width: 0; }
h4 { margin: 0 0 6px; font-size: 15px; }
.meta { margin: 2px 0; font-size: 13px; color: var(--viz-muted); overflow-wrap: anywhere; }
.layers { margin: 0; padding: 0; list-style: none; display: flex; flex-direction: column; gap: 4px; }
.layers li { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; align-items: center; gap: 6px; padding: 4px 6px; border-left: 4px solid var(--viz-edge); border-radius: 4px; transition: border-color 0.2s, background-color 0.2s; }
.layers li.cached { border-left-color: var(--viz-response); }
.layers li.built { border-left-color: var(--viz-cache); background: var(--viz-active-line-bg); }
.layers li.meta { display: block; border-left-color: transparent; }
code { font-size: 12px; overflow-wrap: anywhere; }
.badge { padding: 1px 6px; border: 2px solid var(--viz-edge); border-radius: 6px; font-size: 11px; font-weight: 700; letter-spacing: 0.04em; }
.badge.cached { border-color: var(--viz-response); }
.badge.built { border-color: var(--viz-cache); }
.sec { min-width: 36px; text-align: right; font-size: 12px; font-variant-numeric: tabular-nums; color: var(--viz-muted); }
.files { margin: 0; padding: 0; list-style: none; display: flex; flex-wrap: wrap; gap: 8px; }
.files li { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 8px; padding: 4px 8px; border: 2px solid var(--viz-edge); border-radius: 8px; background: var(--viz-node-bg); font-size: 13px; }
.files li.dirty { border-color: var(--viz-active); border-style: dashed; }
.rev { color: var(--viz-muted); }
.chg { font-weight: 700; }
@container (max-width: 520px) { .cols { grid-template-columns: 1fr; } }
@media (prefers-reduced-motion: reduce) { .layers li { transition: none; } }
</style>
