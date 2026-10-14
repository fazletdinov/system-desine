<script setup lang="ts">
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import ProblemMappingView from '../renderers/ProblemMappingView.vue'
import { problemMappingScenario } from '../simulations/problem-mapping'

const steps = problemMappingScenario()
</script>

<template>
  <VizFrame label="Пять источников ошибок и один формат ответа problem+json">
    <Player :steps="steps" label="Пять источников ошибок и один формат ответа problem+json">
      <template #default="{ step }">
        <ProblemMappingView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
