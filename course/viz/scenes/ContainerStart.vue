<script setup lang="ts">
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import ContainerStartView from '../renderers/ContainerStartView.vue'
import { containerStartScenario } from '../simulations/container-start'

const steps = containerStartScenario()
</script>

<template>
  <VizFrame label="Запуск контейнера: девять системных шагов">
    <Player :steps="steps" label="Запуск контейнера: девять системных шагов">
      <template #default="{ step }">
        <ContainerStartView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
