import type { MarkdownRenderer } from 'vitepress'

const TERM_TAG = /<T\s+id="([^"]*)"/g

// Любой <T ...>, у которого нет литерального id в двойных кавычках, проверить нельзя.
const MALFORMED_TERM_TAG = /<T(?=[\s>/])(?!\s+id="[^"]*")[^>]*>?/g

export function findMalformedTerms(source: string): string[] {
  return source.match(MALFORMED_TERM_TAG) ?? []
}

export function findTermIds(source: string): string[] {
  return [...source.matchAll(TERM_TAG)].map((match) => match[1] ?? '')
}

export function findUnknownTerms(source: string, known: ReadonlySet<string>): string[] {
  return [...new Set(findTermIds(source).filter((id) => !known.has(id)))]
}

// Падаем на сборке, а не показываем пустую подсказку:
// опечатка в id термина должна быть видна сразу.
export function termsCheckPlugin(md: MarkdownRenderer, known: ReadonlySet<string>): void {
  md.core.ruler.push('check-glossary-terms', (state) => {
    const env = state.env as { relativePath?: string }
    const malformed = findMalformedTerms(state.src)
    if (malformed.length > 0) {
      throw new Error(`${env.relativePath ?? 'markdown'}: <T> без литерального id="...": ${malformed.join(', ')}`)
    }
    const unknown = findUnknownTerms(state.src, known)
    if (unknown.length > 0) {
      throw new Error(`${env.relativePath ?? 'markdown'}: нет в glossary/terms.yaml: ${unknown.join(', ')}`)
    }
    return true
  })
}
