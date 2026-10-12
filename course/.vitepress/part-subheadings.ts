import type { MarkdownRenderer } from 'vitepress'

const PART_TAG = /<Part\s+kind="([a-z]+)"/

// Раздел (##) с меткой получает класс sec-<kind>, подраздел (###) — sub-<kind> по метке своего раздела,
// чтобы иерархия была видна цветом. Метка у самого подраздела важнее: так выделяются места вроде «Практики».
export function partSubheadingsPlugin(md: MarkdownRenderer): void {
  md.core.ruler.push('part-subheadings', (state) => {
    let sectionKind: string | null = null
    for (let i = 0; i < state.tokens.length; i++) {
      const token = state.tokens[i]
      if (token?.type !== 'heading_open') continue
      const ownKind = PART_TAG.exec(state.tokens[i + 1]?.content ?? '')?.[1] ?? null
      if (token.tag === 'h2') {
        sectionKind = ownKind
        if (ownKind !== null) addClass(token, `sec-${ownKind}`)
      } else if (token.tag === 'h3') {
        const kind = ownKind ?? sectionKind
        if (kind !== null) addClass(token, `sub-${kind}`)
      }
    }
    return true
  })
}

type HeadingToken = { attrGet(name: string): string | null; attrJoin(name: string, value: string): void }

// Правило может пройти по токенам повторно — класс не дублируем.
function addClass(token: HeadingToken, cls: string): void {
  if (!(token.attrGet('class') ?? '').split(' ').includes(cls)) token.attrJoin('class', cls)
}
