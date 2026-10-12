import type { MarkdownRenderer } from 'vitepress'

const PERMALINK_CLASS = 'header-anchor'

// VitePress берёт для aria-label «сырой» текст заголовка, поэтому вместе с ним попадает
// разметка вроде `<Part kind="theory" />`. Пересчитываем подпись только по text/code_inline.
// Слаги не трогаем: правило работает после markdown-it-anchor и меняет только атрибут ссылки.
export function anchorLabelPlugin(md: MarkdownRenderer): void {
  md.core.ruler.push('course_anchor_label', (state) => {
    for (const token of state.tokens) {
      if (token.type !== 'inline' || token.children === null) continue
      const link = token.children.find((c) => c.type === 'link_open' && c.attrGet('class') === PERMALINK_CLASS)
      if (link === undefined) continue
      const title = token.children
        .filter((c) => c.type === 'text' || c.type === 'code_inline')
        .map((c) => c.content)
        .join('')
        .replace(/\s+/g, ' ')
        .trim()
      link.attrSet('aria-label', `Permalink to "${title}"`)
    }
  })
}
