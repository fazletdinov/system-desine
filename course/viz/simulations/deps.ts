import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Simulation, Step } from '../core/types'

export type Pkg =
  | 'user/domain' | 'user/usecase' | 'user/adapters/memory' | 'user/adapters/httpapi'
  | 'chat/domain' | 'chat/usecase' | 'chat/adapters/memory' | 'chat/adapters/httpapi' | 'chat/adapters/userchecker'
  | 'platform/clock' | 'cmd/messenger' | 'net/http' | 'database/sql'

export type Layer = 'domain' | 'usecase' | 'adapter' | 'platform' | 'root' | 'stdlib'

export const PACKAGES: readonly { id: Pkg; layer: Layer }[] = [
  { id: 'user/domain', layer: 'domain' }, { id: 'chat/domain', layer: 'domain' },
  { id: 'user/usecase', layer: 'usecase' }, { id: 'chat/usecase', layer: 'usecase' },
  { id: 'user/adapters/memory', layer: 'adapter' }, { id: 'user/adapters/httpapi', layer: 'adapter' },
  { id: 'chat/adapters/memory', layer: 'adapter' }, { id: 'chat/adapters/httpapi', layer: 'adapter' },
  { id: 'chat/adapters/userchecker', layer: 'adapter' },
  { id: 'platform/clock', layer: 'platform' }, { id: 'cmd/messenger', layer: 'root' },
  { id: 'net/http', layer: 'stdlib' }, { id: 'database/sql', layer: 'stdlib' },
]

export type Edge = readonly [Pkg, Pkg]

// Импорты настоящего мессенджера на теге m01-l07 (stdlib не показан, кроме net/http).
export const BASE_IMPORTS: readonly Edge[] = [
  ['user/usecase', 'user/domain'], ['chat/usecase', 'chat/domain'],
  ['user/adapters/memory', 'user/domain'], ['user/adapters/httpapi', 'user/usecase'], ['user/adapters/httpapi', 'user/domain'],
  ['chat/adapters/memory', 'chat/domain'], ['chat/adapters/httpapi', 'chat/usecase'], ['chat/adapters/httpapi', 'chat/domain'],
  ['chat/adapters/userchecker', 'chat/domain'], ['chat/adapters/userchecker', 'user/usecase'], ['chat/adapters/userchecker', 'user/domain'],
  ['user/adapters/httpapi', 'net/http'], ['chat/adapters/httpapi', 'net/http'],
  ['cmd/messenger', 'user/usecase'], ['cmd/messenger', 'chat/usecase'], ['cmd/messenger', 'user/adapters/memory'],
  ['cmd/messenger', 'user/adapters/httpapi'], ['cmd/messenger', 'chat/adapters/memory'], ['cmd/messenger', 'chat/adapters/httpapi'],
  ['cmd/messenger', 'chat/adapters/userchecker'], ['cmd/messenger', 'platform/clock'], ['cmd/messenger', 'net/http'],
]

export type Rule = 'cycle' | 'domain-pure' | 'usecase-ports' | 'adapter-adapter' | 'cross-context' | 'platform-pure'

export interface Violation {
  rule: Rule
  title: string
  consequence: string
}

const layerOf = (p: Pkg): Layer => PACKAGES.find((x) => x.id === p)?.layer ?? 'stdlib'
const contextOf = (p: Pkg): 'user' | 'chat' | null => (p.startsWith('user/') ? 'user' : p.startsWith('chat/') ? 'chat' : null)

function reachable(from: Pkg, to: Pkg, edges: readonly Edge[]): boolean {
  const seen = new Set<Pkg>([from])
  const queue: Pkg[] = [from]
  while (queue.length > 0) {
    const cur = queue.shift() as Pkg
    if (cur === to) return true
    for (const [a, b] of edges) if (a === cur && !seen.has(b)) { seen.add(b); queue.push(b) }
  }
  return false
}

const V = (rule: Rule, title: string, consequence: string): Violation => ({ rule, title, consequence })

// Порядок проверок важен: цикл — ошибка компиляции, она важнее нарушения стиля.
export function checkImport(from: Pkg, to: Pkg, edges: readonly Edge[]): Violation | null {
  if (from === to) return V('cycle', 'Пакет импортирует сам себя', 'Go не соберёт проект: import cycle not allowed.')
  if (reachable(to, from, edges))
    return V('cycle', 'Цикл импортов', `${to} уже зависит от ${from}. Go запрещает циклы: компилятор остановится с ошибкой import cycle not allowed.`)
  const fl = layerOf(from), tl = layerOf(to)
  if (fl === 'domain' && to !== from)
    return V('domain-pure', 'Домен зависит от внешнего мира',
      `Домен должен импортировать только стандартную библиотеку без транспорта и хранилищ. С этим импортом смена ${to} заставит переписывать бизнес-правила, а тесты домена потянут за собой инфраструктуру.`)
  if (fl === 'usecase' && (tl === 'adapter' || tl === 'platform' || tl === 'stdlib'))
    return V('usecase-ports', 'Use case зависит от конкретики',
      `Use case должен видеть только порты — интерфейсы, объявленные у него же. Зависимость от ${to} привяжет сценарий к одной реализации: заменить хранилище на Postgres или подменить часы в тесте станет нельзя без правки use case.`)
  if (fl === 'adapter' && tl === 'adapter')
    return V('adapter-adapter', 'Адаптер знает о другом адаптере',
      'Адаптеры — сменные детали снаружи. Если HTTP-адаптер знает про хранилище в памяти, замена хранилища сломает транспорт. Связь между ними идёт только через use case.')
  const fc = contextOf(from), tc = contextOf(to)
  if (fc !== null && tc !== null && fc !== tc && !(from === 'chat/adapters/userchecker' && (to === 'user/usecase' || to === 'user/domain')))
    return V('cross-context', 'Один контекст лезет внутрь другого',
      'Контексты user и chat должны разделиться на сервисы в модуле 6. Прямой импорт склеивает их: при распиле придётся переписывать код. Связь — через порт UserChecker и единственный адаптер userchecker, который станет gRPC-клиентом.')
  if (fl === 'platform' && fc === null && tc !== null)
    return V('platform-pure', 'Технический пакет знает про бизнес',
      'platform — общие технические детали (часы, генератор id). Зависимость от бизнес-контекста превращает его в свалку и тянет бизнес-логику во все сервисы.')
  return null
}

export const CANDIDATES: readonly { from: Pkg; to: Pkg }[] = [
  { from: 'chat/domain', to: 'net/http' }, { from: 'chat/domain', to: 'database/sql' },
  { from: 'chat/domain', to: 'chat/adapters/memory' }, { from: 'chat/usecase', to: 'chat/adapters/memory' },
  { from: 'chat/usecase', to: 'platform/clock' }, { from: 'chat/usecase', to: 'user/usecase' },
  { from: 'user/adapters/memory', to: 'user/adapters/httpapi' }, { from: 'chat/adapters/userchecker', to: 'user/adapters/memory' },
  { from: 'platform/clock', to: 'chat/domain' }, { from: 'chat/adapters/httpapi', to: 'chat/usecase' },
  { from: 'chat/adapters/memory', to: 'chat/domain' },
]

export interface DepsState { added: readonly Edge[] }
export type DepsAction = { type: 'add'; from: Pkg; to: Pkg } | { type: 'reset' }
export interface DepsFrame { edges: readonly Edge[]; last: Edge | null; violation: Violation | null }

const intro: Caption = [
  { text: 'Граф импортов мессенджера. Стрелки идут только внутрь — от адаптеров к use case и от use case к домену. Это ' },
  term('правило зависимостей', 'dependency-rule'),
  { text: '. Добавьте импорт из списка и посмотрите, что скажет проверка.' },
]

export const depsSimulation: Simulation<DepsState, DepsAction, DepsFrame> = {
  initial: () => ({ added: [] }),
  apply: (state, action) => (action.type === 'reset' ? { added: [] } : { added: [...state.added, [action.from, action.to]] }),
  steps: (state) => {
    const steps: Step<DepsFrame>[] = [{ state: { edges: BASE_IMPORTS, last: null, violation: null }, caption: intro }]
    let edges: readonly Edge[] = BASE_IMPORTS
    for (const edge of state.added) {
      const violation = checkImport(edge[0], edge[1], edges)
      edges = [...edges, edge]
      const caption: Caption = violation === null
        ? `${edge[0]} → ${edge[1]}: разрешено. Зависимость направлена внутрь — внешний слой знает о внутреннем, а не наоборот.`
        : `${edge[0]} → ${edge[1]}: ${violation.title}. ${violation.consequence}`
      steps.push({ state: { edges, last: edge, violation }, caption })
    }
    return steps
  },
}
