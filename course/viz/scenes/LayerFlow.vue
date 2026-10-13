<script setup lang="ts">
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import LayerFlowView from '../renderers/LayerFlowView.vue'
import { layerFlowScenario } from '../simulations/layer-flow'

const steps = layerFlowScenario()
</script>

<template>
  <VizFrame label="Путь запроса через слои: POST /chats/{id}/messages">
    <Player :steps="steps" label="Путь запроса через слои: POST /chats/{id}/messages">
      <template #default="{ step }">
        <LayerFlowView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
