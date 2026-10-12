<script setup lang="ts">
import { computed } from 'vue'
import { data as glossary } from '../glossary/terms.data'
import { sortTerms } from '../glossary/schema'

const terms = computed(() => sortTerms(Object.values(glossary)))
</script>

<template>
  <dl class="glossary">
    <template v-for="t in terms" :key="t.id">
      <dt :id="t.id">
        <a class="anchor" :href="`#${t.id}`" :aria-label="`Ссылка на термин ${t.term}`">#</a>
        {{ t.term }}<span v-if="t.en" class="en"> — {{ t.en }}</span>
      </dt>
      <dd>
        <p>{{ t.short }}</p>
        <div v-if="t.longHtml" class="long" v-html="t.longHtml" />
        <ul v-if="t.links.length" class="links">
          <li v-for="link in t.links" :key="link.title">
            <a v-if="link.url" :href="link.url" target="_blank" rel="noopener">{{ link.title }}</a>
            <span v-else>{{ link.title }}</span>
          </li>
        </ul>
      </dd>
    </template>
  </dl>
</template>

<style scoped>
.glossary dt { margin-top: 28px; font-size: 18px; font-weight: 600; scroll-margin-top: calc(var(--vp-nav-height) + 16px); }
.glossary dt:target { color: var(--vp-c-brand-1); }
.anchor { margin-right: 6px; color: var(--vp-c-text-3); text-decoration: none; }
.en { color: var(--vp-c-text-2); font-weight: 400; }
.glossary dd { margin: 6px 0 0; }
.links { font-size: 14px; color: var(--vp-c-text-2); }
</style>
