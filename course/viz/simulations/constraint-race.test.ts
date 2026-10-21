import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { CONSTRAINT, ERROR_CHAIN, USERNAME, constraintRaceScenario, type RaceFrame, type RaceVariant } from './constraint-race'

const VARIANTS: RaceVariant[] = ['code-check', 'unique']
const frames = (v: RaceVariant): RaceFrame[] => constraintRaceScenario(v).map((s) => s.state)
const committed = (f: RaceFrame) => f.rows.filter((r) => r.committed && r.username === USERNAME)
const http = (f: RaceFrame) => f.instances.map((i) => i.http)

describe('проверка в коде', () => {
  const f = frames('code-check')
  it('в финале два пользователя с одним именем, оба клиента получили 201, ошибки нет', () => {
    const last = f.at(-1)!
    expect(last.unique).toBe(false)
    expect(committed(last).map((r) => r.by)).toEqual(['A', 'B'])
    expect(http(last)).toEqual([201, 201])
    expect(f.every((x) => x.sqlstate === null)).toBe(true)
    expect(last.verdict).toBe('broken')
  })
  it('оба SELECT происходят раньше обоих INSERT', () => {
    const firstInsert = f.findIndex((x) => x.instances.some((i) => i.sql?.startsWith('INSERT')))
    const bothSelected = f.findIndex((x) => x.instances.every((i) => i.sql?.startsWith('SELECT')))
    expect(bothSelected).toBeGreaterThan(0)
    expect(bothSelected).toBeLessThan(firstInsert)
  })
})

describe('UNIQUE', () => {
  const f = frames('unique')
  it('в финале один пользователь, ошибка 23505 по users_username_key, клиенту B — 409', () => {
    const last = f.at(-1)!
    expect(last.unique).toBe(true)
    expect(committed(last).map((r) => r.by)).toEqual(['A'])
    expect(last.rows).toHaveLength(1)
    expect(last.sqlstate).toBe('23505')
    expect(http(last)).toEqual([201, 409])
    expect(last.instances[1].result).toContain(`"${CONSTRAINT}"`)
    expect(last.verdict).toBe('held')
    expect(last.chain).toBe(ERROR_CHAIN.length - 1)
  })
  it('B ждёт, пока транзакция A не зафиксирована, и получает ошибку только после COMMIT A', () => {
    const iWait = f.findIndex((x) => x.instances[1].state === 'waiting')
    const iCommitA = f.findIndex((x) => x.instances[0].http === 201)
    const iErr = f.findIndex((x) => x.sqlstate === '23505')
    expect(f[iWait]!.rows.every((r) => !r.committed)).toBe(true)
    expect(iWait).toBeLessThan(iCommitA)
    expect(iCommitA).toBeLessThan(iErr)
  })
  it('цепочка перевода ошибки не идёт назад и начинается с PgError', () => {
    f.forEach((x, i) => i > 0 && expect(x.chain).toBeGreaterThanOrEqual(f[i - 1]!.chain))
    expect(ERROR_CHAIN[0].text).toContain('23505')
    expect(ERROR_CHAIN.map((c) => c.text).join(' ')).toContain('ErrUsernameTaken')
    expect(ERROR_CHAIN.at(-1)!.text).toMatch(/^409 application\/problem\+json/)
  })
})

it('подписи: термины из глоссария, constraint и transaction есть', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  for (const v of VARIANTS) {
    const steps = constraintRaceScenario(v)
    const ids = steps.flatMap((s) => captionTermIds(s.caption))
    expect(ids).toContain('constraint')
    for (const id of ids) expect(known.has(id)).toBe(true)
    for (const s of steps) expect(captionText(s.caption).trim().length).toBeGreaterThan(40)
  }
  expect(constraintRaceScenario('unique').flatMap((s) => captionTermIds(s.caption))).toContain('transaction')
})
