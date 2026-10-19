<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import VariantTabs from '../components/VariantTabs.vue'
import AppLifecycleView from '../renderers/AppLifecycleView.vue'
import { appLifecycleScenario, type LifecycleVariant } from '../simulations/app-lifecycle'

const OPTIONS = [
  { id: 'signal', label: 'Сигнал SIGTERM' },
  { id: 'component-failure', label: 'Сбой фонового компонента' },
  { id: 'second-signal', label: 'Второй сигнал' },
]
const variant = ref<LifecycleVariant>('signal')
const steps = computed(() => appLifecycleScenario(variant.value))
const drainStart = computed(() => steps.value.find((s) => s.state.http === 'draining')?.state.t ?? null)
const exitAt = computed(() => steps.value.find((s) => s.state.exit !== null)?.state.t ?? null)
const onVariant = (id: string): void => {
  variant.value = id === 'component-failure' ? 'component-failure' : id === 'second-signal' ? 'second-signal' : 'signal'
}
</script>

<template>
  <VizFrame label="Жизненный цикл сервиса: остановка по сигналу и по сбою">
    <VariantTabs :options="OPTIONS" :model-value="variant" label="Что запускает остановку" @update:model-value="onVariant" />
    <Player :steps="steps" label="Жизненный цикл сервиса: остановка по сигналу и по сбою">
      <template #default="{ step }">
        <AppLifecycleView :state="step.state" :drain-start="drainStart" :exit-at="exitAt" />
      </template>
    </Player>
  </VizFrame>
</template>
