import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { gStates, netpollerScenario, type NetpollerFrame, type NetpollerVariant } from './netpoller'

const VARIANTS: readonly NetpollerVariant[] = ['data', 'deadline']
const frames = (v: NetpollerVariant): NetpollerFrame[] => netpollerScenario(v).map((s) => s.state)

describe('netpoller', () => {
  it('в каждом шаге у каждой G ровно одно состояние', () => {
    for (const v of VARIANTS) {
      for (const f of frames(v)) {
        for (const g of f.gs) expect(gStates(f, g)).toHaveLength(1)
      }
    }
  })

  it('M не простаивает, пока есть готовые G', () => {
    for (const v of VARIANTS) {
      for (const f of frames(v)) if (f.runq.length > 0) expect(f.m.g).not.toBeNull()
    }
  })

  it('G1 паркуется на pollDesc после EAGAIN, а M в это время выполняет другую G', () => {
    for (const v of VARIANTS) {
      const parked = frames(v).filter((f) => f.pollDesc.rg === 'G1')
      expect(parked.length).toBeGreaterThan(0)
      expect(parked.some((f) => f.m.g !== null && f.m.g !== 'G1')).toBe(true)
      const firstPark = frames(v).findIndex((f) => f.pollDesc.rg === 'G1')
      expect(frames(v).slice(0, firstPark).some((f) => f.syscall?.includes('EAGAIN'))).toBe(true)
    }
  })

  it('данные: G1 будит netpoll из findRunnable через epoll, чтение успешно', () => {
    const fs = frames('data')
    expect(fs.some((f) => f.m.where === 'findRunnable' && f.epoll.ready)).toBe(true)
    expect(fs.at(-1)!.result).toEqual({ ok: true, n: expect.any(Number) })
    expect(fs.every((f) => f.timer === null)).toBe(true)
  })

  it('дедлайн: таймер рантайма будит G1, чтение заканчивается ошибкой', () => {
    const fs = frames('deadline')
    expect(fs.some((f) => f.timer?.fired === true)).toBe(true)
    expect(fs.at(-1)!.result).toEqual({ ok: false, err: 'os.ErrDeadlineExceeded' })
    expect(fs.every((f) => f.socketBytes === 0)).toBe(true)
  })

  it('время не убывает', () => {
    for (const v of VARIANTS) frames(v).forEach((f, i, all) => i > 0 && expect(f.t).toBeGreaterThanOrEqual(all[i - 1]!.t))
  })

  it('подписи содержат ключевые термины из глоссария', () => {
    const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
    for (const v of VARIANTS) {
      const steps = netpollerScenario(v)
      const ids = steps.flatMap((s) => captionTermIds(s.caption))
      for (const id of ['netpoller', 'epoll', 'nonblocking-io']) expect(ids).toContain(id)
      for (const id of ids) expect(known.has(id)).toBe(true)
      for (const s of steps) expect(captionText(s.caption).trim().length).toBeGreaterThan(20)
    }
    expect(netpollerScenario('data').flatMap((s) => captionTermIds(s.caption))).toContain('edge-triggered')
    expect(netpollerScenario('deadline').flatMap((s) => captionTermIds(s.caption))).toContain('deadline')
  })
})
