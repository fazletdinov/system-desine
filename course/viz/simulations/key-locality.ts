import type { Caption, CaptionSegment } from '../core/caption'
import { term } from '../core/caption'
import { plural } from '../core/format'
import type { Simulation, Step } from '../core/types'

// Песочница урока 2.3: листовой уровень B-дерева первичного ключа при вставках bigint, UUIDv4 и UUIDv7.
// Все три дерева растут одновременно от одних и тех же действий, поэтому их можно сравнивать при одном N.
//
// Правила разделения сверены с PostgreSQL 18 (2026-10-21):
// - документация CREATE INDEX, параметр fillfactor: «For B-trees, leaf pages are filled to this
//   percentage during initial index builds, and also when extending the index at the right (adding new
//   largest key values). If pages subsequently become completely full, they will be split… B-trees use
//   a default fillfactor of 90»;
// - src/backend/access/nbtree/nbtsplitloc.c, _bt_findsplitloc: для самого правого листа
//   «fillfactormult always used» — в левой половине остаётся fillfactor % (90 %), «so when we are
//   inserting successively increasing keys… we will end up with a tree whose pages are about fillfactor%
//   full, instead of the 50% full result»; для прочих листьев — «Other leaf page. 50:50 page split.»
//   Страница делится, только когда новый ключ в неё уже не помещается: fillfactor задаёт точку
//   разделения, а не порог.
// Не моделируются: третья ветка _bt_findsplitloc (_bt_afternewitemoff — разделение сразу после нового
// ключа при возрастающих вставках внутри группы составного ключа), дедупликация (у уникального ключа
// дублей нет), внутренние страницы и точный учёт байтов: ёмкость страницы — PAGE_CAPACITY ключей.
// Полные образы считаются только для разделяемого листа: правый сосед (ссылка btpo_prev) и родитель,
// которые при разделении тоже меняются и тоже могут получить образ, не учитываются — поэтому у bigint
// модель показывает 1 образ, а лаба 7. Направление сравнения видов ключа от этого не меняется.

export type KeyKind = 'bigint' | 'uuidv4' | 'uuidv7'
export const KEY_KINDS: readonly KeyKind[] = ['bigint', 'uuidv4', 'uuidv7']

// В настоящей 8-КиБ странице помещаются сотни ключей; 20 — чтобы страницы было видно на экране
// телефона. 20 × 0,9 = 18 — fillfactor даёт целое число ключей.
export const PAGE_CAPACITY = 20
// BTREE_DEFAULT_FILLFACTOR = 90 (nbtree.h; документация CREATE INDEX).
export const LEAF_FILLFACTOR = 0.9
// Страница «горячая», если в неё писали за последние HOT_WINDOW вставок: её нужно держать в памяти.
// Окно — одна страница вставок. Монотонные ключи за это время трогают ровно два листа: правый и тот,
// что только что от него отделился (разделение — каждые 18 вставок, окно — 20).
export const HOT_WINDOW = PAGE_CAPACITY
// Пороги подсветки счётчиков. Монотонным ключам хватает 1–2 горячих листов и 1–2 полных образов
// (правый лист и его левая половина), так что больше трёх — признак записи по всему дереву.
export const HOT_ALERT = 3
export const FPI_ALERT = 3
// Ниже 80 % заполнения — заметно хуже fillfactor 90, к которому сходятся монотонные ключи. Пока листьев
// один-два, заполненность ничего не говорит: правый лист ещё растёт.
export const FILL_ALERT = 0.8
export const FILL_ALERT_MIN_PAGES = 3
// Предел модели: ~60–90 листьев ещё различимы на 375 px.
export const MAX_KEYS = 1200
// UUIDv7 из приложения: 48 бит миллисекунд, потом случайные биты. google/uuid монотонен внутри процесса,
// но несколько экземпляров сервиса пишут вперемешку — в пределах одной миллисекунды порядок случаен.
// Модель: KEYS_PER_MS вставок на миллисекунду, внутри неё ключ случаен. Отношение к ёмкости страницы
// взято порядка реального: в 8-КиБ лист входит ~290 UUID, а «перемешанных» вставок за миллисекунду —
// единицы; при 20 ключах на страницу это 2–3.
export const KEYS_PER_MS = 3
export const INSERT_BATCHES = [1, 20, 200] as const

// Реальные числа лабы 2a (labs/2a/results/keys-n1000000-{1,2,3}.txt, PostgreSQL 18.6, три прогона по
// 10⁶ строк; bigint и UUIDv7 во всех прогонах совпали, у UUIDv4 — разброс между прогонами).
export const LAB = {
  rows: '10⁶',
  topUpRows: '10⁵',
  source: 'labs/2a/results/keys-n1000000-{1,2,3}.txt',
  bigint: { index: '21 960 КиБ', fill: '90 %', topUpWal: '9,0 МиБ', fpi: '7' },
  uuidv4: { index: '38 288–38 856 КиБ', fill: '71,4–72,5 %', topUpWal: '43,1–43,3 МиБ', fpi: '4 788–4 859' },
  uuidv7: { index: '30 824 КиБ', fill: '90 %', topUpWal: '14,9 МиБ', fpi: '7' },
} as const

export interface Page {
  id: number
  low: number // нижняя граница диапазона ключей страницы (разделитель в родителе); у первой — -∞
  keys: readonly number[]
  bornAt: number // номер эпохи CHECKPOINT, в которую страница появилась
}

export interface Tree {
  kind: KeyKind
  pages: readonly Page[]
  nextPageId: number
  rng: number // состояние генератора: модель детерминирована
  inserted: number
  splitsRight: number
  splitsMiddle: number
  epoch: number // сколько раз был CHECKPOINT
  logged: readonly number[] // страницы, у которых после последнего CHECKPOINT уже записан полный образ
  fpi: number // полных образов страниц (full-page images) после последнего CHECKPOINT
  recent: readonly number[] // id страниц последних HOT_WINDOW вставок
  lastSplit: readonly number[] // id страниц, разделённых последним действием
}

export interface KeyLocalityState {
  selected: KeyKind
  trees: Readonly<Record<KeyKind, Tree>>
  checkpoints: number
  last: 'start' | 'insert' | 'checkpoint'
}

export type KeyLocalityAction =
  | { type: 'select'; kind: KeyKind }
  | { type: 'insert'; count: number }
  | { type: 'checkpoint' }
  | { type: 'reset' }

export interface TreeStats {
  kind: KeyKind
  keys: number
  pages: number
  fill: number // средняя заполненность листьев, 0..1
  splitsRight: number
  splitsMiddle: number
  hotPages: number
  fpi: number
}

export interface PageView {
  id: number
  fill: number
  hot: boolean
  split: boolean
}

export interface KeyLocalityFrame {
  selected: KeyKind
  pages: readonly PageView[]
  stats: Readonly<Record<KeyKind, TreeStats>>
  checkpoints: number
  full: boolean
}

// mulberry32: короткий детерминированный генератор, чтобы тесты и экран совпадали.
function next(seed: number): [number, number] {
  const s = (seed + 0x6d2b79f5) | 0
  let t = Math.imul(s ^ (s >>> 15), 1 | s)
  t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
  return [((t ^ (t >>> 14)) >>> 0) / 4294967296, s]
}

const SEEDS: Record<KeyKind, number> = { bigint: 1, uuidv4: 4, uuidv7: 7 }

function emptyTree(kind: KeyKind): Tree {
  return {
    kind,
    pages: [{ id: 0, low: -Infinity, keys: [], bornAt: 0 }],
    nextPageId: 1,
    rng: SEEDS[kind],
    inserted: 0,
    splitsRight: 0,
    splitsMiddle: 0,
    epoch: 0,
    logged: [],
    fpi: 0,
    recent: [],
    lastSplit: [],
  }
}

// Ключи: bigint — счётчик; UUIDv4 — случайное число; UUIDv7 — номер миллисекунды плюс случайная дробь.
function nextKey(kind: KeyKind, n: number, rng: number): [number, number] {
  if (kind === 'bigint') return [n + 1, rng]
  const [r, seed] = next(rng)
  return kind === 'uuidv4' ? [r, seed] : [Math.floor(n / KEYS_PER_MS) + r, seed]
}

// Последняя страница, чья нижняя граница не больше ключа, — туда спускается поиск по B-дереву.
function findLeaf(pages: readonly Page[], key: number): number {
  let lo = 0
  let hi = pages.length - 1
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1
    if (pages[mid]!.low <= key) lo = mid
    else hi = mid - 1
  }
  return lo
}

function insertSorted(keys: readonly number[], key: number): number[] {
  const out = [...keys]
  let i = out.length
  while (i > 0 && out[i - 1]! > key) i--
  out.splice(i, 0, key)
  return out
}

// Мутирует копию дерева, созданную в apply: одно действие — одна копия, а не копия на каждый ключ.
interface MutableTree extends Omit<Tree, 'pages' | 'logged' | 'recent' | 'lastSplit'> {
  pages: Page[]
  logged: Set<number>
  recent: number[]
  lastSplit: Set<number>
}

// Полный образ страницы пишется в WAL при первом изменении после CHECKPOINT (full_page_writes = on).
// Страница, созданная после CHECKPOINT, целиком описана записью о разделении — образ не нужен.
function touch(t: MutableTree, page: Page): void {
  if (page.bornAt < t.epoch && !t.logged.has(page.id)) {
    t.logged.add(page.id)
    t.fpi++
  }
}

function insertKey(t: MutableTree, key: number): void {
  const index = findLeaf(t.pages, key)
  const page = t.pages[index]!
  touch(t, page)
  const keys = insertSorted(page.keys, key)
  let target = page.id
  if (keys.length <= PAGE_CAPACITY) {
    t.pages[index] = { ...page, keys }
  } else {
    const rightmost = index === t.pages.length - 1
    // Правый лист: слева остаётся fillfactor; любой другой — пополам.
    const leftCount = rightmost ? Math.floor(PAGE_CAPACITY * LEAF_FILLFACTOR) : Math.ceil(keys.length / 2)
    const right: Page = { id: t.nextPageId++, low: keys[leftCount]!, keys: keys.slice(leftCount), bornAt: t.epoch }
    t.pages.splice(index, 1, { ...page, keys: keys.slice(0, leftCount) }, right)
    if (rightmost) t.splitsRight++
    else t.splitsMiddle++
    t.lastSplit.add(page.id)
    if (key >= right.low) target = right.id
  }
  t.recent.push(target)
  if (t.recent.length > HOT_WINDOW) t.recent.shift()
}

function insertMany(tree: Tree, count: number): Tree {
  const t: MutableTree = { ...tree, pages: [...tree.pages], logged: new Set(tree.logged), recent: [...tree.recent], lastSplit: new Set() }
  const n = Math.min(count, MAX_KEYS - tree.inserted)
  for (let i = 0; i < n; i++) {
    const [key, rng] = nextKey(t.kind, t.inserted, t.rng)
    t.rng = rng
    insertKey(t, key)
    t.inserted++
  }
  return { ...t, logged: [...t.logged], lastSplit: [...t.lastSplit] }
}

// CHECKPOINT: всё грязное записано на диск, и следующее изменение любой существующей страницы снова
// потребует её полного образа в WAL.
function checkpoint(tree: Tree): Tree {
  return { ...tree, epoch: tree.epoch + 1, logged: [], fpi: 0, lastSplit: [] }
}

export function treeStats(tree: Tree): TreeStats {
  const used = tree.pages.reduce((sum, p) => sum + p.keys.length, 0)
  return {
    kind: tree.kind,
    keys: tree.inserted,
    pages: tree.pages.length,
    fill: tree.inserted === 0 ? 0 : used / (tree.pages.length * PAGE_CAPACITY),
    splitsRight: tree.splitsRight,
    splitsMiddle: tree.splitsMiddle,
    hotPages: new Set(tree.recent).size,
    fpi: tree.fpi,
  }
}

function start(selected: KeyKind = 'bigint'): KeyLocalityState {
  return { selected, trees: { bigint: emptyTree('bigint'), uuidv4: emptyTree('uuidv4'), uuidv7: emptyTree('uuidv7') }, checkpoints: 0, last: 'start' }
}

function mapTrees(s: KeyLocalityState, f: (t: Tree) => Tree): Record<KeyKind, Tree> {
  return { bigint: f(s.trees.bigint), uuidv4: f(s.trees.uuidv4), uuidv7: f(s.trees.uuidv7) }
}

function apply(s: KeyLocalityState, a: KeyLocalityAction): KeyLocalityState {
  switch (a.type) {
    case 'select':
      return { ...s, selected: a.kind }
    case 'insert':
      return { ...s, trees: mapTrees(s, (t) => insertMany(t, a.count)), last: 'insert' }
    case 'checkpoint':
      return { ...s, trees: mapTrees(s, checkpoint), checkpoints: s.checkpoints + 1, last: 'checkpoint' }
    case 'reset':
      return start(s.selected)
  }
}

export const KIND_LABEL: Record<KeyKind, string> = { bigint: 'bigint', uuidv4: 'UUIDv4', uuidv7: 'UUIDv7' }

const pct = (v: number): string => `${Math.round(v * 100)} %`

function labLine(kind: KeyKind): CaptionSegment[] {
  const l = LAB[kind]
  return [
    { text: ` В лабе 2a (PostgreSQL 18.6, ${LAB.rows} строк) индекс ${KIND_LABEL[kind]} занял ${l.index}, листья заполнены на ${l.fill}; дозагрузка ${LAB.topUpRows} строк после CHECKPOINT записала ${l.topUpWal} ` },
    term('WAL', 'wal'),
    { text: `, из них полных образов страниц — ${l.fpi} (${LAB.source}).` },
  ]
}

// Горячие листья монотонных ключей: пока разделений не было — единственный лист, потом — правый и
// только что отделённый от него (HOT_WINDOW чуть больше шага разделений).
function monotonicHot(st: TreeStats): string {
  return st.splitsRight === 0 ? `Горячих страниц ${st.hotPages}: единственный лист` : `Горячих страниц ${st.hotPages}: правый лист и только что отделённый от него`
}

function kindStory(st: TreeStats): CaptionSegment[] {
  switch (st.kind) {
    case 'bigint':
      return [{ text: `Каждый новый ключ больше всех прежних и идёт в самый правый лист. Полный правый лист делится так, что слева остаётся ${pct(LEAF_FILLFACTOR)} (fillfactor), — и больше туда никто не пишет. Разделений в середине: ${st.splitsMiddle}. ${monotonicHot(st)} — остальной индекс можно не держать в памяти.` }]
    case 'uuidv4':
      return [
        { text: 'Случайный ключ попадает в любой лист. Полный лист в середине делится пополам, обе половины заполнены на 50 %, и до следующего разделения их дозаполняют такие же случайные вставки — в среднем около 70 %. ' },
        term('Разделения страниц', 'b-tree-page-split'),
        { text: ` идут по всему дереву (в середине: ${st.splitsMiddle}). Вставки размазаны: за последние ${HOT_WINDOW} вставок ${plural(st.hotPages, 'тронут', 'тронуто', 'тронуто')} ${st.hotPages} ${plural(st.hotPages, 'лист', 'разных листа', 'разных листьев')} из ${st.pages} — почти каждая вставка попадает в свой лист, и за немного большее время тронут весь индекс. Держать в памяти приходится его целиком.` },
      ]
    case 'uuidv7':
      return [
        { text: 'Старшие биты ' },
        term('UUIDv7', 'uuidv7'),
        { text: ` — время, поэтому вставки идут к правому краю, как у bigint. Внутри одной миллисекунды порядок случаен (пишут несколько экземпляров), но разброс укладывается в правый лист: разделений в середине — ${st.splitsMiddle}, заполненность ≈ fillfactor. ${monotonicHot(st)}. Но ключ занимает 16 байт против 8 у bigint: с заголовком кортежа и указателем строки это 28 байт на запись в листе против 20, поэтому при той же заполненности настоящий индекс в 1,4 раза больше.` },
      ]
  }
}

function caption(s: KeyLocalityState): Caption {
  const st = treeStats(s.trees[s.selected])
  if (s.last === 'start') {
    return [
      { text: 'Листья ' },
      term('B-дерева', 'index'),
      { text: ` первичного ключа: каждый столбик — страница на ${PAGE_CAPACITY} ключей. Вставляйте ключи и сравнивайте три вида при одном и том же числе строк.` },
      ...labLine(s.selected),
    ]
  }
  if (s.last === 'checkpoint') {
    return [
      { text: 'CHECKPOINT: грязные страницы записаны на диск. Первое изменение каждой существующей страницы после него пишет в ' },
      term('WAL', 'wal'),
      { text: ' её полный образ (8 КиБ). Вставьте ещё ключи и сравните счётчик полных образов: монотонные ключи трогают одну страницу, случайные — почти все.' },
    ]
  }
  return [
    { text: `${KIND_LABEL[s.selected]}, ${st.keys} ключей: ${st.pages} листьев, заполненность ${pct(st.fill)}. ` },
    ...kindStory(st),
    ...labLine(s.selected),
  ]
}

function frame(s: KeyLocalityState): KeyLocalityFrame {
  const tree = s.trees[s.selected]
  const hot = new Set(tree.recent)
  const split = new Set(tree.lastSplit)
  return {
    selected: s.selected,
    pages: tree.pages.map((p) => ({ id: p.id, fill: p.keys.length / PAGE_CAPACITY, hot: hot.has(p.id), split: split.has(p.id) })),
    stats: { bigint: treeStats(s.trees.bigint), uuidv4: treeStats(s.trees.uuidv4), uuidv7: treeStats(s.trees.uuidv7) },
    checkpoints: s.checkpoints,
    full: tree.inserted >= MAX_KEYS,
  }
}

export const keyLocalitySimulation: Simulation<KeyLocalityState, KeyLocalityAction, KeyLocalityFrame> = {
  initial: () => start(),
  apply,
  steps: (s): readonly Step<KeyLocalityFrame>[] => [{ state: frame(s), caption: caption(s) }],
}
