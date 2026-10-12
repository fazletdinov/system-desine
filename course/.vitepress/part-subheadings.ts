import type { MarkdownRenderer } from 'vitepress'

const PART_TAG = /<Part\s+kind="([a-z]+)"/

// Подраздел (###) окрашивается в цвет метки своего раздела (##), чтобы была видна иерархия.
// Метка у самого подраздела важнее: так выделяются места вроде «Практики».
export function partSubheadingsPlugin(md: MarkdownRenderer): void {
  md.core.ruler.push('part-subheadings', (state) => {
    let sectionKind: string | null = null
    for (let i = 0; i < state.tokens.length; i++) {
      const token = state.tokens[i]
      if (token?.type !== 'heading_open') continue
      const ownKind = PART_TAG.exec(state.tokens[i + 1]?.content ?? '')?.[1] ?? null
      if (token.tag === 'h2') sectionKind = ownKind
      else if (token.tag === 'h3') {
        const kind = ownKind ?? sectionKind
        const cls = `sub-${kind}`
        // Правило может пройти по токенам повторно — класс не дублируем.
        if (kind !== null && !(token.attrGet('class') ?? '').split(' ').includes(cls)) token.attrJoin('class', cls)
      }
    }
    return true
  })
}
