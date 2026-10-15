import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { JOB_IDS, ciGraphScenario } from './ci-graph'

const agg = ciGraphScenario('aggregate')
const onp = ciGraphScenario('on-paths')
const af = agg.map((s) => s.state)
const pf = onp.map((s) => s.state)
const status = (f: (typeof af)[number], id: string) => f.jobs.find((j) => j.id === id)?.status

describe('aggregate', () => {
  it('в каждом кадре все десять джоб, в заданном порядке', () => {
    for (const f of af) expect(f.jobs.map((j) => j.id)).toEqual([...JOB_IDS])
  })

  it('обязательная проверка - только ci-ok; последний кадр mergeable', () => {
    for (const f of af) expect(f.required).toEqual(['ci-ok'])
    expect(af[af.length - 1]!.pr).toBe('mergeable')
    expect(af.slice(0, -1).some((f) => f.pr === 'mergeable')).toBe(false)
  })

  it('ни одна messenger-* не running; они пропущены после changes', () => {
    for (const f of af) for (const j of f.jobs.filter((x) => x.id.startsWith('messenger-'))) expect(j.status).not.toBe('running')
    const last = af[af.length - 1]!
    for (const id of ['messenger-lint', 'messenger-test', 'messenger-generate', 'messenger-vuln', 'labs', 'image', 'workflows']) {
      expect(status(last, id)).toBe('skipped')
    }
    expect(status(last, 'course')).toBe('success')
  })

  it('ci-ok становится success только когда все остальные success или skipped', () => {
    const done = af.filter((f) => status(f, 'ci-ok') === 'success')
    expect(done.length).toBeGreaterThan(0)
    for (const f of done) {
      for (const j of f.jobs.filter((x) => x.id !== 'ci-ok')) expect(['success', 'skipped']).toContain(j.status)
    }
  })

  it('course проходит через running', () => {
    expect(af.some((f) => status(f, 'course') === 'running')).toBe(true)
  })
})

describe('on-paths', () => {
  it('ни одна джоба не выходит из pending', () => {
    for (const f of pf) for (const j of f.jobs) expect(j.status).toBe('pending')
  })

  it('обязательные - джобы мессенджера; последний кадр waiting, не mergeable', () => {
    for (const f of pf) {
      expect(f.required.length).toBeGreaterThan(0)
      for (const id of f.required) expect(id.startsWith('messenger-')).toBe(true)
    }
    expect(pf[pf.length - 1]!.pr).toBe('waiting')
    expect(pf.some((f) => f.pr === 'mergeable')).toBe(false)
  })

  it('в подписи есть фраза GitHub про ожидание статуса', () => {
    expect(onp.map((s) => captionText(s.caption)).join(' ')).toContain('Waiting for status to be reported')
  })
})

it('подписи содержат required-check, ci-job, monorepo; термины из глоссария', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  for (const steps of [agg, onp]) {
    const ids = steps.flatMap((s) => captionTermIds(s.caption))
    for (const id of ['required-check', 'ci-job', 'monorepo']) expect(ids).toContain(id)
    for (const id of ids) expect(known.has(id)).toBe(true)
    for (const s of steps) expect(captionText(s.caption).trim().length).toBeGreaterThan(20)
  }
})
