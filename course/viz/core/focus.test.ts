import { describe, expect, it } from 'vitest'
import { nextFocusIndex } from './focus'

describe('nextFocusIndex', () => {
  it.each([
    [0, 3, false, 1],
    [2, 3, false, 0],
    [0, 3, true, 2],
    [1, 3, true, 0],
    [-1, 3, false, 0],
    [-1, 3, true, 2],
  ])('current=%i count=%i back=%s → %i', (current, count, back, expected) => {
    expect(nextFocusIndex(current, count, back)).toBe(expected)
  })

  it('без элементов возвращает -1', () => {
    expect(nextFocusIndex(0, 0, false)).toBe(-1)
  })
})
