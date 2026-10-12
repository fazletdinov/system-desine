import { describe, expect, it } from 'vitest'
import { createMarkdownRenderer } from 'vitepress'
import { anchorLabelPlugin } from './anchor-label'

async function render(src: string, withPlugin = true): Promise<string> {
  const md = await createMarkdownRenderer(process.cwd())
  if (withPlugin) anchorLabelPlugin(md)
  return md.render(src)
}

const HEADING = '## <Part kind="theory" /> Участники `net`\n'

describe('anchorLabelPlugin', () => {
  it('без плагина подпись содержит разметку метки (регрессия VitePress)', async () => {
    expect(await render(HEADING, false)).toMatch(/aria-label="[^"]*&lt;Part|aria-label="[^"]*<Part/)
  })

  it('убирает разметку метки из aria-label, оставляя текст и код', async () => {
    const html = await render(HEADING)
    expect(html).toContain('aria-label="Ссылка на раздел «Участники net»"')
    expect(html).not.toMatch(/aria-label="[^"]*(Part|kind=)/)
  })

  it('не меняет слаг заголовка', async () => {
    expect(await render(HEADING)).toBe((await render(HEADING, false)).replace(/aria-label="[^"]*"/, 'aria-label="Ссылка на раздел «Участники net»"'))
  })
})
