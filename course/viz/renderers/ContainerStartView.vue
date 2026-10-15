<script setup lang="ts">
import { computed } from 'vue'
import { START_PHASES, type ContainerStartFrame, type ProcNode, type StartPhase } from '../simulations/container-start'

const props = defineProps<{ state: ContainerStartFrame }>()

const PHASE_LABEL: Record<StartPhase, string> = {
  run: 'run',
  clone: 'clone',
  child: 'child',
  hostname: 'hostname',
  'private-mounts': 'private',
  'pivot-root': 'pivot_root',
  'umount-oldroot': 'umount',
  'mount-proc': '/proc',
  exec: 'exec',
}
const current = computed(() => START_PHASES.indexOf(props.state.phase))
const rel = (i: number): 'done' | 'now' | 'todo' => (i < current.value ? 'done' : i === current.value ? 'now' : 'todo')
const procText = (p: ProcNode): string => (p.nsPid === undefined ? `PID ${p.pid}` : `PID ${p.pid} на хосте = PID ${p.nsPid} в контейнере`)
</script>

<template>
  <div class="cs">
    <ol class="phases" aria-label="Фазы запуска">
      <li v-for="(p, i) in START_PHASES" :key="p" :class="rel(i)" :aria-current="rel(i) === 'now' ? 'step' : undefined">
        <span class="mark" aria-hidden="true">{{ rel(i) === 'done' ? '✓' : rel(i) === 'now' ? '▶' : '·' }}</span>{{ PHASE_LABEL[p] }}
      </li>
    </ol>

    <p v-if="state.flags.length" class="flags">
      <span class="lbl">Флаги clone(2):</span>
      <code v-for="f in state.flags" :key="f" class="flag">{{ f }}</code>
    </p>

    <div class="trees">
      <section class="tree" aria-label="Процессы на хосте">
        <h4>Хост: ps</h4>
        <ul>
          <li v-for="p in state.host" :key="p.pid" :class="{ dual: p.nsPid !== undefined }">
            <strong>{{ procText(p) }}</strong><code>{{ p.cmd }}</code>
          </li>
        </ul>
      </section>
      <section class="tree inner" aria-label="Процессы в контейнере">
        <h4>Контейнер: ps</h4>
        <ul v-if="state.container">
          <li v-for="p in state.container" :key="p.pid" class="dual"><strong>PID {{ p.pid }}</strong><code>{{ p.cmd }}</code></li>
        </ul>
        <p v-else class="empty">Контейнера ещё нет</p>
      </section>
    </div>

    <dl class="env">
      <div><dt>Корень</dt><dd><code>{{ state.mounts.root }}</code></dd></div>
      <div><dt>/proc</dt><dd>{{ state.mounts.proc ? 'свой, по PID-namespace контейнера' : 'хостовый' }}</dd></div>
      <div><dt>/.oldroot</dt><dd>{{ state.mounts.oldroot ? 'виден: старый корень хоста ещё доступен' : 'нет' }}</dd></div>
      <div><dt>hostname на хосте</dt><dd><code>{{ state.hostname.host }}</code></dd></div>
      <div><dt>hostname в контейнере</dt><dd><code>{{ state.hostname.container ?? state.hostname.host }}</code><span v-if="state.hostname.container === null" class="same"> (общий с хостом)</span></dd></div>
    </dl>
  </div>
</template>

<style scoped>
.cs { padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
.phases { display: flex; flex-wrap: wrap; gap: 6px; margin: 0 0 12px; padding: 0; list-style: none; font-size: 13px; }
.phases li { padding: 3px 8px; border: 2px solid var(--viz-edge); border-radius: 999px; }
.phases li.done { border-color: var(--viz-response); }
.phases li.now { border-color: var(--viz-active); font-weight: 700; background: var(--viz-active-line-bg); }
.phases li.todo { color: var(--viz-muted); }
.mark { display: inline-block; width: 1.1em; }
.flags { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; margin: 0 0 12px; font-size: 13px; }
.lbl { color: var(--viz-muted); }
.flag { padding: 2px 6px; border: 2px solid var(--viz-active); border-radius: 6px; background: var(--viz-active-line-bg); color: var(--viz-text); font-size: 12px; }
.trees { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 250px), 1fr)); gap: 12px; margin-bottom: 12px; }
.tree { border: 2px solid var(--viz-edge); border-radius: 8px; padding: 8px 12px; min-width: 0; }
.tree.inner { border-color: var(--viz-service); }
.tree h4 { margin: 0 0 6px; font-size: 14px; }
.tree ul { margin: 0; padding: 0; list-style: none; }
.tree li { display: flex; flex-direction: column; margin-top: 4px; padding-left: 8px; border-left: 4px solid var(--viz-edge); font-size: 13px; }
.tree li.dual { border-left-color: var(--viz-service); }
.tree li code { background: none; padding: 0; color: var(--viz-text); overflow-wrap: anywhere; }
.empty { margin: 0; font-size: 14px; color: var(--viz-muted); }
.env { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 220px), 1fr)); gap: 8px 12px; margin: 0; font-size: 13px; }
.env dt { color: var(--viz-muted); }
.env dd { margin: 0; overflow-wrap: anywhere; }
.env code { background: none; padding: 0; color: var(--viz-text); }
.same { color: var(--viz-muted); }
</style>
