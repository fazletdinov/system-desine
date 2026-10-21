import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import { createMarkdownRenderer } from 'vitepress'
import { sqlRegionsPlugin } from './sql-regions'

// Рендер через настоящий VitePress: обёртка завязана на его внутренности (token.src,
// порядок fence-обёрток, env.includes), и при обновлении VitePress сломаться должен этот тест,
// а не урок, в который молча вставится весь SQL-файл.
let dir = ''

beforeAll(() => {
  dir = mkdtempSync(join(tmpdir(), 'sql-regions-'))
  writeFileSync(
    join(dir, 'lab.sql'),
    ['DROP SCHEMA IF EXISTS lab CASCADE;', '-- #region flat', '    SELECT 1;', '    -- #region inner', '    SELECT 2;', '    -- #endregion inner', '-- #endregion flat', 'SELECT 3;', ''].join('\n'),
  )
  writeFileSync(join(dir, 'main.go'), ['package main', '', '// #region main', 'func main() {}', '// #endregion main', ''].join('\n'))
})

afterAll(() => rmSync(dir, { recursive: true, force: true }))

async function render(src: string): Promise<{ html: string; includes: string[] }> {
  const md = await createMarkdownRenderer(dir)
  sqlRegionsPlugin(md)
  const env = { path: join(dir, 'page.md'), includes: [] as string[] }
  return { html: md.render(src, env), includes: env.includes }
}

const text = (html: string): string => html.replace(/<[^>]+>/g, '')

describe('sqlRegionsPlugin', () => {
  it('вставляет SQL-регион «--» без меток и без строк вне региона', async () => {
    const { html, includes } = await render('<<< @/lab.sql#flat{sql}\n')
    const code = text(html)
    expect(code).toContain('SELECT 1;')
    expect(code).toContain('SELECT 2;')
    expect(code).not.toContain('#region')
    expect(code).not.toContain('#endregion')
    expect(code).not.toContain('DROP SCHEMA')
    expect(code).not.toContain('SELECT 3;')
    expect(includes).toContain(join(dir, 'lab.sql'))
  })

  it('несуществующий регион роняет рендер, а не вставляет весь файл', async () => {
    await expect(render('<<< @/lab.sql#missing{sql}\n')).rejects.toThrow(/нет региона/)
  })

  it('регион, который VitePress находит сам, отдаёт ему без изменений', async () => {
    const { html } = await render('<<< @/main.go#main{go}\n')
    const code = text(html)
    expect(code).toContain('func main() {}')
    expect(code).not.toContain('package main')
    expect(code).not.toContain('#region')
  })
})
