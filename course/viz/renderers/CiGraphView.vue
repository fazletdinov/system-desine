<script setup lang="ts">
import { computed } from 'vue'
import { GATE_IDS, type CiFrame, type CiJob, type JobStatus, type PrState } from '../simulations/ci-graph'

const props = defineProps<{ state: CiFrame }>()

const STATUS: Record<JobStatus, { icon: string; text: string }> = {
  pending: { icon: '○', text: 'ожидает' },
  running: { icon: '▶', text: 'идёт' },
  success: { icon: '✓', text: 'успех' },
  failure: { icon: '✕', text: 'сбой' },
  skipped: { icon: '↷', text: 'пропущена' },
}
const PR: Record<PrState, { cls: string; text: string }> = {
  waiting: { cls: 'wait', text: '⏳ Ожидание: Expected — Waiting for status to be reported' },
  mergeable: { cls: 'ok', text: '✓ Можно сливать: все обязательные проверки пройдены' },
  blocked: { cls: 'bad', text: '✕ Слияние заблокировано' },
}
const job = (id: string): CiJob => props.state.jobs.find((j) => j.id === id) as CiJob
const gates = computed(() => GATE_IDS.map(job))
const isRequired = (id: string): boolean => props.state.required.includes(id)
</script>

<template>
  <div class="ci">
    <section class="layer" aria-label="Слой 1: определение изменений">
      <div class="node" :class="job('changes').status" :data-required="isRequired('changes')">
        <strong>changes</strong><span class="st">{{ STATUS[job('changes').status].icon }} {{ STATUS[job('changes').status].text }}</span>
        <span v-if="isRequired('changes')" class="req">обязательная</span>
      </div>
    </section>
    <p class="arrow" aria-hidden="true">↓ needs: changes</p>
    <section class="layer gates" aria-label="Слой 2: ворота">
      <div v-for="j in gates" :key="j.id" class="node" :class="j.status">
        <strong>{{ j.id }}</strong><span class="st">{{ STATUS[j.status].icon }} {{ STATUS[j.status].text }}</span>
        <span v-if="isRequired(j.id)" class="req">обязательная</span>
      </div>
    </section>
    <p class="arrow" aria-hidden="true">↓ needs: все ворота</p>
    <section class="layer" aria-label="Слой 3: итог">
      <div class="node" :class="job('ci-ok').status">
        <strong>ci-ok</strong><span class="st">{{ STATUS[job('ci-ok').status].icon }} {{ STATUS[job('ci-ok').status].text }}</span>
        <span v-if="isRequired('ci-ok')" class="req">обязательная</span>
      </div>
    </section>
    <p class="pr" :class="PR[state.pr].cls" role="status">{{ PR[state.pr].text }}</p>
  </div>
</template>

<style scoped>
.ci { padding: 12px; border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
.layer { display: flex; flex-wrap: wrap; justify-content: center; gap: 8px; }
.node { display: flex; flex-direction: column; min-width: 128px; padding: 6px 10px; border: 2px solid var(--viz-edge); border-left-width: 8px; border-radius: 8px; background: var(--viz-node-bg); color: var(--viz-text); font-size: 13px; }
.node strong { overflow-wrap: anywhere; }
.st { font-weight: 600; }
.req { margin-top: 2px; font-size: 12px; text-decoration: underline; }
.node.running { border-color: var(--viz-active); }
.node.success { border-color: var(--viz-response); }
.node.failure { border-color: var(--viz-error); }
.node.skipped { border-style: dashed; border-color: var(--viz-network); }
.arrow { margin: 6px 0; text-align: center; font-size: 12px; color: var(--viz-muted); }
.pr { margin: 12px 0 0; padding: 6px 12px; border: 2px solid; border-radius: 8px; font-size: 14px; font-weight: 700; }
.pr.wait { border-color: var(--viz-active); }
.pr.ok { border-color: var(--viz-response); }
.pr.bad { border-color: var(--viz-error); }
</style>
