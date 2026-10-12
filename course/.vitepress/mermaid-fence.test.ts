import { describe, expect, it } from 'vitest'
import { createMarkdownRenderer } from 'vitepress'
import { mermaidFencePlugin } from './mermaid-fence'

async function render(src: string): Promise<string> {
  const md = await createMarkdownRenderer(process.cwd())
  mermaidFencePlugin(md)
  return md.render(src)
}

describe('mermaidFencePlugin', () => {
  it('превращает блок mermaid в компонент с закодированным кодом', async () => {
    const html = await render('```mermaid\nflowchart LR\n  A --> B\n```\n')
    expect(html).toContain(`<MermaidDiagram code="${encodeURIComponent('flowchart LR\n  A --> B\n')}" />`)
  })

  it('не трогает другие блоки кода', async () => {
    const html = await render('```go\nfunc main() {}\n```\n')
    expect(html).not.toContain('MermaidDiagram')
    expect(html).toContain('main')
  })
})
