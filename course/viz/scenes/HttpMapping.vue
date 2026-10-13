<script setup lang="ts">
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import HttpMappingView from '../renderers/HttpMappingView.vue'
import { httpMappingScenario } from '../simulations/http-mapping'

const steps = httpMappingScenario()
</script>

<template>
  <VizFrame label="Одно сообщение в шести представлениях: от JSON до домена и обратно">
    <Player :steps="steps" label="Одно сообщение в шести представлениях: от JSON до домена и обратно">
      <template #default="{ step }">
        <HttpMappingView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
