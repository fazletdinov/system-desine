<script setup lang="ts">
defineProps<{ options: readonly { id: string; label: string }[]; modelValue: string; label: string }>()
const emit = defineEmits<{ 'update:modelValue': [id: string] }>()
</script>

<template>
  <div class="variants" role="radiogroup" :aria-label="label">
    <button
      v-for="o in options"
      :key="o.id"
      type="button"
      role="radio"
      :aria-checked="o.id === modelValue"
      :class="{ on: o.id === modelValue }"
      @click="emit('update:modelValue', o.id)"
    >{{ o.label }}</button>
  </div>
</template>

<style scoped>
.variants { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 12px; }
.variants button { min-height: 40px; padding: 0 14px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; cursor: pointer; }
.variants button.on { border-color: var(--vp-c-brand-1); font-weight: 700; }
.variants button.on::before { content: '✓ '; }
.variants button:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
</style>
