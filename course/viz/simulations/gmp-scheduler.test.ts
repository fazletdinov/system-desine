import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { gmpSchedulerScenario, type GmpFrame } from './gmp-scheduler'

const steps = gmpSchedulerScenario()
const frames = steps.map((s) => s.state)

const placements = (f: GmpFrame): string[] => [
  ...f.ps.flatMap((p) => [p.running, p.runnext, ...p.local].filter((g): g is string => g !== null)),
  ...f.global,
]

describe('gmp-scheduler', () => {
  it('сценарий детерминирован', () => {
    expect(gmpSchedulerScenario()).toEqual(steps)
  })

  it('у каждого P на кадре не больше одной running G, и каждая G стоит не более чем в одном месте', () => {
    for (const f of frames) {
      expect(f.ps.map((p) => p.id)).toEqual(['P0', 'P1'])
      const placed = placements(f)
      expect(new Set(placed).size).toBe(placed.length)
    }
  })

  it('go G2, G3, G4: последняя в runnext, предыдущие в локальной очереди по порядку', () => {
    const f = frames.filter((x) => x.note === 'spawn')[2]!
    expect(f.ps[0]!.runnext).toBe('G4')
    expect(f.ps[0]!.local).toEqual(['G2', 'G3'])
  })

  it('после steal длины локальных очередей P0 и P1 отличаются не больше чем на 1', () => {
    const stolen = frames.filter((f) => f.note === 'steal')
    expect(stolen.length).toBeGreaterThan(0)
    for (const f of stolen) {
      expect(Math.abs(f.ps[0]!.local.length - f.ps[1]!.local.length)).toBeLessThanOrEqual(1)
      expect(f.ps[1]!.running).not.toBeNull()
    }
  })

  it('во время syscall-handoff у P1 другой M, чем M1, а M1 в syscall', () => {
    const hand = frames.filter((f) => f.note === 'syscall-handoff')
    expect(hand.length).toBeGreaterThan(0)
    for (const f of hand) {
      expect(f.ps[1]!.m).not.toBe('M1')
      expect(f.ms.find((m) => m.id === 'M1')!.state).toBe('syscall')
    }
  })

  it('preempt: G1 уходит из P0 в хвост глобальной очереди', () => {
    const i = frames.findIndex((f) => f.note === 'preempt')
    expect(frames[i - 1]!.ps[0]!.running).toBe('G1')
    expect(frames[i]!.ps[0]!.running).not.toBe('G1')
    expect(frames[i]!.global.at(-1)).toBe('G1')
  })

  it('после syscall-return G5 в global, M1 idle', () => {
    const f = frames.find((x) => x.note === 'syscall-return')!
    expect(f.global).toContain('G5')
    expect(f.ms.find((m) => m.id === 'M1')!.state).toBe('idle')
  })

  it('подписи содержат gmp, work-stealing, preemption, sysmon; термины из глоссария', () => {
    const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
    const ids = steps.flatMap((s) => captionTermIds(s.caption))
    for (const id of ['gmp', 'work-stealing', 'preemption', 'sysmon']) expect(ids).toContain(id)
    for (const id of ids) expect(known.has(id)).toBe(true)
    for (const s of steps) expect(captionText(s.caption).trim().length).toBeGreaterThan(20)
  })
})
