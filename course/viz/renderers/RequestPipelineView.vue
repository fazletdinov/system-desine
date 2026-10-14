<script setup lang="ts">
import { computed } from 'vue'
import { LAYERS, PIPELINE_IDS, type PipelineAt, type RequestPipelineState } from '../simulations/request-pipeline'

const props = defineProps<{ state: RequestPipelineState }>()

const atIndex = computed(() => PIPELINE_IDS.indexOf(props.state.at))
const isError = computed(() => props.state.status !== null && props.state.status >= 400)

// Слой пройден, если запрос ушёл глубже; на кадре response пройдено всё.
function cls(id: PipelineAt): Record<string, boolean> {
  const i = PIPELINE_IDS.indexOf(id)
  return { active: i === atIndex.value, passed: i < atIndex.value, error: i === atIndex.value && isError.value }
}
function meta(id: PipelineAt) {
  return LAYERS.find((l) => l.id === id)!
}
</script>

<template>
  <div class="pipe">
    <p class="head"><code>{{ state.request }}</code><span class="rid">X-Request-ID: <code>{{ state.requestId }}</code></span></p>
    <div class="grid">
      <div class="chain" role="group" aria-label="Цепочка middleware матрёшкой: внешние слои снаружи">
        <div class="layer" :class="cls('request-id')">
          <p class="lh"><strong>{{ meta('request-id').label }}</strong><span class="role">{{ meta('request-id').role }}</span></p>
          <div class="layer" :class="cls('access-log')">
            <p class="lh"><strong>{{ meta('access-log').label }}</strong><span class="role">{{ meta('access-log').role }}</span></p>
            <div class="layer" :class="cls('recover')">
              <p class="lh"><strong>{{ meta('recover').label }}</strong><span class="role">{{ meta('recover').role }}</span></p>
              <div class="layer" :class="cls('auth')">
                <p class="lh"><strong>{{ meta('auth').label }}</strong><span class="role">{{ meta('auth').role }}</span></p>
                <div class="layer" :class="cls('router')">
                  <p class="lh"><strong>{{ meta('router').label }}</strong><span class="role">{{ meta('router').role }}</span></p>
                  <div class="layer core" :class="cls('handler')">
                    <p class="lh"><strong>{{ meta('handler').label }}</strong><span class="role">{{ meta('handler').role }}</span></p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
        <p class="reply" :class="{ active: state.at === 'response', error: isError }">
          <span class="reply-label">ответ клиенту</span>
          <code>{{ state.reply ?? 'ещё не отправлен' }}</code>
        </p>
      </div>
      <div class="log">
        <p class="log-title">Лог AccessLog</p>
        <ul v-if="state.log.length > 0" class="lines">
          <li v-for="l in state.log" :key="l"><code>{{ l }}</code></li>
        </ul>
        <p v-else class="empty">строки ещё нет: AccessLog пишет её после ответа</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.pipe { padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
.head { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 4px 12px; margin: 0 0 10px; font-size: 14px; }
.head code, .reply code, .lines code { background: none; padding: 0; color: var(--viz-text); overflow-wrap: anywhere; }
.head > code { font-weight: 600; }
.grid { display: grid; grid-template-columns: 1fr; gap: 12px; }
@container (min-width: 640px) { .grid { grid-template-columns: minmax(0, 3fr) minmax(0, 2fr); } }
.layer { margin-top: 0; padding: 6px 8px 8px 10px; border: 2px solid var(--viz-edge); border-radius: 8px; }
.layer .layer { margin-top: 6px; }
.layer.passed { border-color: var(--viz-network); }
.layer.active { border-color: var(--viz-active); border-width: 3px; background: var(--viz-active-line-bg); }
.layer.error { border-color: var(--viz-error); }
.layer.core { border-left-width: 8px; border-left-color: var(--viz-service); }
.lh { display: flex; flex-wrap: wrap; align-items: baseline; gap: 0 10px; margin: 0; font-size: 14px; }
.role { font-size: 13px; color: color-mix(in srgb, var(--viz-muted) 60%, var(--viz-text)); }
.reply { display: flex; flex-wrap: wrap; align-items: baseline; gap: 2px 10px; margin: 8px 0 0; padding: 6px 10px; border: 2px dashed var(--viz-edge); border-radius: 8px; font-size: 14px; }
.reply.active { border-style: solid; border-color: var(--viz-response); }
.reply.error.active { border-color: var(--viz-error); }
.reply-label { color: var(--viz-muted); font-size: 13px; }
.log { border: 2px solid var(--viz-edge); border-radius: 8px; padding: 8px 10px; align-self: start; }
.log-title { margin: 0 0 6px; font-size: 13px; color: var(--viz-muted); }
.lines { margin: 0; padding: 0; list-style: none; }
.lines li { margin-top: 0; font-size: 13px; }
.empty { margin: 0; font-size: 13px; font-style: italic; color: var(--viz-muted); }
</style>
