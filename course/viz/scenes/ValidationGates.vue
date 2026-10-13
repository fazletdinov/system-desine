<script setup lang="ts">
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import GatesView from '../renderers/GatesView.vue'
import { validationGatesScenario } from '../simulations/validation-gates'

const steps = validationGatesScenario()
</script>

<template>
  <VizFrame label="Ворота валидации: где отсекается плохой ввод">
    <Player :steps="steps" label="Ворота валидации: где отсекается плохой ввод">
      <template #default="{ step }">
        <GatesView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
