<script setup lang="ts">
import { computed } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import GmpSchedulerView from '../renderers/GmpSchedulerView.vue'
import { gmpSchedulerScenario } from '../simulations/gmp-scheduler'

const steps = computed(() => gmpSchedulerScenario())
</script>

<template>
  <VizFrame label="Планировщик Go: G, M и P">
    <Player :steps="steps" label="Планировщик Go: G, M и P">
      <template #default="{ step }">
        <GmpSchedulerView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
