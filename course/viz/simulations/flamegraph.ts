import type { Caption, CaptionSegment } from '../core/caption'
import { term } from '../core/caption'
import { formatNs, formatNumber } from '../core/format'
import type { KeyModifiers } from '../core/radio-keys'
import type { Simulation, Step } from '../core/types'
import rawData from '../data/flame-messenger.json'

// Данные — реальные профили мессенджера, свёрнутые программой labs/1-5/fold из вывода
// go tool pprof -traces. В файле — общая таблица кадров frames и по объекту на тип выборки,
// где стек — индексы в frames от корня к листу ({"f": [0, 4, 9], "v": …}): имена функций
// повторяются в сотнях стеков, и индексы ужимают файл в несколько раз. Разбор ниже
// разворачивает индексы в имена — дальше модуль работает с FoldedProfile.
export const SAMPLE_KINDS = ['cpu', 'alloc_space', 'inuse_space'] as const
export type SampleKind = (typeof SAMPLE_KINDS)[number]

export interface FoldedStack {
  frames: readonly string[]
  value: number
}

export interface FoldedProfile {
  sampleType: string
  unit: string
  total: number
  stacks: readonly FoldedStack[]
}

export type FlameData = Record<SampleKind, FoldedProfile>

// Кадр, в который fold слил стеки тоньше порога.
export const OTHER_FRAME = 'прочее'
export const ROOT_NAME = 'все образцы'

// Чей код в кадре — цвет блока. root и other — служебные блоки, а не код.
export type FrameKind = 'messenger' | 'runtime' | 'stdlib' | 'external' | 'other' | 'root'

export interface FlameNode {
  id: string // путь индексов детей от корня: «r», «r.0», «r.0.3» — уникален и стабилен для данных
  name: string
  kind: FrameKind
  value: number // с учётом вызванных функций
  self: number // в самой функции: value минус сумма детей
  start: number // левый край в единицах значения корня: дети лежат подряд внутри родителя
  depth: number
  parentId: string | null
  children: FlameNode[]
}

export interface FlameTree {
  sampleType: SampleKind
  unit: string
  root: FlameNode
  byId: ReadonlyMap<string, FlameNode>
}

const UNITS = new Set(['nanoseconds', 'bytes', 'count'])

function fail(where: string, what: string): never {
  throw new Error(`flame-messenger.json, ${where}: ${what}`)
}

// Проверка формы данных: файл снимается руками и мог разойтись с ожиданиями модуля.
export function parseFrames(raw: unknown): readonly string[] {
  if (!Array.isArray(raw) || raw.length === 0) fail('frames', 'нет таблицы кадров')
  for (const [i, f] of raw.entries()) if (typeof f !== 'string' || f === '') fail('frames', `кадр ${i}: ${String(f)}`)
  return raw as string[]
}

interface CompactProfile {
  sampleType: unknown
  unit: unknown
  total: unknown
  stacks: unknown
}

export function parseProfile(raw: unknown, kind: SampleKind, frames: readonly string[]): FoldedProfile {
  const p = raw as Partial<CompactProfile> | null
  if (p === null || typeof p !== 'object') fail(kind, 'не объект')
  if (p.sampleType !== kind) fail(kind, `sampleType ${String(p.sampleType)}, ждали ${kind}`)
  if (typeof p.unit !== 'string' || !UNITS.has(p.unit)) fail(kind, `незнакомая единица ${String(p.unit)}`)
  if (!Array.isArray(p.stacks) || p.stacks.length === 0) fail(kind, 'нет стеков')
  const stacks: FoldedStack[] = []
  let sum = 0
  for (const [i, s] of (p.stacks as { f?: unknown; v?: unknown }[]).entries()) {
    const ids = s?.f
    const ok = Array.isArray(ids) && ids.length > 0 && ids.every((id: unknown) => Number.isInteger(id) && (id as number) >= 0 && (id as number) < frames.length)
    if (!ok) fail(kind, `стек ${i}: кадры`)
    const v = s.v
    if (typeof v !== 'number' || !Number.isSafeInteger(v) || v <= 0) fail(kind, `стек ${i}: значение ${String(v)}`)
    stacks.push({ frames: (ids as number[]).map((id) => frames[id]!), value: v })
    sum += v
  }
  if (p.total !== sum) fail(kind, `total ${String(p.total)} не равен сумме стеков ${sum}`)
  return { sampleType: kind, unit: p.unit, total: sum, stacks }
}

export function parseFlameData(raw: unknown): FlameData {
  const obj = raw as Record<string, unknown>
  const frames = parseFrames(obj.frames)
  return {
    cpu: parseProfile(obj.cpu, 'cpu', frames),
    alloc_space: parseProfile(obj.alloc_space, 'alloc_space', frames),
    inuse_space: parseProfile(obj.inuse_space, 'inuse_space', frames),
  }
}

// Пакет функции: «net/http.(*conn).serve» → «net/http». Точку ищем после последнего «/»
// до скобок: внутри «(*T)» и параметров дженериков тоже бывают точки и слеши.
export function packageOf(name: string): string {
  const brackets = [name.indexOf('('), name.indexOf('[')].filter((i) => i >= 0)
  const head = name.slice(0, brackets.length > 0 ? Math.min(...brackets) : name.length)
  const dot = head.indexOf('.', head.lastIndexOf('/') + 1)
  return dot < 0 ? '' : head.slice(0, dot)
}

// Модуль мессенджера называется messenger, его main — cmd/messenger. Символ без пакета
// (memeqbody, aeshashbody) — ассемблер рантайма.
export function frameKind(name: string): FrameKind {
  if (name === OTHER_FRAME) return 'other'
  const pkg = packageOf(name)
  if (pkg === 'main' || pkg === 'messenger' || pkg.startsWith('messenger/')) return 'messenger'
  if (pkg === '' || pkg === 'runtime' || pkg.startsWith('internal/runtime/')) return 'runtime'
  return pkg.split('/')[0]!.includes('.') ? 'external' : 'stdlib'
}

export function buildTree(profile: FoldedProfile, kind: SampleKind): FlameTree {
  const root: FlameNode = { id: 'r', name: ROOT_NAME, kind: 'root', value: 0, self: 0, start: 0, depth: 0, parentId: null, children: [] }
  for (const s of profile.stacks) {
    root.value += s.value
    let node = root
    for (const name of s.frames) {
      let child = node.children.find((c) => c.name === name)
      if (child === undefined) {
        child = { id: '', name, kind: frameKind(name), value: 0, self: 0, start: 0, depth: node.depth + 1, parentId: node.id, children: [] }
        node.children.push(child)
      }
      child.value += s.value
      node = child
    }
  }
  const byId = new Map<string, FlameNode>()
  // Порядок блоков по горизонтали в флейм-графе смысла не несёт — сортируем по имени,
  // как Brendan Gregg и pprof: одинаковые функции соседних запусков встают на одно место.
  const place = (node: FlameNode): void => {
    byId.set(node.id, node)
    node.children.sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0))
    let offset = node.start
    let childSum = 0
    node.children.forEach((c, i) => {
      c.id = `${node.id}.${i}`
      c.parentId = node.id
      c.start = offset
      offset += c.value
      childSum += c.value
      place(c)
    })
    node.self = node.value - childSum
  }
  place(root)
  return { sampleType: kind, unit: profile.unit, root, byId }
}

export function pathTo(tree: FlameTree, id: string): FlameNode[] {
  const path: FlameNode[] = []
  for (let n = tree.byId.get(id); n !== undefined; n = n.parentId === null ? undefined : tree.byId.get(n.parentId)) path.unshift(n)
  return path
}

export interface ViewWindow {
  start: number
  end: number
}

export const windowOf = (node: FlameNode): ViewWindow => ({ start: node.start, end: node.start + node.value })

export interface ViewRect {
  node: FlameNode
  x: number // 0…1 от ширины графа
  w: number
}

// Блоки, видимые в окне [start, end) значений корня, в долях ширины. Окно увеличенного
// узла даёт ему ширину 1, а его поддереву — те же доли, что и без увеличения; окно между
// двумя узлами — кадр анимации увеличения. Блоки уже minW не рисуются, и их дети тоже.
export function project(tree: FlameTree, win: ViewWindow, minW = 0): ViewRect[] {
  const span = win.end - win.start
  const out: ViewRect[] = []
  const walk = (node: FlameNode): void => {
    const left = Math.max(node.start, win.start)
    const right = Math.min(node.start + node.value, win.end)
    if (right <= left) return
    const w = (right - left) / span
    if (w < minW) return
    out.push({ node, x: (left - win.start) / span, w })
    node.children.forEach(walk)
  }
  walk(tree.root)
  return out
}

// Клавиши навигации по графу: в графе одна точка остановки Tab, а между блоками ходят стрелками.
// Граф растёт вверх, поэтому «вверх» — к вызванной функции, «вниз» — к вызвавшей.
export type FlameNavKey = 'ArrowUp' | 'ArrowDown' | 'ArrowLeft' | 'ArrowRight' | 'Home'

// С Alt/Ctrl/Meta стрелки принадлежат браузеру (Alt+← — «назад»): граф их не перехватывает.
export const isFlameNavKey = (key: string, modifiers: KeyModifiers = {}): key is FlameNavKey =>
  !(modifiers.altKey || modifiers.ctrlKey || modifiers.metaKey) &&
  (key === 'ArrowUp' || key === 'ArrowDown' || key === 'ArrowLeft' || key === 'ArrowRight' || key === 'Home')

// Видимый блок, куда ведёт клавиша от блока fromId; undefined — идти некуда (край графа).
// Вверх — самый левый видимый ребёнок, вниз — родитель, влево и вправо — соседний видимый
// блок той же глубины, в том числе под другим родителем: так обходят всю строку графа.
export function neighbour(rects: readonly ViewRect[], fromId: string, key: FlameNavKey): FlameNode | undefined {
  const from = rects.find((r) => r.node.id === fromId)
  if (from === undefined) return undefined
  const visible = (id: string | null): FlameNode | undefined => rects.find((r) => r.node.id === id)?.node
  switch (key) {
    case 'Home':
      return rects[0]?.node
    case 'ArrowDown':
      return visible(from.node.parentId)
    case 'ArrowUp':
      return from.node.children.map((c) => visible(c.id)).find((c) => c !== undefined)
    case 'ArrowLeft':
    case 'ArrowRight': {
      const row = rects.filter((r) => r.node.depth === from.node.depth).sort((a, b) => a.x - b.x)
      const i = row.indexOf(from)
      return row[key === 'ArrowLeft' ? i - 1 : i + 1]?.node
    }
  }
}

// Доля узла от всего профиля и от увеличенного узла (у предков — 1: они шире окна).
export const shareOfTotal = (tree: FlameTree, node: FlameNode): number => node.value / tree.root.value
export const shareOfFocus = (node: FlameNode, focus: FlameNode): number => Math.min(1, node.value / focus.value)

export interface SearchResult {
  query: string
  ids: ReadonlySet<string>
  value: number // без двойного счёта: совпадение внутри совпадения уже учтено предком
  share: number
}

export function search(tree: FlameTree, query: string): SearchResult {
  const q = query.trim().toLowerCase()
  const ids = new Set<string>()
  let value = 0
  if (q !== '') {
    const walk = (node: FlameNode, counted: boolean): void => {
      const hit = node !== tree.root && node.name.toLowerCase().includes(q)
      if (hit) ids.add(node.id)
      if (hit && !counted) value += node.value
      for (const c of node.children) walk(c, counted || hit)
    }
    walk(tree.root, false)
  }
  return { query: q, ids, value, share: value / tree.root.value }
}

// Короткое имя без пути пакета: «net/http.(*conn).serve» → «http.(*conn).serve», как в pprof.
export function shortName(name: string): string {
  const pkg = packageOf(name)
  const slash = pkg.lastIndexOf('/')
  return slash < 0 ? name : name.slice(slash + 1)
}

// Подпись блока шириной widthPx при моноширинном шрифте: полное имя, иначе короткое,
// иначе короткое с «…»; меньше трёх знаков — без подписи (имя есть в подсказке).
export function fitLabel(name: string, widthPx: number, charPx: number, padPx = 4): string {
  const room = Math.floor((widthPx - 2 * padPx) / charPx)
  if (name.length <= room) return name
  const short = shortName(name)
  if (short.length <= room) return short
  if (room < 3) return ''
  return `${short.slice(0, room - 1)}…`
}

const MIB = 1 << 20
const KIB = 1 << 10

// Байты — в двоичных единицах, как их печатает go tool pprof (у него MB = 2^20 байт).
export function formatValue(value: number, unit: string): string {
  if (unit === 'nanoseconds') return formatNs(value)
  if (unit === 'bytes') {
    if (value >= MIB) return `${formatNumber(value / MIB, value < 10 * MIB ? 1 : 0)} МиБ`
    if (value >= KIB) return `${formatNumber(value / KIB, value < 10 * KIB ? 1 : 0)} КиБ`
    return `${formatNumber(value)} Б`
  }
  return formatNumber(value)
}

export function formatShare(share: number): string {
  const pct = share * 100
  return `${formatNumber(pct, pct < 10 ? 1 : 0)} %`
}

export const DATA: FlameData = parseFlameData(rawData)
const TREES: Record<SampleKind, FlameTree> = {
  cpu: buildTree(DATA.cpu, 'cpu'),
  alloc_space: buildTree(DATA.alloc_space, 'alloc_space'),
  inuse_space: buildTree(DATA.inuse_space, 'inuse_space'),
}
export const treeOf = (kind: SampleKind): FlameTree => TREES[kind]

export interface FlameState {
  kind: SampleKind
  focusId: string
  query: string
}

export type FlameAction =
  | { type: 'select-kind'; kind: SampleKind }
  | { type: 'zoom'; id: string }
  | { type: 'reset' }
  | { type: 'search'; query: string }

export interface FlameFrame {
  tree: FlameTree
  focus: FlameNode
  path: readonly FlameNode[] // от корня до увеличенного узла
  matches: SearchResult
  hottest: FlameNode // наибольшее собственное значение в увеличенном поддереве
}

const t = (text: string): CaptionSegment => ({ text })
const FLAME = term('Флейм-граф', 'flame-graph')

const KIND_CAPTION: Record<SampleKind, Caption> = {
  cpu: [
    term('Профилировщик', 'profiler'),
    t(' CPU 100 раз в секунду записывает стек работающей горутины. Ширина блока — доля таких образцов, над блоком — функции, которые он вызвал. '),
  ],
  alloc_space: [
    t('alloc_space — сколько байт выделено в '),
    term('куче', 'heap-allocation'),
    t(' с запуска процесса; выборка — в среднем один образец на 512 КиБ выделенного. Чем шире блок, тем больше работы он даёт сборщику. '),
  ],
  inuse_space: [
    t('inuse_space — сколько из выделенного ещё живо после последней '),
    term('сборки мусора', 'garbage-collector'),
    t('. Это то, что держит память процесса, а не то, что её «прокачивает». '),
  ],
}

function hottestIn(node: FlameNode): FlameNode {
  let best = node
  const walk = (n: FlameNode): void => {
    // «прочее» — сотни мелких стеков вместе, а не функция: горячей точкой её не называем.
    if (n.kind !== 'other' && n.self > best.self) best = n
    n.children.forEach(walk)
  }
  walk(node)
  return best
}

function caption(frame: FlameFrame, state: FlameState): Caption {
  const { tree, focus, hottest, matches } = frame
  const fmt = (n: FlameNode): string => `${formatShare(shareOfTotal(tree, n))} (${formatValue(n.value, tree.unit)})`
  const parts: CaptionSegment[] = [...(KIND_CAPTION[state.kind] as readonly CaptionSegment[])]
  if (focus === tree.root) {
    parts.push(FLAME, t(` показывает весь профиль: ${formatValue(tree.root.value, tree.unit)}. `))
  } else {
    parts.push(t(`Увеличен блок ${shortName(focus.name)}: он теперь во всю ширину, а это ${fmt(focus)} всего профиля. `))
  }
  // Блок, а не функция: -top складывает функцию по всем стекам, и там цифра бывает больше.
  const what = state.kind === 'cpu' ? 'собственного времени' : 'собственных байт'
  parts.push(t(`Больше всего ${what} (без вызванных функций) у блока ${shortName(hottest.name)}: ${formatShare(hottest.self / tree.root.value)} профиля.`))
  if (matches.query !== '') {
    parts.push(
      t(
        matches.ids.size === 0
          ? ` Поиск «${matches.query}»: ничего.`
          : ` Поиск «${matches.query}»: блоков ${matches.ids.size}, вместе ${formatShare(matches.share)} профиля (вложенные совпадения не считаются дважды).`,
      ),
    )
  }
  return parts
}

const initialState: FlameState = { kind: 'cpu', focusId: 'r', query: '' }

export const flamegraphSimulation: Simulation<FlameState, FlameAction, FlameFrame> = {
  initial: () => initialState,
  apply(state, action) {
    switch (action.type) {
      case 'select-kind':
        return { ...state, kind: action.kind, focusId: 'r' }
      case 'zoom':
        return treeOf(state.kind).byId.has(action.id) ? { ...state, focusId: action.id } : state
      case 'reset':
        return { ...state, focusId: 'r' }
      case 'search':
        return { ...state, query: action.query }
    }
  },
  steps(state): readonly Step<FlameFrame>[] {
    const tree = treeOf(state.kind)
    const focus = tree.byId.get(state.focusId) ?? tree.root
    const frame: FlameFrame = { tree, focus, path: pathTo(tree, focus.id), matches: search(tree, state.query), hottest: hottestIn(focus) }
    return [{ state: frame, caption: caption(frame, state) }]
  },
}
