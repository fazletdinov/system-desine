import { describe, expect, it } from 'vitest'
import { explainIdFromEvent } from './selection'

// Окружения DOM в тестах нет: цель клика — ручной узел с closest, как у Element.
function target(id: string | null): Event {
  const el = { closest: (sel: string) => (sel === '[data-explain]' && id !== null ? { getAttribute: (n: string) => (n === 'data-explain' ? id : null) } : null) }
  return { target: el } as unknown as Event
}

describe('explainIdFromEvent', () => {
  it('клик по вложенному элементу даёт id ближайшего [data-explain]', () => {
    expect(explainIdFromEvent(target('row:ram'))).toBe('row:ram')
  })

  it('клик вне размеченных элементов даёт null', () => {
    expect(explainIdFromEvent(target(null))).toBeNull()
  })

  it('цель без closest (не элемент) даёт null', () => {
    expect(explainIdFromEvent({ target: {} } as unknown as Event)).toBeNull()
  })
})
