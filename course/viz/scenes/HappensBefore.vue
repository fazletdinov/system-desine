<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import VariantTabs from '../components/VariantTabs.vue'
import HappensBeforeView from '../renderers/HappensBeforeView.vue'
import { happensBeforeScenario, type HbVariant } from '../simulations/happens-before'

const OPTIONS = [
  { id: 'none', label: 'Без синхронизации' },
  { id: 'channel', label: 'Канал' },
  { id: 'mutex', label: 'Мьютекс' },
]
const variant = ref<HbVariant>('none')
const steps = computed(() => happensBeforeScenario(variant.value))
const onVariant = (id: string): void => {
  variant.value = id === 'channel' ? 'channel' : id === 'mutex' ? 'mutex' : 'none'
}
</script>

<template>
  <VizFrame label="Отношение «происходит раньше» между двумя горутинами">
    <VariantTabs :options="OPTIONS" :model-value="variant" label="Чем связаны горутины" @update:model-value="onVariant" />
    <Player :steps="steps" label="Отношение «происходит раньше» между двумя горутинами">
      <template #default="{ step }">
        <HappensBeforeView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
