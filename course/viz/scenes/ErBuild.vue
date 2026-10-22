<script setup lang="ts">
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import ErBuildView from '../renderers/ErBuildView.vue'
import { erBuildScenario } from '../simulations/er-build'

const steps = erBuildScenario()
</script>

<template>
  <VizFrame label="Модель данных мессенджера растёт по требованиям">
    <Player :steps="steps" label="Модель данных мессенджера растёт по требованиям">
      <template #default="{ step }">
        <ErBuildView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
