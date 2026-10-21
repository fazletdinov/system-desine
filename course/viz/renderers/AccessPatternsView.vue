<script setup lang="ts">
import { computed } from 'vue'
import { TABLES, formatRows, type AccessKind, type AccessPatternsFrame } from '../simulations/access-patterns'

const props = defineProps<{ frame: AccessPatternsFrame }>()

// Цвет со смыслом: --viz-db — запрос обслужен индексом (точка или диапазон), --viz-active — индекс
// есть, но строк читается больше нужного (сортировка, фильтр), --viz-error — полный проход таблицы.
const KIND_TEXT: Record<AccessKind, string> = {
  point: 'точечный поиск',
  range: 'диапазон',
  'range-sort': 'диапазон + сортировка',
  full: 'полный проход',
}
const tone = (k: AccessKind): string => (k === 'full' ? 'bad' : k === 'range-sort' ? 'mid' : 'good')

const r = computed(() => props.frame.selected)
const total = computed(() => TABLES[r.value.query.table].rows)

// Логарифмическая шкала: 1 строка и 2·10⁸ строк на одной оси различимы только так. Сдвиг 0,3 даёт
// одной строке видимую, а не нулевую полосу.
const LOG_SHIFT = 0.3
const logMax = computed(() => Math.log10(total.value) + LOG_SHIFT)
const barPct = (rows: number): number => ((Math.log10(Math.max(1, rows)) + LOG_SHIFT) / logMax.value) * 100
const bars = computed(() => {
  const out = [{ key: 'plan', text: 'с этим планом', rows: r.value.plan.rowsRead, kind: r.value.plan.kind }]
  if (r.value.withoutIndex !== null) out.push({ key: 'without', text: `без ${r.value.plan.index!.label}`, rows: r.value.withoutIndex.rowsRead, kind: r.value.withoutIndex.kind })
  out.push({ key: 'table', text: `вся таблица ${r.value.query.table}`, rows: total.value, kind: 'full' as AccessKind })
  return out
})
const summary = computed(() => bars.value.map((b) => `${b.text}: ${formatRows(b.rows)} строк`).join('; '))
</script>

<template>
  <div class="ap">
    <p class="note">{{ r.query.note }}</p>
    <pre class="sql"><code>{{ r.query.sql }}</code></pre>

    <div class="indexes">
      <p class="h">Индексы <code>{{ r.query.table }}</code> ({{ formatRows(total) }} строк в модели)</p>
      <ul>
        <li v-for="ix in frame.tableIndexes" :key="ix.id" :class="{ used: ix.id === r.plan.index?.id }">
          <code>{{ ix.label }}</code><span v-if="ix.id === r.plan.index?.id" class="tag">используется</span>
        </li>
      </ul>
    </div>

    <dl class="counters">
      <div :class="tone(r.plan.kind)"><dt>Доступ</dt><dd>{{ KIND_TEXT[r.plan.kind] }}</dd></div>
      <div :class="tone(r.plan.kind)"><dt>Строк прочитано</dt><dd>{{ formatRows(r.plan.rowsRead) }}</dd></div>
      <div><dt>Строк в ответе</dt><dd>{{ formatRows(r.plan.rowsReturned) }}</dd></div>
    </dl>

    <figure class="chart">
      <figcaption><b>Сколько строк читается</b> (логарифмическая шкала)</figcaption>
      <ul class="bars" role="img" :aria-label="summary">
        <li v-for="b in bars" :key="b.key">
          <span class="lbl">{{ b.text }}: <b>{{ formatRows(b.rows) }}</b></span>
          <span :class="['bar', b.key === 'table' ? 'total' : tone(b.kind)]" :style="{ width: `${barPct(b.rows)}%` }" aria-hidden="true"></span>
        </li>
      </ul>
    </figure>

    <table class="all">
      <caption>Все запросы при этой схеме</caption>
      <thead><tr><th scope="col">запрос</th><th scope="col">доступ</th><th scope="col">строк</th></tr></thead>
      <tbody>
        <tr v-for="q in frame.all" :key="q.query.id" :class="{ on: q.query.id === r.query.id }">
          <th scope="row">{{ q.query.title }}</th>
          <td :class="tone(q.plan.kind)">{{ KIND_TEXT[q.plan.kind] }}</td>
          <td>{{ formatRows(q.plan.rowsRead) }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.ap { color: var(--viz-text); }
.note { margin: 0 0 6px; font-size: 14px; color: var(--viz-muted); }
.sql { margin: 0 0 10px; padding: 8px 10px; overflow-x: auto; max-width: 100%; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg-soft); font-family: var(--vp-font-family-mono); font-size: 13px; line-height: 1.5; white-space: pre-wrap; overflow-wrap: anywhere; }
.indexes { margin-bottom: 10px; padding: 6px 10px; border: 2px solid var(--viz-db); border-radius: 8px; }
.indexes .h { margin: 0 0 4px; font-size: 13px; color: var(--viz-muted); overflow-wrap: anywhere; }
.indexes ul { margin: 0; padding: 0; list-style: none; font-size: 13px; }
.indexes li { margin: 0; display: flex; flex-wrap: wrap; align-items: center; gap: 4px 8px; padding: 2px 4px; border-radius: 4px; overflow-wrap: anywhere; }
.indexes li code { overflow-wrap: anywhere; }
.indexes li.used { background: var(--viz-active-line-bg); box-shadow: inset 4px 0 0 var(--viz-active); padding-left: 10px; font-weight: 600; }
.tag { font-size: 12px; color: var(--viz-text); }
.counters { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 110px), 1fr)); gap: 8px; margin: 0 0 10px; }
.counters div { min-width: 0; padding: 4px 10px; border: 2px solid var(--viz-edge); border-left-width: 8px; border-radius: 8px; }
.counters .good { border-left-color: var(--viz-db); }
.counters .mid { border-left-color: var(--viz-active); }
.counters .bad { border-left-color: var(--viz-error); }
dt { font-size: 12px; color: var(--viz-muted); }
dd { margin: 0; font-size: 16px; font-weight: 700; font-variant-numeric: tabular-nums; }
.chart { margin: 0 0 10px; }
figcaption { font-size: 14px; margin-bottom: 4px; }
.bars { margin: 0; padding: 0; list-style: none; }
.bars li { margin: 0 0 6px; }
.lbl { display: block; font-size: 14px; overflow-wrap: anywhere; font-variant-numeric: tabular-nums; }
.bar { display: block; height: 18px; margin-top: 2px; border-radius: 3px; box-sizing: border-box; }
.bar.good { background: var(--viz-db); }
.bar.mid { background: var(--viz-active); }
.bar.bad { background: var(--viz-error); }
.bar.total { border: 2px dashed var(--viz-muted); }
/* .ap перед селекторами: перебить стили .vp-doc table (display: block, поля 8×16 px, зебра) и custom.css (min-width: 7rem у ячеек на узком экране). */
.ap .all { display: table; width: 100%; margin: 0; border-collapse: collapse; font-size: 13px; font-variant-numeric: tabular-nums; }
.all caption { text-align: left; font-size: 14px; margin-bottom: 2px; }
.ap .all tr { background: transparent; border: 0; }
.ap .all th, .ap .all td { min-width: 0; padding: 3px 4px; font-size: 13px; background: transparent; border-bottom: 1px solid var(--viz-edge); text-align: left; }
.all td:last-child { text-align: right; }
.all thead th { font-weight: 600; color: var(--viz-muted); }
.all th[scope='row'] { font-weight: 400; }
.all tr.on th[scope='row'] { box-shadow: inset 4px 0 0 var(--viz-client); padding-left: 8px; font-weight: 600; }
.all td.bad { color: var(--viz-error); font-weight: 700; }
.all td.mid { font-weight: 700; }
</style>
