<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import VariantTabs from '../components/VariantTabs.vue'
import SegmentedToggle from '../components/SegmentedToggle.vue'
import ExplainPanel from '../components/ExplainPanel.vue'
import CaptionText from '../components/CaptionText.vue'
import BtreeView from '../renderers/BtreeView.vue'
import type { Step } from '../core/types'
import type { Explanation } from '../core/explain'
import { BT_SCENARIOS, btreeScenario, sqlCondition, type BtFrame, type BtScenarioMeta, type BtView } from '../simulations/btree'
import { BT_SIMPLIFICATIONS, explainBtree } from '../simulations/btree-explain'

const LABEL = 'B-tree изнутри: поиск и вставка по индексу members_user_id_idx'
const TABS = BT_SCENARIOS.map((s) => ({ id: s.id, label: s.label }))
const VIEWS: readonly { value: BtView; text: string }[] = [
  { value: 'simple', text: 'Схема' },
  { value: 'detail', text: 'Страницы' },
]

const meta = ref<BtScenarioMeta>(BT_SCENARIOS[0]!)
const steps = computed(() => btreeScenario(meta.value.id))
const sql = computed(() => sqlCondition(meta.value.sql))
// Вид по умолчанию задаёт сценарий: вставку удобнее смотреть по блокам, введение — схемой.
const view = ref<BtView>(meta.value.view)
// Клик закрепляет элемент до следующего шага; без клика панель объясняет то, о чём шаг.
const selected = ref<string | null>(null)
// У списка упрощений нет элемента на схеме, поэтому он открывается отдельным флагом.
const showSimplifications = ref(false)

function onScenario(id: string): void {
  const next = BT_SCENARIOS.find((s) => s.id === id)
  if (next === undefined) return
  meta.value = next
  view.value = next.view
}

// Player сообщает о шаге и при смене сценария: закреплённое прошлым шагом здесь уже не к месту.
function onStep(): void {
  selected.value = null
  showSimplifications.value = false
}

function onSelect(id: string): void {
  selected.value = id
  showSimplifications.value = false
}

const explanation = (step: Step<BtFrame>): Explanation | null =>
  showSimplifications.value ? BT_SIMPLIFICATIONS : explainBtree(selected.value ?? step.state.focus, step)
</script>

<template>
  <VizFrame :label="LABEL">
    <VariantTabs :options="TABS" :model-value="meta.id" label="Сценарий сцены B-tree" @update:model-value="onScenario" />
    <div class="bar">
      <SegmentedToggle v-model="view" label="Вид схемы" :options="VIEWS" />
      <button type="button" class="simple-btn" :aria-pressed="showSimplifications" @click="showSimplifications = !showSimplifications">
        Что в модели упрощено
      </button>
    </div>
    <pre class="sql" :aria-label="`SQL сценария «${meta.label}»`"><code>{{ sql.before }}<mark v-if="sql.cond" class="cond">{{ sql.cond }}</mark>{{ sql.after }}</code></pre>
    <p class="note"><CaptionText :caption="meta.note" /></p>
    <Player :steps="steps" :label="LABEL" @step="onStep">
      <template #default="{ step }">
        <p class="learn"><span class="muted">Сейчас изучаю:</span> {{ step.state.learn }}</p>
        <ul class="counters" aria-label="Счётчики страниц">
          <li>Страниц индекса прочитано: <b>{{ step.state.counters.index }}</b></li>
          <li>
            Обращений к таблице: <b>{{ step.state.counters.heapAccess }}</b> <span class="muted">(разных страниц: {{ step.state.counters.heapPages.length }} из {{ step.state.model.pages.length }})</span>
          </li>
          <li v-if="meta.id === 'ios'">Heap Fetches: <b>{{ step.state.counters.heapFetches }}</b></li>
        </ul>
        <BtreeView :frame="step.state" :view="view" :selected="selected" @select="onSelect" />
        <p class="honest">Данные и номера страниц — учебные (в лабе 2a в chat.members 2 млн строк). Реальные числа в шагах — из labs/2a/results/messenger-explain.txt.</p>
      </template>
      <template #side="{ step }">
        <ExplainPanel :explanation="explanation(step)" />
      </template>
    </Player>
  </VizFrame>
</template>

<style scoped>
.bar { display: flex; flex-wrap: wrap; align-items: flex-start; gap: 8px 16px; }
.simple-btn { min-height: 36px; padding: 0 12px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; cursor: pointer; }
.simple-btn[aria-pressed='true'] { border-color: var(--vp-c-brand-1); font-weight: 700; }
.simple-btn:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
.sql { margin: 0 0 8px; padding: 10px 12px; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--viz-text); font-family: var(--vp-font-family-mono); font-size: 13px; line-height: 1.5; white-space: pre-wrap; overflow-wrap: anywhere; }
.cond { padding: 1px 3px; border-radius: 4px; background: var(--viz-hit-bg); color: var(--viz-hit-text); font-weight: 600; }
.note { margin: 0 0 4px; font-size: 14px; line-height: 1.55; color: var(--viz-muted); white-space: pre-line; }
.learn { margin: 0 0 6px; font-size: 14px; }
.muted { color: var(--viz-muted); }
.counters { display: flex; flex-wrap: wrap; gap: 4px 20px; margin: 0 0 8px; padding: 0; list-style: none; font-size: 14px; }
.counters li { margin: 0; }
.honest { margin: 6px 0 0; color: var(--viz-muted); font-size: 12px; }
</style>
