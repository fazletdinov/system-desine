import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { MEM_LIMIT_MIB, gomemlimitScenario } from './gomemlimit'

const without = gomemlimitScenario('without')
const withLimit = gomemlimitScenario('with')
const wo = without.map((s) => s.state)
const wi = withLimit.map((s) => s.state)

describe('without', () => {
  it('цель GC превышает лимит, последний кадр killed', () => {
    expect(wo.some((f) => f.heapGoalMiB > f.limitMiB)).toBe(true)
    expect(wo[wo.length - 1]!.killed).toBe(true)
    expect(wo.slice(0, -1).some((f) => f.killed)).toBe(false)
  })

  it('цель GC равна живой куче, умноженной на два (GOGC=100)', () => {
    for (const f of wo) expect(f.heapGoalMiB).toBe(f.heapLiveMiB * 2)
  })
})

describe('with', () => {
  it('цель не выше 230, RSS не выше лимита, процесс жив', () => {
    for (const f of wi) {
      expect(f.heapGoalMiB).toBeLessThanOrEqual(230)
      expect(f.rssMiB).toBeLessThanOrEqual(f.limitMiB)
      expect(f.killed).toBe(false)
    }
  })

  it('GC запускается чаще: gcRuns на пике больше, чем без лимита', () => {
    const last = wo[wo.length - 1]!
    const same = wi.find((f) => f.t === last.t)!
    expect(same.gcRuns).toBeGreaterThan(last.gcRuns)
  })
})

describe('общее', () => {
  it('лимит 256 МиБ, шкала времени и живая куча одинаковы в обоих вариантах', () => {
    for (const f of [...wo, ...wi]) expect(f.limitMiB).toBe(MEM_LIMIT_MIB)
    wo.forEach((f, i) => {
      expect(wi[i]!.t).toBe(f.t)
      expect(wi[i]!.heapLiveMiB).toBe(f.heapLiveMiB)
    })
  })

  it('gcRuns не убывает', () => {
    for (const fr of [wo, wi]) fr.forEach((f, i) => { if (i > 0) expect(f.gcRuns).toBeGreaterThanOrEqual(fr[i - 1]!.gcRuns) })
  })

  it('первая подпись говорит об упрощённой модели', () => {
    expect(captionText(without[0]!.caption).toLowerCase()).toContain('упрощённая модель')
    expect(captionText(withLimit[0]!.caption).toLowerCase()).toContain('упрощённая модель')
  })

  it('подписи содержат gomemlimit и oom-kill; термины из глоссария', () => {
    const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
    for (const steps of [without, withLimit]) {
      const ids = steps.flatMap((s) => captionTermIds(s.caption))
      expect(ids).toContain('gomemlimit')
      expect(ids).toContain('oom-kill')
      for (const id of ids) expect(known.has(id)).toBe(true)
    }
  })
})
