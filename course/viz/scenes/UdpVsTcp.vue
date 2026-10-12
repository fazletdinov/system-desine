<script setup lang="ts">
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import UdpTcpView from '../renderers/UdpTcpView.vue'
import { udpVsTcpScenario } from '../simulations/udp-vs-tcp'

const steps = udpVsTcpScenario()
</script>

<template>
  <VizFrame label="UDP и TCP: что будет, если потерять сообщение">
    <Player :steps="steps" label="UDP и TCP: что будет, если потерять сообщение">
      <template #default="{ step }">
        <UdpTcpView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
