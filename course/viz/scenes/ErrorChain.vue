<script setup lang="ts">
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import ErrorChainView from '../renderers/ErrorChainView.vue'
import { errorChainScenario } from '../simulations/error-chain'

const steps = errorChainScenario()
</script>

<template>
  <VizFrame label="Цепочка ошибок: %w, errors.Is и errors.As">
    <Player :steps="steps" label="Цепочка ошибок: %w, errors.Is и errors.As">
      <template #default="{ step }">
        <ErrorChainView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
