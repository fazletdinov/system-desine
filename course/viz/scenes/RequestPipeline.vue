<script setup lang="ts">
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import RequestPipelineView from '../renderers/RequestPipelineView.vue'
import { requestPipelineScenario } from '../simulations/request-pipeline'

const steps = requestPipelineScenario()
</script>

<template>
  <VizFrame label="Путь запроса через цепочку middleware: пять исходов">
    <Player :steps="steps" label="Путь запроса через цепочку middleware: пять исходов">
      <template #default="{ step }">
        <RequestPipelineView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
