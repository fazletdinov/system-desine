import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { STARVATION_MS, mutexModesScenario } from './mutex-modes'

const steps = mutexModesScenario()
const frames = steps.map((s) => s.state)

describe('mutex-modes', () => {
  it('порог голодания — 1 мс', () => {
    expect(STARVATION_MS).toBe(1)
  })

  it('время не убывает', () => {
    frames.forEach((f, i) => i > 0 && expect(f.t).toBeGreaterThanOrEqual(frames[i - 1]!.t))
  })

  it('в normal захват новичком (barging) случается раньше, чем наступает starving', () => {
    expect(frames.some((f) => f.mode === 'normal' && f.handoff === 'barging')).toBe(true)
    expect(frames.findIndex((f) => f.handoff === 'barging')).toBeLessThan(frames.findIndex((f) => f.mode === 'starving'))
  })

  it('starving наступает только когда первый ожидающий ждёт дольше 1 мс', () => {
    frames.forEach((f, i) => {
      if (f.mode !== 'starving' || frames[i - 1]?.mode === 'starving') return
      expect(f.waiters.length).toBeGreaterThan(0)
      expect(f.t - f.waiters[0]!.since).toBeGreaterThan(STARVATION_MS)
    })
  })

  it('в starving handoff всегда direct, новоприбывший не становится owner', () => {
    frames.forEach((f, i) => {
      if (f.mode !== 'starving') return
      expect(f.handoff).toBe('direct')
      if (f.arriving !== null) {
        expect(f.owner).not.toBe(f.arriving)
        const next = frames[i + 1]
        if (next?.mode === 'starving') expect(next.owner).not.toBe(f.arriving)
        expect(frames.slice(i).find((x) => x.owner === f.arriving)?.mode ?? 'normal').toBe('normal')
      }
    })
  })

  it('последний кадр normal', () => {
    expect(frames.at(-1)!.mode).toBe('normal')
  })

  it('подписи содержат mutex и starvation-mode; термины из глоссария', () => {
    const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
    const ids = steps.flatMap((s) => captionTermIds(s.caption))
    for (const id of ['mutex', 'starvation-mode']) expect(ids).toContain(id)
    for (const id of ids) expect(known.has(id)).toBe(true)
    for (const s of steps) expect(captionText(s.caption).trim().length).toBeGreaterThan(20)
  })
})
