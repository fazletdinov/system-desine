<script setup lang="ts">
import T from '../../components/T.vue'
import { captionSegments, type Caption } from '../core/caption'
import { keepDigitGroups } from '../core/format'

// Общий рендер подписи: сегменты-термины превращаются в подсказки глоссария,
// разряды чисел склеиваются, чтобы «16 384» не рвалось на две строки.
defineProps<{ caption: Caption }>()
</script>

<template>
  <template v-for="(seg, i) in captionSegments(caption)" :key="i"><T v-if="seg.termId" :id="seg.termId">{{ keepDigitGroups(seg.text) }}</T><template v-else>{{ keepDigitGroups(seg.text) }}</template></template>
</template>
