<script setup lang="ts">
import { computed } from 'vue'
import { LAYERS, MAX_SEND_ATTEMPTS, type LayerFlowState } from '../simulations/layer-flow'

const props = defineProps<{ state: LayerFlowState }>()

const attemptText = computed(() => (props.state.attempt === 0 ? 'цикл повторов ещё не начался' : `попытка ${props.state.attempt} из ${MAX_SEND_ATTEMPTS}`))
</script>

<template>
  <div class="flow">
    <p class="head"><code>POST /chats/{id}/messages</code><span class="attempt">{{ attemptText }}</span></p>
    <ol class="bands" aria-label="Слои сервиса, через которые проходит запрос">
      <li v-for="l in LAYERS" :key="l.id" class="band" :class="[l.id, { active: l.id === state.active }]" :aria-current="l.id === state.active ? 'step' : undefined">
        <div class="band-head">
          <span class="name">{{ l.label }}</span>
          <code class="pkg">{{ l.pkg }}</code>
          <span v-if="l.id === state.active" class="here">▶ сейчас здесь</span>
        </div>
        <template v-if="l.id === state.active">
          <p class="sees-title">Что видит:</p>
          <ul class="sees">
            <li v-for="item in state.sees" :key="item"><code>{{ item }}</code></li>
          </ul>
          <p class="note">{{ state.note }}</p>
        </template>
      </li>
    </ol>
  </div>
</template>

<style scoped>
.flow { padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
.head { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 4px 12px; margin: 0 0 10px; }
.head code { font-size: 14px; font-weight: 600; background: none; padding: 0; color: var(--viz-text); }
.attempt { font-size: 14px; font-weight: 700; font-variant-numeric: tabular-nums; }
.bands { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 6px; }
.band { margin-top: 0; padding: 6px 12px; border: 2px solid var(--viz-edge); border-left-width: 8px; border-radius: 8px; }
.band.http { border-left-color: var(--viz-client); }
.band.usecase { border-left-color: var(--viz-service); }
.band.domain { border-left-color: var(--viz-cache); }
.band.store { border-left-color: var(--viz-db); }
.band.active { border-top-color: var(--viz-active); border-right-color: var(--viz-active); border-bottom-color: var(--viz-active); border-top-width: 3px; border-right-width: 3px; border-bottom-width: 3px; padding: 8px 12px; }
.band-head { display: flex; flex-wrap: wrap; align-items: baseline; gap: 2px 12px; }
.name { font-size: 15px; font-weight: 700; }
.pkg { font-size: 13px; color: var(--viz-muted); background: none; padding: 0; }
.here { margin-left: auto; font-size: 13px; font-weight: 700; }
.sees-title { margin: 6px 0 2px; font-size: 13px; color: var(--viz-muted); }
.sees { margin: 0; padding-left: 20px; }
.sees li { margin-top: 2px; }
.sees code { font-size: 14px; background: none; padding: 0; color: var(--viz-text); overflow-wrap: anywhere; }
.note { margin: 6px 0 0; font-size: 14px; font-style: italic; overflow-wrap: anywhere; }
</style>
