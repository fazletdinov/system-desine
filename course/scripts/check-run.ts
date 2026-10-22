import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs'
import { join, posix, relative } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { parse } from 'yaml'
import { commandBlocksWithoutRun, type ComposeMounts, machinesWithoutRun, type PathRef, runRefs, scanRunBlocks } from './run-blocks.ts'

const COURSE_DIR = fileURLToPath(new URL('..', import.meta.url))
const REPO_DIR = join(COURSE_DIR, '..')
const MODULES_DIR = join(COURSE_DIR, 'modules')

function markdownFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = join(dir, e.name)
    if (e.isDirectory()) return markdownFiles(p)
    return e.name.endsWith('.md') ? [p] : []
  })
}

interface ComposeFile {
  services?: Record<string, { volumes?: unknown[] } | null>
}

// Короткая форма томов compose «./src:/dst[:ro]»: только привязки каталогов репозитория.
// Длинную форму и именованные тома проверка не разбирает — их пути не о файлах лабы.
const repoMounts: ComposeMounts = {
  mounts(composeFile) {
    const result = new Map<string, string>()
    const abs = join(REPO_DIR, composeFile)
    if (!existsSync(abs)) return result
    const doc = parse(readFileSync(abs, 'utf8')) as ComposeFile | null
    for (const service of Object.values(doc?.services ?? {})) {
      for (const v of service?.volumes ?? []) {
        if (typeof v !== 'string') continue
        const [src = '', dst = ''] = v.split(':')
        if (!src.startsWith('.') || !dst.startsWith('/')) continue
        result.set(dst, posix.normalize(posix.join(posix.dirname(composeFile), src)))
      }
    }
    return result
  },
}

// Блоки «$ …», которые из репозитория не воспроизвести, — без <Run>, каждый с причиной.
// Ключ — файл урока и первая строка блока без «$ »: номер строки сдвигается при любой правке выше.
const ILLUSTRATIVE_BLOCKS: { file: string; command: string; reason: string }[] = [
  {
    file: 'modules/01-go-service/17-service-in-container.md',
    command: 'docker run --rm --privileged --pid=host busybox:1.37@sha256:bdf5… \\',
    reason: 'конфиг dockerd внутри виртуальной машины Docker Desktop; на Docker Engine в Linux такого файла нет',
  },
]

const isIllustrative = (file: string, command: string): boolean =>
  ILLUSTRATIVE_BLOCKS.some((b) => b.file === file && b.command === command)

function missing(ref: PathRef): string | null {
  const abs = join(REPO_DIR, ref.path)
  if (!existsSync(abs)) return 'нет такого пути'
  const isDir = statSync(abs).isDirectory()
  if (ref.kind === 'dir' && !isDir) return 'это не каталог'
  if (ref.kind === 'package') {
    if (!isDir) return 'это не каталог пакета'
    if (!readdirSync(abs).some((f) => f.endsWith('.go'))) return 'в каталоге нет .go-файлов'
  }
  return null
}

export function checkRunBlocks(): number {
  const problems: string[] = []
  let total = 0
  for (const file of markdownFiles(MODULES_DIR)) {
    const where = relative(COURSE_DIR, file)
    const scan = scanRunBlocks(readFileSync(file, 'utf8'))
    for (const p of scan.problems) problems.push(`${where}:${p.line}: ${p.message}`)
    for (const line of machinesWithoutRun(scan)) problems.push(`${where}:${line}: у прогона нет <Run> перед абзацем «машина…»`)
    for (const block of commandBlocksWithoutRun(scan)) {
      if (isIllustrative(where, block.command)) continue
      problems.push(`${where}:${block.line}: у блока «$ ${block.command}» нет <Run> в его разделе (или причины в ILLUSTRATIVE_BLOCKS)`)
    }
    for (const run of scan.runs) {
      total++
      for (const ref of runRefs(run, repoMounts)) {
        const why = missing(ref)
        if (why !== null) problems.push(`${where}:${run.line}: ${ref.via} → ${ref.path}: ${why}`)
      }
    }
  }
  if (problems.length > 0) {
    console.error(`Блоки «Запустить самому» с ошибками:\n${problems.join('\n')}`)
    process.exit(1)
  }
  return total
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  console.log(`Блоки «Запустить самому» — без ошибок (блоков: ${checkRunBlocks()}).`)
}
