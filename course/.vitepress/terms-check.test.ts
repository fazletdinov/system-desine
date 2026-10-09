import { describe, expect, it } from 'vitest'
import { findTermIds, findUnknownTerms } from './terms-check'

describe('findTermIds', () => {
  it('находит id во всех вхождениях <T>', () => {
    const src = 'Про <T id="wal">WAL</T> и <T id="rps">RPS</T>, снова <T  id="wal">журнал</T>.'
    expect(findTermIds(src)).toEqual(['wal', 'rps', 'wal'])
  })

  it('не путает <T> с другими тегами', () => {
    expect(findTermIds('<Table id="x"></Table><Tip id="y" />')).toEqual([])
  })
})

describe('findUnknownTerms', () => {
  it('возвращает уникальные неизвестные id', () => {
    const src = '<T id="wal">a</T><T id="nope">b</T><T id="nope">c</T>'
    expect(findUnknownTerms(src, new Set(['wal']))).toEqual(['nope'])
  })

  it('пусто, если все известны', () => {
    expect(findUnknownTerms('<T id="wal">a</T>', new Set(['wal']))).toEqual([])
  })
})
