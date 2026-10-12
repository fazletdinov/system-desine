import { defineLoader } from 'vitepress'
import { loadGlossaryFile } from './load'
import { toShortTerms, type ShortTerm } from './schema'

// Подсказкам терминов на каждой странице нужны только короткие определения.
// Полный глоссарий с HTML грузится лишь на странице /glossary.
declare const data: Record<string, ShortTerm>
export { data }

export default defineLoader({
  watch: ['./terms.yaml'],
  load(watchedFiles: string[]): Record<string, ShortTerm> {
    const file = watchedFiles[0]
    if (file === undefined) throw new Error('glossary/terms.yaml не найден')
    return toShortTerms(loadGlossaryFile(file))
  },
})
