import { readFileSync } from 'node:fs'
import type { MarkdownRenderer } from 'vitepress'
import { extractRegion, vitepressFindsRegion } from '../scripts/snippet-regions'

// Вставки `<<< файл#регион` с метками «-- #region» (SQL). VitePress 1.6 таких меток не знает
// и молча вставляет весь файл, а блочный /* #region */ в SQL портит вывод psql (он отправляет
// комментарий вместе со следующим запросом). Поэтому такие регионы вырезаем сами — той же
// функцией extractRegion, что проверяет check:refs.
//
// Привязано к внутренностям VitePress 1.6.4 (snippetPlugin в dist/node/chunk-*.js): блочное
// правило «snippet» кладёт в fence-токен `token.src = [абсолютный путь, имя региона]`, а его
// обёртка над fence читает файл, если token.src задан. Обёртка ниже ставится в markdown.config,
// то есть после snippetPlugin, и вызывается первой. При обновлении VitePress проверьте, что
// token.src и env.includes устроены так же: тест сборки — урок 2.2 (только регионы в HTML).
type SnippetToken = { src?: [string, string]; content: string }

export function sqlRegionsPlugin(md: MarkdownRenderer): void {
  const fence = md.renderer.rules.fence
  if (fence === undefined) throw new Error('sql-regions: у markdown-it нет правила fence')
  md.renderer.rules.fence = (tokens, idx, options, env, self) => {
    const token = tokens[idx] as unknown as SnippetToken | undefined
    const [path, region] = token?.src ?? []
    if (token === undefined || path === undefined || !region) return fence(tokens, idx, options, env, self)
    const source = readFileSync(path, 'utf8')
    if (vitepressFindsRegion(source, region)) return fence(tokens, idx, options, env, self)
    const content = extractRegion(source, region)
    // Падаем на сборке: молча вставленный целиком файл в уроке не заметит никто.
    if (content === null) throw new Error(`${path}#${region}: нет региона`)
    const includes = (env as { includes?: string[] }).includes
    includes?.push(path) // файл-источник — зависимость страницы для HMR, как у VitePress
    token.content = content
    delete token.src
    return fence(tokens, idx, options, env, self)
  }
}
