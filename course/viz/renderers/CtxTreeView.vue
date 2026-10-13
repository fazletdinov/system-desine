<script setup lang="ts">
import { computed } from 'vue'
import { CTX_NODES, effectiveDeadline, type CtxId, type CtxNode, type CtxState } from '../simulations/ctx-tree'

const props = defineProps<{ state: CtxState }>()
const emit = defineEmits<{ cancel: [id: CtxId] }>()

const CAUSE_TEXT = { canceled: 'context.Canceled', deadline: 'context.DeadlineExceeded' } as const

interface Row { node: CtxNode; depth: number }

// Узлы в порядке обхода в глубину: отступ показывает вложенность, без рекурсивного компонента.
function flatten(parent: CtxId | null, depth: number): Row[] {
  return CTX_NODES.filter((n) => n.parent === parent).flatMap((node) => [{ node, depth }, ...flatten(node.id, depth + 1)])
}
const rows = flatten(null, 0)

function deadlineText(id: CtxId): string {
  const d = effectiveDeadline(id)
  return Number.isFinite(d) ? `до ${d} мс` : 'без дедлайна'
}

const view = computed(() =>
  rows.map(({ node, depth }) => {
    const done = props.state.done[node.id]
    return {
      node,
      depth,
      deadline: deadlineText(node.id),
      status: done === undefined ? 'работает' : CAUSE_TEXT[done.cause],
      stopped: done !== undefined,
      stoppedAt: done?.atMs,
    }
  }),
)
</script>

<template>
  <div class="ctx">
    <p class="clock">t = {{ state.nowMs }} мс</p>
    <ul class="tree" aria-label="Дерево контекстов запроса">
      <li v-for="r in view" :key="r.node.id" class="node" :class="[r.node.kind, { stopped: r.stopped }]" :style="{ marginLeft: `calc(${r.depth} * var(--ctx-indent))` }">
        <div class="head">
          <span class="work">{{ r.node.work }}</span>
          <span class="status" :class="r.stopped ? 'stop' : 'run'">{{ r.stopped ? '■' : '▶' }} {{ r.status }}<template v-if="r.stoppedAt !== undefined"> (t = {{ r.stoppedAt }} мс)</template></span>
        </div>
        <code class="label">{{ r.node.label }}</code>
        <div class="foot">
          <span class="meta">{{ r.deadline }}</span>
          <span v-if="r.node.kind === 'without-cancel'" class="meta detached">отвязан от отмены родителя</span>
          <button v-if="r.node.id !== 'root'" type="button" class="cancel" :disabled="r.stopped" :aria-label="`Отменить: ${r.node.work}`" @click="emit('cancel', r.node.id)">Отменить</button>
        </div>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.ctx { --ctx-indent: 22px; }
.clock { margin: 0 0 8px; font-size: 16px; font-weight: 700; font-variant-numeric: tabular-nums; color: var(--viz-text); }
.tree { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 8px; }
.node { margin-top: 0; padding: 8px 12px; border: 2px solid var(--viz-service); border-radius: 10px; background: var(--viz-node-bg); color: var(--viz-text); }
.node.background { border-color: var(--viz-network); }
.node.timeout { border-color: var(--viz-cache); }
.node.without-cancel { border-color: var(--viz-broker); border-style: dashed; }
.node.stopped { border-color: var(--viz-error); }
.head { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 4px 12px; }
.work { font-weight: 600; }
.status { font-size: 14px; font-weight: 600; }
.status.run { color: var(--viz-text); }
.status.stop { color: var(--viz-error); }
.label { display: block; margin: 2px 0; font-size: 13px; color: var(--viz-muted); overflow-wrap: anywhere; background: none; padding: 0; }
.foot { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 12px; }
.meta { font-size: 13px; color: var(--viz-muted); }
.meta.detached { color: var(--viz-text); font-weight: 600; }
.cancel { margin-left: auto; min-height: 36px; padding: 0 12px; border: 2px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); color: var(--vp-c-text-1); font: inherit; cursor: pointer; }
.cancel:disabled { opacity: 0.45; cursor: not-allowed; }
.cancel:focus-visible { outline: 3px solid var(--vp-c-brand-1); outline-offset: 2px; }
@media (max-width: 520px) { .ctx { --ctx-indent: 12px; } }
</style>
