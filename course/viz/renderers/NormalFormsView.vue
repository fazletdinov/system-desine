<script setup lang="ts">
import { computed } from 'vue'
import { ANOMALY_EFFECT, ANOMALY_TITLE, type NormalFormsFrame, type Outcome, type TableName } from '../simulations/normal-forms'

const props = defineProps<{ frame: NormalFormsFrame }>()

// Цвет со смыслом: --viz-active — строка, которую тронуло последнее действие; --viz-error — аномалия
// (противоречивое значение, потерянный или не вставленный факт); рамка таблиц — --viz-db.
const outcome = computed<Outcome | null>(() => (props.frame.last === null ? null : props.frame.schema === 'flat' ? props.frame.last.flat : props.frame.last.nf3))
const changed = (table: TableName, key: number): boolean => outcome.value?.changed.some((r) => r.table === table && r.key === key) ?? false
const bad = computed(() => new Set(props.frame.inconsistent))
const removedText = computed(() => {
  const o = outcome.value
  if (o === null || o.removed.length === 0) return ''
  return o.removed.map((r) => `${r.table}: строка ${r.key}`).join(', ')
})
const missing = computed(() =>
  Object.entries(props.frame.missing).map(([id, reason]) => ({
    id: Number(id),
    name: props.frame.users.find((u) => u.id === Number(id))?.name ?? id,
    reason: reason === 'deleted' ? 'потерян при удалении' : 'не удалось вставить',
  })),
)
const titleOf = (id: number): string => props.frame.chats.find((c) => c.id === id)?.title ?? String(id)
const changedCount = (o: Outcome): string => (o.rejected ? 'отказ' : `${o.changed.length + o.removed.length}`)
</script>

<template>
  <div class="nf">
    <dl class="counters">
      <div :class="{ bad: frame.last?.flat.anomaly }">
        <dt>Одна таблица: строк затронуто</dt>
        <dd>{{ frame.last ? changedCount(frame.last.flat) : '—' }}</dd>
      </div>
      <div>
        <dt>3НФ: строк затронуто</dt>
        <dd>{{ frame.last ? changedCount(frame.last.nf3) : '—' }}</dd>
      </div>
    </dl>

    <p v-if="outcome?.anomaly" class="anomaly" role="status">
      <b>{{ ANOMALY_TITLE[outcome.anomaly] }}</b> — {{ ANOMALY_EFFECT[outcome.anomaly] }}
    </p>
    <p v-else-if="removedText" class="removed">Удалено: {{ removedText }}</p>

    <template v-if="frame.schema === 'flat'">
      <figure class="tbl">
        <figcaption><code>messages_flat</code> — сообщение вместе с автором и чатом</figcaption>
        <table>
          <thead>
            <tr><th scope="col">id</th><th scope="col">чат</th><th scope="col">название</th><th scope="col">автор</th><th scope="col">имя</th><th scope="col">текст</th></tr>
          </thead>
          <tbody>
            <tr v-for="r in frame.flat" :key="r.messageId" :class="{ hit: changed('messages_flat', r.messageId), sel: r.authorId === frame.authorId }">
              <td>{{ r.messageId }}<span v-if="changed('messages_flat', r.messageId)" class="sr-only"> (изменена)</span></td>
              <td>{{ r.chatId }}</td>
              <td class="dup">{{ r.chatTitle }}</td>
              <td>{{ r.authorId }}</td>
              <td :class="['dup', { conflict: bad.has(r.authorId) }]">{{ r.authorName }}<span v-if="bad.has(r.authorId)" class="mark" aria-hidden="true"> ≠</span><span v-if="bad.has(r.authorId)" class="sr-only"> (противоречие)</span></td>
              <td class="txt">{{ r.text }}</td>
            </tr>
          </tbody>
        </table>
      </figure>
      <p class="legend"><i>Курсив</i> — копии одного факта (название чата, имя автора)</p>
      <ul v-if="missing.length > 0" class="missing" aria-label="Нет в таблице">
        <li v-for="m in missing" :key="m.id">нет в таблице: <b>{{ m.name }}</b> ({{ m.id }}) — {{ m.reason }}</li>
      </ul>
    </template>

    <div v-else class="nf3">
      <figure class="tbl">
        <figcaption><code>users</code></figcaption>
        <table>
          <thead><tr><th scope="col">id</th><th scope="col">имя</th></tr></thead>
          <tbody>
            <tr v-for="u in frame.users" :key="u.id" :class="{ hit: changed('users', u.id), sel: u.id === frame.authorId }"><td>{{ u.id }}<span v-if="changed('users', u.id)" class="sr-only"> (изменена)</span></td><td>{{ u.name }}</td></tr>
          </tbody>
        </table>
      </figure>
      <figure class="tbl">
        <figcaption><code>chats</code></figcaption>
        <table>
          <thead><tr><th scope="col">id</th><th scope="col">название</th></tr></thead>
          <tbody>
            <tr v-for="c in frame.chats" :key="c.id"><td>{{ c.id }}</td><td>{{ c.title }}</td></tr>
          </tbody>
        </table>
      </figure>
      <figure class="tbl wide">
        <figcaption><code>messages</code> — только ссылки на автора и чат</figcaption>
        <table>
          <thead><tr><th scope="col">id</th><th scope="col">чат</th><th scope="col">автор</th><th scope="col">текст</th></tr></thead>
          <tbody>
            <tr v-for="m in frame.messages" :key="m.id" :class="{ hit: changed('messages', m.id), sel: m.authorId === frame.authorId }">
              <td>{{ m.id }}<span v-if="changed('messages', m.id)" class="sr-only"> (изменена)</span></td>
              <td :title="titleOf(m.chatId)">{{ m.chatId }}</td>
              <td>{{ m.authorId }}</td>
              <td class="txt">{{ m.text }}</td>
            </tr>
          </tbody>
        </table>
      </figure>
    </div>
  </div>
</template>

<style scoped>
.nf { color: var(--viz-text); }
.counters { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 140px), 1fr)); gap: 8px; margin: 0 0 10px; }
.counters div { min-width: 0; padding: 4px 10px; border: 2px solid var(--viz-edge); border-left-width: 8px; border-radius: 8px; }
.counters .bad { border-left-color: var(--viz-error); }
dt { font-size: 12px; color: var(--viz-muted); }
dd { margin: 0; font-size: 18px; font-weight: 700; font-variant-numeric: tabular-nums; }
.anomaly { margin: 0 0 10px; padding: 6px 10px; border: 2px solid var(--viz-error); border-radius: 8px; font-size: 14px; }
.anomaly b { color: var(--viz-error); }
.removed { margin: 0 0 10px; font-size: 14px; color: var(--viz-muted); }
.tbl { margin: 0 0 8px; min-width: 0; }
figcaption { font-size: 13px; margin-bottom: 2px; color: var(--viz-muted); }
/* .nf перед селекторами: перебить стили .vp-doc table (display: block, поля 8×16 px, зебра) и custom.css (min-width: 7rem у ячеек на узком экране). */
.nf table { display: table; width: 100%; margin: 0; border-collapse: collapse; font-size: 13px; line-height: 1.35; border: 2px solid var(--viz-db); }
.nf th, .nf td { min-width: 0; padding: 3px 4px; font-size: 13px; background: transparent; border-bottom: 1px solid var(--viz-edge); text-align: left; vertical-align: top; font-variant-numeric: tabular-nums; }
.nf tr { background: transparent; border: 0; }
.nf th { font-weight: 600; color: var(--viz-muted); }
td.txt { overflow-wrap: anywhere; }
td.dup { font-style: italic; }
tr.sel td:first-child { box-shadow: inset 3px 0 0 var(--viz-client); }
tr.hit td { background: var(--viz-active-line-bg); }
tr.hit td:first-child { box-shadow: inset 5px 0 0 var(--viz-active); }
td.conflict { color: var(--viz-error); font-weight: 700; }
.mark { font-style: normal; }
.sr-only { position: absolute; width: 1px; height: 1px; margin: -1px; padding: 0; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; border: 0; }
.legend { margin: 0 0 6px; font-size: 12px; color: var(--viz-muted); }
.missing { margin: 0; padding: 0; list-style: none; font-size: 14px; }
.missing li { padding: 4px 10px; margin: 0 0 4px; border: 2px dashed var(--viz-error); border-radius: 8px; }
.nf3 { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 140px), 1fr)); gap: 0 12px; }
.nf3 .wide { grid-column: 1 / -1; }
</style>
