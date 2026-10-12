import type { MarkdownRenderer } from 'vitepress'
import { PARTS } from '../components/parts'

const PART_TAG = /<Part\s+kind="([a-z]+)"/
// Ведущее слово заголовка — название части урока с двоеточием: «Проблема: …», «Компромисс: …».
const LEAD = new RegExp(`^(\\s*)((?:${Object.values(PARTS).join('|')}):)(\\s)`)

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
        if (ownKind !== null) {
          addClass(token, `sec-${ownKind}`)
          wrapLeadWord(state, state.tokens[i + 1])
        }
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

type InlineState = Parameters<Parameters<MarkdownRenderer['core']['ruler']['push']>[1]>[0]
type InlineToken = InlineState['tokens'][number]

// «Проблема: код работает…» → <span class="sec-lead">Проблема:</span> код работает…
// Якорь уже посчитан правилом anchor раньше, поэтому html-обёртка его не меняет.
function wrapLeadWord(state: InlineState, inline: InlineToken | undefined): void {
  const children = inline?.children
  if (!children) return
  const index = children.findIndex((child) => child.type === 'text' && LEAD.test(child.content))
  if (index === -1) return
  const text = children[index] as InlineToken
  const m = LEAD.exec(text.content)
  if (m === null) return
  const [whole, space, lead, after] = m
  const make = (type: string, content: string): InlineToken => Object.assign(new state.Token(type, '', 0), { content })
  children.splice(
    index,
    1,
    make('text', space ?? ''),
    make('html_inline', '<span class="sec-lead">'),
    make('text', lead ?? ''),
    make('html_inline', '</span>'),
    make('text', (after ?? '') + text.content.slice(whole.length)),
  )
}
