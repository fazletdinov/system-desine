import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { PARTS } from './parts'

const css = readFileSync(new URL('./parts.css', import.meta.url), 'utf8')

describe('PARTS', () => {
  it('у каждой метки есть подпись', () => {
    for (const label of Object.values(PARTS)) expect(label.trim().length).toBeGreaterThan(0)
  })

  it.each(Object.keys(PARTS))('у «%s» есть цвета в обеих темах', (kind) => {
    // Переменные объявлены дважды: в :root (светлая тема) и в .dark.
    expect(css.match(new RegExp(`--part-${kind}-fg:`, 'g'))?.length).toBe(2)
    expect(css.match(new RegExp(`--part-${kind}-bg:`, 'g'))?.length).toBe(2)
  })
})
