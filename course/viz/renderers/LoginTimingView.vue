<script setup lang="ts">
import { computed } from 'vue'
import type { LoginTimingState } from '../simulations/login-timing'

const props = defineProps<{ state: LoginTimingState }>()

// Шкала линейная до 60 мс: 1 мс остаётся видимой узкой полоской.
const SCALE_MS = 60
const width = computed(() => `${Math.max(2, Math.min(100, (props.state.elapsedMs / SCALE_MS) * 100))}%`)
const attemptText = computed(() => (props.state.attempt === 'unknown-user' ? 'логина нет в базе' : 'логин есть, пароль неверный'))
const implText = computed(() => (props.state.impl === 'naive' ? 'Наивная реализация' : 'Правильная реализация'))
</script>

<template>
  <div class="lt">
    <p class="head"><strong class="impl" :class="state.impl">{{ implText }}</strong><span class="attempt">{{ attemptText }}</span></p>
    <ol class="steps" aria-label="Что делает сервер">
      <li v-for="s in state.steps" :key="s"><code>{{ s }}</code></li>
    </ol>
    <div class="resp" :class="state.impl">
      <p class="label">Ответ клиенту</p>
      <p class="line"><strong class="status">{{ state.response.status }}</strong> <code>{{ state.response.code }}</code></p>
      <p class="text">«{{ state.response.text }}»</p>
    </div>
    <div class="time">
      <p class="label">Время ответа (порядок величины)</p>
      <div class="track" role="img" :aria-label="`${state.elapsedMs} миллисекунд`"><div class="bar" :class="state.impl" :style="{ width }"></div></div>
      <p class="ms">~{{ state.elapsedMs }} мс</p>
    </div>
  </div>
</template>

<style scoped>
.lt { padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
.head { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 4px 12px; margin: 0 0 8px; font-size: 15px; }
.impl.naive { color: color-mix(in srgb, var(--viz-error) 60%, var(--viz-text)); }
.impl.safe { color: color-mix(in srgb, var(--viz-response) 60%, var(--viz-text)); }
.attempt { font-size: 14px; color: var(--viz-muted); }
.steps { margin: 0 0 10px; padding-left: 20px; }
.steps li { margin-top: 2px; }
.steps code { font-size: 13px; background: none; padding: 0; color: var(--viz-text); overflow-wrap: anywhere; }
.resp { border: 2px solid var(--viz-edge); border-left-width: 8px; border-radius: 8px; padding: 6px 12px; margin-bottom: 10px; }
.resp.naive { border-left-color: var(--viz-error); }
.resp.safe { border-left-color: var(--viz-response); }
.label { margin: 0 0 4px; font-size: 13px; color: var(--viz-muted); }
.line { margin: 0; font-size: 15px; }
.line code { background: none; padding: 0; color: var(--viz-text); }
.status { font-variant-numeric: tabular-nums; }
.text { margin: 2px 0 0; font-size: 14px; overflow-wrap: anywhere; }
.track { height: 16px; border: 2px solid var(--viz-edge); border-radius: 8px; overflow: hidden; }
.bar { height: 100%; transition: width 0.3s; }
.bar.naive { background: var(--viz-error); }
.bar.safe { background: var(--viz-response); }
.ms { margin: 4px 0 0; font-size: 14px; font-weight: 700; font-variant-numeric: tabular-nums; }
@media (prefers-reduced-motion: reduce) { .bar { transition: none; } }
</style>
