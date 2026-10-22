<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import Player from '../components/Player.vue'
import VariantTabs from '../components/VariantTabs.vue'
import SegmentedToggle from '../components/SegmentedToggle.vue'
import ExplainPanel from '../components/ExplainPanel.vue'
import CaptionText from '../components/CaptionText.vue'
import BtreeView from '../renderers/BtreeView.vue'
import type { Step } from '../core/types'
import type { Explanation } from '../core/explain'
import { nextFocusIndex } from '../core/focus'
import { BT_SCENARIOS, btreeScenario, sqlCondition, type BtFrame, type BtScenarioMeta, type BtView } from '../simulations/btree'
import { BT_SIMPLIFICATIONS, explainBtree } from '../simulations/btree-explain'

const LABEL = 'B-tree изнутри: поиск и вставка по индексу members_user_id_idx'
const TABS = BT_SCENARIOS.map((s) => ({ id: s.id, label: s.label }))
const VIEWS: readonly { value: BtView; text: string }[] = [
  { value: 'simple', text: 'Схема' },
  { value: 'detail', text: 'Страницы' },
]
// Превью в колонке урока — первый кадр введения: схема целиком, без подписей шагов.
const previewFrame = btreeScenario('intro')[0]!.state

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

// --- полноэкранный режим ---------------------------------------------------------
// В колонке урока (≈ 600 px) подписи схемы шириной 1080 единиц выходят по 5–6 px, поэтому сцена
// работает только во весь экран: превью в тексте, по кнопке — оверлей на всё окно.
// Свой оверлей, а не VizFrame: тому нужен один и тот же контент в обоих режимах, а здесь в колонке — превью.
const open = ref(false)
const openBtn = ref<HTMLButtonElement>()
const dialog = ref<HTMLElement>()

const FOCUSABLE = 'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex="0"]'

function onDocKeydown(event: KeyboardEvent): void {
  const el = dialog.value
  if (el === undefined) return
  if (event.key === 'Escape') {
    event.preventDefault()
    void closeScene()
  } else if (event.key === 'Tab') {
    // Фокус не уходит под оверлей: Tab по кругу внутри диалога.
    const items = [...el.querySelectorAll<HTMLElement>(FOCUSABLE)].filter((n) => n.getClientRects().length > 0)
    const next = nextFocusIndex(items.indexOf(document.activeElement as HTMLElement), items.length, event.shiftKey)
    event.preventDefault()
    if (next >= 0) items[next]?.focus()
  }
}

function lockScroll(on: boolean): void {
  document.documentElement.style.overflow = on ? 'hidden' : ''
}

async function openScene(): Promise<void> {
  open.value = true
  lockScroll(true)
  document.addEventListener('keydown', onDocKeydown)
  await nextTick()
  // Плеер сразу ловит ← → и пробел.
  dialog.value?.querySelector<HTMLElement>('.viz-player')?.focus()
}

async function closeScene(): Promise<void> {
  open.value = false
  lockScroll(false)
  document.removeEventListener('keydown', onDocKeydown)
  await nextTick()
  openBtn.value?.focus()
}

onBeforeUnmount(() => {
  if (!open.value) return
  lockScroll(false)
  document.removeEventListener('keydown', onDocKeydown)
})
</script>

<template>
  <div class="bt-preview">
    <p class="kicker">Сцена по шагам · открывается во весь экран</p>
    <p class="lead">
      Как PostgreSQL выполняет <code>WHERE me.user_id = $1</code> по индексу <code>members_user_id_idx</code>: какие страницы индекса и таблицы читаются, в каком порядке и почему. Клик по любому элементу схемы объясняет его.
    </p>
    <!-- Миниатюра только для узнавания: её элементы не фокусируются (inert), клик по ней открывает сцену. -->
    <button type="button" class="thumb" tabindex="-1" aria-hidden="true" @click="openScene">
      <div inert><BtreeView :frame="previewFrame" view="simple" :selected="null" /></div>
    </button>
    <p class="tabs-list"><span class="muted">Сценарии:</span> {{ TABS.map((t) => t.label).join(' · ') }}</p>
    <button ref="openBtn" type="button" class="open-btn" @click="openScene">Открыть сцену</button>
  </div>

  <Teleport v-if="open" to="body">
    <div ref="dialog" class="bt-overlay" role="dialog" aria-modal="true" :aria-label="LABEL">
      <header class="bt-head">
        <h2 class="bt-title">B-tree изнутри</h2>
        <button type="button" class="close-btn" @click="closeScene">Закрыть (Esc)</button>
      </header>
      <VariantTabs :options="TABS" :model-value="meta.id" label="Сценарий сцены B-tree" @update:model-value="onScenario" />
      <div class="bar">
        <SegmentedToggle v-model="view" label="Вид схемы" :options="VIEWS" />
        <button type="button" class="simple-btn" :aria-pressed="showSimplifications" @click="showSimplifications = !showSimplifications">
          Что в модели упрощено
        </button>
      </div>
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
          <pre class="sql" :aria-label="`SQL сценария «${meta.label}»`"><code>{{ sql.before }}<mark v-if="sql.cond" class="cond">{{ sql.cond }}</mark>{{ sql.after }}</code></pre>
          <p class="note"><CaptionText :caption="meta.note" /></p>
          <ExplainPanel :explanation="explanation(step)" />
        </template>
      </Player>
    </div>
  </Teleport>
</template>

<style scoped>
.bt-preview { margin: 24px 0; padding: 16px; border: 1px solid var(--vp-c-divider); border-radius: 12px; background: var(--vp-c-bg-soft); }
.kicker { margin: 0; color: var(--viz-muted); font-size: 13px; font-weight: 600; }
.lead { margin: 6px 0 10px; line-height: 1.6; }
.thumb { display: block; width: 100%; padding: 8px; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); cursor: zoom-in; }
.thumb:hover { border-color: var(--vp-c-brand-1); }
.tabs-list { margin: 10px 0; font-size: 14px; line-height: 1.6; }
.open-btn { min-height: 44px; padding: 0 20px; border: 2px solid var(--vp-c-brand-1); border-radius: 8px; background: var(--vp-c-brand-1); color: var(--vp-c-white); font: inherit; font-weight: 700; cursor: pointer; }
.open-btn:hover { background: var(--vp-c-brand-2); border-color: var(--vp-c-brand-2); }
.open-btn:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 3px; }

/* Оверлей выше шапки и сайдбара VitePress; сам прокручивается, страница под ним — нет. */
.bt-overlay { position: fixed; inset: 0; z-index: 200; overflow: auto; padding: 10px 16px 16px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font-size: 15px; }
.bt-head { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 8px; }
.bt-title { margin: 0; border: 0; padding: 0; font-size: 20px; line-height: 1.3; }
.close-btn { min-height: 36px; padding: 0 14px; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg-soft); color: var(--vp-c-text-1); font: inherit; cursor: pointer; }
.close-btn:hover { border-color: var(--vp-c-brand-1); color: var(--vp-c-brand-1); }
.close-btn:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
.bt-overlay :deep(.variants) { margin-bottom: 8px; }
.bar { display: flex; flex-wrap: wrap; align-items: flex-start; gap: 8px 16px; }
.bar :deep(.panel) { margin-bottom: 0; }
.simple-btn { min-height: 36px; padding: 0 12px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; cursor: pointer; }
.simple-btn[aria-pressed='true'] { border-color: var(--vp-c-brand-1); font-weight: 700; }
.simple-btn:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
/* Холст забирает всё, что не нужно панели: 300 px справа, схема на 1280 px ≈ 880 px шириной. */
.bt-overlay :deep(.viz-player) { margin: 10px 0 0; padding: 12px; }
.bt-overlay :deep(.viz-stage.has-side) { grid-template-columns: minmax(0, 1fr) 300px; align-items: start; }
/* Длинное объяснение прокручивается в своей колонке и не уводит схему за край. */
.bt-overlay :deep(.viz-side) { position: sticky; top: 0; max-height: calc(100vh - 32px); overflow: auto; }
.sql { margin: 0 0 8px; padding: 8px 10px; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--viz-text); font-family: var(--vp-font-family-mono); font-size: 13px; line-height: 1.5; white-space: pre-wrap; overflow-wrap: anywhere; }
.cond { padding: 1px 3px; border-radius: 4px; background: var(--viz-hit-bg); color: var(--viz-hit-text); font-weight: 600; }
.note { margin: 0 0 8px; font-size: 14px; line-height: 1.55; color: var(--viz-muted); white-space: pre-line; }
.learn { margin: 0 0 4px; font-size: 15px; }
.muted { color: var(--viz-muted); }
.counters { display: flex; flex-wrap: wrap; gap: 4px 20px; margin: 0 0 6px; padding: 0; list-style: none; font-size: 15px; }
.counters li { margin: 0; }
.honest { margin: 6px 0 0; color: var(--viz-muted); font-size: 13px; }
</style>
