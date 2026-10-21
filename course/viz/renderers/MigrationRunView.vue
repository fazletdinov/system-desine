<script setup lang="ts">
import { computed } from 'vue'
import { VERSION_TABLE, schemaVersion, type FileStatus, type MigFrame, type ProcState, type StmtStatus } from '../simulations/migration-run'

const props = defineProps<{ state: MigFrame }>()

// Цвет со смыслом: --viz-service — процесс работает, --viz-response — успех (код 0, сервер принимает запросы,
// COMMIT), --viz-error — сбой (код 1, откат), --viz-active — ожидание и незафиксированное; рамка базы — --viz-db.
const PROC_TEXT: Record<ProcState, string> = {
  'not-started': '— не запускался',
  running: '● работает',
  'exit-0': '✓ вышел с кодом 0',
  'exit-1': '✕ вышел с кодом 1',
  serving: '✓ принимает запросы',
  blocked: '… ждёт migrate (не запущен)',
}
const procClass = (s: ProcState): string => (s === 'exit-0' || s === 'serving' ? 'ok' : s === 'exit-1' ? 'bad' : s === 'running' ? 'run' : 'idle')
const FILE_TEXT: Record<FileStatus, string> = { applied: 'применена', pending: 'не применена', running: 'применяется', failed: 'сбой, откат' }
const STMT_MARK: Record<StmtStatus, string> = { pending: '·', done: '✓', waiting: '◐', failed: '✕' }
const STMT_SR: Record<StmtStatus, string> = { pending: 'ещё не выполнен', done: 'выполнен', waiting: 'ждёт блокировку', failed: 'ошибка' }
const TX_TEXT = { open: 'открыта', committed: 'COMMIT — зафиксирована', 'rolled-back': 'ROLLBACK — откачена целиком' } as const

const version = computed(() => schemaVersion(props.state))
const versionOk = computed(() => version.value !== null && version.value >= props.state.required)
const pad = (v: number): string => String(v).padStart(5, '0')
</script>

<template>
  <div class="mr">
    <div class="procs">
      <section :class="['proc', procClass(state.migrate)]" aria-label="Процесс migrate">
        <h4>messenger migrate up</h4>
        <p>{{ PROC_TEXT[state.migrate] }}</p>
      </section>
      <section :class="['proc', procClass(state.server)]" aria-label="Сервер messenger">
        <h4>сервер messenger</h4>
        <p>{{ PROC_TEXT[state.server] }}</p>
        <p class="sub">нужна версия схемы ≥ {{ state.required }}</p>
      </section>
    </div>

    <p class="log" role="status"><span class="sr-only">Журнал: </span>{{ state.log }}</p>
    <p v-if="state.lock" class="lock">Блокировка: {{ state.lock }}</p>

    <ul class="files" aria-label="Файлы миграций в бинаре">
      <li v-for="f in state.files" :key="f.version" :class="f.status">
        <code>{{ f.name }}</code> <span class="fs">{{ FILE_TEXT[f.status] }}</span>
      </li>
    </ul>

    <section v-if="state.tx" :class="['tx', state.tx.state]" aria-label="Транзакция миграции">
      <h4>Транзакция миграции {{ pad(state.tx.version) }}: {{ TX_TEXT[state.tx.state] }}</h4>
      <ol>
        <li v-for="(s, i) in state.tx.stmts" :key="i" :class="[s.status, { undone: state.tx.state === 'rolled-back' && s.status === 'done' }]">
          <span class="mk" aria-hidden="true">{{ STMT_MARK[s.status] }}</span>
          <code>{{ s.sql }}</code>
          <span class="sr-only"> — {{ STMT_SR[s.status] }}<template v-if="state.tx.state === 'rolled-back' && s.status === 'done'">, отменён откатом</template></span>
        </li>
      </ol>
    </section>

    <section class="db" aria-label="База PostgreSQL">
      <h4>PostgreSQL</h4>
      <p class="ver" :class="{ ok: versionOk, bad: version !== null && !versionOk }">
        Версия схемы: <strong>{{ version === null ? 'нет' : version }}</strong>
        <template v-if="version !== null"> · {{ versionOk ? `≥ ${state.required}, сервер может стартовать` : `< ${state.required}, сервер не стартует` }}</template>
      </p>
      <div class="cols">
        <figure class="vt">
          <figcaption><code>{{ VERSION_TABLE }}</code></figcaption>
          <table v-if="state.versions">
            <thead><tr><th scope="col">version_id</th><th scope="col">is_applied</th></tr></thead>
            <tbody>
              <tr v-for="r in state.versions" :key="r.version_id"><td>{{ r.version_id }}</td><td>{{ r.is_applied }}</td></tr>
            </tbody>
          </table>
          <p v-else class="none">таблицы ещё нет</p>
        </figure>
        <figure class="tabs">
          <figcaption>Таблицы приложения</figcaption>
          <ul v-if="state.tables.length > 0">
            <li v-for="t in state.tables" :key="t.name" :class="t.visibility">
              <code>{{ t.name }}</code><span v-if="t.visibility === 'tx'" class="tag"> видна только в транзакции</span>
            </li>
          </ul>
          <p v-else class="none">нет</p>
        </figure>
      </div>
    </section>
  </div>
</template>

<style scoped>
.mr { padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
h4 { margin: 0; font-size: 14px; font-weight: 700; overflow-wrap: anywhere; }
/* .mr перед code: перебить фон и цвет .vp-doc :not(pre) > code. */
.mr code { font-size: 12.5px; background: transparent; padding: 0; color: inherit; overflow-wrap: anywhere; }
.procs { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 130px), 1fr)); gap: 8px; }
.proc { min-width: 0; padding: 6px 8px; border: 2px solid var(--viz-edge); border-left: 8px solid var(--viz-edge); border-radius: 8px; }
.proc.run { border-left-color: var(--viz-service); }
.proc.ok { border-left-color: var(--viz-response); }
.proc.bad { border-left-color: var(--viz-error); }
.proc.idle { border-style: dashed; border-left-style: solid; color: var(--viz-muted); }
.proc p { margin: 2px 0 0; font-size: 14px; font-weight: 700; }
.proc p.sub { font-weight: 400; font-size: 12.5px; color: var(--viz-muted); }
.log { margin: 8px 0 0; padding: 4px 8px; border-left: 4px solid var(--viz-muted); font-family: ui-monospace, monospace; font-size: 12.5px; overflow-wrap: anywhere; }
.lock { margin: 6px 0 0; padding: 4px 8px; border: 2px dashed var(--viz-active); border-radius: 6px; font-size: 13px; }
.files { margin: 8px 0 0; padding: 0; list-style: none; font-size: 13px; }
.files li { margin: 0 0 3px; padding: 2px 8px; border-left: 6px solid var(--viz-edge); }
.files li.applied { border-left-color: var(--viz-response); }
.files li.running { border-left-color: var(--viz-active); }
.files li.failed { border-left-color: var(--viz-error); }
.fs { color: var(--viz-muted); }
.tx { margin-top: 8px; padding: 6px 8px; border: 2px solid var(--viz-active); border-radius: 8px; }
.tx.committed { border-color: var(--viz-response); }
.tx.rolled-back { border-color: var(--viz-error); }
.tx ol { margin: 4px 0 0; padding: 0; list-style: none; }
.tx li { display: flex; gap: 6px; margin: 2px 0 0; font-size: 13px; min-width: 0; }
.tx li code { min-width: 0; }
.mk { flex: none; width: 1em; font-weight: 700; }
.tx li.done .mk { color: var(--viz-response); }
.tx li.waiting .mk { color: var(--viz-active); }
.tx li.failed { color: var(--viz-error); font-weight: 700; }
.tx li.pending { color: var(--viz-muted); }
.tx li.undone code { text-decoration: line-through; }
.tx li.undone .mk { color: var(--viz-muted); }
.db { margin-top: 8px; padding: 6px 8px; border: 2px solid var(--viz-db); border-radius: 8px; min-width: 0; }
.ver { margin: 4px 0 6px; font-size: 13px; }
.ver.ok strong { color: var(--viz-response); }
.ver.bad strong { color: var(--viz-error); }
.cols { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 220px), 1fr)); gap: 8px; }
figure { margin: 0; min-width: 0; }
figcaption { font-size: 12.5px; color: var(--viz-muted); margin-bottom: 2px; }
/* .mr перед селекторами: перебить стили .vp-doc table (display: block, поля, зебра) и custom.css (min-width у ячеек на узком экране). */
.mr table { display: table; width: 100%; margin: 0; border-collapse: collapse; font-size: 13px; border: 0; }
.mr th, .mr td { min-width: 0; padding: 2px 4px; font-size: 13px; background: transparent; border: 0; border-bottom: 1px solid var(--viz-edge); text-align: left; font-variant-numeric: tabular-nums; }
.mr tr { background: transparent; border: 0; }
.mr th { font-weight: 600; color: var(--viz-muted); }
.tabs ul { margin: 0; padding: 0; list-style: none; }
.tabs li { margin: 0 0 3px; padding: 1px 6px; border: 2px solid var(--viz-db); border-radius: 5px; font-size: 13px; }
.tabs li.tx { border-style: dashed; border-color: var(--viz-active); }
.tag { font-size: 12px; color: var(--viz-muted); }
.none { margin: 0; font-size: 13px; color: var(--viz-muted); }
.sr-only { position: absolute; width: 1px; height: 1px; margin: -1px; padding: 0; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; border: 0; }
</style>
