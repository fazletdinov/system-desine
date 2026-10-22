<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import VariantTabs from '../components/VariantTabs.vue'
import ConstraintRaceView from '../renderers/ConstraintRaceView.vue'
import { constraintRaceScenario, type RaceVariant } from '../simulations/constraint-race'

const OPTIONS = [
  { id: 'code-check', label: 'Проверка в коде' },
  { id: 'unique', label: 'Ограничение UNIQUE' },
]
const variant = ref<RaceVariant>('code-check')
const steps = computed(() => constraintRaceScenario(variant.value))
const onVariant = (id: string): void => {
  variant.value = id === 'unique' ? 'unique' : 'code-check'
}
</script>

<template>
  <VizFrame label="Два экземпляра регистрируют одно имя: проверка в коде против UNIQUE">
    <VariantTabs :options="OPTIONS" :model-value="variant" label="Кто следит за уникальностью имени" @update:model-value="onVariant" />
    <Player :steps="steps" label="Два экземпляра регистрируют одно имя: проверка в коде против UNIQUE">
      <template #default="{ step }">
        <ConstraintRaceView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
