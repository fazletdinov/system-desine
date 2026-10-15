<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import VariantTabs from '../components/VariantTabs.vue'
import GracefulShutdownView from '../renderers/GracefulShutdownView.vue'
import { gracefulShutdownScenario, type ShutdownVariant } from '../simulations/graceful-shutdown'

const OPTIONS = [
  { id: 'exec-form', label: 'exec-форма: приложение — PID 1' },
  { id: 'wrapper-script', label: 'start.sh без exec: PID 1 — sh' },
]
const variant = ref<ShutdownVariant>('exec-form')
const steps = computed(() => gracefulShutdownScenario(variant.value))
const onVariant = (id: string): void => {
  variant.value = id === 'wrapper-script' ? 'wrapper-script' : 'exec-form'
}
</script>

<template>
  <VizFrame label="Плавная остановка контейнера по SIGTERM">
    <VariantTabs :options="OPTIONS" :model-value="variant" label="Как собран ENTRYPOINT" @update:model-value="onVariant" />
    <Player :steps="steps" label="Плавная остановка контейнера по SIGTERM">
      <template #default="{ step }">
        <GracefulShutdownView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
