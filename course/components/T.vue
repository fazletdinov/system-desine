<script setup lang="ts">
import { computed, useId } from 'vue'
import { withBase } from 'vitepress'
import { data as glossary } from '../glossary/terms.data'

const props = defineProps<{ id: string }>()
const term = computed(() => glossary[props.id])
const href = computed(() => withBase(`/glossary#${props.id}`))
const tipId = useId()
</script>

<template>
  <span class="term">
    <a class="term-link" :href="href" :aria-describedby="term ? tipId : undefined"><slot /></a>
    <span v-if="term" :id="tipId" role="tooltip" class="term-tip">
      <strong>{{ term.term }}</strong>
      <span v-if="term.en" class="term-en"> ({{ term.en }})</span>
      <span class="term-short">{{ term.short }}</span>
    </span>
  </span>
</template>

<style scoped>
.term { position: relative; }
.term-link {
  color: inherit;
  text-decoration: underline dotted var(--vp-c-brand-1);
  text-decoration-thickness: 2px;
  text-underline-offset: 3px;
  font-weight: inherit;
}
.term-tip {
  display: none;
  position: absolute;
  z-index: 20;
  left: 0;
  top: calc(100% + 6px);
  width: max-content;
  max-width: min(340px, 80vw);
  padding: 10px 12px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 8px;
  background: var(--vp-c-bg-elv);
  box-shadow: var(--vp-shadow-3);
  color: var(--vp-c-text-1);
  font-size: 14px;
  line-height: 1.5;
  font-weight: 400;
}
.term-en { color: var(--vp-c-text-2); }
.term-short { display: block; margin-top: 4px; }
.term:hover .term-tip,
.term:focus-within .term-tip { display: block; }

/* На узком экране подсказка прижимается к низу окна — иначе уходит за край. */
@media (max-width: 640px) {
  .term-tip { position: fixed; left: 16px; right: 16px; top: auto; bottom: 16px; width: auto; max-width: none; }
}
</style>
