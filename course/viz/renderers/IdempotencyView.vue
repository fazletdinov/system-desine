<script setup lang="ts">
import type { IdemState } from '../simulations/idempotency'

defineProps<{ state: IdemState }>()

const OUTCOME_TEXT = {
  created: 'создан чат',
  replayed: 'повтор: отдан сохранённый ответ',
  'in-progress': 'ключ занят, запрос ещё идёт',
  'key-reused': 'ключ уже использован с другим телом',
  'server-error': 'сбой, ключ освобождён',
} as const
</script>

<template>
  <div class="idem">
    <div class="cols">
      <section class="col client" aria-label="Клиент">
        <h4>Клиент</h4>
        <p v-if="state.loseNext" class="flag">Следующий ответ потеряется</p>
        <p v-if="state.mode === 'key'" class="meta">Заголовок <code>Idempotency-Key: k-1</code></p>
        <p v-else class="meta">Заголовка нет</p>
      </section>
      <section class="col net" aria-label="Сеть">
        <h4>Сеть</h4>
        <p class="meta">Ответ может не дойти — клиент не отличает это от потерянного запроса.</p>
      </section>
      <section class="col server" aria-label="Сервер">
        <h4>Сервер</h4>
        <p v-if="state.failNext" class="flag">Следующий запрос упадёт с 500</p>
        <p class="chats" :class="{ dup: state.mode === 'no-key' && state.chats > 1 }">Создано чатов: <b>{{ state.chats }}</b></p>
        <p v-if="state.mode === 'no-key' && state.chats > 1" class="bad">дубль: один чат создан дважды</p>
        <div v-if="state.mode === 'key'" class="store">
          <b>Хранилище ключей</b>
          <p v-if="state.record === null" class="meta">пусто</p>
          <p v-else class="meta">k-1 → тело {{ state.record.body }}, {{ state.record.done ? `готово, ответ ${state.record.status}` : 'в работе' }}</p>
        </div>
      </section>
    </div>
    <ol class="log" aria-label="Попытки">
      <li v-for="a in state.attempts" :key="a.n" :class="{ lost: !a.delivered }">
        <b>#{{ a.n }}</b> тело {{ a.body }} → <b>{{ a.status }}</b>
        <span class="arrow" :class="{ cut: !a.delivered }" :aria-label="a.delivered ? 'ответ доставлен' : 'ответ потерян'">←</span>
        <span class="meta">{{ a.delivered ? 'доставлен' : 'потерян' }} · {{ OUTCOME_TEXT[a.outcome] }}</span>
      </li>
      <li v-if="state.attempts.length === 0" class="meta">Попыток пока нет.</li>
    </ol>
  </div>
</template>

<style scoped>
.idem { container-type: inline-size; color: var(--viz-text); }
.cols { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; margin-bottom: 10px; }
.col { padding: 8px 10px; border: 2px solid var(--viz-edge); border-radius: 10px; background: var(--viz-node-bg); min-width: 0; }
.client { border-color: var(--viz-client); }
.net { border-color: var(--viz-network); border-style: dashed; }
.server { border-color: var(--viz-service); }
h4 { margin: 0 0 4px; font-size: 15px; }
.meta { margin: 2px 0; font-size: 13px; color: var(--viz-muted); overflow-wrap: anywhere; }
.flag { margin: 2px 0; font-size: 13px; font-weight: 700; color: var(--viz-cache); }
.chats { margin: 2px 0; font-size: 14px; }
.chats b { font-size: 28px; font-variant-numeric: tabular-nums; }
.chats.dup b, .bad { color: var(--viz-error); }
.bad { margin: 2px 0; font-size: 13px; font-weight: 700; }
.store { margin-top: 6px; padding-top: 6px; border-top: 1px dashed var(--viz-edge); font-size: 13px; }
.log { margin: 0; padding-left: 22px; font-size: 14px; display: flex; flex-direction: column; gap: 2px; }
.arrow { margin: 0 4px; position: relative; }
.arrow.cut::after { content: ''; position: absolute; left: -2px; right: -2px; top: 50%; border-top: 2px solid var(--viz-error); transform: rotate(-20deg); }
.lost b { color: var(--viz-error); }
code { font-size: 12px; overflow-wrap: anywhere; }
@container (max-width: 520px) { .cols { grid-template-columns: 1fr; } }
</style>
