<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import VariantTabs from '../components/VariantTabs.vue'
import CgroupLimitsView from '../renderers/CgroupLimitsView.vue'
import { cgroupScenario, type CgroupVariant } from '../simulations/cgroup-limits'

const OPTIONS = [
  { id: 'memory', label: 'Лимит памяти' },
  { id: 'cpu', label: 'Лимит CPU' },
]
const variant = ref<CgroupVariant>('memory')
const steps = computed(() => cgroupScenario(variant.value))
const onVariant = (id: string): void => {
  variant.value = id === 'cpu' ? 'cpu' : 'memory'
}
</script>

<template>
  <VizFrame label="Лимиты cgroup: память и CPU">
    <VariantTabs :options="OPTIONS" :model-value="variant" label="Какой лимит показать" @update:model-value="onVariant" />
    <Player :steps="steps" label="Лимиты cgroup: память и CPU">
      <template #default="{ step }">
        <CgroupLimitsView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
