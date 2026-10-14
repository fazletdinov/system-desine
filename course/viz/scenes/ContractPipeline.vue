<script setup lang="ts">
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import ContractPipelineView from '../renderers/ContractPipelineView.vue'
import { contractPipelineScenario } from '../simulations/contract-pipeline'

const steps = contractPipelineScenario()
</script>

<template>
  <VizFrame label="От контракта до проверки: что ловит компилятор, а что контрактный тест">
    <Player :steps="steps" label="От контракта до проверки: что ловит компилятор, а что контрактный тест">
      <template #default="{ step }">
        <ContractPipelineView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
