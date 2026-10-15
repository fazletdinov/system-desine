import { readFileSync } from 'node:fs'
import { basename } from 'node:path'
import { createMarkdownRenderer, defineLoader, type SiteConfig } from 'vitepress'
import { parse } from 'yaml'
import { parseBank, type Bank, type Question } from './schema'

// Ответы и фоллоу-апы пишутся в Markdown — рендерим их при сборке, как глоссарий.
// Эталон — блочный Markdown; вопрос, пункты чеклиста, флаги и фоллоу-апы — строчный
// (`код`, **жирный**), иначе имена функций в них читались бы с обратными кавычками.
export interface RenderedFollowUp {
  qHtml: string
  aHtml: string
}
export interface RenderedQuestion extends Question {
  questionHtml: string
  answerHtml: string
  checklistHtml: readonly string[]
  redFlagsHtml: readonly string[]
  followUpsHtml: readonly RenderedFollowUp[]
}
export interface RenderedBank extends Omit<Bank, 'questions'> {
  questions: readonly RenderedQuestion[]
}

declare const data: Record<string, RenderedBank>
export { data }

export default defineLoader({
  watch: ['./*.yaml'],
  async load(files: string[]): Promise<Record<string, RenderedBank>> {
    const config = (globalThis as { VITEPRESS_CONFIG?: SiteConfig }).VITEPRESS_CONFIG
    if (config === undefined) throw new Error('VITEPRESS_CONFIG недоступен: loader запущен вне VitePress')
    const md = await createMarkdownRenderer(config.srcDir, config.markdown, config.site.base, config.logger)
    const inline = (s: string): string => md.renderInline(s)
    const out: Record<string, RenderedBank> = {}
    for (const file of files) {
      const bank = parseBank(parse(readFileSync(file, 'utf8')), basename(file))
      if (out[bank.module] !== undefined) throw new Error(`${basename(file)}: модуль ${bank.module} уже описан`)
      out[bank.module] = {
        ...bank,
        questions: bank.questions.map((q) => ({
          ...q,
          questionHtml: inline(q.question),
          answerHtml: md.render(q.answer),
          checklistHtml: q.checklist.map(inline),
          redFlagsHtml: q.red_flags.map(inline),
          followUpsHtml: q.follow_ups.map((f) => ({ qHtml: inline(f.q), aHtml: inline(f.a) })),
        })),
      }
    }
    return out
  },
})
