import { existsSync, readdirSync, readFileSync } from 'node:fs'
import { join, relative, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { hasRegion, scanSnippetRefs } from './snippet-regions.ts'
import { scanShots, webpSize } from './shot-refs.ts'
import { markProblems } from '../components/shot.ts'

const COURSE_DIR = fileURLToPath(new URL('..', import.meta.url))
const MODULES_DIR = join(COURSE_DIR, 'modules')
const PUBLIC_DIR = join(COURSE_DIR, 'public')
const SHOTS_DIR = join(PUBLIC_DIR, 'shots')

function filesWith(dir: string, ext: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = join(dir, e.name)
    if (e.isDirectory()) return filesWith(p, ext)
    return e.name.endsWith(ext) ? [p] : []
  })
}

const markdownFiles = (dir: string): string[] => filesWith(dir, '.md')

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

// Путь снимка в <Shot> — строка в свойстве компонента: сборка VitePress его не
// проверяет, и опечатка дала бы пустую рамку. Заодно сверяем размеры с файлом и
// номера пометок, а лишние файлы в public/shots считаем ошибкой: их никто не видит.
export function checkShotRefs(): number {
  const problems: string[] = []
  const used = new Set<string>()
  for (const file of markdownFiles(MODULES_DIR)) {
    const scan = scanShots(readFileSync(file, 'utf8'))
    for (const bad of scan.unparseable) problems.push(`${relative(COURSE_DIR, file)}:${bad.line}: не разобрать <Shot> «${bad.text}»`)
    for (const ref of scan.refs) {
      const where = `${relative(COURSE_DIR, file)}:${ref.line} → ${ref.src}`
      const target = join(PUBLIC_DIR, ref.src)
      used.add(target)
      for (const p of markProblems(ref.marks)) problems.push(`${where}: ${p}`)
      if (!existsSync(target)) {
        problems.push(`${where}: нет файла`)
        continue
      }
      const size = webpSize(readFileSync(target))
      if (size === null) problems.push(`${where}: не WebP`)
      else if (size.width !== ref.width || size.height !== ref.height) problems.push(`${where}: в уроке ${ref.width}×${ref.height}, у файла ${size.width}×${size.height}`)
    }
  }
  const orphans = existsSync(SHOTS_DIR) ? filesWith(SHOTS_DIR, '.webp').filter((f) => !used.has(f)) : []
  for (const f of orphans) problems.push(`${relative(COURSE_DIR, f)}: снимок не используется ни в одном уроке`)
  if (problems.length > 0) {
    console.error(`Битые снимки:\n${problems.join('\n')}`)
    process.exit(1)
  }
  return used.size
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  console.log(`Ссылки <<< на код — без ошибок (ссылок: ${checkSnippetRefs()}).`)
  console.log(`Снимки <Shot> — без ошибок (снимков: ${checkShotRefs()}).`)
}
