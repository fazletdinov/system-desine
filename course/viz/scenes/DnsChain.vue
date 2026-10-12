<script setup lang="ts">
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import DnsChainView from '../renderers/DnsChainView.vue'
import { dnsChainScenario } from '../simulations/dns-chain'

const steps = dnsChainScenario()
</script>

<template>
  <VizFrame label="DNS: путь запроса от телефона до авторитетного сервера">
    <Player :steps="steps" label="DNS: путь запроса от телефона до авторитетного сервера">
      <template #default="{ step }">
        <DnsChainView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
