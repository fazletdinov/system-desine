<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import VariantTabs from '../components/VariantTabs.vue'
import MigrationRunView from '../renderers/MigrationRunView.vue'
import { migrationRunScenario, type MigVariant } from '../simulations/migration-run'

const OPTIONS = [
  { id: 'first', label: 'Первый запуск' },
  { id: 'failure', label: 'Сбой и повтор' },
  { id: 'outdated', label: 'Старт без миграции' },
]
const variant = ref<MigVariant>('first')
const steps = computed(() => migrationRunScenario(variant.value))
const onVariant = (id: string): void => {
  variant.value = id === 'failure' ? 'failure' : id === 'outdated' ? 'outdated' : 'first'
}
</script>

<template>
  <VizFrame label="Миграции goose: применение, сбой с откатом, отказ старта">
    <VariantTabs :options="OPTIONS" :model-value="variant" label="Что происходит с миграцией" @update:model-value="onVariant" />
    <Player :steps="steps" label="Миграции goose: применение, сбой с откатом, отказ старта">
      <template #default="{ step }">
        <MigrationRunView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>
