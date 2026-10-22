<script setup lang="ts">
import { ref } from 'vue'
import type { StepQuiz } from '../core/types'
import CaptionText from './CaptionText.vue'

// Вид и поведение как у SelfCheck.vue; сброс при смене шага — через :key в Player.
defineProps<{ quiz: StepQuiz }>()
const open = ref(false)
</script>

<template>
  <div class="step-quiz">
    <p class="question"><strong>Вопрос.</strong> <CaptionText :caption="quiz.question" /></p>
    <button type="button" class="toggle" :aria-expanded="open" @click="open = !open">
      {{ open ? 'Скрыть ответ' : 'Сначала ответьте сами, затем откройте ответ' }}
    </button>
    <p v-show="open" class="answer"><CaptionText :caption="quiz.answer" /></p>
  </div>
</template>

<style scoped>
.step-quiz { margin: 12px 0 0; padding: 14px 18px; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); }
.question { margin: 0 0 10px; white-space: pre-line; }
.toggle { padding: 6px 12px; border: 1px solid var(--vp-c-brand-1); border-radius: 6px; color: var(--vp-c-brand-1); font-size: 14px; }
.toggle:hover { background: var(--vp-c-brand-soft); }
.answer { margin: 12px 0 0; white-space: pre-line; }
</style>
