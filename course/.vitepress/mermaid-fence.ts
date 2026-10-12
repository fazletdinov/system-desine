import type { MarkdownRenderer } from 'vitepress'

// Блок ```mermaid становится компонентом, который сам грузит mermaid в браузере.
// Код кодируется: в атрибуте не должно быть кавычек и переводов строк.
export function mermaidFencePlugin(md: MarkdownRenderer): void {
  const fallback = md.renderer.rules.fence
  if (fallback === undefined) throw new Error('mermaid-fence: у markdown-it нет правила fence')
  md.renderer.rules.fence = (tokens, idx, options, env, self) => {
    const token = tokens[idx]
    if (token !== undefined && token.info.trim() === 'mermaid') {
      return `<MermaidDiagram code="${encodeURIComponent(token.content)}" />`
    }
    return fallback(tokens, idx, options, env, self)
  }
}
