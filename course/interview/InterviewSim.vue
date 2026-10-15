<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { withBase } from 'vitepress'
import VizFrame from '../viz/components/VizFrame.vue'
import VariantTabs from '../viz/components/VariantTabs.vue'
import type { RenderedBank, RenderedQuestion } from './interview.data'
import { pickSession, scoreSession, type Score, type Session } from './session'
import { appendHistory, historyKey, lessonLabel, parseHistory, parseSeed, randomSeed, type HistoryEntry } from './history'

const props = defineProps<{ bank: RenderedBank }>()

// Восемь вопросов по три минуты — примерно техническая часть настоящего собеседования.
const SESSION_SIZE = 8
const QUESTION_SECONDS = 180
const LAST_MINUTE = 60
const TICK_MS = 250

type Mode = 'session' | 'all'
type Phase = 'idle' | 'question' | 'summary'

const MODES = [
  { id: 'session', label: 'Сессия' },
  { id: 'all', label: 'Все вопросы' },
] as const

const moduleLabel = computed(() => String(Number(props.bank.module)))
const byId = computed(() => new Map(props.bank.questions.map((q) => [q.id, q])))
const lessonHref = (lesson: string): string => withBase(`/modules/${lesson}`)

const mode = ref<Mode>('session')
function setMode(id: string): void {
  if (id === 'session' || id === 'all') mode.value = id
}

// ---- Сессия ----

// Сид появляется только в браузере: случайное значение при серверной сборке
// не совпало бы с клиентским, и Vue ругался бы на расхождение разметки.
const seedText = ref('')
const seed = computed(() => parseSeed(seedText.value))
const phase = ref<Phase>('idle')
const session = ref<Session>()
const index = ref(0)
const revealed = ref(false)
const draft = ref('')
const marks = ref<Record<string, boolean[]>>({})
const score = ref<Score>()

const current = computed<RenderedQuestion | undefined>(() => {
  const id = session.value?.ids[index.value]
  return id === undefined ? undefined : byId.value.get(id)
})
const total = computed(() => session.value?.ids.length ?? 0)
const isLast = computed(() => index.value + 1 >= total.value)
const ratioPercent = (got: number, all: number): number => (all === 0 ? 0 : Math.round((got / all) * 100))

const questionHeading = ref<HTMLElement>()
const referenceHeading = ref<HTMLElement>()
const summaryHeading = ref<HTMLElement>()
const seedInput = ref<HTMLInputElement>()

// Фокус переносится вслед за содержимым: кнопка, которую нажали, исчезает,
// и без этого клавиатурный пользователь оказался бы в начале страницы.
function focusLater(el: () => HTMLElement | undefined): void {
  void nextTick(() => el()?.focus())
}

function start(withSeed: number): void {
  seedText.value = String(withSeed)
  session.value = pickSession(props.bank.questions, SESSION_SIZE, withSeed)
  marks.value = Object.fromEntries(session.value.ids.map((id) => [id, (byId.value.get(id)?.checklist ?? []).map(() => false)]))
  score.value = undefined
  phase.value = 'question'
  openQuestion(0)
}

function mark(id: string, i: number, on: boolean): void {
  const list = marks.value[id]
  if (list !== undefined && i < list.length) list[i] = on
}

function startFromField(): void {
  if (seed.value !== undefined) start(seed.value)
}

function openQuestion(i: number): void {
  index.value = i
  revealed.value = false
  draft.value = ''
  startTimer()
  focusLater(() => questionHeading.value)
}

function reveal(): void {
  revealed.value = true
  focusLater(() => referenceHeading.value)
}

function next(): void {
  if (!isLast.value) {
    openQuestion(index.value + 1)
    return
  }
  finish()
}

function finish(): void {
  stopTimer()
  const s = session.value
  if (s === undefined) return
  score.value = scoreSession(props.bank.questions, s, marks.value)
  phase.value = 'summary'
  saveHistory(s.seed, score.value)
  focusLater(() => summaryHeading.value)
}

function newSession(): void {
  stopTimer()
  seedText.value = String(randomSeed())
  phase.value = 'idle'
  session.value = undefined
  focusLater(() => seedInput.value)
}

// ---- Таймер ----

// Отсчёт по часам, а не по числу тиков: фоновая вкладка тикает реже, и счётчик тиков отстал бы.
const deadline = ref(0)
const remaining = ref(QUESTION_SECONDS)
const announcement = ref('')
let timer: ReturnType<typeof setInterval> | undefined

function stopTimer(): void {
  if (timer !== undefined) clearInterval(timer)
  timer = undefined
}

function tick(): void {
  const left = Math.max(0, Math.ceil((deadline.value - Date.now()) / 1000))
  if (left === remaining.value) return
  // Экранный диктор слышит только две вехи, а не каждую секунду.
  if (left <= LAST_MINUTE && remaining.value > LAST_MINUTE) announcement.value = 'Осталась минута'
  if (left === 0) {
    announcement.value = 'Время вышло'
    stopTimer()
  }
  remaining.value = left
}

function startTimer(): void {
  stopTimer()
  deadline.value = Date.now() + QUESTION_SECONDS * 1000
  remaining.value = QUESTION_SECONDS
  announcement.value = ''
  timer = setInterval(tick, TICK_MS)
}

const clock = computed(() => {
  const m = Math.floor(remaining.value / 60)
  const s = remaining.value % 60
  return `${m}:${String(s).padStart(2, '0')}`
})

// ---- История ----

// undefined — хранилище недоступно (приватный режим, запрет cookie): блок не показываем.
const history = ref<HistoryEntry[]>()

function readHistory(): HistoryEntry[] | undefined {
  try {
    return parseHistory(window.localStorage.getItem(historyKey(props.bank.module)))
  } catch {
    return undefined
  }
}

function saveHistory(s: number, sc: Score): void {
  const entry: HistoryEntry = {
    at: new Date().toISOString(),
    seed: s,
    ratio: sc.total.total === 0 ? 0 : sc.total.got / sc.total.total,
    weak: sc.weak.map((t) => t.topic),
  }
  const list = appendHistory(history.value ?? [], entry)
  try {
    window.localStorage.setItem(historyKey(props.bank.module), JSON.stringify(list))
    history.value = list
  } catch {
    // Квота или запрет хранилища: итог уже на экране, теряется только история.
  }
}

const dateFormat = new Intl.DateTimeFormat('ru-RU', { dateStyle: 'short', timeStyle: 'short' })
const formatDate = (iso: string): string => {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : dateFormat.format(d)
}

onMounted(() => {
  seedText.value = String(randomSeed())
  history.value = readHistory()
})
onBeforeUnmount(stopTimer)

// ---- Все вопросы ----

const lessonFilter = ref('')
const levelFilter = ref('')
const lessons = computed(() => [...new Set(props.bank.questions.map((q) => q.lesson))].sort())
const filtered = computed(() =>
  props.bank.questions.filter(
    (q) => (lessonFilter.value === '' || q.lesson === lessonFilter.value) && (levelFilter.value === '' || q.level === levelFilter.value),
  ),
)
const levelTitle = (level: string): string => (level === 'S' ? 'Senior' : 'Middle+')
</script>

<template>
  <VizFrame :label="`Симулятор собеседования: модуль ${moduleLabel}`">
    <div class="sim">
      <VariantTabs :options="MODES" :model-value="mode" label="Режим симулятора" @update:model-value="setMode" />

      <!-- ===== Сессия ===== -->
      <section v-if="mode === 'session'" class="session" aria-label="Сессия">
        <div v-if="phase === 'idle'" class="intro">
          <p>
            {{ SESSION_SIZE }} вопросов из {{ bank.questions.length }}, темы не повторяются подряд.
            На каждый — {{ QUESTION_SECONDS / 60 }} минуты: ответьте вслух или письменно, затем откройте эталон и честно отметьте, что вы сказали.
          </p>
          <div class="seed-row">
            <label for="interview-seed">Сид сессии</label>
            <input
              id="interview-seed"
              ref="seedInput"
              v-model="seedText"
              class="seed-input"
              inputmode="numeric"
              autocomplete="off"
              :aria-invalid="seedText !== '' && seed === undefined"
              aria-describedby="interview-seed-hint"
              @keydown.enter.prevent="startFromField"
            />
            <button type="button" class="btn" @click="seedText = String(randomSeed())">Новый сид</button>
          </div>
          <p id="interview-seed-hint" class="hint" :class="{ bad: seedText !== '' && seed === undefined }">
            {{ seedText !== '' && seed === undefined ? 'Сид — целое число от 0 до 4294967295.' : 'Тот же сид — та же сессия: так можно повторить её позже или дать другу.' }}
          </p>
          <button type="button" class="btn primary" :disabled="seed === undefined" @click="startFromField">Начать</button>

          <div v-if="history !== undefined && history.length > 0" class="history">
            <h4>Прошлые результаты</h4>
            <ul>
              <li v-for="h in history" :key="h.at + h.seed">
                <span class="mono">{{ formatDate(h.at) }}</span> · сид <span class="mono">{{ h.seed }}</span> ·
                <strong>{{ Math.round(h.ratio * 100) }}%</strong>
                <template v-if="h.weak.length > 0"> · слабые: {{ h.weak.join(', ') }}</template>
                <button type="button" class="link-btn" :aria-label="`Повторить сессию с сидом ${h.seed}`" @click="start(h.seed)">повторить</button>
              </li>
            </ul>
          </div>
        </div>

        <div v-else-if="phase === 'question' && current !== undefined" class="question">
          <div class="meta">
            <span class="counter mono">{{ index + 1 }} / {{ total }}</span>
            <span class="tag">{{ current.topic }}</span>
            <span class="tag level" :class="current.level === 'S' ? 'level-s' : 'level-m'">{{ levelTitle(current.level) }}</span>
            <span class="clock mono" :class="{ over: remaining === 0, last: remaining > 0 && remaining <= LAST_MINUTE }">
              <span aria-hidden="true">⏱</span> {{ clock }}<template v-if="remaining === 0"> — время вышло</template>
            </span>
            <span class="sr-only" aria-live="polite">{{ announcement }}</span>
          </div>
          <h4 ref="questionHeading" class="q-text" tabindex="-1" v-html="current.questionHtml" />
          <label class="draft-label" for="interview-draft">Черновик ответа (не сохраняется)</label>
          <textarea id="interview-draft" v-model="draft" class="draft" rows="5" placeholder="Тезис → почему → компромисс → пример" />
          <button v-if="!revealed" type="button" class="btn primary" @click="reveal">Показать эталон</button>

          <div v-else class="reference">
            <h4 ref="referenceHeading" tabindex="-1">Эталон</h4>
            <div class="answer" v-html="current.answerHtml" />

            <fieldset class="block checklist">
              <legend>Что должен услышать интервьюер — отметьте, что вы сказали</legend>
              <div v-for="(item, i) in current.checklistHtml" :key="i" class="check">
                <input
                  :id="`check-${current.id}-${i}`"
                  type="checkbox"
                  :checked="marks[current.id]?.[i] ?? false"
                  @change="mark(current.id, i, ($event.target as HTMLInputElement).checked)"
                />
                <label :for="`check-${current.id}-${i}`" v-html="item" />
              </div>
            </fieldset>

            <div class="block flags">
              <h5>Красные флаги</h5>
              <ul>
                <li v-for="(f, i) in current.redFlagsHtml" :key="i" v-html="f" />
              </ul>
            </div>

            <div class="block follow">
              <h5>Углубления</h5>
              <details v-for="(f, i) in current.followUpsHtml" :key="i">
                <summary v-html="f.qHtml" />
                <p v-html="f.aHtml" />
              </details>
            </div>

            <p class="lesson-link">
              Повторить тему: <a :href="lessonHref(current.lesson)">урок {{ lessonLabel(current.lesson) }}</a>
            </p>
            <button type="button" class="btn primary" @click="next">{{ isLast ? 'Завершить сессию' : 'Следующий вопрос' }}</button>
          </div>
        </div>

        <div v-else-if="phase === 'summary' && score !== undefined" class="summary">
          <h4 ref="summaryHeading" tabindex="-1">Итог сессии</h4>
          <p class="total">
            Отмечено <strong>{{ score.total.got }}</strong> из <strong>{{ score.total.total }}</strong> пунктов —
            <strong>{{ ratioPercent(score.total.got, score.total.total) }}%</strong>. Сид: <span class="mono">{{ session?.seed }}</span>.
          </p>
          <table class="topics">
            <thead>
              <tr><th scope="col">Тема</th><th scope="col">Пункты</th><th scope="col">Доля</th></tr>
            </thead>
            <tbody>
              <tr v-for="t in score.topics" :key="t.topic">
                <th scope="row">{{ t.topic }}</th>
                <td class="mono">{{ t.got }} / {{ t.total }}</td>
                <td>
                  <span class="bar" aria-hidden="true">
                    <span class="fill" :class="{ weak: score.weak.includes(t) }" :style="{ width: `${ratioPercent(t.got, t.total)}%` }" />
                  </span>
                  <span class="mono">{{ ratioPercent(t.got, t.total) }}%</span>
                </td>
              </tr>
            </tbody>
          </table>
          <div v-if="score.weak.length > 0" class="block weak">
            <h5>Слабые темы — повторите уроки</h5>
            <ul>
              <li v-for="t in score.weak" :key="t.topic">
                {{ t.topic }}:
                <template v-for="(l, i) in t.lessons" :key="l">
                  <template v-if="i > 0">, </template><a :href="lessonHref(l)">урок {{ lessonLabel(l) }}</a>
                </template>
              </li>
            </ul>
          </div>
          <p v-else class="block strong">Слабых тем нет: по каждой теме отмечено не меньше 60 % пунктов.</p>
          <div class="actions">
            <button type="button" class="btn primary" @click="newSession">Ещё сессия</button>
            <button v-if="session !== undefined" type="button" class="btn" @click="start(session.seed)">Повторить по сиду {{ session.seed }}</button>
          </div>
        </div>
      </section>

      <!-- ===== Все вопросы ===== -->
      <section v-else class="all" aria-label="Все вопросы">
        <div class="filters">
          <label>
            Урок
            <select v-model="lessonFilter">
              <option value="">все</option>
              <option v-for="l in lessons" :key="l" :value="l">{{ lessonLabel(l) }}</option>
            </select>
          </label>
          <label>
            Уровень
            <select v-model="levelFilter">
              <option value="">все</option>
              <option value="M+">Middle+</option>
              <option value="S">Senior</option>
            </select>
          </label>
          <span class="found" aria-live="polite">Вопросов: {{ filtered.length }}</span>
        </div>
        <details v-for="q in filtered" :key="q.id" class="item">
          <summary>
            <span class="tags">
              <span class="tag">урок {{ lessonLabel(q.lesson) }}</span>
              <span class="tag">{{ q.topic }}</span>
              <span class="tag level" :class="q.level === 'S' ? 'level-s' : 'level-m'">{{ levelTitle(q.level) }}</span>
            </span>
            <span class="item-q" v-html="q.questionHtml" />
          </summary>
          <div class="item-body">
            <div class="answer" v-html="q.answerHtml" />
            <div class="block checklist">
              <h5>Что должен услышать интервьюер</h5>
              <ul>
                <li v-for="(c, i) in q.checklistHtml" :key="i" v-html="c" />
              </ul>
            </div>
            <div class="block flags">
              <h5>Красные флаги</h5>
              <ul>
                <li v-for="(f, i) in q.redFlagsHtml" :key="i" v-html="f" />
              </ul>
            </div>
            <div class="block follow">
              <h5>Углубления</h5>
              <details v-for="(f, i) in q.followUpsHtml" :key="i">
                <summary v-html="f.qHtml" />
                <p v-html="f.aHtml" />
              </details>
            </div>
            <p class="lesson-link"><a :href="lessonHref(q.lesson)">Урок {{ lessonLabel(q.lesson) }}</a></p>
          </div>
        </details>
      </section>
    </div>
  </VizFrame>
</template>

<style scoped>
.sim { margin: 16px 0; padding: 16px; border: 1px solid var(--vp-c-divider); border-radius: 12px; background: var(--vp-c-bg-soft); color: var(--vp-c-text-1); }
.sim p { margin: 8px 0; }
.mono { font-family: var(--vp-font-family-mono); font-variant-numeric: tabular-nums; }
.sr-only { position: absolute; width: 1px; height: 1px; margin: -1px; padding: 0; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; border: 0; }

.btn { min-height: 40px; padding: 0 16px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; cursor: pointer; }
.btn:hover:not(:disabled) { border-color: var(--vp-c-brand-1); }
.btn.primary { border-color: var(--vp-c-brand-1); background: var(--vp-c-brand-1); color: var(--vp-button-brand-text, #fff); font-weight: 600; }
.btn:disabled { opacity: 0.55; cursor: not-allowed; }
.btn:focus-visible, .link-btn:focus-visible, .seed-input:focus-visible, .draft:focus-visible, select:focus-visible, summary:focus-visible, input[type='checkbox']:focus-visible, a:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
[tabindex='-1']:focus { outline: none; }
[tabindex='-1']:focus-visible { outline: 2px dashed var(--vp-c-brand-1); outline-offset: 4px; }
.link-btn { margin-left: 6px; padding: 0; border: 0; background: none; color: var(--vp-c-brand-1); font: inherit; text-decoration: underline; cursor: pointer; }

.seed-row { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-top: 12px; }
.seed-input { width: 14ch; min-height: 40px; padding: 0 10px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font-family: var(--vp-font-family-mono); }
.seed-input[aria-invalid='true'] { border-color: var(--viz-error); }
.hint { font-size: 14px; color: var(--vp-c-text-2); }
/* Текст ошибки — основным цветом: красный на светлом фоне рамки не дотягивает до 4,5:1. */
.hint.bad { padding-left: 8px; border-left: 4px solid var(--viz-error); color: var(--vp-c-text-1); }

.history { margin-top: 16px; }
.history h4, .summary h4 { margin: 0 0 8px; font-size: 16px; }
.history ul { margin: 0; padding-left: 20px; font-size: 14px; }

.meta { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.counter { font-weight: 700; }
.tag { display: inline-block; padding: 2px 8px; border: 1px solid var(--vp-c-divider); border-radius: 999px; background: var(--vp-c-bg); font-size: 13px; }
/* Уровень различается и цветом рамки, и текстом: цвет не единственный носитель смысла. */
.level-m { border-color: var(--viz-client); }
.level-s { border-color: var(--viz-service); font-weight: 600; }
.clock { margin-left: auto; padding: 2px 10px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); }
.clock.last { border-color: var(--viz-active); }
.clock.over { border-color: var(--viz-error); color: var(--viz-error); font-weight: 700; }
.q-text { margin: 12px 0; font-size: 17px; line-height: 1.5; }
.draft-label { display: block; margin-bottom: 4px; font-size: 14px; color: var(--vp-c-text-2); }
.draft { box-sizing: border-box; width: 100%; margin-bottom: 12px; padding: 8px 10px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; resize: vertical; }

.reference h4 { margin: 8px 0; font-size: 16px; }
.answer { padding: 12px 14px; border-radius: 8px; background: var(--vp-c-bg); }
.answer :deep(p) { margin: 0 0 10px; }
.answer :deep(p:last-child) { margin-bottom: 0; }
.block { margin: 12px 0; padding: 10px 14px; border: 1px solid var(--vp-c-divider); border-left-width: 4px; border-radius: 8px; background: var(--vp-c-bg); }
.block h5, .block legend { margin: 0 0 6px; padding: 0; font-size: 15px; font-weight: 700; }
.block ul { margin: 0; padding-left: 20px; }
/* Цвет полосы — смысл блока: зелёный — что сказать, красный — чего не говорить, фиолетовый — куда копнёт интервьюер. */
.checklist { border-left-color: var(--viz-response); }
.flags { border-left-color: var(--viz-error); }
.follow { border-left-color: var(--viz-service); }
.weak { border-left-color: var(--viz-error); }
.strong { border-left-color: var(--viz-response); }
fieldset.block { min-width: 0; }
/* Легенда по умолчанию садится на рамку fieldset; внутри блока она читается как обычный заголовок. */
fieldset.block > legend { float: left; width: 100%; }
fieldset.block > legend + * { clear: both; }
.check { display: flex; gap: 8px; align-items: flex-start; padding: 4px 0; }
.check input { flex: none; width: 18px; height: 18px; margin-top: 3px; accent-color: var(--viz-response); }
.follow details { margin: 6px 0; }
.follow summary { cursor: pointer; font-weight: 600; }
.follow details p { margin: 6px 0 0 18px; }
.lesson-link { font-size: 14px; }

.total { font-size: 16px; }
.topics { display: table; width: 100%; margin: 12px 0; border-collapse: collapse; font-size: 14px; }
.topics th, .topics td { min-width: 0 !important; padding: 6px 8px; border: 1px solid var(--vp-c-divider); text-align: left; background: var(--vp-c-bg); color: var(--vp-c-text-1); }
.topics tbody th { font-weight: 500; }
.bar { display: inline-block; width: 64px; height: 8px; margin-right: 6px; border-radius: 4px; background: var(--vp-c-divider); vertical-align: middle; overflow: hidden; }
.fill { display: block; height: 100%; background: var(--viz-response); }
.fill.weak { background: var(--viz-error); }
.actions { display: flex; flex-wrap: wrap; gap: 8px; }

.filters { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; margin-bottom: 12px; font-size: 14px; }
.filters select { min-height: 36px; margin-left: 6px; padding: 0 8px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; }
.found { color: var(--vp-c-text-2); }
.item { margin: 8px 0; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); }
.item > summary { display: block; padding: 10px 14px; cursor: pointer; list-style: none; }
.item > summary::-webkit-details-marker { display: none; }
.item > summary::before { content: '▸ '; color: var(--vp-c-brand-1); }
.item[open] > summary::before { content: '▾ '; }
.tags { display: inline-flex; flex-wrap: wrap; gap: 6px; margin-bottom: 4px; }
.item-q { display: block; font-weight: 600; }
.item-body { padding: 0 14px 12px; }
.item-body .answer, .item-body .block { background: var(--vp-c-bg-soft); }

:deep(code) { font-size: 0.9em; }

@media (max-width: 480px) {
  .sim { padding: 12px; }
  .clock { margin-left: 0; }
  .bar { width: 40px; }
}
</style>
