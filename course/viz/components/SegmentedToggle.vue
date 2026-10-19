<script setup lang="ts" generic="T extends string | number | boolean">
import { nextTick, ref } from 'vue'
import { nextRadioIndex } from '../core/radio-keys'

const props = defineProps<{ label: string; options: readonly { value: T; text: string }[]; modelValue: T }>()
const emit = defineEmits<{ 'update:modelValue': [value: T] }>()

const buttons = ref<HTMLButtonElement[]>([])

const onKeydown = async (event: KeyboardEvent, index: number): Promise<void> => {
  const target = props.options[nextRadioIndex(index, props.options.length, event.key)]
  if (!target) return
  event.preventDefault()
  emit('update:modelValue', target.value)
  await nextTick()
  buttons.value[props.options.indexOf(target)]?.focus()
}
</script>

<template>
  <div class="panel" role="radiogroup" :aria-label="label">
    <button
      v-for="(o, i) in options"
      :key="String(o.value)"
      ref="buttons"
      type="button"
      role="radio"
      :aria-checked="o.value === modelValue"
      :tabindex="o.value === modelValue ? 0 : -1"
      :class="{ on: o.value === modelValue }"
      @click="emit('update:modelValue', o.value)"
      @keydown="onKeydown($event, i)"
    >{{ o.text }}</button>
  </div>
</template>

<style scoped>
.panel { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 10px; }
.panel button { min-width: 36px; min-height: 36px; padding: 0 12px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; cursor: pointer; }
.panel button.on { border-color: var(--vp-c-brand-1); font-weight: 700; }
.panel button:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
</style>
