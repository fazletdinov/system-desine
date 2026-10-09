import { createMarkdownRenderer, defineLoader, type SiteConfig } from 'vitepress'
import { loadGlossaryFile } from './load'
import type { Term } from './schema'

export interface RenderedTerm extends Term {
  longHtml?: string
}

declare const data: Record<string, RenderedTerm>
export { data }

export default defineLoader({
  watch: ['./terms.yaml'],
  async load(watchedFiles: string[]): Promise<Record<string, RenderedTerm>> {
    const file = watchedFiles[0]
    if (file === undefined) throw new Error('glossary/terms.yaml не найден')
    const config = (globalThis as { VITEPRESS_CONFIG?: SiteConfig }).VITEPRESS_CONFIG
    if (config === undefined) throw new Error('VITEPRESS_CONFIG недоступен: loader запущен вне VitePress')
    const md = await createMarkdownRenderer(config.srcDir, config.markdown, config.site.base, config.logger)
    const result: Record<string, RenderedTerm> = {}
    for (const [id, term] of Object.entries(loadGlossaryFile(file))) {
      result[id] = term.long === undefined ? term : { ...term, longHtml: md.render(term.long) }
    }
    return result
  },
})
