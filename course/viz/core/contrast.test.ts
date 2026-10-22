import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { contrastRatio } from './contrast'

const palette = readFileSync(fileURLToPath(new URL('../palette.css', import.meta.url)), 'utf8')

// Блок берём по селектору целиком, чтобы тест читал ровно то, что видит браузер.
function tokens(selector: string): Record<string, string> {
  const block = palette.match(new RegExp(`(?:^|\\n)${selector.replace('.', '\\.')}\\s*\\{([^}]*)\\}`))
  if (!block) throw new Error(`в palette.css нет блока ${selector}`)
  return Object.fromEntries([...block[1]!.matchAll(/(--viz-[\w-]+):\s*(#[0-9a-fA-F]{6})\s*;/g)].map((m) => [m[1]!, m[2]!]))
}

describe('contrastRatio', () => {
  it('белое на чёрном — 21:1', () => {
    expect(contrastRatio('#ffffff', '#000000')).toBeCloseTo(21, 2)
  })

  it('#777 на белом — около 4,48:1 (порядок аргументов не важен)', () => {
    expect(contrastRatio('#777777', '#ffffff')).toBeCloseTo(4.48, 2)
    expect(contrastRatio('#ffffff', '#777777')).toBeCloseTo(4.48, 2)
  })
})

// Фон страницы: тёмная — --vp-c-bg из .vitepress/theme/custom.css;
// светлая — белый, значение VitePress по умолчанию (custom.css его не переопределяет).
const themes = [
  { name: 'light', selector: ':root', pageBg: '#ffffff' },
  { name: 'dark', selector: '.dark', pageBg: '#0b1220' },
]

describe.each(themes)('палитра текста схем, тема $name', ({ selector, pageBg }) => {
  const all = tokens(selector)
  const t = (name: string): string => {
    const v = all[name]
    if (!v) throw new Error(`в ${selector} не задан ${name}`)
    return v
  }

  it.each(['--viz-text', '--viz-muted', '--viz-addr', '--viz-hit-text'])('%s читается на фоне узла и страницы (≥ 4,5:1)', (name) => {
    expect(contrastRatio(t(name), t('--viz-node-bg'))).toBeGreaterThanOrEqual(4.5)
    expect(contrastRatio(t(name), pageBg)).toBeGreaterThanOrEqual(4.5)
  })

  it('--viz-hit-text читается на заливке --viz-hit-bg (≥ 4,5:1)', () => {
    expect(contrastRatio(t('--viz-hit-text'), t('--viz-hit-bg'))).toBeGreaterThanOrEqual(4.5)
  })
})
