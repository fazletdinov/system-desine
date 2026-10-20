import { execFileSync } from 'node:child_process'
import { existsSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'
import { buildGoFile, extractGoSnippets, findUnmarkedGoBlocks } from './go-snippets.ts'

const ROOT = fileURLToPath(new URL('..', import.meta.url))
const MODULES = join(ROOT, 'modules')
const HARNESS = join(ROOT, 'scripts', 'go-harness')
// Общий go.mod с закреплёнными версиями внешних пакетов (pgx) и его go.sum: сборка
// воспроизводима, а модули скачиваются в кэш один раз на все фрагменты.
const HARNESS_GO_MOD = readFileSync(join(HARNESS, 'go.mod'), 'utf8')
const HARNESS_GO_SUM = readFileSync(join(HARNESS, 'go.sum'), 'utf8')

try {
  execFileSync('go', ['version'], { stdio: 'ignore' })
} catch {
  console.error('Go не найден в PATH: установите Go, чтобы проверять фрагменты.')
  process.exit(1)
}

const files = readdirSync(MODULES, { recursive: true, encoding: 'utf8' })
  .filter((f) => f.endsWith('.md'))
  .map((f) => join(MODULES, f))

const failures: string[] = []
let checked = 0

for (const path of files) {
  const md = readFileSync(path, 'utf8')
  const rel = relative(ROOT, path)
  for (const line of findUnmarkedGoBlocks(md)) failures.push(`${rel}:${line}: блок go без маркера <!-- go-check -->`)
  for (const snippet of extractGoSnippets(md, rel)) {
    checked++
    let template: string | null = null
    if (snippet.harness !== null) {
      const tmplPath = join(HARNESS, `${snippet.harness}.go.tmpl`)
      if (!existsSync(tmplPath)) {
        failures.push(`${rel}:${snippet.line}: нет обвязки ${relative(ROOT, tmplPath)}`)
        continue
      }
      template = readFileSync(tmplPath, 'utf8')
    }
    const dir = mkdtempSync(join(tmpdir(), 'go-snippet-'))
    try {
      writeFileSync(join(dir, 'go.mod'), HARNESS_GO_MOD)
      writeFileSync(join(dir, 'go.sum'), HARNESS_GO_SUM)
      writeFileSync(join(dir, 'main.go'), buildGoFile(snippet, template))
      const unformatted = execFileSync('gofmt', ['-l', 'main.go'], { cwd: dir, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim()
      if (unformatted !== '') failures.push(`${rel}:${snippet.line}: gofmt требует форматирования`)
      execFileSync('go', ['vet', './...'], { cwd: dir, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] })
    } catch (e) {
      const err = e as { stderr?: string; message: string }
      failures.push(`${rel}:${snippet.line}: ${err.stderr?.trim() || err.message}`)
    } finally {
      rmSync(dir, { recursive: true, force: true })
    }
  }
}

if (failures.length > 0) {
  console.error(failures.join('\n'))
  process.exit(1)
}
console.log(`Проверено Go-фрагментов: ${checked}, ошибок нет.`)
