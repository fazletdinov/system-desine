import { describe, expect, it } from 'vitest'
import { explainIdFromEvent, isExplainActivationKey, pickExplainId } from './selection'

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

describe('pickExplainId', () => {
  it('клик по размеченному элементу даёт его id', () => {
    expect(pickExplainId(target('row:ram'), null)).toBe('row:ram')
  })

  it('клик по другому элементу заменяет прежний выбор', () => {
    expect(pickExplainId(target('row:ssd'), 'row:ram')).toBe('row:ssd')
  })

  it('клик мимо размеченных элементов оставляет прежний выбор', () => {
    expect(pickExplainId(target(null), 'row:ram')).toBe('row:ram')
  })

  it('клик мимо без прежнего выбора оставляет null', () => {
    expect(pickExplainId(target(null), null)).toBeNull()
  })
})

// Слушатель висит на самой строке: currentTarget — она, target — где стоит фокус.
function key(k: string, onSelf: boolean): KeyboardEvent {
  const self = {}
  return { key: k, currentTarget: self, target: onSelf ? self : {} } as unknown as KeyboardEvent
}

describe('isExplainActivationKey', () => {
  it.each(['Enter', ' '])('%j на самой строке выбирает её', (k) => {
    expect(isExplainActivationKey(key(k, true))).toBe(true)
  })

  it.each(['Enter', ' '])('%j на вложенной ссылке-термине строку не выбирает', (k) => {
    expect(isExplainActivationKey(key(k, false))).toBe(false)
  })

  it.each(['a', 'Tab', 'ArrowDown', 'Spacebar'])('%j не выбирает строку', (k) => {
    expect(isExplainActivationKey(key(k, true))).toBe(false)
  })
})
