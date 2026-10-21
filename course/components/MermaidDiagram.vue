<script setup lang="ts">
import { onMounted, ref, useId, watch } from 'vue'
import { useData } from 'vitepress'
import VizFrame from '../viz/components/VizFrame.vue'
import { mermaidThemeVariables, withPaletteClasses } from '../viz/core/mermaid-palette'

const props = defineProps<{ code: string; title?: string }>()
const { isDark } = useData()
const host = ref<HTMLElement | null>(null)
const error = ref<string | null>(null)
const baseId = `mmd-${useId().replace(/[^a-zA-Z0-9-]/g, '')}`
// Каждый рендер получает свой id: при смене темы на странице не появятся два SVG с одинаковым id.
let renderSeq = 0
// Нижняя граница масштаба схемы. Без неё широкая схема (ER-диаграмма урока 2.1 — 2000 px)
// ужимается до ширины колонки: на 375 px это 15 % и текст в 2 px. При 0,6 шрифт mermaid
// (14–16 px) остаётся не меньше 8–10 px, а то, что не влезло, прокручивается внутри рамки,
// а не всей страницей.
const MIN_SCALE = 0.6

async function render(): Promise<void> {
  const seq = ++renderSeq
  error.value = null
  const theme = isDark.value ? 'dark' : 'light'
  const id = `${baseId}-${seq}`
  try {
    const { default: mermaid } = await import('mermaid')
    mermaid.initialize({ startOnLoad: false, theme: 'base', themeVariables: mermaidThemeVariables(theme), securityLevel: 'strict' })
    const { svg } = await mermaid.render(id, withPaletteClasses(decodeURIComponent(props.code), theme))
    if (seq !== renderSeq || !host.value) return
    host.value.innerHTML = svg
    const el = host.value.querySelector('svg')
    const width = el?.viewBox.baseVal?.width ?? 0
    // min-width сильнее max-width: 100 %, поэтому схема шире колонки уходит в прокрутку.
    if (el && width > 0) el.style.minWidth = `${Math.floor(width * MIN_SCALE)}px`
  } catch (e) {
    document.getElementById(id)?.remove()
    document.getElementById(`d${id}`)?.remove()
    if (seq !== renderSeq) return
    error.value = e instanceof Error ? e.message : String(e)
  }
}

onMounted(render)
watch([isDark, () => props.code], render)
</script>

<template>
  <VizFrame :label="props.title ?? 'Схема'">
    <div class="mermaid-diagram">
      <div ref="host" class="mermaid-host" />
      <pre v-if="error" class="mermaid-error">Ошибка схемы: {{ error }}</pre>
    </div>
  </VizFrame>
</template>

<style scoped>
.mermaid-diagram { margin: 20px 0; }
/* Место под схему резервируется заранее, чтобы страница не прыгала после загрузки mermaid. */
.mermaid-host { min-height: 120px; display: flex; overflow-x: auto; }
/* Центрирование полями, а не justify-content: center — при переполнении flex-центр уводит левый
   край схемы за начало прокрутки, и до него не добраться; auto-поля в этом случае обнуляются. */
.mermaid-host :deep(svg) { max-width: 100%; height: auto; margin-inline: auto; }
/* Mermaid кладёт текст узлов в <p> и считает высоту по своему line-height (1.5), измеряя вне .vp-doc.
   Стиль VitePress для .vp-doc p (line-height 28px) растягивал текст, и последняя строка вылезала за рамку. */
.mermaid-host :deep(svg p) { margin: 0; line-height: inherit; }
.mermaid-error { color: var(--vp-c-danger-1); white-space: pre-wrap; }
</style>
