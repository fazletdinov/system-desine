import { existsSync, readdirSync, readFileSync } from 'node:fs'
import { join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { hasRegion, scanSnippetRefs } from './snippet-regions.ts'

const COURSE_DIR = fileURLToPath(new URL('..', import.meta.url))
const MODULES_DIR = join(COURSE_DIR, 'modules')

function markdownFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = join(dir, e.name)
    if (e.isDirectory()) return markdownFiles(p)
    return e.name.endsWith('.md') ? [p] : []
  })
}

// Ссылки `<<< @/…` в уроках не проверяет сборка VitePress: пропавший регион
// молча даёт пустой блок кода. Поэтому проверяем их сами.
export function checkSnippetRefs(): number {
  const problems: string[] = []
  let total = 0
  for (const file of markdownFiles(MODULES_DIR)) {
    const scan = scanSnippetRefs(readFileSync(file, 'utf8'))
    for (const bad of scan.unparseable) problems.push(`${relative(COURSE_DIR, file)}:${bad.line}: unparseable ref «${bad.text}»`)
    for (const ref of scan.refs) {
      total++
      const target = resolve(COURSE_DIR, ref.path)
      const where = `${relative(COURSE_DIR, file)}:${ref.line} → ${ref.path}${ref.region === null ? '' : `#${ref.region}`}`
      if (!existsSync(target)) problems.push(`${where}: нет файла`)
      else if (ref.region !== null && !hasRegion(readFileSync(target, 'utf8'), ref.region)) problems.push(`${where}: нет региона`)
    }
  }
  if (problems.length > 0) {
    console.error(`Битые ссылки на код:\n${problems.join('\n')}`)
    process.exit(1)
  }
  return total
}

if (import.meta.url === `file://${process.argv[1]}`) {
  console.log(`Ссылки <<< на код — без ошибок (ссылок: ${checkSnippetRefs()}).`)
}
