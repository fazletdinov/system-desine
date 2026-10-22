<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { inlineCode, runLine } from './run'

// dir — каталог от корня репозитория; cmd — команда из этого каталога; needs — что нужно
// кроме Go (сейчас только Docker); before — подготовка: старт стенда, сборка, правка кода.
const props = defineProps<{ dir: string; cmd: string; needs?: 'docker'; before?: string }>()

const line = computed(() => runLine(props.dir, props.cmd))
const beforeParts = computed(() => (props.before === undefined ? [] : inlineCode(props.before)))

type CopyState = 'idle' | 'done' | 'failed'
const copyState = ref<CopyState>('idle')
const COPY_LABEL: Record<CopyState, string> = { idle: 'копировать', done: 'скопировано', failed: 'не вышло' }
// Сколько держится подпись «скопировано»: достаточно, чтобы заметить, и не мешает следующему клику.
const COPY_FEEDBACK_MS = 2000
let resetTimer: ReturnType<typeof setTimeout> | undefined

async function copy(): Promise<void> {
  try {
    await navigator.clipboard.writeText(line.value)
    copyState.value = 'done'
  } catch {
    // Буфер обмена недоступен (не https, запрет браузера) — команда остаётся на экране, её можно выделить.
    copyState.value = 'failed'
  }
  clearTimeout(resetTimer)
  resetTimer = setTimeout(() => (copyState.value = 'idle'), COPY_FEEDBACK_MS)
}

onBeforeUnmount(() => clearTimeout(resetTimer))
</script>

<template>
  <div class="run">
    <div class="run-row">
      <span class="run-label"><span aria-hidden="true">▶</span> Запустить самому:</span>
      <code class="run-cmd">{{ line }}</code>
      <span v-if="needs === 'docker'" class="run-needs">· нужен Docker</span>
      <button type="button" class="run-copy" @click="copy">
        <span aria-live="polite">{{ COPY_LABEL[copyState] }}</span>
      </button>
    </div>
    <p v-if="before" class="run-before">
      сначала:
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
  white-space: nowrap;
}
/* Длинная команда переносится внутри строки блока, а не растягивает страницу. */
.run-cmd {
  flex: 1 1 18rem;
  min-width: 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.run-needs {
  color: var(--vp-c-text-2);
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
.run-before {
  margin: 4px 0 0;
  color: var(--vp-c-text-2);
  overflow-wrap: anywhere;
}
</style>
