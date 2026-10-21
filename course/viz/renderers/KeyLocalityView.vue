<script setup lang="ts">
import { computed } from 'vue'
import { FILL_ALERT, FILL_ALERT_MIN_PAGES, FPI_ALERT, HOT_ALERT, HOT_WINDOW, KEY_KINDS, KIND_LABEL, LEAF_FILLFACTOR, PAGE_CAPACITY, type KeyLocalityFrame } from '../simulations/key-locality'

const props = defineProps<{ frame: KeyLocalityFrame }>()

// viewBox 360 при кегле 15 даёт ≥ 12 px на экране 375 px (как GcPacerView).
const W = 360
const H = 150
const LEFT = 40
const RIGHT = 8
const TOP = 10
const BOTTOM = 24
const FONT = 15
const plotW = W - LEFT - RIGHT
const plotH = H - TOP - BOTTOM
// Ширина столбика не растёт выше, чем при 20 страницах: иначе первые вставки выглядят огромными.
const MIN_SLOTS = 20

const slot = computed(() => plotW / Math.max(MIN_SLOTS, props.frame.pages.length))
const gap = computed(() => (slot.value > 4 ? 1 : 0))
const y = (fill: number): number => TOP + plotH * (1 - fill)
const st = computed(() => props.frame.stats[props.frame.selected])
const pct = (v: number): string => `${Math.round(v * 100)} %`
const summary = computed(
  () => `${KIND_LABEL[props.frame.selected]}: ${st.value.pages} листьев, заполненность ${pct(st.value.fill)}, горячих ${st.value.hotPages}, разделений справа ${st.value.splitsRight}, в середине ${st.value.splitsMiddle}`,
)
</script>

<template>
  <div class="kl">
    <dl class="counters">
      <div><dt>Ключей</dt><dd>{{ st.keys }}</dd></div>
      <div><dt>Листьев</dt><dd>{{ st.pages }}</dd></div>
      <div :class="{ mid: st.pages >= FILL_ALERT_MIN_PAGES && st.fill < FILL_ALERT }"><dt>Заполненность</dt><dd>{{ st.keys === 0 ? '—' : pct(st.fill) }}</dd></div>
      <div :class="{ bad: st.splitsMiddle > 0 }"><dt>Разделений: справа / в середине</dt><dd>{{ st.splitsRight }} / {{ st.splitsMiddle }}</dd></div>
      <div :class="{ mid: st.hotPages > HOT_ALERT }"><dt>Горячих страниц</dt><dd>{{ st.hotPages }}</dd></div>
      <div :class="{ bad: st.fpi > FPI_ALERT }"><dt>Полных образов после CHECKPOINT</dt><dd>{{ frame.checkpoints === 0 ? '—' : st.fpi }}</dd></div>
    </dl>

    <figure class="chart">
      <figcaption><b>Листья по порядку ключей</b> — высота столбика: сколько из {{ PAGE_CAPACITY }} мест занято</figcaption>
      <svg :viewBox="`0 0 ${W} ${H}`" role="img" :aria-label="summary">
        <line :x1="LEFT" :x2="W - RIGHT" :y1="y(0)" :y2="y(0)" class="axis" />
        <line :x1="LEFT" :x2="W - RIGHT" :y1="y(1)" :y2="y(1)" class="grid" />
        <line :x1="LEFT" :x2="W - RIGHT" :y1="y(0.5)" :y2="y(0.5)" class="grid" stroke-dasharray="2 4" />
        <g v-for="(p, i) in frame.pages" :key="p.id">
          <rect :x="LEFT + i * slot + gap" :width="Math.max(1, slot - 2 * gap)" :y="y(p.fill)" :height="y(0) - y(p.fill)" :class="['bar', { hot: p.hot }]" />
          <rect v-if="p.split" :x="LEFT + i * slot + gap" :width="Math.max(1, slot - 2 * gap)" :y="y(1) - 6" height="4" class="split" />
        </g>
        <line :x1="LEFT" :x2="W - RIGHT" :y1="y(LEAF_FILLFACTOR)" :y2="y(LEAF_FILLFACTOR)" class="ff" stroke-dasharray="6 4" />
        <text :x="LEFT - 4" :y="y(1) + FONT * 0.35" text-anchor="end" :font-size="FONT" class="lbl">100</text>
        <text :x="LEFT - 4" :y="y(0.5) + FONT * 0.35" text-anchor="end" :font-size="FONT" class="lbl">50</text>
        <text :x="LEFT - 4" :y="y(0) + FONT * 0.35" text-anchor="end" :font-size="FONT" class="lbl">0</text>
        <text :x="LEFT" :y="H - 6" :font-size="FONT" class="lbl">меньшие ключи</text>
        <text :x="W - RIGHT" :y="H - 6" text-anchor="end" :font-size="FONT" class="lbl">бо́льшие →</text>
        <text v-if="st.keys === 0" :x="LEFT + plotW / 2" :y="TOP + plotH / 2" text-anchor="middle" :font-size="FONT" class="lbl">вставьте ключи</text>
      </svg>
      <ul class="legend">
        <li><span class="sw bar-sw" aria-hidden="true"></span>лист</li>
        <li><span class="sw hot-sw" aria-hidden="true"></span>горячий: запись за последние {{ HOT_WINDOW }} вставок</li>
        <li><span class="sw split-sw" aria-hidden="true"></span>разделён последним действием</li>
        <li><svg viewBox="0 0 28 8" aria-hidden="true"><line x1="0" y1="4" x2="28" y2="4" class="ff" stroke-dasharray="6 4" /></svg>fillfactor {{ Math.round(LEAF_FILLFACTOR * 100) }} %</li>
      </ul>
    </figure>

    <table class="cmp">
      <caption>Все три ключа при одном и том же числе вставок</caption>
      <thead>
        <tr><th scope="col">ключ</th><th scope="col">листьев</th><th scope="col">заполн.</th><th scope="col">в середине</th><th scope="col">образов</th></tr>
      </thead>
      <tbody>
        <tr v-for="k in KEY_KINDS" :key="k" :class="{ on: k === frame.selected }">
          <th scope="row">{{ KIND_LABEL[k] }}</th>
          <td>{{ frame.stats[k].pages }}</td>
          <td>{{ frame.stats[k].keys === 0 ? '—' : pct(frame.stats[k].fill) }}</td>
          <td :class="{ bad: frame.stats[k].splitsMiddle > 0 }">{{ frame.stats[k].splitsMiddle }}</td>
          <td :class="{ bad: frame.stats[k].fpi > FPI_ALERT }">{{ frame.checkpoints === 0 ? '—' : frame.stats[k].fpi }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.kl { color: var(--viz-text); }
.counters { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 140px), 1fr)); gap: 8px; margin: 0 0 10px; }
.counters div { min-width: 0; padding: 4px 10px; border: 2px solid var(--viz-edge); border-left-width: 8px; border-radius: 8px; }
.counters .bad { border-left-color: var(--viz-error); }
.counters .mid { border-left-color: var(--viz-active); }
dt { font-size: 12px; color: var(--viz-muted); }
dd { margin: 0; font-size: 18px; font-weight: 700; font-variant-numeric: tabular-nums; }
.chart { margin: 0 0 8px; }
figcaption { font-size: 14px; margin-bottom: 2px; }
svg { display: block; width: 100%; height: auto; }
.axis { stroke: var(--viz-muted); stroke-width: 1.5; }
.grid { stroke: var(--viz-edge); stroke-width: 1; }
.lbl { fill: var(--viz-muted); font-variant-numeric: tabular-nums; }
.bar { fill: var(--viz-db); }
.bar.hot { fill: var(--viz-active); }
.split { fill: var(--viz-text); }
.ff { stroke: var(--viz-service); stroke-width: 2; }
.legend { display: flex; flex-wrap: wrap; gap: 4px 14px; margin: 0; padding: 0; list-style: none; font-size: 13px; color: var(--viz-muted); }
.legend li { margin: 0; display: flex; align-items: center; gap: 6px; }
.legend svg { width: 28px; height: 8px; }
.sw { display: inline-block; width: 14px; height: 10px; border-radius: 2px; }
.bar-sw { background: var(--viz-db); }
.hot-sw { background: var(--viz-active); }
.split-sw { height: 4px; background: var(--viz-text); }
/* .kl перед селекторами: перебить стили .vp-doc table (display: block, поля 8×16 px, зебра) и custom.css (min-width: 7rem у ячеек на узком экране). */
.kl .cmp { display: table; width: 100%; margin: 0; border-collapse: collapse; font-size: 13px; font-variant-numeric: tabular-nums; }
.cmp caption { text-align: left; font-size: 14px; margin-bottom: 2px; }
.kl .cmp tr { background: transparent; border: 0; }
.kl .cmp th, .kl .cmp td { min-width: 0; padding: 3px; font-size: 13px; background: transparent; border-bottom: 1px solid var(--viz-edge); text-align: right; }
.cmp th:first-child, .cmp thead th:first-child { text-align: left; }
.cmp thead th { font-weight: 600; color: var(--viz-muted); }
/* Отступ под полосу выбора — у всех строк: иначе выбор UUIDv4/UUIDv7 расширял первый столбец,
   и на 375 px таблица вылезала за рамку на 4 px. */
.cmp th[scope='row'] { padding-left: 8px; }
.cmp tr.on th[scope='row'] { box-shadow: inset 4px 0 0 var(--viz-client); }
.cmp td.bad { color: var(--viz-error); font-weight: 700; }
</style>
