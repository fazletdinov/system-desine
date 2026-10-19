import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { appLifecycleScenario, BACK_STOP_TIMEOUT_S, DRAIN_DELAY_S, HASH_CONCURRENCY, HASH_QUEUE, SHUTDOWN_TIMEOUT_S, type LifecycleVariant } from './app-lifecycle'

const VARIANTS: LifecycleVariant[] = ['signal', 'component-failure', 'second-signal']
const frames = (v: LifecycleVariant) => appLifecycleScenario(v).map((s) => s.state)

it('значения мессенджера', () => {
  expect([DRAIN_DELAY_S, SHUTDOWN_TIMEOUT_S, BACK_STOP_TIMEOUT_S, HASH_CONCURRENCY, HASH_QUEUE]).toEqual([5, 10, 2, 2, 8])
})

describe.each(VARIANTS)('%s', (v) => {
  it('время не убывает, очередь хешей в пределах лимитов, имя компонента idempotency-sweeper', () => {
    frames(v).forEach((f, i) => {
      if (i > 0) expect(f.t).toBeGreaterThanOrEqual(frames(v)[i - 1]!.t)
      expect(f.hashQueue.running).toBeLessThanOrEqual(HASH_CONCURRENCY)
      expect(f.hashQueue.waiting).toBeLessThanOrEqual(HASH_QUEUE)
      expect(f.back.map((b) => b.name)).toEqual(['idempotency-sweeper'])
    })
  })

  it('back не переходит в stopping, пока http не stopped (кроме немедленного выхода)', () => {
    if (v === 'second-signal') return
    for (const f of frames(v)) if (f.back[0]!.state === 'stopping') expect(f.http).toBe('stopped')
  })
})

describe('signal', () => {
  const f = frames('signal')
  it('работа: 2 хеша выполняются, 3 ждут; затем SIGTERM даёт 503 и draining', () => {
    expect(f[0]!.http).toBe('serving')
    expect(f[0]!.readyz).toBe(200)
    expect(f[0]!.hashQueue).toEqual({ running: 2, waiting: 3, rejected: 0 })
    expect(f.find((x) => x.http === 'draining')!.readyz).toBe(503)
  })
  it('shutting-down наступает на t = 5 после SIGTERM, порядок http → back → exit 0', () => {
    const sig = f.find((x) => x.http === 'draining')!
    const down = f.find((x) => x.http === 'shutting-down')!
    expect(down.t - sig.t).toBe(DRAIN_DELAY_S)
    const iStopped = f.findIndex((x) => x.http === 'stopped')
    const iStopping = f.findIndex((x) => x.back[0]!.state === 'stopping')
    const iBackStopped = f.findIndex((x) => x.back[0]!.state === 'stopped')
    expect(iStopped).toBeLessThan(iStopping)
    expect(iStopping).toBeLessThan(iBackStopped)
    expect(f.at(-1)!.exit).toBe(0)
    expect(f.at(-1)!.t - f[iStopping]!.t).toBeLessThanOrEqual(BACK_STOP_TIMEOUT_S)
  })
})

describe('component-failure', () => {
  const f = frames('component-failure')
  it('back failed, readyz 503 раньше shutting-down, последний кадр exit 1', () => {
    const iFail = f.findIndex((x) => x.back[0]!.state === 'failed')
    const iDown = f.findIndex((x) => x.http === 'shutting-down')
    const i503 = f.findIndex((x) => x.readyz === 503)
    expect(iFail).toBeGreaterThan(0)
    expect(i503).toBeLessThan(iDown)
    expect(f[iFail]!.http).not.toBe('serving')
    expect(f.at(-1)!.exit).toBe(1)
    expect(f.at(-1)!.http).toBe('stopped')
  })
})

describe('second-signal', () => {
  const f = frames('second-signal')
  it('exit 1 при t < 5, HTTP не дошёл до stopped', () => {
    const last = f.at(-1)!
    expect(last.exit).toBe(1)
    expect(last.t).toBeLessThan(DRAIN_DELAY_S)
    expect(last.t).toBe(2)
    expect(f.some((x) => x.http === 'stopped')).toBe(false)
  })
})

it('подписи содержат component-lifecycle, graceful-shutdown, errgroup, crash-only; термины из глоссария', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  for (const v of VARIANTS) {
    const steps = appLifecycleScenario(v)
    const ids = steps.flatMap((s) => captionTermIds(s.caption))
    for (const id of ['component-lifecycle', 'graceful-shutdown', 'errgroup', 'crash-only']) expect(ids).toContain(id)
    for (const id of ids) expect(known.has(id)).toBe(true)
    for (const s of steps) expect(captionText(s.caption).trim().length).toBeGreaterThan(20)
  }
})
