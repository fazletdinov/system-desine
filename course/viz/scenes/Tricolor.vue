<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import SegmentedToggle from '../components/SegmentedToggle.vue'
import TricolorView from '../renderers/TricolorView.vue'
import { tricolorScenario, type TricolorVariant } from '../simulations/tricolor'

const variant = ref<TricolorVariant>('no-barrier')
const steps = computed(() => tricolorScenario(variant.value))
const OPTIONS: { value: TricolorVariant; text: string }[] = [
  { value: 'no-barrier', text: 'без барьера записи' },
  { value: 'barrier', text: 'с барьером записи' },
]
</script>

<template>
  <VizFrame label="Трёхцветная разметка и барьер записи">
    <SegmentedToggle label="Барьер записи" :options="OPTIONS" :model-value="variant" @update:model-value="(v) => (variant = v)" />
    <Player :steps="steps" label="Трёхцветная разметка и барьер записи">
      <template #default="{ step }">
        <TricolorView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
