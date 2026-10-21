import { describe, expect, it } from 'vitest'
import { MERMAID_CLASSES, mermaidThemeVariables, withPaletteClasses } from './mermaid-palette'

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
