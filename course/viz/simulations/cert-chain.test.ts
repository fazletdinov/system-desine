import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { ERROR_BRANCH_START, UNKNOWN_AUTHORITY_ERROR, certChainScenario, type CertStatus } from './cert-chain'

const steps = certChainScenario()
const success = steps.slice(0, ERROR_BRANCH_START)
const failure = steps.slice(ERROR_BRANCH_START)

function statusesOf(branch: typeof steps, id: 'leaf' | 'intermediate' | 'root'): CertStatus[] {
  return branch.map((s) => s.state.certs.find((c) => c.id === id)!.status)
}

describe('certChainScenario', () => {
  it('успешная ветка — 6 шагов, затем ветка-ошибка', () => {
    expect(success).toHaveLength(6)
    expect(failure.length).toBeGreaterThanOrEqual(3)
  })

  it.each([
    ['успех', success],
    ['ошибка', failure],
  ])('ветка «%s»: каждая ветка начинается со всех pending, статусы меняются только pending → ok|fail', (_n, branch) => {
    expect(branch[0]!.state.certs.every((c) => c.status === 'pending')).toBe(true)
    for (const id of ['leaf', 'intermediate', 'root'] as const) {
      const seq = statusesOf(branch, id)
      for (let i = 1; i < seq.length; i++) {
        if (seq[i] !== seq[i - 1]) expect(seq[i - 1]).toBe('pending')
      }
    }
  })

  it('успех: в конце все сертификаты ok', () => {
    expect(success.at(-1)!.state.certs.every((c) => c.status === 'ok')).toBe(true)
  })

  it('ошибка: корень fail, текст ошибки содержит unknown authority', () => {
    const last = failure.at(-1)!
    expect(last.state.certs.find((c) => c.id === 'root')!.status).toBe('fail')
    expect(last.state.check).toContain('unknown authority')
    expect(UNKNOWN_AUTHORITY_ERROR).toBe('x509: certificate signed by unknown authority')
  })

  it('у шагов есть пояснения', () => {
    for (const s of steps) expect(captionText(s.caption).trim().length).toBeGreaterThan(20)
  })
})

it('все термины в подписях есть в глоссарии', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  const ids = steps.flatMap((s) => captionTermIds(s.caption))
  expect(ids.length).toBeGreaterThanOrEqual(4)
  for (const id of ids) expect(known.has(id)).toBe(true)
})
