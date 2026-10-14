import { execFileSync } from 'node:child_process'
import { existsSync, readdirSync, readFileSync } from 'node:fs'
import { homedir } from 'node:os'
import { join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { hasRegion, scanSnippetRefs } from './snippet-regions.ts'

const COURSE_DIR = fileURLToPath(new URL('..', import.meta.url))
const MESSENGER = fileURLToPath(new URL('../../messenger', import.meta.url))
const MODULES_DIR = join(COURSE_DIR, 'modules')
// golangci-lint ставится через `go install` и лежит в ~/go/bin, которого может не быть в PATH.
// Каталог идёт первым, чтобы старая версия из системного PATH не заслонила установленную.
const ENV = { ...process.env, PATH: `${join(homedir(), 'go', 'bin')}:${process.env.PATH ?? ''}` }

function run(cmd: string, args: string[], timeout = 60_000): string {
  try {
    return execFileSync(cmd, args, {
      cwd: MESSENGER,
      env: ENV,
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
      timeout,
      maxBuffer: 64 * 1024 * 1024,
    })
  } catch (e) {
    const err = e as { stderr?: string; stdout?: string; message: string }
    console.error(`${cmd} ${args.join(' ')}:\n${err.stdout ?? ''}${err.stderr || err.message}`)
    process.exit(1)
  }
}

function markdownFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = join(dir, e.name)
    if (e.isDirectory()) return markdownFiles(p)
    return e.name.endsWith('.md') ? [p] : []
  })
}

// Ссылки `<<< @/…` в уроках не проверяет сборка VitePress: пропавший регион
// молча даёт пустой блок кода. Поэтому проверяем их сами.
function checkSnippetRefs(): number {
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

// Сгенерированный код (каталоги gen/) коммитится. Если контракт api/openapi.yaml поменяли,
// а go generate не запустили, код и контракт разойдутся молча — ловим это здесь.
function generatedFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = join(dir, e.name)
    if (e.isDirectory()) return generatedFiles(p)
    return p.split('/').includes('gen') && e.name.endsWith('.go') ? [p] : []
  })
}

function checkGenerated(): number {
  const snapshot = (): Map<string, string> => new Map(generatedFiles(join(MESSENGER, 'internal')).map((f) => [f, readFileSync(f, 'utf8')]))
  const before = snapshot()
  run('go', ['generate', './...'], 300_000)
  const after = snapshot()
  const changed = [...new Set([...before.keys(), ...after.keys()])].filter((f) => before.get(f) !== after.get(f))
  if (changed.length > 0) {
    console.error(`Сгенерированный код устарел — go generate ./... изменил файлы (закоммитьте их):\n${changed.map((f) => relative(MESSENGER, f)).join('\n')}`)
    process.exit(1)
  }
  return after.size
}

run('go', ['version'])
const unformatted = run('gofmt', ['-l', '.']).trim()
if (unformatted !== '') {
  console.error(`gofmt требует форматирования:\n${unformatted}`)
  process.exit(1)
}
const generated = checkGenerated()
run('go', ['vet', './...'])
run('golangci-lint', ['run', './...'], 300_000)
run('go', ['test', '-race', './...'], 600_000)
const refs = checkSnippetRefs()
console.log(`Мессенджер: gofmt, go generate (файлов: ${generated}), go vet, golangci-lint, go test -race и регионы <<< — без ошибок (ссылок: ${refs}).`)
