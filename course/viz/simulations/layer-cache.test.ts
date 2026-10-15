import { describe, expect, it } from 'vitest'
import { fileURLToPath } from 'node:url'
import { captionTermIds } from '../core/caption'
import { loadGlossaryFile } from '../../glossary/load'
import { layerCacheSimulation as sim, type LayerAction } from './layer-cache'

const run = (...actions: LayerAction[]) => actions.reduce(sim.apply, sim.initial())
const statuses = (s: ReturnType<typeof run>) => s.lastBuild.map((l) => l.status)

describe('layer-cache', () => {
  it('первая сборка — всё собирается заново', () => {
    const s = run({ type: 'build' })
    expect(statuses(s).every((x) => x === 'built')).toBe(true)
  })
  it('без изменений повторная сборка берёт все слои из кэша', () => {
    const s = run({ type: 'build' }, { type: 'build' })
    expect(statuses(s).every((x) => x === 'cached')).toBe(true)
  })
  it('правильный порядок: правка исходника не трогает слой зависимостей', () => {
    const s = run({ type: 'build' }, { type: 'edit', file: 'source' }, { type: 'build' })
    expect(s.lastBuild.find((l) => l.id === 'download')?.status).toBe('cached')
    expect(s.lastBuild.find((l) => l.id === 'build')?.status).toBe('built')
  })
  it('наивный порядок: правка исходника заново качает зависимости', () => {
    const s = run({ type: 'order', order: 'naive' }, { type: 'build' }, { type: 'edit', file: 'source' }, { type: 'build' })
    expect(s.lastBuild.find((l) => l.id === 'download')?.status).toBe('built')
  })
  it('правка go.mod инвалидирует зависимости и всё после них в стадии сборки', () => {
    const s = run({ type: 'build' }, { type: 'edit', file: 'gomod' }, { type: 'build' })
    const stage = s.lastBuild.filter((l) => l.stage === 'build')
    const i = stage.findIndex((l) => l.id === 'copy-mod')
    expect(stage.slice(0, i).every((l) => l.status === 'cached')).toBe(true)
    expect(stage.slice(i).every((l) => l.status === 'built')).toBe(true)
    // Новый бинарь — новый слой COPY --from, а FROM финальной стадии остаётся в кэше.
    expect(s.lastBuild.find((l) => l.id === 'runtime')?.status).toBe('cached')
    expect(s.lastBuild.find((l) => l.id === 'copy-bin')?.status).toBe('built')
  })
  it('README без .dockerignore инвалидирует COPY . ., с ним — нет', () => {
    const without = run({ type: 'dockerignore', on: false }, { type: 'build' }, { type: 'edit', file: 'readme' }, { type: 'build' })
    expect(without.lastBuild.find((l) => l.id === 'copy-src')?.status).toBe('built')
    const withIgnore = run({ type: 'build' }, { type: 'edit', file: 'readme' }, { type: 'build' })
    expect(withIgnore.lastBuild.every((l) => l.status === 'cached')).toBe(true)
  })
  it('cache mount ускоряет пересобранный шаг сборки, но не делает его кэшированным', () => {
    const on = run({ type: 'build' }, { type: 'edit', file: 'source' }, { type: 'build' })
    const off = run({ type: 'cacheMount', on: false }, { type: 'build' }, { type: 'edit', file: 'source' }, { type: 'build' })
    const t = (s: typeof on) => s.lastBuild.find((l) => l.id === 'build')?.seconds ?? 0
    expect(t(on)).toBeLessThan(t(off))
  })
  it('итоговый образ — только слои финальной стадии', () => {
    const s = run({ type: 'build' })
    expect(s.imageMB).toBeLessThan(30)
    expect(run({ type: 'stage', multi: false }, { type: 'build' }).imageMB).toBeGreaterThan(800)
  })
  it('каждый шаг подписан, подписи с терминами', () => {
    const steps = sim.steps(run({ type: 'build' }))
    expect(steps).toHaveLength(1)
    expect(JSON.stringify(steps[0]?.caption)).toContain('build-cache')
  })
  it('все термины в подписях есть в глоссарии', () => {
    const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
    const states = [run(), run({ type: 'build' }), run({ type: 'build' }, { type: 'build' }), run({ type: 'build' }, { type: 'edit', file: 'source' }, { type: 'build' })]
    const ids = states.flatMap((s) => sim.steps(s).flatMap((st) => captionTermIds(st.caption)))
    expect(ids.length).toBeGreaterThanOrEqual(3)
    for (const id of ids) expect(known.has(id)).toBe(true)
  })
})
