import { describe, expect, it } from 'vitest'
import { fileURLToPath } from 'node:url'
import { captionTermIds } from '../core/caption'
import { loadGlossaryFile } from '../../glossary/load'
import { supplyChainSimulation as sim, type ChainAction } from './supply-chain'

const run = (...actions: ChainAction[]) => actions.reduce(sim.apply, sim.initial())

describe('supply-chain', () => {
  it('без атаки выполняется исходный код всех actions', () => {
    const s = run({ type: 'run' })
    expect(s.result?.executed.every((e) => !e.malicious)).toBe(true)
    expect(s.result?.leaked).toEqual([])
  })
  it('закрепление по тегу: подменённый тег выполняет код злоумышленника', () => {
    const s = run({ type: 'pin', mode: 'tag' }, { type: 'attack' }, { type: 'run' })
    expect(s.result?.executed.some((e) => e.malicious)).toBe(true)
  })
  it('закрепление по SHA: подмена тега не влияет', () => {
    const s = run({ type: 'pin', mode: 'sha' }, { type: 'attack' }, { type: 'run' })
    expect(s.result?.executed.every((e) => !e.malicious)).toBe(true)
    expect(s.result?.leaked).toEqual([])
  })
  it('тег + write-all + секрет в env: утекают токен с правом записи и секрет', () => {
    const s = run({ type: 'pin', mode: 'tag' }, { type: 'permissions', mode: 'write-all' }, { type: 'secret', inEnv: true }, { type: 'attack' }, { type: 'run' })
    expect(s.result?.leaked).toEqual(expect.arrayContaining(['GITHUB_TOKEN: contents:write', 'DEPLOY_KEY']))
  })
  it('тег + минимальные права: утекает только токен на чтение', () => {
    const s = run({ type: 'pin', mode: 'tag' }, { type: 'permissions', mode: 'minimal' }, { type: 'secret', inEnv: false }, { type: 'attack' }, { type: 'run' })
    expect(s.result?.leaked).toEqual(['GITHUB_TOKEN: contents:read'])
  })
  it('подпись называет, что спасло или что утекло', () => {
    const s = run({ type: 'pin', mode: 'sha' }, { type: 'attack' }, { type: 'run' })
    expect(JSON.stringify(sim.steps(s)[0]?.caption)).toContain('action-pinning')
  })
  it('все термины в подписях есть в глоссарии', () => {
    const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
    const states = [run(), run({ type: 'run' }), run({ type: 'attack' }, { type: 'run' }), run({ type: 'pin', mode: 'sha' }, { type: 'attack' }, { type: 'run' })]
    const ids = states.flatMap((s) => sim.steps(s).flatMap((st) => captionTermIds(st.caption)))
    expect(ids.length).toBeGreaterThanOrEqual(3)
    for (const id of ids) expect(known.has(id)).toBe(true)
  })
})
