<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import SegmentedToggle from '../components/SegmentedToggle.vue'
import NetpollerView from '../renderers/NetpollerView.vue'
import { netpollerScenario, type NetpollerVariant } from '../simulations/netpoller'

const variant = ref<NetpollerVariant>('data')
const steps = computed(() => netpollerScenario(variant.value))
const OPTIONS: { value: NetpollerVariant; text: string }[] = [
  { value: 'data', text: 'данные пришли' },
  { value: 'deadline', text: 'дедлайн истёк' },
]
</script>

<template>
  <VizFrame label="Сетевой поллер: conn.Read без блокировки потока">
    <SegmentedToggle label="Сценарий" :options="OPTIONS" :model-value="variant" @update:model-value="(v) => (variant = v)" />
    <Player :steps="steps" label="Сетевой поллер: conn.Read без блокировки потока">
      <template #default="{ step }">
        <NetpollerView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
