<script setup lang="ts">
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import TestLayersView from '../renderers/TestLayersView.vue'
import { testLayersScenario } from '../simulations/test-layers'

const steps = testLayersScenario()
</script>

<template>
  <VizFrame label="Уровни тестов: кто ловит какой баг">
    <Player :steps="steps" label="Уровни тестов: кто ловит какой баг">
      <template #default="{ step }">
        <TestLayersView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
