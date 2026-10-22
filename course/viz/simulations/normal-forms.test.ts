import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { inconsistentAuthors, normalFormsSimulation as sim, type NormalFormsAction, type NormalFormsState } from './normal-forms'

const run = (...actions: NormalFormsAction[]): NormalFormsState => actions.reduce(sim.apply, sim.initial())
const ALICE = 100
const BOB = 101

describe('normal-forms: аномалия обновления', () => {
  it('переименование в одной большой таблице меняет по строке на каждое сообщение автора', () => {
    const s = run({ type: 'author', authorId: ALICE }, { type: 'rename' })
    expect(s.last!.flat.changed).toHaveLength(3)
    expect(s.last!.flat.anomaly).toBeNull()
    expect(s.flat.filter((r) => r.authorId === ALICE).every((r) => r.authorName === 'alice_v2')).toBe(true)
  })

  it('частичное переименование оставляет у автора два имени — аномалия обновления', () => {
    const s = run({ type: 'partial', partial: true }, { type: 'rename' })
    expect(s.last!.flat.changed).toHaveLength(1)
    expect(s.last!.flat.anomaly).toBe('update')
    expect(inconsistentAuthors(s.flat)).toEqual([ALICE])
    expect(captionText(sim.steps(s)[0]!.caption)).toContain('Аномалия обновления')
  })

  it('в 3НФ переименование — одна строка users, и противоречия нет даже при «частичной» правке', () => {
    const s = run({ type: 'partial', partial: true }, { type: 'schema', schema: 'nf3' }, { type: 'rename' })
    expect(s.last!.nf3.changed).toEqual([{ table: 'users', key: ALICE }])
    expect(s.last!.nf3.anomaly).toBeNull()
    expect(s.users.find((u) => u.id === ALICE)!.name).toBe('alice_v2')
  })

  it('полное переименование после частичного восстанавливает согласованность', () => {
    const s = run({ type: 'partial', partial: true }, { type: 'rename' }, { type: 'partial', partial: false }, { type: 'rename' })
    expect(inconsistentAuthors(s.flat)).toEqual([])
  })
})

describe('normal-forms: аномалия удаления', () => {
  it('удаление последнего сообщения теряет автора только в одной большой таблице', () => {
    const s = run({ type: 'author', authorId: BOB }, { type: 'delete-last' })
    expect(s.last!.flat.anomaly).toBe('delete')
    expect(s.flat.some((r) => r.authorId === BOB)).toBe(false)
    expect(s.missing[BOB]).toBe('deleted')
    expect(s.last!.nf3.anomaly).toBeNull()
    expect(s.users.some((u) => u.id === BOB)).toBe(true)
  })

  it('удаление не последнего сообщения факт не теряет', () => {
    const s = run({ type: 'delete-last' })
    expect(s.last!.flat.removed).toEqual([{ table: 'messages_flat', key: 4 }])
    expect(s.last!.flat.anomaly).toBeNull()
    expect(s.missing[ALICE]).toBeUndefined()
  })
})

describe('normal-forms: аномалия вставки', () => {
  it('пользователя без сообщения нельзя вставить только в одну большую таблицу', () => {
    const s = run({ type: 'register' })
    expect(s.last!.flat.rejected).toBe(true)
    expect(s.last!.flat.anomaly).toBe('insert')
    expect(s.missing[102]).toBe('never-inserted')
    expect(s.last!.nf3.rejected).toBe(false)
    expect(s.last!.nf3.changed).toEqual([{ table: 'users', key: 102 }])
    expect(s.authorId).toBe(102)
  })

  it('первое сообщение нового пользователя возвращает его в таблицу — с ещё одной копией имени', () => {
    const s = run({ type: 'register' }, { type: 'message' })
    expect(s.missing[102]).toBeUndefined()
    expect(s.flat.at(-1)).toMatchObject({ authorId: 102, authorName: 'carol', chatTitle: 'Бэкенд' })
    expect(s.last!.nf3.changed).toEqual([{ table: 'messages', key: 5 }])
  })
})

describe('normal-forms: подписи', () => {
  it('термины подписей есть в глоссарии', () => {
    const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
    const scenarios: NormalFormsAction[][] = [
      [],
      [{ type: 'schema', schema: 'nf3' }],
      [{ type: 'rename' }],
      [{ type: 'partial', partial: true }, { type: 'rename' }],
      [{ type: 'register' }],
      [{ type: 'author', authorId: BOB }, { type: 'delete-last' }, { type: 'rename' }],
    ]
    const ids = new Set<string>()
    for (const actions of scenarios) {
      for (const schema of ['flat', 'nf3'] as const) {
        const caption = sim.steps(run(...actions, { type: 'schema', schema }))[0]!.caption
        expect(captionText(caption).length).toBeGreaterThan(0)
        for (const id of captionTermIds(caption)) ids.add(id)
      }
    }
    expect(ids.size).toBeGreaterThan(0)
    for (const id of ids) expect(known.has(id), id).toBe(true)
  })
})
