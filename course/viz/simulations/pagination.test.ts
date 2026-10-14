import { describe, expect, it } from 'vitest'
import { duplicates, paginationSimulation as sim, PAGE_SIZE, skipped, type PagingAction, type PagingMode } from './pagination'

const run = (mode: PagingMode, ...actions: PagingAction[]) => actions.reduce(sim.apply, sim.apply(sim.initial(), { type: 'mode', mode }))
const next: PagingAction = { type: 'next' }

describe('pagination', () => {
  it('без изменений оба режима дают одну и ту же ленту', () => {
    const a = run('offset', next, next, next)
    const b = run('cursor', next, next, next)
    expect(a.pages).toEqual(b.pages)
    expect(a.pages.flat()).toEqual([30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17, 16])
  })
  it('вставка между страницами: offset дублирует, курсор — нет', () => {
    const off = run('offset', next, { type: 'insert' }, next)
    expect(duplicates(off)).toEqual([26])
    const cur = run('cursor', next, { type: 'insert' }, next)
    expect(duplicates(cur)).toEqual([])
    expect(cur.pages[1]).toEqual([25, 24, 23, 22, 21])
  })
  it('удаление между страницами: offset пропускает, курсор — нет', () => {
    const off = run('offset', next, { type: 'delete' }, next)
    expect(skipped(off)).toEqual([25])
    const cur = run('cursor', next, { type: 'delete' }, next)
    expect(skipped(cur)).toEqual([])
  })
  it('новые сообщения не считаются пропущенными', () => {
    const s = run('cursor', next, { type: 'insert' }, { type: 'insert' }, next)
    expect(skipped(s)).toEqual([])
  })
  it('лента кончилась — пустая страница, курсор не двигается', () => {
    const actions = Array.from({ length: 30 / PAGE_SIZE + 1 }, () => next)
    const s = run('cursor', ...actions)
    expect(s.pages.at(-1)).toEqual([])
    expect(s.cursor).toBe(1)
  })
  it('смена режима и сброс начинают чтение заново', () => {
    const s = run('offset', next, { type: 'insert' }, { type: 'mode', mode: 'cursor' })
    expect(s.pages).toEqual([])
    expect(s.feed).toHaveLength(30)
    expect(sim.apply(s, { type: 'reset' }).mode).toBe('cursor')
  })
  it('у каждого шага есть подпись с термином', () => {
    const steps = sim.steps(run('offset', next, { type: 'insert' }, next))
    expect(steps).toHaveLength(1)
    expect(JSON.stringify(steps[0]?.caption)).toContain('page-drift')
  })
})
