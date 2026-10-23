<script setup lang="ts">
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import CodePanel from '../components/CodePanel.vue'
import RequestPathView from '../renderers/RequestPathView.vue'
import { REQUEST_PATH_CONTEXT, REQUEST_PATH_HONESTY, REQUEST_PATH_REQUEST, requestPathScenario } from '../simulations/request-path'

const steps = requestPathScenario()
</script>

<template>
  <VizFrame label="Путь запроса «Отправить сообщение»">
  <Player :steps="steps" label="Путь запроса «Отправить сообщение»">
    <template #default="{ step }">
      <p class="context"><code>{{ REQUEST_PATH_REQUEST }}</code> — {{ REQUEST_PATH_CONTEXT }}</p>
      <RequestPathView :state="step.state" />
      <p class="honest">{{ REQUEST_PATH_HONESTY }}</p>
    </template>
    <template v-if="$slots.default" #code="{ line }">
      <CodePanel :line="line"><slot /></CodePanel>
    </template>
  </Player>
  </VizFrame>
</template>

<style scoped>
.context { margin: 0 0 6px; color: var(--viz-text); font-size: 15px; line-height: 1.5; }
/* Встроенный код VitePress красится в brand-1 — синий текст в схеме запрещён палитрой. */
.context code { color: var(--viz-text); font-family: var(--vp-font-family-mono); font-size: 13px; overflow-wrap: anywhere; }
.honest { margin: 6px 0 0; color: var(--viz-muted); font-size: 13px; line-height: 1.5; }
</style>
