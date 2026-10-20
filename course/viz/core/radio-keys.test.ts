import { describe, expect, it } from 'vitest'
import { nextRadioIndex } from './radio-keys'

describe('nextRadioIndex', () => {
  it.each([
    [0, 3, 'ArrowRight', 1],
    [2, 3, 'ArrowRight', 0],
    [1, 3, 'ArrowDown', 2],
    [0, 3, 'ArrowLeft', 2],
    [2, 3, 'ArrowLeft', 1],
    [1, 3, 'ArrowUp', 0],
  ])('current=%i count=%i %s → %i', (current, count, key, expected) => {
    expect(nextRadioIndex(current, count, key)).toBe(expected)
  })

  it('не стрелка и пустая группа дают -1', () => {
    expect(nextRadioIndex(0, 3, 'Enter')).toBe(-1)
    expect(nextRadioIndex(0, 0, 'ArrowRight')).toBe(-1)
  })

  it.each(['altKey', 'ctrlKey', 'metaKey'] as const)('стрелка с %s не перехватывается', (modifier) => {
    expect(nextRadioIndex(0, 3, 'ArrowLeft', { [modifier]: true })).toBe(-1)
    expect(nextRadioIndex(0, 3, 'ArrowRight', { [modifier]: true })).toBe(-1)
  })
})
