import { describe, expect, it } from 'vitest'
import { MERMAID_CLASSES, classTextColors, mermaidThemeVariables, withPaletteClasses } from './mermaid-palette'

describe('withPaletteClasses', () => {
  it('добавляет classDef для всех классов сразу после строки flowchart', () => {
    const out = withPaletteClasses('flowchart LR\n  A --> B', 'dark').split('\n')
    expect(out[0]).toBe('flowchart LR')
    for (const name of MERMAID_CLASSES) expect(out.some((l) => l.trim().startsWith(`classDef ${name} `))).toBe(true)
    expect(out.at(-1)).toBe('  A --> B')
  })

  it('понимает graph и пустые строки в начале', () => {
    const out = withPaletteClasses('\n\ngraph TB\nA', 'light').split('\n')
    expect(out.findIndex((l) => l.startsWith('graph TB'))).toBeLessThan(out.findIndex((l) => l.includes('classDef client')))
  })

  it('добавляет classDef и в ER-диаграмму', () => {
    const out = withPaletteClasses('erDiagram\n  u ||--o{ m : ""', 'light').split('\n')
    expect(out[0]).toBe('erDiagram')
    for (const name of MERMAID_CLASSES) expect(out.some((l) => l.trim().startsWith(`classDef ${name} `))).toBe(true)
    expect(out.at(-1)).toBe('  u ||--o{ m : ""')
  })

  it('не трогает другие типы диаграмм', () => {
    const seq = 'sequenceDiagram\n  A->>B: hi'
    expect(withPaletteClasses(seq, 'dark')).toBe(seq)
  })

  it.each([
    ['dark', '#f87171'],
    ['light', '#dc2626'],
  ] as const)('%s: класс error обведён цветом --viz-error', (theme, stroke) => {
    const def = withPaletteClasses('flowchart LR', theme).split('\n').find((l) => l.trim().startsWith('classDef error '))
    expect(def).toContain(`stroke:${stroke}`)
    expect(def).not.toContain('stroke-dasharray')
  })

  it('в тёмной и светлой теме цвета разные', () => {
    expect(withPaletteClasses('flowchart LR', 'dark')).not.toBe(withPaletteClasses('flowchart LR', 'light'))
  })
})

describe('mermaidThemeVariables', () => {
  it.each(['dark', 'light'] as const)('%s: задаёт фон, текст, линии и шрифт', (theme) => {
    const v = mermaidThemeVariables(theme)
    for (const key of ['background', 'primaryColor', 'primaryTextColor', 'lineColor', 'fontFamily']) expect(v[key]).toBeTruthy()
  })
})

// Относительная яркость и контраст по WCAG 2.x.
function luminance(hex: string): number {
  const [r, g, b] = [1, 3, 5].map((i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255
    return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * r! + 0.7152 * g! + 0.0722 * b!
}
function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi! + 0.05) / (lo! + 0.05)
}

describe('строки ER-диаграммы', () => {
  it.each(['dark', 'light'] as const)('%s: текст классов и темы читается на нечётных и чётных строках (≥ 4,5:1)', (theme) => {
    const v = mermaidThemeVariables(theme)
    for (const row of [v.rowOdd!, v.rowEven!]) {
      for (const text of [...classTextColors(theme), v.primaryTextColor!, v.textColor!]) expect(contrast(text, row)).toBeGreaterThanOrEqual(4.5)
    }
  })
})
