<script setup lang="ts">
import Player from '../components/Player.vue'
import CodePanel from '../components/CodePanel.vue'
import RequestPathView from '../renderers/RequestPathView.vue'
import { requestPathScenario } from '../simulations/request-path'

const steps = requestPathScenario()
</script>

<template>
  <Player :steps="steps" label="Путь запроса «Отправить сообщение»">
    <template #default="{ step }">
      <RequestPathView :state="step.state" />
    </template>
    <template v-if="$slots.default" #code="{ line }">
      <CodePanel :line="line"><slot /></CodePanel>
    </template>
  </Player>
</template>
