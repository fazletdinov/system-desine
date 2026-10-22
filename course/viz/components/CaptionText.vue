<script setup lang="ts">
import T from '../../components/T.vue'
import { captionSegments, type Caption } from '../core/caption'

// Общий рендер подписи: сегменты-термины превращаются в подсказки глоссария.
defineProps<{ caption: Caption }>()
</script>

<template>
  <template v-for="(seg, i) in captionSegments(caption)" :key="i"><T v-if="seg.termId" :id="seg.termId">{{ seg.text }}</T><template v-else>{{ seg.text }}</template></template>
</template>
