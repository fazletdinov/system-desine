import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import rawData from '../data/flame-messenger.json'
import {
  OTHER_FRAME,
  SAMPLE_KINDS,
  buildTree,
  fitLabel,
  flamegraphSimulation as sim,
  frameKind,
  packageOf,
  parseFlameData,
  parseProfile,
  project,
  search,
  shareOfFocus,
  shareOfTotal,
  treeOf,
  windowOf,
  type FlameAction,
  type FlameNode,
  type FlameTree,
  type FoldedProfile,
} from './flamegraph'

// Два образца с общим префиксом main → handle и третий — в другой ветке.
const SMALL: FoldedProfile = {
  sampleType: 'cpu',
  unit: 'nanoseconds',
  total: 100,
  stacks: [
    { frames: ['main.main', 'main.handle', 'main.hash'], value: 50 },
    { frames: ['main.main', 'main.handle', 'main.encode'], value: 30 },
    { frames: ['main.main', 'main.handle'], value: 10 },
    { frames: ['runtime.gcBgMarkWorker', 'runtime.scanobject'], value: 6 },
    { frames: [OTHER_FRAME], value: 4 },
  ],
}

const small = (): FlameTree => buildTree(SMALL, 'cpu')
const byName = (tree: FlameTree, name: string): FlameNode => [...tree.byId.values()].find((n) => n.name === name)!
const allNodes = (tree: FlameTree): FlameNode[] => [...tree.byId.values()]
const apply = (actions: FlameAction[]) => actions.reduce(sim.apply, sim.initial())

describe('flamegraph: свёртка стеков в дерево', () => {
  it('общий префикс — один узел, значения суммируются, self — без детей', () => {
    const tree = small()
    expect(tree.root.children.map((c) => c.name)).toEqual(['main.main', OTHER_FRAME, 'runtime.gcBgMarkWorker'].sort())
    const handle = byName(tree, 'main.handle')
    expect(handle.value).toBe(90)
    expect(handle.self).toBe(10)
    expect(handle.children.map((c) => [c.name, c.value])).toEqual([
      ['main.encode', 30],
      ['main.hash', 50],
    ])
    expect(byName(tree, 'main.main').self).toBe(0)
  })

  it('корень — 100 %, доли детей в сумме не больше доли родителя', () => {
    for (const tree of [small(), ...SAMPLE_KINDS.map(treeOf)]) {
      expect(shareOfTotal(tree, tree.root)).toBe(1)
      for (const n of allNodes(tree)) {
        const kids = n.children.reduce((s, c) => s + shareOfTotal(tree, c), 0)
        expect(kids).toBeLessThanOrEqual(shareOfTotal(tree, n) + 1e-12)
        expect(n.self).toBeGreaterThanOrEqual(0)
      }
    }
  })

  it('дети лежат подряд внутри родителя', () => {
    for (const n of allNodes(small())) {
      let offset = n.start
      for (const c of n.children) {
        expect(c.start).toBe(offset)
        offset += c.value
      }
      expect(offset).toBeLessThanOrEqual(n.start + n.value)
    }
  })
})

describe('flamegraph: зум', () => {
  it('увеличенный узел — во всю ширину, доля 100 %', () => {
    const tree = small()
    const handle = byName(tree, 'main.handle')
    const rects = project(tree, windowOf(handle), handle.id)
    const own = rects.find((r) => r.node === handle)!
    expect(own.x).toBe(0)
    expect(own.w).toBe(1)
    expect(shareOfFocus(handle, handle)).toBe(1)
  })

  it('доли поддерева относительно его корня сохраняются', () => {
    const tree = small()
    const handle = byName(tree, 'main.handle')
    const before = project(tree, windowOf(tree.root), tree.root.id)
    const after = project(tree, windowOf(handle), handle.id)
    const w = (rects: typeof before, name: string): number => rects.find((r) => r.node.name === name)!.w
    for (const name of ['main.hash', 'main.encode']) {
      expect(w(after, name)).toBeCloseTo(w(before, name) / w(before, 'main.handle'), 12)
    }
    expect(w(after, 'main.hash')).toBeCloseTo(50 / 90, 12)
    expect(shareOfFocus(byName(tree, 'main.hash'), handle)).toBeCloseTo(50 / 90, 12)
  })

  it('предки остаются на всю ширину, соседние ветки не видны', () => {
    const tree = small()
    const handle = byName(tree, 'main.handle')
    const rects = project(tree, windowOf(handle), handle.id)
    const names = rects.map((r) => r.node.name)
    expect(names).not.toContain('runtime.gcBgMarkWorker')
    expect(names).not.toContain(OTHER_FRAME)
    for (const anc of [tree.root, byName(tree, 'main.main')]) {
      const r = rects.find((x) => x.node === anc)!
      expect([r.x, r.w, r.ancestor]).toEqual([0, 1, true])
    }
    expect(rects.find((r) => r.node === handle)!.ancestor).toBe(false)
  })

  it('слишком узкие блоки не рисуются', () => {
    const tree = small()
    const rects = project(tree, windowOf(tree.root), tree.root.id, 0.05)
    expect(rects.map((r) => r.node.name)).not.toContain(OTHER_FRAME)
    expect(rects.map((r) => r.node.name)).toContain('runtime.gcBgMarkWorker')
  })

  it('действия: зум, к корню, неизвестный узел не меняет состояние', () => {
    const tree = treeOf('cpu')
    const target = tree.root.children[0]!.children[0]!
    const zoomed = apply([{ type: 'zoom', id: target.id }])
    expect(sim.steps(zoomed)[0]!.state.focus).toBe(target)
    expect(sim.steps(zoomed)[0]!.state.path.at(-1)).toBe(target)
    expect(sim.apply(zoomed, { type: 'zoom', id: 'нет-такого' })).toBe(zoomed)
    expect(sim.apply(zoomed, { type: 'reset' }).focusId).toBe('r')
  })
})

describe('flamegraph: поиск', () => {
  it('подсвечивает все узлы с подстрокой, без учёта регистра', () => {
    const tree = small()
    const res = search(tree, 'MAIN.H')
    expect([...res.ids].map((id) => tree.byId.get(id)!.name).sort()).toEqual(['main.handle', 'main.hash'])
  })

  it('доля без двойного счёта вложенных совпадений', () => {
    const tree = small()
    // main.handle (90) содержит main.hash (50): вместе 90, а не 140.
    expect(search(tree, 'main.h').value).toBe(90)
    expect(search(tree, 'main.h').share).toBeCloseTo(0.9, 12)
    // Независимые ветки складываются: main.main (90, с вложенными) + runtime.gcBgMarkWorker (6).
    expect(search(tree, 'ma').value).toBe(96)
    expect(search(tree, 'ma').ids.size).toBe(5)
  })

  it('пустой запрос ничего не подсвечивает', () => {
    expect(search(small(), '  ').ids.size).toBe(0)
    expect(search(small(), '  ').share).toBe(0)
  })
})

describe('flamegraph: типы выборки и данные мессенджера', () => {
  it('данные из flame-messenger.json разбираются без ошибок', () => {
    const data = parseFlameData(rawData)
    expect(data.cpu.unit).toBe('nanoseconds')
    expect(data.alloc_space.unit).toBe('bytes')
    expect(data.inuse_space.unit).toBe('bytes')
  })

  it('файл данных не больше 200 КБ', () => {
    const bytes = readFileSync(fileURLToPath(new URL('../data/flame-messenger.json', import.meta.url))).length
    expect(bytes).toBeLessThanOrEqual(200 * 1024)
  })

  it('в каждом профиле есть код мессенджера', () => {
    for (const kind of SAMPLE_KINDS) {
      expect(allNodes(treeOf(kind)).some((n) => n.kind === 'messenger'), kind).toBe(true)
    }
  })

  it('переключение типа выборки меняет дерево и сбрасывает зум', () => {
    const cpu = sim.steps(sim.initial())[0]!.state
    const zoomed = apply([{ type: 'zoom', id: cpu.tree.root.children[0]!.id }, { type: 'select-kind', kind: 'alloc_space' }])
    const alloc = sim.steps(zoomed)[0]!.state
    expect(alloc.tree).not.toBe(cpu.tree)
    expect(alloc.tree.sampleType).toBe('alloc_space')
    expect(alloc.focus).toBe(alloc.tree.root)
    expect(alloc.tree.root.value).not.toBe(cpu.tree.root.value)
  })

  it('битые данные отвергаются', () => {
    const frames = ['main.main', 'main.a']
    const good = { sampleType: 'cpu', unit: 'nanoseconds', total: 3, stacks: [{ f: [0, 1], v: 3 }] }
    expect(parseProfile(good, 'cpu', frames).stacks).toEqual([{ frames: ['main.main', 'main.a'], value: 3 }])
    expect(() => parseProfile({ ...good, total: 2 }, 'cpu', frames)).toThrow(/total/)
    expect(() => parseProfile({ ...good, sampleType: 'heap' }, 'cpu', frames)).toThrow(/sampleType/)
    expect(() => parseProfile({ ...good, stacks: [{ f: [], v: 3 }] }, 'cpu', frames)).toThrow(/кадры/)
    expect(() => parseProfile({ ...good, stacks: [{ f: [0, 2], v: 3 }] }, 'cpu', frames)).toThrow(/кадры/)
    expect(() => parseProfile({ ...good, stacks: [{ f: [0], v: -1 }], total: -1 }, 'cpu', frames)).toThrow(/значение/)
    expect(() => parseFlameData({ ...rawData, frames: [] })).toThrow(/таблицы кадров/)
  })

  it('мелкие стеки слиты в «прочее» под родителем: в корне «прочее» меньше 2 %', () => {
    for (const kind of SAMPLE_KINDS) {
      const tree = treeOf(kind)
      const rootOther = tree.root.children.find((c) => c.kind === 'other')
      expect(rootOther === undefined ? 0 : shareOfTotal(tree, rootOther), kind).toBeLessThan(0.02)
    }
    expect(allNodes(treeOf('cpu')).some((n) => n.kind === 'other' && n.depth > 1)).toBe(true)
  })

  it('подписи: осмысленный текст, термины есть в глоссарии', () => {
    const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
    for (const kind of SAMPLE_KINDS) {
      for (const s of [apply([{ type: 'select-kind', kind }]), apply([{ type: 'select-kind', kind }, { type: 'search', query: 'json' }])]) {
        const c = sim.steps(s)[0]!.caption
        expect(captionText(c).length).toBeGreaterThan(80)
        for (const id of captionTermIds(c)) expect(known.has(id), `${kind}: ${id}`).toBe(true)
      }
    }
  })
})

describe('flamegraph: чей код и подписи блоков', () => {
  it.each([
    ['main.main', 'main', 'messenger'],
    ['messenger/internal/chat/adapters/httpapi.toMessagePage', 'messenger/internal/chat/adapters/httpapi', 'messenger'],
    ['messenger/internal/platform/middleware.Chain.func1', 'messenger/internal/platform/middleware', 'messenger'],
    ['runtime.mallocgc', 'runtime', 'runtime'],
    ['internal/runtime/syscall/linux.Syscall6', 'internal/runtime/syscall/linux', 'runtime'],
    ['memeqbody', '', 'runtime'],
    ['net/http.(*conn).serve', 'net/http', 'stdlib'],
    ['runtime/trace.readBatch', 'runtime/trace', 'stdlib'],
    ['encoding/json/v2.makeStructArshaler.func2', 'encoding/json/v2', 'stdlib'],
    ['golang.org/x/crypto/argon2.blamkaSSE4', 'golang.org/x/crypto/argon2', 'external'],
    ['github.com/golang-jwt/jwt/v5.(*SigningMethodEd25519).Verify', 'github.com/golang-jwt/jwt/v5', 'external'],
    ['slices.SortFunc[go.shape.[]string,go.shape.string]', 'slices', 'stdlib'],
    [OTHER_FRAME, '', 'other'],
  ] as const)('%s → пакет %s, %s', (name, pkg, kind) => {
    if (name !== OTHER_FRAME) expect(packageOf(name)).toBe(pkg)
    expect(frameKind(name)).toBe(kind)
  })

  it('подпись обрезается по ширине блока', () => {
    const name = 'net/http.(*conn).serve'
    expect(fitLabel(name, 400, 7)).toBe(name)
    expect(fitLabel(name, 7 * 18 + 8, 7)).toBe('http.(*conn).serve')
    expect(fitLabel(name, 7 * 10 + 8, 7)).toBe('http.(*co…')
    expect(fitLabel(name, 7 * 10 + 8, 7).length).toBeLessThanOrEqual(10)
    expect(fitLabel(name, 20, 7)).toBe('')
  })
})
