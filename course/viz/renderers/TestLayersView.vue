<script setup lang="ts">
import { computed } from 'vue'
import { LEVELS, type TestLayersState } from '../simulations/test-layers'

const props = defineProps<{ state: TestLayersState }>()

// Пирамида рисуется сверху вниз: наверху самые дорогие и немногочисленные тесты.
const PYRAMID = [...LEVELS.filter((l) => l.id !== 'race')].reverse()
const RACE = LEVELS.find((l) => l.id === 'race')!

const caught = computed(() => props.state.caughtBy)
</script>

<template>
  <div class="tests">
    <div class="bug">
      <p class="bug-line"><span class="key">Баг:</span> <strong>{{ state.bug }}</strong></p>
      <p class="cost"><span class="key">Цена прогона:</span> {{ state.cost }}</p>
    </div>
    <ol class="pyramid" aria-label="Пирамида тестов: сверху самые дорогие">
      <li v-for="(l, i) in PYRAMID" :key="l.id" class="tier" :class="{ caught: l.id === caught }" :style="{ width: `calc(100% - ${PYRAMID.length - 1 - i} * var(--tier-step))` }">
        <span class="name">{{ l.label }}</span>
        <code class="tool">{{ l.tool }}</code>
        <strong v-if="l.id === caught" class="mark">✓ поймал баг</strong>
      </li>
    </ol>
    <div class="race" :class="{ caught: caught === 'race' }">
      <span class="name">{{ RACE.label }}</span>
      <code class="tool">{{ RACE.tool }}</code>
      <span class="hint">режим запуска любого уровня, а не отдельный этаж</span>
      <strong v-if="caught === 'race'" class="mark">✓ поймал баг</strong>
    </div>
  </div>
</template>

<style scoped>
.tests { --tier-step: 14%; padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
.bug { margin-bottom: 12px; }
.bug p { margin: 0; overflow-wrap: anywhere; }
.bug-line { font-size: 16px; }
.cost { margin-top: 4px !important; font-size: 14px; }
.key { color: var(--viz-muted); }
.pyramid { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; align-items: center; gap: 4px; }
.tier { margin-top: 0; box-sizing: border-box; display: flex; flex-direction: column; align-items: center; gap: 2px; padding: 6px 8px; border: 2px solid var(--viz-service); border-radius: 8px; text-align: center; }
.tier.caught, .race.caught { border-color: var(--viz-active); border-width: 4px; }
.name { font-size: 14px; font-weight: 700; }
.tool { font-size: 13px; color: var(--viz-muted); background: none; padding: 0; overflow-wrap: anywhere; }
.mark { font-size: 14px; }
.race { margin-top: 10px; display: flex; flex-direction: column; align-items: center; gap: 2px; padding: 6px 8px; border: 2px dashed var(--viz-broker); border-radius: 8px; text-align: center; }
.hint { font-size: 13px; color: var(--viz-muted); }
/* На телефоне узкая вершина не вместила бы название инструмента. */
@container (max-width: 520px) { .tests { --tier-step: 7%; } }
</style>
