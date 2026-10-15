<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import VariantTabs from '../components/VariantTabs.vue'
import GoMemLimitView from '../renderers/GoMemLimitView.vue'
import { gomemlimitScenario, type MemLimitVariant } from '../simulations/gomemlimit'

const OPTIONS = [
  { id: 'without', label: 'Без GOMEMLIMIT' },
  { id: 'with', label: 'GOMEMLIMIT=230MiB' },
]
const variant = ref<MemLimitVariant>('without')
const steps = computed(() => gomemlimitScenario(variant.value))
const frames = computed(() => steps.value.map((s) => s.state))
const onVariant = (id: string): void => {
  variant.value = id === 'with' ? 'with' : 'without'
}
</script>

<template>
  <VizFrame label="GOMEMLIMIT: упрощённая модель памяти Go-процесса в контейнере на 256 МиБ">
    <VariantTabs :options="OPTIONS" :model-value="variant" label="Задан ли GOMEMLIMIT" @update:model-value="onVariant" />
    <Player :steps="steps" label="GOMEMLIMIT: упрощённая модель памяти Go-процесса">
      <template #default="{ index }">
        <GoMemLimitView :frames="frames" :index="index" :with-limit="variant === 'with'" />
      </template>
    </Player>
  </VizFrame>
</template>
