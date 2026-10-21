import { describe, expect, it } from 'vitest'
import { MIN_DIAGRAM_SCALE, diagramMinWidth } from './diagram-scale'

describe('diagramMinWidth', () => {
  it('ширина viewBox, умноженная на нижнюю границу масштаба', () => {
    expect(MIN_DIAGRAM_SCALE).toBe(0.6)
    expect(diagramMinWidth(2000)).toBe(1200)
  })

  it('округляет вниз: 1016 × 0,6 = 609,6 → 609, без прокрутки в 1 px у колонки 609 px', () => {
    expect(diagramMinWidth(1016)).toBe(609)
  })

  it('маленькой схеме даёт маленький минимум — max-width: 100 % её не растягивает', () => {
    expect(diagramMinWidth(200)).toBe(120)
  })

  it.each([0, -10, Number.NaN, Number.POSITIVE_INFINITY])('без осмысленной ширины (%s) — null', (w) => {
    expect(diagramMinWidth(w)).toBeNull()
  })
})
