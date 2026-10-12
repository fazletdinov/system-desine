<script setup lang="ts">
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import EncapsulationView from '../renderers/EncapsulationView.vue'
import { encapsulationScenario } from '../simulations/encapsulation'

const steps = encapsulationScenario()
</script>

<template>
  <VizFrame label="Инкапсуляция: как 200 байт запроса растут до 292">
    <Player :steps="steps" label="Инкапсуляция: как 200 байт запроса растут до 292">
      <template #default="{ step }">
        <EncapsulationView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
