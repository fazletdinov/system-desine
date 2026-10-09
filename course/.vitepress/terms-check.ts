import type { MarkdownRenderer } from 'vitepress'

const TERM_TAG = /<T\s+id="([^"]*)"/g

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
    const unknown = findUnknownTerms(state.src, known)
    if (unknown.length > 0) {
      const env = state.env as { relativePath?: string }
      throw new Error(`${env.relativePath ?? 'markdown'}: нет в glossary/terms.yaml: ${unknown.join(', ')}`)
    }
    return true
  })
}
