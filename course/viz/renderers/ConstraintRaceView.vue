<script setup lang="ts">
import { computed } from 'vue'
import { CONSTRAINT, ERROR_CHAIN, type InstanceState, type RaceFrame } from '../simulations/constraint-race'

const props = defineProps<{ state: RaceFrame }>()

// Цвет со смыслом: левая полоса экземпляра — его состояние (--viz-service работает, --viz-active ждёт,
// --viz-response ответил 2xx, --viz-error получил ошибку); фон — кто действует на этом шаге;
// рамка таблицы — --viz-db; строки-дубликаты и пройденные звенья ошибки — --viz-error.
const STATE_TEXT: Record<InstanceState, string> = {
  idle: 'ждёт запроса',
  running: '● выполняет запрос',
  waiting: '◐ ждёт исхода чужой транзакции',
  done: '✓ ответил',
  failed: '✕ ошибка базы',
}
const counts = computed(() => {
  const m = new Map<string, number>()
  for (const r of props.state.rows) if (r.committed) m.set(r.username, (m.get(r.username) ?? 0) + 1)
  return m
})
const isDup = (username: string, committed: boolean): boolean => committed && (counts.value.get(username) ?? 0) > 1
const badge = computed(() => {
  if (props.state.verdict === 'broken') return { cls: 'bad', text: '✕ два пользователя alice: правило «имя уникально» нарушено' }
  if (props.state.verdict === 'held') return { cls: 'ok', text: '✓ один пользователь alice: правило держит база' }
  return { cls: 'mid', text: '… регистрации выполняются одновременно' }
})
const httpText = (code: 201 | 409 | null): string => (code === 201 ? '201 Created' : code === 409 ? '409 Conflict' : 'ответа пока нет')
</script>

<template>
  <div class="cr">
    <p class="badge" :class="badge.cls" role="status">{{ badge.text }}</p>

    <div class="insts">
      <section
        v-for="i in state.instances"
        :key="i.id"
        :class="['inst', i.state, { actor: state.actor === i.id }]"
        :aria-label="`Экземпляр ${i.id}`"
      >
        <h4>Экземпляр {{ i.id }}<span v-if="state.actor === i.id" class="sr-only"> (действует сейчас)</span></h4>
        <p class="st">{{ STATE_TEXT[i.state] }}</p>
        <code v-if="i.sql" class="sql">{{ i.sql }}</code>
        <p v-if="i.result" class="res">→ {{ i.result }}</p>
        <p class="http" :class="{ ok: i.http === 201, bad: i.http === 409 }">HTTP: {{ httpText(i.http) }}</p>
      </section>
    </div>

    <section class="db" aria-label="PostgreSQL, таблица users.users">
      <h4>PostgreSQL · users.users</h4>
      <p class="cons" :class="{ has: state.unique }">
        {{ state.unique ? `UNIQUE (username) — ограничение ${CONSTRAINT}` : 'ограничения на username нет: уникальность проверяет только код' }}
      </p>
      <table v-if="state.rows.length > 0">
        <thead>
          <tr><th scope="col">username</th><th scope="col">кто</th><th scope="col">видимость</th></tr>
        </thead>
        <tbody>
          <tr v-for="r in state.rows" :key="r.id" :class="{ dup: isDup(r.username, r.committed), pending: !r.committed }">
            <td>{{ r.username }}<span v-if="isDup(r.username, r.committed)" class="sr-only"> (дубликат)</span></td>
            <td>{{ r.by }}</td>
            <td>{{ r.committed ? 'зафиксирована: видят все' : `не зафиксирована: видит только ${r.by}` }}</td>
          </tr>
        </tbody>
      </table>
      <p v-else class="empty">строк с именем alice нет</p>
    </section>

    <section v-if="state.unique" class="chain" aria-label="Перевод ошибки от базы к клиенту">
      <h4>Путь ошибки к клиенту B</h4>
      <ol>
        <li v-for="(c, i) in ERROR_CHAIN" :key="c.layer" :class="{ hit: i <= state.chain, now: i === state.chain }">
          <span class="layer">{{ c.layer }}</span>
          <code>{{ c.text }}</code>
          <span v-if="i <= state.chain" class="sr-only"> (пройдено)</span>
        </li>
      </ol>
    </section>
  </div>
</template>

<style scoped>
.cr { padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
h4 { margin: 0; font-size: 14px; font-weight: 700; }
.badge { margin: 0 0 10px; padding: 6px 10px; border: 2px solid var(--viz-edge); border-left-width: 8px; border-radius: 8px; font-size: 14px; font-weight: 700; }
.badge.ok { border-left-color: var(--viz-response); }
.badge.bad { border-left-color: var(--viz-error); }
.badge.mid { border-left-color: var(--viz-active); }
.insts { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 130px), 1fr)); gap: 8px; }
.inst { min-width: 0; padding: 6px 8px; border: 2px solid var(--viz-edge); border-left: 8px solid var(--viz-service); border-radius: 8px; }
.inst.waiting { border-left-color: var(--viz-active); }
.inst.done { border-left-color: var(--viz-response); }
.inst.failed { border-left-color: var(--viz-error); }
.inst.actor { background: var(--viz-active-line-bg); }
.inst p { margin: 4px 0 0; font-size: 13px; overflow-wrap: anywhere; }
.st { color: var(--viz-muted); }
.cr .sql { display: block; margin-top: 4px; padding: 3px 5px; font-size: 12px; line-height: 1.4; white-space: pre-wrap; overflow-wrap: anywhere; background: transparent; border: 1px solid var(--viz-edge); border-radius: 4px; color: var(--viz-text); }
.http { font-weight: 700; }
.http.ok { color: var(--viz-response); }
.http.bad { color: var(--viz-error); }
.db { margin-top: 10px; padding: 6px 8px; border: 2px solid var(--viz-db); border-radius: 8px; min-width: 0; }
.cons { margin: 4px 0 6px; font-size: 13px; color: var(--viz-muted); }
.cons.has { color: var(--viz-text); font-weight: 600; }
/* .cr перед селекторами: перебить стили .vp-doc table (display: block, поля, зебра) и custom.css (min-width у ячеек на узком экране). */
.cr table { display: table; width: 100%; margin: 0; border-collapse: collapse; font-size: 13px; line-height: 1.35; border: 0; }
.cr th, .cr td { min-width: 0; padding: 3px 4px; font-size: 13px; background: transparent; border: 0; border-bottom: 1px solid var(--viz-edge); text-align: left; vertical-align: top; overflow-wrap: break-word; }
.cr tr { background: transparent; border: 0; }
.cr th { font-weight: 600; color: var(--viz-muted); }
.cr tr.pending td { font-style: italic; color: var(--viz-muted); }
.cr tr.dup td { color: var(--viz-error); font-weight: 700; }
.cr tr.dup td:first-child { box-shadow: inset 4px 0 0 var(--viz-error); }
.empty { margin: 0; font-size: 13px; color: var(--viz-muted); }
.chain { margin-top: 10px; }
.chain ol { margin: 4px 0 0; padding: 0; list-style: none; }
.chain li { margin: 0 0 4px; padding: 3px 8px; border: 2px solid var(--viz-edge); border-left-width: 6px; border-radius: 6px; font-size: 13px; color: var(--viz-muted); overflow-wrap: anywhere; }
.chain li.hit { border-left-color: var(--viz-error); color: var(--viz-text); }
.chain li.now { background: var(--viz-active-line-bg); }
.layer { display: block; font-size: 12px; color: var(--viz-muted); }
.cr .chain code { font-size: 12.5px; background: transparent; padding: 0; color: inherit; }
.sr-only { position: absolute; width: 1px; height: 1px; margin: -1px; padding: 0; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; border: 0; }
</style>
