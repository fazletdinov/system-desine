<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { inlineCode, runDir, runLine, worktreeAdd, worktreeRemove } from './run'

// dir — каталог от корня репозитория; cmd — команда из этого каталога; needs — что нужно
// кроме Go (сейчас только Docker); before — подготовка, тоже из dir; tag — тег урока,
// код которого нужен команде: тогда dir берётся в рабочем дереве этого тега.
const props = defineProps<{ dir: string; cmd: string; needs?: 'docker'; before?: string; tag?: string }>()

// В compose.yaml мессенджера задано name: messenger на всех тегах: стенд из дерева тега —
// тот же проект compose, что и стенд основной копии, и пересоздал бы его контейнеры.
const MAIN_STAND_DOWN = 'docker compose -f messenger/compose.yaml down'

const line = computed(() => runLine(props.dir, props.cmd, props.tag))
const beforeDir = computed(() => runDir(props.dir, props.tag))
const beforeParts = computed(() => (props.before === undefined ? [] : inlineCode(props.before)))

type CopyState = 'idle' | 'done' | 'selected'
const copyState = ref<CopyState>('idle')
const COPY_LABEL: Record<CopyState, string> = { idle: 'копировать', done: 'скопировано', selected: 'выделено — Ctrl+C' }
const COPY_STATUS: Record<CopyState, string> = { idle: '', done: 'Команда скопирована', selected: 'Команда выделена, скопируйте её сочетанием Ctrl+C' }
// Сколько держится подпись «скопировано»: достаточно, чтобы заметить, и не мешает следующему клику.
const COPY_FEEDBACK_MS = 2000
let resetTimer: ReturnType<typeof setTimeout> | undefined
const cmdEl = ref<HTMLElement | null>(null)

function selectCommand(): void {
  const el = cmdEl.value
  const selection = window.getSelection()
  if (el === null || selection === null) return
  const range = document.createRange()
  range.selectNodeContents(el)
  selection.removeAllRanges()
  selection.addRange(range)
}

// Clipboard API есть только в безопасном контексте (https, localhost). Сайт, открытый
// с другой машины по http://<IP>, его не получит — тогда выделяем команду и пробуем
// старый execCommand; не вышло — выделение остаётся, и хватает Ctrl+C.
async function copy(): Promise<void> {
  try {
    if (navigator.clipboard === undefined) throw new Error('нет Clipboard API')
    await navigator.clipboard.writeText(line.value)
    copyState.value = 'done'
  } catch {
    selectCommand()
    copyState.value = document.execCommand('copy') ? 'done' : 'selected'
  }
  clearTimeout(resetTimer)
  resetTimer = setTimeout(() => (copyState.value = 'idle'), COPY_FEEDBACK_MS)
}

onBeforeUnmount(() => clearTimeout(resetTimer))
</script>

<template>
  <div class="run">
    <div class="run-row">
      <span class="run-label"><span aria-hidden="true">▶</span> Запустить самому <span class="run-from">(из корня репозитория)</span>:</span>
      <code ref="cmdEl" class="run-cmd">{{ line }}</code>
      <span v-if="needs === 'docker'" class="run-needs">нужен Docker</span>
      <button type="button" class="run-copy" aria-label="копировать команду" @click="copy">{{ COPY_LABEL[copyState] }}</button>
      <span role="status" class="run-sr-only">{{ COPY_STATUS[copyState] }}</span>
    </div>
    <div v-if="tag" class="run-before">
      код урока — тег <code>{{ tag }}</code> в отдельном рабочем дереве, курс остаётся на main (команды — из корня репозитория):
      <span class="run-hint">создать: <code>{{ worktreeAdd(tag) }}</code></span>
      <span v-if="needs === 'docker'" class="run-hint">стенд основной копии сначала остановите: <code>{{ MAIN_STAND_DOWN }}</code></span>
      <span class="run-hint">убрать: <code>{{ worktreeRemove(tag) }}</code></span>
    </div>
    <p v-if="before" class="run-before">
      сначала, из <code>{{ beforeDir }}</code>:
      <template v-for="(part, i) in beforeParts" :key="i">
        <code v-if="part.code">{{ part.text }}</code>
        <template v-else>{{ part.text }}</template>
      </template>
    </p>
  </div>
</template>

<style scoped>
.run {
  margin: 8px 0 16px;
  padding: 8px 12px;
  border: 1px solid var(--vp-c-divider);
  border-left: 3px solid var(--vp-c-brand-1);
  border-radius: 8px;
  background: var(--vp-c-bg-soft);
  font-size: 14px;
  line-height: 1.6;
}
.run-row {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 4px 8px;
}
.run-label {
  color: var(--vp-c-brand-1);
  font-weight: 600;
}
.run-from {
  color: var(--vp-c-text-2);
  font-weight: 400;
}
/* Длинная команда переносится внутри строки блока, а не растягивает страницу. */
.run-cmd {
  flex: 1 1 18rem;
  min-width: 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
/* Бейдж без разделителя-точки: при переносе на узком экране строка не начинается с «·». */
.run-needs {
  padding: 0 6px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 4px;
  color: var(--vp-c-text-2);
  font-size: 12px;
  white-space: nowrap;
}
.run-copy {
  margin-left: auto;
  padding: 2px 10px;
  border: 1px solid var(--vp-c-brand-1);
  border-radius: 6px;
  color: var(--vp-c-brand-1);
  font-size: 13px;
  white-space: nowrap;
}
.run-copy:hover { background: var(--vp-c-brand-soft); }
.run-copy:focus-visible { outline: 2px solid var(--vp-c-brand-1); outline-offset: 2px; }
.run-sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
  white-space: nowrap;
}
/* Подсказка — своей строкой; её команда не рвётся (иначе «--force» читается как «--» и
   «force»): целиком уходит на следующую строку, а слишком длинная прокручивается внутри. */
.run-hint {
  display: block;
}
.run-hint code {
  display: inline-block;
  max-width: 100%;
  overflow-x: auto;
  vertical-align: bottom;
  white-space: nowrap;
  scrollbar-width: thin;
}
.run-before {
  margin: 4px 0 0;
  color: var(--vp-c-text-2);
  overflow-wrap: anywhere;
}
</style>
