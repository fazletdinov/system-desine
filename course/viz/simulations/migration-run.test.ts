import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { EDITS_TABLE, INIT_TABLES, migrationRunScenario, schemaVersion, type MigFrame, type MigVariant } from './migration-run'

const VARIANTS: MigVariant[] = ['first', 'failure', 'outdated']
const frames = (v: MigVariant): MigFrame[] => migrationRunScenario(v).map((s) => s.state)
const visible = (f: MigFrame): string[] => f.tables.filter((t) => t.visibility === 'committed').map((t) => t.name)
const all = (f: MigFrame): string[] => f.tables.map((t) => t.name)

describe.each(VARIANTS)('%s', (v) => {
  it('версия схемы не убывает; сервер принимает запросы только при версии ≥ required', () => {
    const f = frames(v)
    f.forEach((x, i) => {
      if (i > 0) expect(schemaVersion(x) ?? -1).toBeGreaterThanOrEqual(schemaVersion(f[i - 1]!) ?? -1)
      if (x.server === 'serving') expect(schemaVersion(x)!).toBeGreaterThanOrEqual(x.required)
    })
  })

  it('таблица видна вне транзакции только после COMMIT, и версия записана в той же транзакции', () => {
    for (const x of frames(v)) {
      if (x.tx?.state === 'open') expect(x.tables.some((t) => t.visibility === 'tx')).toBe(true)
      if (x.tx?.state === 'committed') expect(schemaVersion(x)).toBe(x.tx.version)
      if (x.tx === null || x.tx.state !== 'open') expect(x.tables.every((t) => t.visibility === 'committed')).toBe(true)
    }
  })
})

describe('first', () => {
  const f = frames('first')
  it('сначала нет таблицы версий, затем строка 0, в конце версия 1, шесть таблиц и сервер работает', () => {
    expect(f[0]!.versions).toBeNull()
    expect(f[1]!.versions).toEqual([{ version_id: 0, is_applied: true }])
    const last = f.at(-1)!
    expect(schemaVersion(last)).toBe(1)
    expect(visible(last)).toEqual([...INIT_TABLES])
    expect(last.migrate).toBe('exit-0')
    expect(last.server).toBe('serving')
  })
  it('сервер не стартует, пока migrate не вышел с кодом 0', () => {
    for (const x of f) if (x.migrate !== 'exit-0') expect(x.server).toBe('blocked')
  })
})

describe('failure', () => {
  const f = frames('failure')
  const iFail = f.findIndex((x) => x.tx?.state === 'rolled-back')
  it('после сбоя версия не меняется, а таблиц из миграции нет', () => {
    expect(iFail).toBeGreaterThan(0)
    const failed = f[iFail]!
    expect(schemaVersion(failed)).toBe(1)
    expect(all(failed)).not.toContain(EDITS_TABLE)
    expect(failed.tx!.stmts.some((s) => s.status === 'failed')).toBe(true)
    expect(failed.tx!.stmts.some((s) => s.status === 'done' && s.sql.startsWith('CREATE TABLE'))).toBe(true)
    expect(failed.migrate).toBe('exit-1')
    expect(failed.server).toBe('blocked')
  })
  it('до сбоя таблица была видна только внутри транзакции; повтор применяет миграцию', () => {
    expect(f.slice(0, iFail).some((x) => x.tables.some((t) => t.name === EDITS_TABLE && t.visibility === 'tx'))).toBe(true)
    const last = f.at(-1)!
    expect(schemaVersion(last)).toBe(2)
    expect(visible(last)).toContain(EDITS_TABLE)
    expect(last.migrate).toBe('exit-0')
  })
})

describe('outdated', () => {
  const f = frames('outdated')
  it('при версии ниже required сервер выходит с кодом 1, после migrate up — стартует', () => {
    const iRefuse = f.findIndex((x) => x.server === 'exit-1')
    expect(schemaVersion(f[iRefuse]!)).toBeLessThan(f[iRefuse]!.required)
    expect(f[iRefuse]!.log).toContain('примените миграции')
    expect(f.slice(0, iRefuse).some((x) => x.server === 'serving')).toBe(false)
    const last = f.at(-1)!
    expect(last.server).toBe('serving')
    expect(schemaVersion(last)).toBe(2)
  })
})

it('подписи: термины из глоссария, migration и transaction есть', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  const ids = VARIANTS.flatMap((v) => migrationRunScenario(v).flatMap((s) => captionTermIds(s.caption)))
  for (const id of ['migration', 'transaction']) expect(ids).toContain(id)
  for (const id of ids) expect(known.has(id)).toBe(true)
  for (const v of VARIANTS) for (const s of migrationRunScenario(v)) expect(captionText(s.caption).trim().length).toBeGreaterThan(40)
})
