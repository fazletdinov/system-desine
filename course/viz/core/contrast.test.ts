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

// Фоны, на которых стоит текст схем. Тёмная тема — --vp-c-bg и --vp-c-bg-soft из .vitepress/theme/custom.css.
// Светлая — значения VitePress по умолчанию (custom.css их не переопределяет): страница белая,
// bg-soft #f6f6f7 — фон плеера (.viz-player), на нём подписи рядов сцены и текст боковой колонки.
const themes = [
  { name: 'light', selector: ':root', pageBg: '#ffffff', softBg: '#f6f6f7' },
  { name: 'dark', selector: '.dark', pageBg: '#0b1220', softBg: '#0f172a' },
]

describe.each(themes)('палитра текста схем, тема $name', ({ selector, pageBg, softBg }) => {
  const all = tokens(selector)
  const t = (name: string): string => {
    const v = all[name]
    if (!v) throw new Error(`в ${selector} не задан ${name}`)
    return v
  }

  it.each(['--viz-text', '--viz-muted', '--viz-addr', '--viz-hit-text'])('%s читается на фоне узла, страницы и плеера (≥ 4,5:1)', (name) => {
    expect(contrastRatio(t(name), t('--viz-node-bg'))).toBeGreaterThanOrEqual(4.5)
    expect(contrastRatio(t(name), pageBg)).toBeGreaterThanOrEqual(4.5)
    expect(contrastRatio(t(name), softBg)).toBeGreaterThanOrEqual(4.5)
  })

  it('--viz-hit-text читается на заливке --viz-hit-bg (≥ 4,5:1)', () => {
    expect(contrastRatio(t('--viz-hit-text'), t('--viz-hit-bg'))).toBeGreaterThanOrEqual(4.5)
  })
})

// Шкала задержек: точка отсчёта выделена рамкой --viz-active-border, фон строки остаётся фоном песочницы (bg-soft).
// Поэтому текст в ней — те же --viz-text и --viz-muted на bg-soft; заливка --viz-active-line-bg давала 4,33:1.
describe.each(themes)('шкала задержек, тема $name', ({ selector, softBg }) => {
  const all = tokens(selector)

  it.each(['--viz-text', '--viz-muted'])('%s в строке точки отсчёта и под полосами (фон bg-soft) ≥ 4,5:1', (name) => {
    expect(contrastRatio(all[name]!, softBg)).toBeGreaterThanOrEqual(4.5)
  })

  // WCAG 1.4.11: рамка и кольцо фокуса — неконтрастный элемент, порог 3:1.
  it.each(['--viz-active-border', '--viz-text'])('%s — рамка точки отсчёта и кольцо фокуса на bg-soft ≥ 3:1', (name) => {
    expect(contrastRatio(all[name]!, softBg)).toBeGreaterThanOrEqual(3)
  })
})
