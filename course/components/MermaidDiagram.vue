<script setup lang="ts">
import { onMounted, ref, useId, watch } from 'vue'
import { useData } from 'vitepress'
import { mermaidThemeVariables, withPaletteClasses } from '../viz/core/mermaid-palette'

const props = defineProps<{ code: string }>()
const { isDark } = useData()
const host = ref<HTMLElement | null>(null)
const error = ref<string | null>(null)
const baseId = `mmd-${useId().replace(/[^a-zA-Z0-9-]/g, '')}`
// Каждый рендер получает свой id: при смене темы на странице не появятся два SVG с одинаковым id.
let renderSeq = 0

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
  <div class="mermaid-diagram">
    <div ref="host" class="mermaid-host" role="img" aria-label="Схема" />
    <pre v-if="error" class="mermaid-error">Ошибка схемы: {{ error }}</pre>
  </div>
</template>

<style scoped>
.mermaid-diagram { margin: 20px 0; }
/* Место под схему резервируется заранее, чтобы страница не прыгала после загрузки mermaid. */
.mermaid-host { min-height: 120px; display: flex; justify-content: center; overflow-x: auto; }
.mermaid-host :deep(svg) { max-width: 100%; height: auto; }
.mermaid-error { color: var(--vp-c-danger-1); white-space: pre-wrap; }
</style>
