<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { enterFullscreen, exitFullscreen, type FullscreenApi, type FullscreenMode } from '../core/fullscreen'

const props = defineProps<{ label: string }>()
const frame = ref<HTMLElement>()
const mode = ref<FullscreenMode>('off')
const rotateHint = ref(false)

function browserApi(): FullscreenApi {
  return {
    enabled: document.fullscreenEnabled === true,
    request: (el) => el.requestFullscreen(),
    exit: () => document.exitFullscreen(),
  }
}

async function toggle(): Promise<void> {
  const el = frame.value
  if (el === undefined) return
  if (mode.value !== 'off') {
    mode.value = await exitFullscreen(mode.value, browserApi())
    return
  }
  mode.value = await enterFullscreen(el, browserApi())
  rotateHint.value = window.matchMedia('(max-width: 640px) and (orientation: portrait)').matches
  await nextTick()
  focusContent()
}

// Клавиши визуализации должны работать сразу после входа.
function focusContent(): void {
  const el = frame.value
  if (el === undefined) return
  // Собственные кнопки рамки пропускаем: иначе фокус всегда останется на «Выйти».
  const target = [...el.querySelectorAll<HTMLElement>('[tabindex="0"], button, input, select')].find((n) => n.closest('.viz-frame-bar, .viz-rotate') === null)
  target?.focus()
}

function onFullscreenChange(): void {
  if (mode.value === 'native' && document.fullscreenElement !== frame.value) mode.value = 'off'
  // Браузер сам переносит фокус при входе в полноэкранный режим — возвращаем его на визуализацию.
  else if (document.fullscreenElement === frame.value) focusContent()
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape' && mode.value === 'overlay') mode.value = 'off'
}

watch(mode, (m) => document.documentElement.classList.toggle('viz-scroll-lock', m === 'overlay'))

onMounted(() => {
  document.addEventListener('fullscreenchange', onFullscreenChange)
  document.addEventListener('keydown', onKeydown)
})
onBeforeUnmount(() => {
  document.removeEventListener('fullscreenchange', onFullscreenChange)
  document.removeEventListener('keydown', onKeydown)
  // Класс общий для всех рамок: снимаем его, только если оверлей открыт именно у этой.
  if (mode.value === 'overlay') document.documentElement.classList.remove('viz-scroll-lock')
})
</script>

<template>
  <div
    ref="frame"
    class="viz-frame"
    :class="{ 'is-fullscreen': mode !== 'off', 'is-overlay': mode === 'overlay' }"
    :role="mode === 'overlay' ? 'dialog' : undefined"
    :aria-modal="mode === 'overlay' ? 'true' : undefined"
    :aria-label="props.label"
  >
    <div class="viz-frame-bar">
      <button type="button" class="viz-fs-btn" @click="toggle">
        <span aria-hidden="true">{{ mode === 'off' ? '⛶' : '✕' }}</span>
        {{ mode === 'off' ? 'На весь экран' : 'Выйти (Esc)' }}
      </button>
    </div>
    <p v-if="mode !== 'off' && rotateHint" class="viz-rotate">
      Поверните телефон горизонтально — так визуализация видна лучше.
      <button type="button" aria-label="Скрыть подсказку" @click="rotateHint = false">✕</button>
    </p>
    <slot />
  </div>
</template>

<style scoped>
.viz-frame { position: relative; }
.viz-frame-bar { display: flex; justify-content: flex-end; margin-bottom: -8px; }
.viz-fs-btn { padding: 4px 10px; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg-soft); color: var(--vp-c-text-2); font-size: 13px; }
.viz-fs-btn:hover { color: var(--vp-c-brand-1); border-color: var(--vp-c-brand-1); }
.viz-fs-btn:focus-visible { outline: 2px solid var(--vp-c-brand-1); outline-offset: 2px; }
.is-fullscreen { display: flex; flex-direction: column; gap: 8px; padding: 16px; background: var(--vp-c-bg); overflow: auto; }
.is-overlay { position: fixed; inset: 0; z-index: 100; }
.is-fullscreen .viz-frame-bar { margin-bottom: 0; }
.is-fullscreen :deep(.viz-player), .is-fullscreen :deep(.latency), .is-fullscreen :deep(.calc), .is-fullscreen :deep(.mermaid-diagram) { margin: 0; }
.is-fullscreen :deep(svg) { max-height: calc(100vh - 240px); }
/* !important: Mermaid пишет на svg инлайновый max-width равный натуральной ширине схемы, иначе её не растянуть на весь экран. */
.is-fullscreen :deep(.mermaid-host) { flex: 1; align-items: center; }
.is-fullscreen :deep(.mermaid-host svg) { width: 100%; max-width: none !important; height: auto; max-height: calc(100vh - 140px); }
.is-fullscreen :deep(.viz-caption) { font-size: 1.15rem; }
.viz-rotate { display: flex; gap: 8px; align-items: center; margin: 0; padding: 8px 12px; border-radius: 8px; background: var(--vp-c-brand-soft); font-size: 14px; }
:global(html.viz-scroll-lock) { overflow: hidden; }
</style>
