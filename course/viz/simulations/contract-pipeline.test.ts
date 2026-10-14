import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { CONTRACT_STAGE_IDS, contractPipelineScenario } from './contract-pipeline'

const steps = contractPipelineScenario()

describe('contractPipelineScenario', () => {
  it('от 8 до 10 кадров, у каждого подпись и файлы', () => {
    expect(steps.length).toBeGreaterThanOrEqual(8)
    expect(steps.length).toBeLessThanOrEqual(10)
    for (const s of steps) {
      expect(captionText(s.caption).trim().length).toBeGreaterThan(20)
      expect(s.state.files.length).toBeGreaterThan(0)
      for (const f of s.state.files) expect(f.snippet.trim().length).toBeGreaterThan(0)
    }
  })

  it('каждая подсветка - существующая стадия', () => {
    for (const s of steps) for (const h of s.highlight ?? []) expect(CONTRACT_STAGE_IDS).toContain(h)
  })

  it('ровно один кадр поймал компилятор и один - контрактный тест, компилятор раньше', () => {
    const c = steps.findIndex((s) => s.state.caught === 'compiler')
    const t = steps.findIndex((s) => s.state.caught === 'contract-test')
    expect(steps.filter((s) => s.state.caught === 'compiler')).toHaveLength(1)
    expect(steps.filter((s) => s.state.caught === 'contract-test')).toHaveLength(1)
    expect(c).toBeGreaterThanOrEqual(0)
    expect(c).toBeLessThan(t)
  })

  it('имена как у oapi-codegen встречаются в сниппетах', () => {
    const all = steps.flatMap((s) => s.state.files.map((f) => f.snippet)).join('\n')
    for (const name of ['SendMessageRequest', 'ChatId', 'ServerInterface', 'sendMessage', 'gen.ID']) expect(all).toContain(name)
  })
})

it('все термины в подписях есть в глоссарии', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  const ids = steps.flatMap((s) => captionTermIds(s.caption))
  expect(ids.length).toBeGreaterThanOrEqual(2)
  for (const id of ids) expect(known.has(id)).toBe(true)
})
