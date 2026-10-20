import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { blackToWhite, reachable, tricolorScenario, type TricolorFrame, type TricolorVariant } from './tricolor'

const VARIANTS: readonly TricolorVariant[] = ['no-barrier', 'barrier']
const frames = (v: TricolorVariant): TricolorFrame[] => tricolorScenario(v).map((s) => s.state)
const reachableWhite = (f: TricolorFrame): string[] => {
  const live = reachable(f)
  return f.objects.filter((o) => o.color === 'white' && live.has(o.id)).map((o) => o.id)
}

describe('tricolor', () => {
  it('граф из 7–9 объектов, оба варианта начинаются одинаково, всё белое', () => {
    const [a, b] = VARIANTS.map((v) => frames(v)[0]!)
    expect(a!.objects.length).toBeGreaterThanOrEqual(7)
    expect(a!.objects.length).toBeLessThanOrEqual(9)
    expect(a!.objects).toEqual(b!.objects)
    expect(a!.edges).toEqual(b!.edges)
    expect(a!.objects.every((o) => o.color === 'white' && !o.freed)).toBe(true)
  })

  it('без барьера в конце есть достижимый белый объект, и он освобождён', () => {
    const last = frames('no-barrier').at(-1)!
    const lost = reachableWhite(last)
    expect(lost.length).toBeGreaterThan(0)
    expect(last.objects.filter((o) => lost.includes(o.id)).every((o) => o.freed)).toBe(true)
  })

  it('с барьером в конце достижимых белых нет, освобождён только мусор', () => {
    const last = frames('barrier').at(-1)!
    expect(reachableWhite(last)).toEqual([])
    const live = reachable(last)
    const freed = last.objects.filter((o) => o.freed)
    expect(freed.length).toBeGreaterThan(0)
    expect(freed.every((o) => !live.has(o.id))).toBe(true)
  })

  it('инвариант «чёрный не указывает на белый» нарушается только без барьера', () => {
    expect(frames('no-barrier').some((f) => blackToWhite(f).length > 0)).toBe(true)
    expect(frames('barrier').every((f) => blackToWhite(f).length === 0)).toBe(true)
  })

  it('мутатор кладёт указатель в чёрный объект и стирает его из серого', () => {
    for (const v of VARIANTS) {
      const writes = frames(v).flatMap((f) => (f.write ? [f.write] : []))
      expect(writes.some((w) => w.to !== null)).toBe(true)
      expect(writes.some((w) => w.to === null && w.old !== null)).toBe(true)
    }
  })

  it('к уборке серых нет; освобождаются ровно белые', () => {
    for (const v of VARIANTS) {
      for (const f of frames(v).filter((x) => x.phase === 'sweep')) {
        expect(f.objects.some((o) => o.color === 'grey')).toBe(false)
        expect(f.objects.every((o) => !o.freed || o.color === 'white')).toBe(true)
      }
    }
  })

  it('подписи содержат ключевые термины из глоссария', () => {
    const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
    for (const v of VARIANTS) {
      const steps = tricolorScenario(v)
      const ids = steps.flatMap((s) => captionTermIds(s.caption))
      for (const id of ['tricolor-marking', 'stop-the-world', 'write-barrier']) expect(ids).toContain(id)
      for (const id of ids) expect(known.has(id)).toBe(true)
      for (const s of steps) expect(captionText(s.caption).trim().length).toBeGreaterThan(20)
    }
  })
})
