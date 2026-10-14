<script setup lang="ts">
import { computed } from 'vue'
import { encodeToken, verify, type JwtState, type Rule } from '../simulations/jwt'

const props = defineProps<{ state: JwtState }>()

const RULE_NAME: Record<Rule, string> = { ok: 'все проверки', alg: 'alg', kid: 'kid', signature: 'подпись', exp: 'exp', iat: 'iat', nbf: 'nbf', aud: 'aud', iss: 'iss' }

const segments = computed(() => {
  const [h, p, s] = encodeToken(props.state).split('.')
  return { h, p, s }
})
const verdict = computed(() => verify(props.state))
const headerJson = computed(() => JSON.stringify(props.state.header, null, 2))
const payloadJson = computed(() => JSON.stringify(props.state.payload, null, 2))
</script>

<template>
  <div class="jwt">
    <p class="token" aria-label="Токен"><span class="h">{{ segments.h }}</span>.<span class="p">{{ segments.p }}</span>.<span class="s">{{ segments.s }}</span></p>
    <div class="legend">
      <span class="h">заголовок</span>
      <span class="p">payload (claims)</span>
      <span class="s">подпись</span>
    </div>
    <div class="decoded">
      <pre class="h" aria-label="Заголовок">{{ headerJson }}</pre>
      <pre class="p" aria-label="Payload">{{ payloadJson }}</pre>
    </div>
    <p class="meta">Часы сервера: {{ state.clockOffsetSec === 0 ? 'идут верно' : `${state.clockOffsetSec > 0 ? '+' : '−'}${Math.abs(state.clockOffsetSec)} с к моменту выпуска` }}</p>
    <p class="verdict" :class="verdict.ok ? 'ok' : 'no'" role="status">
      <b>{{ verdict.ok ? '✓ принят' : '✗ отклонён' }}</b>
      <span> · правило: {{ RULE_NAME[verdict.rule] }}</span>
    </p>
  </div>
</template>

<style scoped>
.jwt { container-type: inline-size; color: var(--viz-text); }
.token { margin: 0 0 6px; font-family: var(--vp-font-family-mono); font-size: 13px; line-height: 1.5; overflow-wrap: anywhere; word-break: break-all; }
.h { color: var(--viz-error); }
.p { color: var(--viz-service); }
.s { color: var(--viz-broker); }
.legend { display: flex; flex-wrap: wrap; gap: 4px 14px; margin-bottom: 10px; font-size: 13px; font-weight: 600; }
.decoded { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
pre { margin: 0; padding: 8px; border: 2px solid currentColor; border-radius: 8px; background: var(--viz-node-bg); font-size: 12px; white-space: pre-wrap; overflow-wrap: anywhere; }
.meta { margin: 8px 0; font-size: 13px; color: var(--viz-muted); }
.verdict { margin: 0; padding: 8px 12px; border: 2px solid; border-radius: 8px; background: var(--viz-node-bg); font-size: 15px; }
.verdict.ok { border-color: var(--viz-db); }
.verdict.ok b { color: var(--viz-db); }
.verdict.no { border-color: var(--viz-error); }
.verdict.no b { color: var(--viz-error); }
@container (max-width: 520px) { .decoded { grid-template-columns: 1fr; } }
</style>
