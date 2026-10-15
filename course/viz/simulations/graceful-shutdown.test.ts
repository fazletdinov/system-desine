import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { gracefulShutdownScenario } from './graceful-shutdown'

const exec = gracefulShutdownScenario('exec-form')
const wrap = gracefulShutdownScenario('wrapper-script')
const ef = exec.map((s) => s.state)
const wf = wrap.map((s) => s.state)

describe('exec-form', () => {
  it('readyz 503 с первого кадра, SIGTERM получило приложение', () => {
    expect(ef[0]!.readyz).toBe(503)
    expect(ef[0]!.t).toBe(0)
    expect(ef[0]!.signalAt).toBe('pid1-app')
    expect(ef[0]!.newRequests).toBe('served')
    expect(ef[0]!.inLb).toBe(true)
  })

  it('балансировщик убирает инстанс раньше, чем процесс начинает останавливаться', () => {
    const lbOut = ef.findIndex((f) => !f.inLb)
    const shutting = ef.findIndex((f) => f.process === 'shutting-down')
    expect(lbOut).toBeGreaterThan(0)
    expect(lbOut).toBeLessThan(shutting)
  })

  it('во время остановки запросы отклоняются и inflight не растёт', () => {
    const phase = ef.filter((f) => f.process === 'shutting-down')
    expect(phase.every((f) => f.newRequests === 'refused')).toBe(true)
    phase.forEach((f, i) => {
      if (i > 0) expect(f.inflight).toBeLessThanOrEqual(phase[i - 1]!.inflight)
    })
  })

  it('последний кадр exited, код 0, t < 30; время не убывает', () => {
    const last = ef[ef.length - 1]!
    expect(last.process).toBe('exited')
    expect(last.exitCode).toBe(0)
    expect(last.inflight).toBe(0)
    expect(last.t).toBeLessThan(30)
    ef.forEach((f, i) => {
      if (i > 0) expect(f.t).toBeGreaterThanOrEqual(ef[i - 1]!.t)
    })
  })
})

describe('wrapper-script', () => {
  it('SIGTERM получил sh, readyz остаётся 200 до killed', () => {
    expect(wf[0]!.signalAt).toBe('pid1-sh')
    expect(wf.slice(0, -1).every((f) => f.readyz === 200 && f.process === 'running')).toBe(true)
  })

  it('последний кадр killed при t = 10, код 137, соединения оборваны', () => {
    const last = wf[wf.length - 1]!
    expect(last.process).toBe('killed')
    expect(last.t).toBe(10)
    expect(last.exitCode).toBe(137)
    expect(last.inflight).toBeGreaterThan(0)
  })
})

it('подписи содержат graceful-shutdown, readiness-probe, pid-1; термины из глоссария', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  for (const steps of [exec, wrap]) {
    const ids = steps.flatMap((s) => captionTermIds(s.caption))
    for (const id of ['graceful-shutdown', 'readiness-probe', 'pid-1']) expect(ids).toContain(id)
    for (const id of ids) expect(known.has(id)).toBe(true)
    for (const s of steps) expect(captionText(s.caption).trim().length).toBeGreaterThan(20)
  }
})

it('подпись wrapper-script не утверждает, что Go-приложение само игнорирует SIGTERM', () => {
  const text = wrap.map((s) => captionText(s.caption)).join(' ')
  expect(text).toMatch(/sh/)
  expect(text).toMatch(/SIGKILL/)
  expect(text).toMatch(/137/)
})
