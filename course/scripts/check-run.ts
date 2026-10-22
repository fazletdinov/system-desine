import { execFileSync } from 'node:child_process'
import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs'
import { join, posix, relative } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { parse } from 'yaml'
import {
  applyIllustrative,
  commandBlocksWithoutRun,
  type ComposeMounts,
  type IllustrativeBlock,
  machinesWithoutRun,
  type PathRef,
  runRefs,
  scanRunBlocks,
  type UncoveredBlock,
} from './run-blocks.ts'

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
const ILLUSTRATIVE_BLOCKS: IllustrativeBlock[] = [
  {
    file: 'modules/01-go-service/17-service-in-container.md',
    command: 'docker run --rm --privileged --pid=host busybox:1.37@sha256:bdf5… \\',
    reason: 'конфиг dockerd внутри виртуальной машины Docker Desktop; на Docker Engine в Linux такого файла нет',
  },
  ...[
    'docker buildx imagetools inspect ghcr.io/fazletdinov/messenger:main',
    'gh attestation verify oci://ghcr.io/fazletdinov/messenger@sha256:eae29acc8584f54b96e4076f699d57b70a778bc8f1df5fd674b6ad5386a5ed97 --owner fazletdinov',
    'gh attestation verify oci://ghcr.io/fazletdinov/messenger@sha256:eae29acc… \\',
    'gh attestation verify oci://docker.io/library/golang@sha256:37cfcca63222ec499bec5e8ad7aad52dbf7529db36d5812b8eae2f0621aa92d6 --repo fazletdinov/system-desine',
  ].map((command) => ({
    file: 'modules/01-go-service/19-delivery.md',
    command,
    reason: 'шаг первой публикации: образ и аттестации появляются в вашем GHCR после release.yml, digest у каждой публикации свой',
  })),
]

const git = (...args: string[]): string | null => {
  try {
    return execFileSync('git', args, { cwd: REPO_DIR, encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] })
  } catch {
    return null
  }
}

// Ревизии в уроках — теги: неподвижны, в отличие от ветки, и одинаковы у читателя и в CI.
const revisions = new Map<string, string | null>()
function resolveRev(rev: string): string | null {
  if (!revisions.has(rev)) revisions.set(rev, git('rev-parse', '--verify', '-q', `${rev}^{commit}`) === null ? null : rev)
  return revisions.get(rev) ?? null
}

// Клон без тегов (git clone --no-tags, actions/checkout с малой глубиной) провалил бы
// каждый <Run> с тегом по отдельности; причина одна, и сказать её нужно один раз.
function exitIfNoTags(): void {
  if ((git('tag', '-l') ?? '').trim() !== '') return
  console.error('Блоки «Запустить самому»: в клоне нет тегов, а <Run tag="…"> проверяются в дереве тега. Загрузите их: git fetch --tags')
  process.exit(1)
}

// Путь в ревизии: тип объекта (tree/blob) и, для каталога, имена внутри.
function missingInRev(ref: PathRef, rev: string): string | null {
  const resolved = resolveRev(rev)
  if (resolved === null) return `нет ревизии ${rev}`
  const object = `${resolved}:${ref.path === '.' ? '' : ref.path}`
  const type = git('cat-file', '-t', object)?.trim()
  if (type === undefined) return `нет такого пути в ${rev}`
  if (ref.kind === 'file') return null
  if (type !== 'tree') return `это не каталог в ${rev}`
  if (ref.kind === 'package' && !(git('ls-tree', '--name-only', object) ?? '').split('\n').some((f) => f.endsWith('.go')))
    return `в каталоге нет .go-файлов в ${rev}`
  return null
}

function missingOnDisk(ref: PathRef): string | null {
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

// С тегом команда идёт в рабочем дереве тега: пути проверяются в нём, а не на диске.
function missing(ref: PathRef, tag: string | undefined): string | null {
  const rev = ref.rev ?? tag
  return rev === undefined ? missingOnDisk(ref) : missingInRev(ref, rev)
}

export function checkRunBlocks(): number {
  exitIfNoTags()
  const problems: string[] = []
  const uncovered: UncoveredBlock[] = []
  let total = 0
  for (const file of markdownFiles(MODULES_DIR)) {
    const where = relative(COURSE_DIR, file)
    const scan = scanRunBlocks(readFileSync(file, 'utf8'))
    for (const p of scan.problems) problems.push(`${where}:${p.line}: ${p.message}`)
    for (const line of machinesWithoutRun(scan)) problems.push(`${where}:${line}: у прогона нет <Run> перед абзацем «машина…»`)
    for (const block of commandBlocksWithoutRun(scan)) uncovered.push({ file: where, block })
    for (const run of scan.runs) {
      total++
      if (run.tag !== undefined && git('tag', '-l', run.tag)?.trim() !== run.tag) {
        problems.push(`${where}:${run.line}: tag="${run.tag}": такого тега нет`)
        continue
      }
      for (const ref of runRefs(run, repoMounts)) {
        const why = missing(ref, run.tag)
        if (why !== null) problems.push(`${where}:${run.line}: ${ref.via} → ${ref.path}: ${why}`)
      }
    }
  }
  const { problems: notCovered, stale } = applyIllustrative(uncovered, ILLUSTRATIVE_BLOCKS)
  for (const { file, block } of notCovered)
    problems.push(`${file}:${block.line}: у блока «$ ${block.command}» нет <Run> после него в его разделе (или причины в ILLUSTRATIVE_BLOCKS)`)
  for (const e of stale) problems.push(`${e.file}: в ILLUSTRATIVE_BLOCKS есть «${e.command}», а такого блока без <Run> нет — уберите запись`)
  if (problems.length > 0) {
    console.error(`Блоки «Запустить самому» с ошибками:\n${problems.join('\n')}`)
    process.exit(1)
  }
  return total
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  console.log(`Блоки «Запустить самому» — без ошибок (блоков: ${checkRunBlocks()}).`)
}
