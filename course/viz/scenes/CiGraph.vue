<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import VariantTabs from '../components/VariantTabs.vue'
import CiGraphView from '../renderers/CiGraphView.vue'
import { ciGraphScenario, type CiVariant } from '../simulations/ci-graph'

const OPTIONS = [
  { id: 'aggregate', label: 'Агрегирующая джоба ci-ok' },
  { id: 'on-paths', label: 'Фильтр on.paths на весь workflow' },
]
const variant = ref<CiVariant>('aggregate')
const steps = computed(() => ciGraphScenario(variant.value))
const onVariant = (id: string): void => {
  variant.value = id === 'on-paths' ? 'on-paths' : 'aggregate'
}
</script>

<template>
  <VizFrame label="Граф джоб CI монорепозитория: PR меняет только урок">
    <VariantTabs :options="OPTIONS" :model-value="variant" label="Как устроены обязательные проверки" @update:model-value="onVariant" />
    <Player :steps="steps" label="Граф джоб CI монорепозитория: PR меняет только урок">
      <template #default="{ step }">
        <CiGraphView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
