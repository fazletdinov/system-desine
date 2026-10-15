import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { cgroupScenario, cpuShare, type CpuFrame, type MemoryFrame } from './cgroup-limits'

const mem = cgroupScenario('memory').map((s) => s.state) as MemoryFrame[]
const cpu = cgroupScenario('cpu').map((s) => s.state) as CpuFrame[]
const memSteps = cgroupScenario('memory')
const cpuSteps = cgroupScenario('cpu')

describe('cgroup memory', () => {
  it('использование не убывает до OOM и никогда не выше лимита', () => {
    const beforeKill = mem.filter((f) => !f.oomKilled)
    beforeKill.forEach((f, i) => {
      if (i > 0) expect(f.usageMiB).toBeGreaterThanOrEqual(beforeKill[i - 1]!.usageMiB)
    })
    for (const f of mem) expect(f.usageMiB).toBeLessThanOrEqual(f.maxMiB)
  })

  it('растёт 16, 32, 48, 60, затем лимит 64', () => {
    const usage = mem.map((f) => f.usageMiB)
    expect(usage.slice(0, 4)).toEqual([16, 32, 48, 60])
    expect(Math.max(...usage)).toBe(64)
    expect(mem.every((f) => f.maxMiB === 64)).toBe(true)
  })

  it('ядро пытается освободить кэш около 60 МиБ', () => {
    const f = mem.find((x) => x.reclaimed)
    expect(f?.usageMiB).toBeGreaterThanOrEqual(60)
  })

  it('oom_kill равен 1 только в последнем кадре, oom - 1 с кадра достижения лимита', () => {
    mem.forEach((f, i) => expect(f.events.oom_kill).toBe(i === mem.length - 1 ? 1 : 0))
    expect(mem[mem.length - 1]!.oomKilled).toBe(true)
    expect(mem.filter((f) => f.oomKilled)).toHaveLength(1)
    expect(mem[mem.length - 2]!.events.oom).toBe(1)
  })

  it('в последней подписи код выхода 137', () => {
    expect(captionText(memSteps[memSteps.length - 1]!.caption)).toContain('137')
  })
})

describe('cgroup cpu', () => {
  it('4-5 кадров, в каждом периоде used не больше квоты', () => {
    expect(cpu.length).toBeGreaterThanOrEqual(4)
    expect(cpu.length).toBeLessThanOrEqual(5)
    for (const f of cpu) for (const p of f.periods) expect(p.used).toBeLessThanOrEqual(f.quotaUs)
  })

  it('при спросе 100% все периоды throttled, доля CPU 50%', () => {
    const last = cpu[cpu.length - 1]!
    expect(last.periods.every((p) => p.throttled)).toBe(true)
    expect(cpuShare(last)).toBe(0.5)
  })

  it('в последнем кадре есть nr_throttled и throttled_usec, согласованные с периодами', () => {
    const last = cpu[cpu.length - 1]!
    expect(last.stat?.nr_throttled).toBe(last.periods.length)
    expect(last.stat?.throttled_usec).toBe(last.periods.length * (last.periodUs - last.quotaUs))
    for (const f of cpu.slice(0, -1)) expect(f.stat).toBeUndefined()
  })
})

it('подписи содержат cgroup, oom-kill (memory), cpu-throttling (cpu); термины из глоссария', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  const m = memSteps.flatMap((s) => captionTermIds(s.caption))
  const c = cpuSteps.flatMap((s) => captionTermIds(s.caption))
  expect(m).toContain('cgroup')
  expect(m).toContain('oom-kill')
  expect(c).toContain('cgroup')
  expect(c).toContain('cpu-throttling')
  for (const id of [...m, ...c]) expect(known.has(id)).toBe(true)
  for (const s of [...memSteps, ...cpuSteps]) expect(captionText(s.caption).trim().length).toBeGreaterThan(20)
})
