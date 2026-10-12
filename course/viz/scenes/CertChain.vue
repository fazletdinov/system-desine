<script setup lang="ts">
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import CertChainView from '../renderers/CertChainView.vue'
import { certChainScenario } from '../simulations/cert-chain'

const steps = certChainScenario()
</script>

<template>
  <VizFrame label="Цепочка сертификатов: как клиент проверяет сервер">
    <Player :steps="steps" label="Цепочка сертификатов: как клиент проверяет сервер">
      <template #default="{ step }">
        <CertChainView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
