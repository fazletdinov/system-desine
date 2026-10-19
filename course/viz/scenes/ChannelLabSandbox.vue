<script setup lang="ts">
import { computed, ref } from 'vue'
import VizFrame from '../components/VizFrame.vue'
import Player from '../components/Player.vue'
import SegmentedToggle from '../components/SegmentedToggle.vue'
import ChannelLabView from '../renderers/ChannelLabView.vue'
import { channelLab, type Cap, type ChannelLabAction, type GState } from '../simulations/channel-lab'

const state = ref(channelLab.initial())
const steps = computed(() => channelLab.steps(state.value))
// Каждая отправка несёт новое значение, чтобы на экране было видно, куда оно попало.
const nextValue = ref(1)

const dispatch = (action: ChannelLabAction): void => {
  state.value = channelLab.apply(state.value, action)
}
const reset = (): void => {
  nextValue.value = 1
  dispatch({ type: 'reset' })
}
const sendValue = (g: string, op: 'send' | 'select'): void => {
  const value = nextValue.value++
  dispatch(op === 'send' ? { type: 'send', g, value } : { type: 'selectDefault', g, op: 'send', value })
}
const isBlocked = (s: GState): boolean => s === 'blocked-send' || s === 'blocked-recv'
const disabledTitle = (id: string, s: GState): string | undefined => (isBlocked(s) ? `${id} заблокирована` : s === 'running' ? undefined : `${id} уже не работает`)

const capOptions = [
  { value: 0, text: 'ёмкость 0' },
  { value: 1, text: 'ёмкость 1' },
  { value: 3, text: 'ёмкость 3' },
] as const
const nilOptions = [
  { value: false, text: 'канал создан' },
  { value: true, text: 'nil-канал' },
] as const
const setCap = (cap: Cap): void => {
  nextValue.value = 1
  dispatch({ type: 'setCap', cap })
}
const setNil = (nil: boolean): void => {
  nextValue.value = 1
  dispatch({ type: 'setNil', nil })
}
</script>

<template>
  <VizFrame label="Песочница: канал Go">
    <p class="intro">Один канал и четыре горутины. Отправляйте, получайте, закрывайте — и смотрите, кто блокируется, кто просыпается и когда случается паника.</p>
    <SegmentedToggle label="Ёмкость канала" :options="capOptions" :model-value="state.cap" @update:model-value="setCap" />
    <SegmentedToggle label="Состояние канала" :options="nilOptions" :model-value="state.nil" @update:model-value="setNil" />
    <div class="gpanels">
      <div v-for="g in state.gs" :key="g.id" class="panel" role="group" :aria-label="`Действия ${g.id}`">
        <b class="gname">{{ g.id }}</b>
        <button type="button" :disabled="g.state !== 'running'" :title="disabledTitle(g.id, g.state)" @click="sendValue(g.id, 'send')">Отправить</button>
        <button type="button" :disabled="g.state !== 'running'" :title="disabledTitle(g.id, g.state)" @click="dispatch({ type: 'recv', g: g.id })">Получить</button>
        <button type="button" :disabled="g.state !== 'running'" :title="disabledTitle(g.id, g.state)" @click="dispatch({ type: 'close', g: g.id })">Закрыть</button>
        <button type="button" :disabled="g.state !== 'running'" :title="disabledTitle(g.id, g.state)" @click="sendValue(g.id, 'select')">select: отправить / default</button>
        <button type="button" :disabled="g.state !== 'running'" :title="disabledTitle(g.id, g.state)" @click="dispatch({ type: 'selectDefault', g: g.id, op: 'recv' })">select: получить / default</button>
      </div>
    </div>
    <div class="panel"><button type="button" @click="reset">Сброс</button></div>
    <Player :steps="steps" label="Песочница: канал">
      <template #default="{ step }">
        <ChannelLabView :state="step.state" />
      </template>
    </Player>
  </VizFrame>
</template>

<style scoped>
.intro { margin: 0 0 10px; font-size: 14px; color: var(--viz-muted); }
.gpanels { display: flex; flex-direction: column; gap: 4px; margin-bottom: 6px; }
.panel { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-bottom: 6px; }
.gname { min-width: 28px; }
.panel button { min-width: 36px; min-height: 36px; padding: 0 12px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; cursor: pointer; }
.panel button:disabled { opacity: 0.55; cursor: not-allowed; }
.panel button:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
</style>
