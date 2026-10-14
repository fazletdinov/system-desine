<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import JwtView from '../renderers/JwtView.vue'
import { jwtSimulation, PUBLIC_X, type JwtAction } from '../simulations/jwt'

const state = ref(jwtSimulation.initial())
const steps = computed(() => jwtSimulation.steps(state.value))

const dispatch = (action: JwtAction): void => {
  state.value = jwtSimulation.apply(state.value, action)
}
const actions: { label: string; action: JwtAction }[] = [
  { label: 'Сменить sub на Еву', action: { type: 'sub', value: 'eve' } },
  { label: 'alg: none', action: { type: 'alg', value: 'none' } },
  { label: 'alg: HS256', action: { type: 'alg', value: 'HS256' } },
  { label: 'Чужой kid', action: { type: 'kid', value: 'chuzhoi-kluch' } },
  { label: 'Изменить байт подписи', action: { type: 'tamper-signature' } },
  { label: 'Часы +16 мин', action: { type: 'clock', seconds: 16 * 60 } },
  { label: 'Часы −5 мин', action: { type: 'clock', seconds: -5 * 60 } },
  { label: 'Токен для другого сервиса', action: { type: 'token', which: 'foreign-aud' } },
  { label: 'Сброс', action: { type: 'reset' } },
]
</script>

<template>
  <VizFrame label="Песочница: проверка JWT">
    <p class="intro">Настоящий токен, подписанный Ed25519. Портите его по одному месту и смотрите, какое правило сработает первым.</p>
    <div class="panel" role="group" aria-label="Действия с токеном">
      <button v-for="a in actions" :key="a.label" type="button" @click="dispatch(a.action)">{{ a.label }}</button>
    </div>
    <Player :steps="steps" label="Песочница: JWT">
      <template #default="{ step }">
        <JwtView :state="step.state" />
      </template>
    </Player>
    <p class="note">Учебный ключ: seed — байты 0x01…0x20, открытый ключ (x, Ed25519): <code>{{ PUBLIC_X }}</code>. Токен можно проверить на jwt.io.</p>
  </VizFrame>
</template>

<style scoped>
.intro { margin: 0 0 10px; font-size: 14px; color: var(--viz-muted); }
.panel { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 10px; }
.panel button { min-width: 36px; min-height: 36px; padding: 0 12px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; cursor: pointer; }
.panel button:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
.note { margin: 10px 0 0; font-size: 13px; color: var(--viz-muted); overflow-wrap: anywhere; }
code { font-size: 12px; overflow-wrap: anywhere; }
</style>
