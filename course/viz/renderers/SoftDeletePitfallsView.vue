<script setup lang="ts">
import { computed } from 'vue'
import { ERASED_NAME, PARTIAL_KEY, PLAIN_KEY, type SdFrame, type SdRef } from '../simulations/soft-delete-pitfalls'

const props = defineProps<{ state: SdFrame }>()

// Цвет со смыслом: рамка таблиц — --viz-db; строки, в которых видна проблема шага, — --viz-error;
// результат команды — полоса --viz-response (прошла), --viz-error (ошибка), --viz-active (вернула строки);
// текст цветом --viz-response не красим: в светлой теме его контраст с фоном ниже 4,5:1;
// «удалённые» строки — приглушённые --viz-muted с пометкой словами, а не только цветом.
const badge = computed(() => {
  if (props.state.verdict === 'broken') return { cls: 'bad', text: '✕ ловушка: правило, которое держала база, больше не держит никто' }
  if (props.state.verdict === 'fixed') return { cls: 'ok', text: '✓ исправлено: правило снова держится' }
  return { cls: 'mid', text: '… исходное состояние' }
})
const ruleText = computed(() =>
  props.state.rule === 'unique'
    ? `UNIQUE (username) — ${PLAIN_KEY}: среди всех строк`
    : `UNIQUE INDEX ${PARTIAL_KEY} (username) WHERE deleted_at IS NULL: среди живых`,
)
const OUTCOME_TEXT = { ok: '✓', error: '✕', rows: '→' } as const
const deletedIds = computed(() => new Set(props.state.users.filter((u) => u.deleted).map((u) => u.id)))
const flaggedUser = (id: number): boolean => props.state.flagUsers.includes(id)
const flaggedRef = (r: SdRef): boolean => props.state.flagRefs.includes(r.key)
const refLabel = (r: SdRef): string => (r.table === 'sessions' ? `сессия ${r.key}` : `«${r.key}»`)
const nameLeft = computed(() => {
  if (props.state.audit === null) return null
  const inUsers = props.state.users.some((u) => u.username === ERASED_NAME && u.id === 1)
  const inAudit = props.state.audit.filter((a) => a.oldName === ERASED_NAME || a.newName === ERASED_NAME).length
  return { inUsers, inAudit }
})
</script>

<template>
  <div class="sd">
    <p class="badge" :class="badge.cls" role="status">{{ badge.text }}</p>

    <p class="rule" :class="state.rule">
      <span class="k">уникальность имени:</span> {{ ruleText }}
      <span v-if="state.view" class="view">VIEW active_users … WHERE deleted_at IS NULL</span>
    </p>

    <section v-if="state.sql" class="cmd" :class="state.outcome ?? ''" aria-label="Последняя команда">
      <code class="sql">{{ state.sql }}</code>
      <p v-if="state.result" class="res">
        <span aria-hidden="true">{{ state.outcome ? OUTCOME_TEXT[state.outcome] : '' }}</span>
        {{ state.result }}
      </p>
    </section>

    <section class="tbl" aria-label="Таблица users">
      <h4>users</h4>
      <table>
        <thead>
          <tr><th scope="col">id</th><th scope="col">username</th><th scope="col">display_<wbr>name</th><th scope="col">deleted_<wbr>at</th></tr>
        </thead>
        <tbody>
          <tr v-for="u in state.users" :key="u.id" :class="{ del: u.deleted, flag: flaggedUser(u.id) }">
            <td>{{ u.id }}</td>
            <td class="name">{{ u.username }}<span v-if="flaggedUser(u.id)" class="sr-only"> (проблема на этом шаге)</span></td>
            <td>{{ u.displayName }}</td>
            <td>{{ u.deleted ? 'удалён' : 'NULL' }}</td>
          </tr>
        </tbody>
      </table>
    </section>

    <section v-if="state.refs.length > 0" class="tbl" aria-label="Ссылки на users: sessions и messages">
      <h4>sessions (ON DELETE CASCADE) и messages (NO ACTION)</h4>
      <ul class="refs">
        <li v-for="r in state.refs" :key="r.key" :class="{ flag: flaggedRef(r), dead: deletedIds.has(r.userId) }">
          <span class="t">{{ r.table }}</span> {{ refLabel(r) }} → user_id {{ r.userId }}<span v-if="deletedIds.has(r.userId)" class="note"> (удалён)</span><span v-if="flaggedRef(r)" class="sr-only"> (проблема на этом шаге)</span>
        </li>
      </ul>
    </section>

    <section v-if="state.audit !== null" class="tbl audit" aria-label="Журнал аудита users_audit, строки alice">
      <h4>users_audit · история alice (id 1)</h4>
      <table>
        <thead>
          <tr><th scope="col">№</th><th scope="col">op</th><th scope="col">old_<wbr>row → username</th><th scope="col">new_<wbr>row → username</th></tr>
        </thead>
        <tbody>
          <tr v-for="a in state.audit" :key="a.id" :class="{ flag: state.flagAudit.includes(a.id) }">
            <td>{{ a.id }}</td>
            <td>{{ a.op }}</td>
            <td :class="{ pii: a.oldName === ERASED_NAME }">{{ a.oldName ?? '—' }}</td>
            <td :class="{ pii: a.newName === ERASED_NAME }">{{ a.newName ?? '—' }}</td>
          </tr>
        </tbody>
      </table>
      <p v-if="nameLeft" class="left" :class="{ bad: nameLeft.inUsers || nameLeft.inAudit > 0 }">
        Имя alice в базе: в users — {{ nameLeft.inUsers ? 'есть' : 'нет' }}, в журнале — {{ nameLeft.inAudit }} {{ nameLeft.inAudit === 1 ? 'строка' : nameLeft.inAudit >= 2 && nameLeft.inAudit <= 4 ? 'строки' : 'строк' }}
      </p>
    </section>
  </div>
</template>

<style scoped>
.sd { padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
h4 { margin: 0 0 4px; font-size: 14px; font-weight: 700; overflow-wrap: anywhere; }
.badge { margin: 0 0 10px; padding: 6px 10px; border: 2px solid var(--viz-edge); border-left-width: 8px; border-radius: 8px; font-size: 14px; font-weight: 700; }
.badge.ok { border-left-color: var(--viz-response); }
.badge.bad { border-left-color: var(--viz-error); }
.badge.mid { border-left-color: var(--viz-active); }
.rule { margin: 0 0 8px; font-size: 13px; overflow-wrap: anywhere; }
.rule .k { color: var(--viz-muted); }
.rule .view { display: block; margin-top: 2px; font-weight: 600; }
.cmd { margin: 0 0 10px; padding: 6px 8px; border: 2px solid var(--viz-edge); border-left-width: 8px; border-radius: 8px; min-width: 0; }
.cmd.ok { border-left-color: var(--viz-response); }
.cmd.error { border-left-color: var(--viz-error); }
.cmd.rows { border-left-color: var(--viz-active); }
.sd .sql { display: block; padding: 0; font-size: 12px; line-height: 1.4; white-space: pre-wrap; overflow-wrap: anywhere; background: transparent; color: var(--viz-text); }
.res { margin: 4px 0 0; font-size: 13px; font-weight: 600; overflow-wrap: anywhere; }
.cmd.error .res { color: var(--viz-error); }
.tbl { margin-top: 10px; padding: 6px 8px; border: 2px solid var(--viz-db); border-radius: 8px; min-width: 0; }
/* .sd перед селекторами: перебить стили .vp-doc table (display: block, поля, зебра) и custom.css (min-width у ячеек на узком экране). */
.sd table { display: table; width: 100%; margin: 0; border-collapse: collapse; font-size: 13px; line-height: 1.35; border: 0; table-layout: auto; }
.sd th, .sd td { min-width: 0; padding: 3px 3px; font-size: 13px; background: transparent; border: 0; border-bottom: 1px solid var(--viz-edge); text-align: left; vertical-align: top; overflow-wrap: break-word; }
.sd tr { background: transparent; border: 0; }
.sd th { font-weight: 600; color: var(--viz-muted); }
.sd tr.del td { color: var(--viz-muted); }
.sd tr.del td.name { text-decoration: line-through; }
.sd tr.flag td { color: var(--viz-error); font-weight: 700; }
.sd tr.flag td:first-child { box-shadow: inset 4px 0 0 var(--viz-error); }
.sd td.pii { color: var(--viz-error); font-weight: 700; }
.refs { margin: 0; padding: 0; list-style: none; }
.refs li { margin: 0 0 4px; padding: 3px 8px; border: 2px solid var(--viz-edge); border-left-width: 6px; border-radius: 6px; font-size: 13px; overflow-wrap: anywhere; }
.refs li.dead { color: var(--viz-muted); }
.refs li.flag { border-left-color: var(--viz-error); color: var(--viz-error); font-weight: 700; }
.refs .t { font-family: var(--vp-font-family-mono); font-size: 12px; }
.left { margin: 6px 0 0; padding-left: 6px; border-left: 6px solid var(--viz-response); font-size: 13px; font-weight: 700; }
.left.bad { border-left-color: var(--viz-error); color: var(--viz-error); }
.sr-only { position: absolute; width: 1px; height: 1px; margin: -1px; padding: 0; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; border: 0; }
</style>
