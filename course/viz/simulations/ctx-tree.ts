import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Simulation, Step } from '../core/types'

export type CtxId = 'root' | 'request' | 'usecase' | 'store' | 'userchecker' | 'audit'
export type CtxKind = 'background' | 'timeout' | 'value' | 'without-cancel'

export interface CtxNode {
  id: CtxId
  parent: CtxId | null
  kind: CtxKind
  timeoutMs?: number
  label: string
  work: string
}

// Сценарий урока 1.5: запрос с таймаутом 1 с, проверка участников с таймаутом 100 мс,
// аудит, который должен дописаться, даже если клиент ушёл.
export const CTX_NODES: readonly CtxNode[] = [
  { id: 'root', parent: null, kind: 'background', label: 'context.Background()', work: 'main' },
  { id: 'request', parent: 'root', kind: 'timeout', timeoutMs: 1000, label: 'WithTimeout(r.Context(), 1s)', work: 'HTTP-обработчик' },
  { id: 'usecase', parent: 'request', kind: 'value', label: 'тот же ctx', work: 'SendMessage.Execute' },
  { id: 'store', parent: 'usecase', kind: 'value', label: 'тот же ctx', work: 'хранилище: AppendMessage' },
  { id: 'userchecker', parent: 'usecase', kind: 'timeout', timeoutMs: 100, label: 'WithTimeout(ctx, 100ms)', work: 'UserChecker' },
  { id: 'audit', parent: 'request', kind: 'without-cancel', label: 'WithoutCancel(ctx)', work: 'запись в аудит' },
]

const node = (id: CtxId): CtxNode => CTX_NODES.find((n) => n.id === id) as CtxNode
const children = (id: CtxId): CtxNode[] => CTX_NODES.filter((n) => n.parent === id)

export function effectiveDeadline(id: CtxId): number {
  const n = node(id)
  if (n.kind === 'background' || n.kind === 'without-cancel') return n.timeoutMs ?? Infinity
  const parent = n.parent === null ? Infinity : effectiveDeadline(n.parent)
  return Math.min(parent, n.timeoutMs ?? Infinity)
}

export type Cause = 'canceled' | 'deadline'
export interface CtxState { nowMs: number; done: Partial<Record<CtxId, { cause: Cause; atMs: number }>>; last: string }
export type CtxAction = { type: 'cancel'; id: CtxId } | { type: 'advance'; ms: number } | { type: 'reset' }

function finish(done: CtxState['done'], id: CtxId, cause: Cause, atMs: number): CtxState['done'] {
  if (done[id] !== undefined || node(id).kind === 'background') return done
  let next = { ...done, [id]: { cause, atMs } }
  // WithoutCancel обрывает распространение: потомок не видит отмену родителя.
  for (const c of children(id)) if (c.kind !== 'without-cancel') next = finish(next, c.id, cause, atMs)
  return next
}

export const ctxTreeSimulation: Simulation<CtxState, CtxAction> = {
  initial: () => ({ nowMs: 0, done: {}, last: 'start' }),
  apply: (s, a) => {
    if (a.type === 'reset') return ctxTreeSimulation.initial()
    if (a.type === 'cancel') return { ...s, done: finish(s.done, a.id, 'canceled', s.nowMs), last: `cancel:${a.id}` }
    const nowMs = s.nowMs + a.ms
    let done = s.done
    // Сначала самые ранние дедлайны: причина у потомка — его собственный дедлайн, если он раньше.
    const byDeadline = [...CTX_NODES].sort((x, y) => effectiveDeadline(x.id) - effectiveDeadline(y.id))
    for (const n of byDeadline) {
      const d = effectiveDeadline(n.id)
      if (d <= nowMs) done = finish(done, n.id, 'deadline', d)
    }
    return { nowMs, done, last: `advance:${a.ms}` }
  },
  steps: (s) => [{ state: s, caption: captionFor(s) } satisfies Step<CtxState>],
}

function captionFor(s: CtxState): Caption {
  if (s.last === 'start')
    return [
      { text: 'Дерево ' }, term('контекстов', 'go-context'),
      { text: ': каждый WithTimeout или WithCancel создаёт потомка. Отмена идёт только вниз — от родителя к детям. Отмените узел или промотайте время.' },
    ]
  const stopped = CTX_NODES.filter((n) => s.done[n.id] !== undefined).map((n) => n.work)
  const live = CTX_NODES.filter((n) => n.kind !== 'background' && s.done[n.id] === undefined).map((n) => n.work)
  const what = s.last.startsWith('cancel') ? 'Вызвали cancel()' : `Прошло ${s.nowMs} мс`
  return [
    { text: `${what}. Остановились: ${stopped.length > 0 ? stopped.join(', ') : 'никто'}. Работают: ${live.length > 0 ? live.join(', ') : 'никто'}. ` },
    term('Дедлайн', 'deadline'),
    { text: ' потомка не может быть позже родительского, а ' },
    term('отмена', 'cancellation'),
    { text: ' потомка не трогает родителя.' },
  ]
}
