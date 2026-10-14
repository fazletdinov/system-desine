<script setup lang="ts">
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import LoginTimingView from '../renderers/LoginTimingView.vue'
import { loginTimingScenario } from '../simulations/login-timing'

const steps = loginTimingScenario()
</script>

<template>
  <VizFrame label="Вход в систему: утечки текста и времени и их устранение">
    <Player :steps="steps" label="Вход в систему: утечки текста и времени и их устранение">
      <template #default="{ step }">
        <LoginTimingView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
