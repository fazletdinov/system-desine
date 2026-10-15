import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Simulation, Step } from '../core/types'

// Модель атаки марта 2025 года на tj-actions/changed-files: злоумышленник перевесил
// теги действия на свой коммит, и все workflow, закреплённые по тегу, выполнили его код.
// Код печатал секреты раннера в журнал сборки.

export type PinMode = 'tag' | 'sha'
export type PermMode = 'write-all' | 'minimal'

export interface ActionRef {
  name: string
  ref: string // что написано в uses:
}

export interface Executed {
  name: string
  malicious: boolean
}

export interface RunResult {
  executed: readonly Executed[]
  leaked: readonly string[]
}

export interface ChainState {
  pin: PinMode
  permissions: PermMode
  secretInEnv: boolean
  attacked: boolean
  actions: readonly ActionRef[]
  result: RunResult | null
  last: ChainAction['type'] | 'start'
}

export type ChainAction =
  | { type: 'pin'; mode: PinMode }
  | { type: 'permissions'; mode: PermMode }
  | { type: 'secret'; inEnv: boolean }
  | { type: 'attack' }
  | { type: 'run' }
  | { type: 'reset' }

const VICTIM = 'changed-files'

function refs(pin: PinMode): ActionRef[] {
  return pin === 'tag'
    ? [
        { name: 'checkout', ref: 'actions/checkout@v7' },
        { name: VICTIM, ref: 'example/changed-files@v46' },
        { name: 'setup-go', ref: 'actions/setup-go@v7' },
      ]
    : [
        { name: 'checkout', ref: 'actions/checkout@3d3c42e5… # v7.0.1' },
        { name: VICTIM, ref: 'example/changed-files@a1b2c3d4… # v46.0.1' },
        { name: 'setup-go', ref: 'actions/setup-go@b7ad1dad… # v7.0.0' },
      ]
}

function execute(s: ChainState): RunResult {
  // Тег — изменяемая ссылка: после подмены он указывает на коммит злоумышленника.
  // SHA — неизменяемая: тот же коммит, что был при ревью.
  const executed = s.actions.map((a) => ({ name: a.name, malicious: s.attacked && s.pin === 'tag' && a.name === VICTIM }))
  if (!executed.some((e) => e.malicious)) return { executed, leaked: [] }
  const leaked = [`GITHUB_TOKEN: ${s.permissions === 'write-all' ? 'contents:write' : 'contents:read'}`]
  if (s.secretInEnv) leaked.push('DEPLOY_KEY')
  return { executed, leaked }
}

const start = (): ChainState => ({ pin: 'tag', permissions: 'write-all', secretInEnv: true, attacked: false, actions: refs('tag'), result: null, last: 'start' })

export const supplyChainSimulation: Simulation<ChainState, ChainAction> = {
  initial: start,
  apply: (s, a) => {
    switch (a.type) {
      case 'reset':
        return start()
      case 'pin':
        return { ...s, pin: a.mode, actions: refs(a.mode), result: null, last: 'pin' }
      case 'permissions':
        return { ...s, permissions: a.mode, result: null, last: 'permissions' }
      case 'secret':
        return { ...s, secretInEnv: a.inEnv, result: null, last: 'secret' }
      case 'attack':
        return { ...s, attacked: true, result: null, last: 'attack' }
      case 'run':
        return { ...s, result: execute(s), last: 'run' }
    }
  },
  steps: (s) => [{ state: s, caption: captionFor(s) } satisfies Step<ChainState>],
}

function captionFor(s: ChainState): Caption {
  if (s.result === null)
    return [{ text: 'Настройте workflow, подмените тег одного action и запустите. Это модель реальной ' }, term('атаки на цепочку поставок', 'supply-chain-attack'), { text: ' 2025 года.' }]
  if (s.result.leaked.length === 0 && s.attacked)
    return [term('Закрепление по SHA', 'action-pinning'), { text: ': подменённый тег не изменил коммит, который выполнился. Ничего не утекло.' }]
  if (s.result.leaked.length === 0) return 'Атаки не было: выполнился исходный код всех actions.'
  return [
    { text: `Выполнился код злоумышленника. Утекло: ${s.result.leaked.join(', ')}. ` },
    term('Минимальные права', 'least-privilege'),
    { text: ' сужают ущерб, но не предотвращают атаку.' },
  ]
}
