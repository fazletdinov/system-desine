<script setup lang="ts">
import { computed } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import MutexModesView from '../renderers/MutexModesView.vue'
import { mutexModesScenario } from '../simulations/mutex-modes'

const steps = computed(() => mutexModesScenario())
</script>

<template>
  <VizFrame label="Режимы sync.Mutex: нормальный и голодание">
    <Player :steps="steps" label="Режимы sync.Mutex: нормальный и голодание">
      <template #default="{ step }">
        <MutexModesView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
