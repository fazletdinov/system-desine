import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { ERASED_NAME, FK_VIOLATION, PARTIAL_KEY, PLAIN_KEY, UNIQUE_VIOLATION, softDeleteScenario, type SdFrame, type SoftDeleteVariant } from './soft-delete-pitfalls'

const VARIANTS: SoftDeleteVariant[] = ['unique', 'references', 'erasure']
const frames = (v: SoftDeleteVariant): SdFrame[] => softDeleteScenario(v).map((s) => s.state)
const live = (f: SdFrame, username: string) => f.users.filter((u) => u.username === username && !u.deleted)
const all = (f: SdFrame, username: string) => f.users.filter((u) => u.username === username)

describe('UNIQUE и повторная регистрация', () => {
  const f = frames('unique')
  it('при обычном UNIQUE новая alice получает 23505 по users_username_key, хотя живой alice нет', () => {
    const i = f.findIndex((x) => x.sqlstate === UNIQUE_VIOLATION)
    expect(f[i]!.rule).toBe('unique')
    expect(live(f[i]!, 'alice')).toHaveLength(0)
    expect(f[i]!.result).toContain(`"${PLAIN_KEY}"`)
    expect(f[i]!.verdict).toBe('broken')
  })
  it('мягкое удаление не убирает строку: после него alice в таблице, но помечена', () => {
    const i = f.findIndex((x) => x.sql?.startsWith('UPDATE users SET deleted_at'))
    expect(all(f[i]!, 'alice')).toHaveLength(1)
    expect(all(f[i]!, 'alice')[0]!.deleted).toBe(true)
  })
  it('с частичным индексом в любом кадре не больше одной живой alice, а всего их может быть две', () => {
    for (const x of f.filter((x) => x.rule === 'partial')) expect(live(x, 'alice').length).toBeLessThanOrEqual(1)
    expect(all(f.at(-1)!, 'alice')).toHaveLength(2)
  })
  it('в финале вторая живая alice отвергнута ошибкой с именем частичного индекса', () => {
    const last = f.at(-1)!
    expect(last.rule).toBe('partial')
    expect(last.sqlstate).toBe(UNIQUE_VIOLATION)
    expect(last.result).toContain(`"${PARTIAL_KEY}"`)
    expect(last.verdict).toBe('fixed')
  })
  it('правило уникальности только переходит от UNIQUE к частичному индексу, но не назад', () => {
    const switchAt = f.findIndex((x) => x.rule === 'partial')
    expect(switchAt).toBeGreaterThan(0)
    f.slice(switchAt).forEach((x) => expect(x.rule).toBe('partial'))
  })
})

describe('забытый фильтр и ссылки', () => {
  const f = frames('references')
  it('вход без deleted_at IS NULL находит обе alice — живую и удалённую', () => {
    const login = f.find((x) => x.sql?.includes("WHERE username = 'alice'") && !x.view)!
    expect(login.flagUsers).toEqual(expect.arrayContaining(all(login, 'alice').map((u) => u.id)))
    expect(all(login, 'alice').some((u) => u.deleted)).toBe(true)
    expect(login.verdict).toBe('broken')
  })
  it('сессия удалённого пользователя жива, пока её не удалит код', () => {
    const deletedIds = new Set(f[0]!.users.filter((u) => u.deleted).map((u) => u.id))
    const flagged = f.find((x) => x.flagRefs.includes('tok-alice'))!
    expect(flagged.refs.some((r) => r.table === 'sessions' && deletedIds.has(r.userId))).toBe(true)
    expect(f.at(-1)!.refs.some((r) => r.table === 'sessions' && deletedIds.has(r.userId))).toBe(false)
  })
  it('внешний ключ принимает сообщение от удалённого пользователя без ошибки', () => {
    const i = f.findIndex((x) => x.sql?.startsWith('INSERT INTO messages'))
    const deleted = f[i]!.users.find((u) => u.id === 1)!
    expect(deleted.deleted).toBe(true)
    expect(f[i]!.outcome).toBe('ok')
    expect(f[i]!.sqlstate).toBeNull()
    expect(f[i]!.refs.length).toBe(f[i - 1]!.refs.length + 1)
  })
  it('представление появляется к исправлению и через него находится одна живая alice по частичному индексу', () => {
    const v = f.find((x) => x.view)!
    expect(v.result).toMatch(/^1 строка: id 4/)
    expect(v.result).toContain(PARTIAL_KEY)
    expect(f.at(-1)!.verdict).toBe('fixed')
  })
})

describe('право на удаление', () => {
  const f = frames('erasure')
  const namesInAudit = (x: SdFrame) => (x.audit ?? []).filter((a) => a.oldName === ERASED_NAME || a.newName === ERASED_NAME).length
  const namesInUsers = (x: SdFrame) => x.users.filter((u) => u.id === 1 && u.username === ERASED_NAME).length
  it('у варианта есть журнал аудита, и он только растёт', () => {
    f.forEach((x, i) => {
      expect(x.audit).not.toBeNull()
      if (i > 0) expect(x.audit!.length).toBeGreaterThanOrEqual(f[i - 1]!.audit!.length)
    })
  })
  it('мягкое удаление ничего не стёрло: имя в users и в журнале', () => {
    expect(f[1]!.users.find((u) => u.id === 1)!.deleted).toBe(true)
    expect(namesInUsers(f[1]!)).toBe(1)
    expect(namesInAudit(f[1]!)).toBeGreaterThan(0)
    expect(f[1]!.verdict).toBe('broken')
  })
  it('DELETE упирается во внешний ключ сообщений — 23503, строка на месте', () => {
    const i = f.findIndex((x) => x.sql?.startsWith('DELETE FROM users'))
    expect(f[i]!.sqlstate).toBe(FK_VIOLATION)
    expect(f[i]!.users).toEqual(f[i - 1]!.users)
    expect(f[i]!.refs.some((r) => r.table === 'messages' && r.userId === 1)).toBe(true)
  })
  it('обезличивание убирает имя из users и сессию, но журнал получает ещё одну строку с именем', () => {
    const i = f.findIndex((x) => x.sql?.includes("username = 'deleted_' || id"))
    expect(namesInUsers(f[i]!)).toBe(0)
    expect(f[i]!.refs.some((r) => r.table === 'sessions')).toBe(false)
    expect(f[i]!.refs.some((r) => r.table === 'messages' && r.userId === 1)).toBe(true)
    expect(namesInAudit(f[i]!)).toBe(namesInAudit(f[i - 1]!) + 1)
    expect(f[i]!.verdict).toBe('broken')
  })
  it('в финале имени нет нигде в базе, а строки журнала сохранились', () => {
    const last = f.at(-1)!
    expect(namesInUsers(last)).toBe(0)
    expect(namesInAudit(last)).toBe(0)
    expect(last.audit!.length).toBe(f.at(-2)!.audit!.length)
    expect(last.verdict).toBe('fixed')
  })
})

describe('все варианты', () => {
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  it.each(VARIANTS)('%s: подписи по-русски, длинные, термины — из глоссария, soft-delete назван', (v) => {
    const steps = softDeleteScenario(v)
    const ids = steps.flatMap((s) => captionTermIds(s.caption))
    for (const id of ids) expect(known.has(id)).toBe(true)
    for (const s of steps) expect(captionText(s.caption).trim().length).toBeGreaterThan(60)
    expect(ids.length).toBeGreaterThan(0)
  })
  it.each(VARIANTS)('%s: первый кадр без вердикта, последний — исправлено; ошибка всегда с SQLSTATE', (v) => {
    const f = frames(v)
    expect(f[0]!.verdict).toBe('pending')
    expect(f[0]!.sql).toBeNull()
    expect(f.at(-1)!.verdict).toBe('fixed')
    for (const x of f) expect(x.outcome === 'error').toBe(x.sqlstate !== null)
  })
  it.each(VARIANTS)('%s: отмеченные строки существуют в кадре', (v) => {
    for (const x of frames(v)) {
      for (const id of x.flagUsers) expect(x.users.some((u) => u.id === id)).toBe(true)
      for (const key of x.flagRefs) expect(x.refs.some((r) => r.key === key)).toBe(true)
      for (const id of x.flagAudit) expect((x.audit ?? []).some((a) => a.id === id)).toBe(true)
    }
  })
  it('все три термина урока есть в подписях', () => {
    const ids = VARIANTS.flatMap((v) => softDeleteScenario(v).flatMap((s) => captionTermIds(s.caption)))
    for (const id of ['soft-delete', 'partial-index', 'foreign-key', 'audit-log', 'right-to-erasure']) expect(ids).toContain(id)
  })
})
