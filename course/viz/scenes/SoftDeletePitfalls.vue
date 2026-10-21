<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import VariantTabs from '../components/VariantTabs.vue'
import SoftDeletePitfallsView from '../renderers/SoftDeletePitfallsView.vue'
import { softDeleteScenario, type SoftDeleteVariant } from '../simulations/soft-delete-pitfalls'

const OPTIONS = [
  { id: 'unique', label: 'Уникальность' },
  { id: 'references', label: 'Запросы и ссылки' },
  { id: 'erasure', label: 'Право на удаление' },
]
const variant = ref<SoftDeleteVariant>('unique')
const steps = computed(() => softDeleteScenario(variant.value))
const onVariant = (id: string): void => {
  variant.value = id === 'references' || id === 'erasure' ? id : 'unique'
}
</script>

<template>
  <VizFrame label="Ловушки мягкого удаления: уникальность, ссылки и право на удаление">
    <VariantTabs :options="OPTIONS" :model-value="variant" label="Какую ловушку мягкого удаления показать" @update:model-value="onVariant" />
    <Player :steps="steps" label="Ловушки мягкого удаления: уникальность, ссылки и право на удаление">
      <template #default="{ step }">
        <SoftDeletePitfallsView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
