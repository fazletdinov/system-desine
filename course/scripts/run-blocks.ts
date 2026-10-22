// Разбор блоков <Run dir cmd needs before /> в уроках и команд внутри них.
// Чистые функции: файловую систему трогает только check-run.ts.
import { posix } from 'node:path'

export interface RunTag {
  line: number
  dir: string
  cmd: string
  needs?: string
  before?: string
}

export interface RunScan {
  runs: RunTag[]
  // Строки абзацев «машина: …» / «машина та же…» — признак настоящего прогона.
  machines: number[]
  // Блоки кода, первая строка которых — «$ команда»: каждому нужен <Run> в его разделе.
  commandBlocks: CommandBlock[]
  headings: { line: number; level: number }[]
  problems: { line: number; message: string }[]
}

export interface CommandBlock {
  // Строка открывающей ограды.
  line: number
  // Первая строка блока без «$ ».
  command: string
}

const FENCE = /^(`{3,}|~{3,})/
const MACHINE = /^машина/
const RUN_START = /^<Run\b/
const HEADING = /^(#{1,6})\s/
const COMMAND_LINE = /^\$ (.*)$/
const RUN_TAG = /^<Run((?:\s+[a-z]+="[^"]*")*)\s*\/>$/
const ATTR = /([a-z]+)="([^"]*)"/g
const KNOWN_ATTRS = new Set(['dir', 'cmd', 'needs', 'before'])
const NEEDS = new Set(['docker'])

// Vue раскодирует сущности в атрибутах; разбираем так же, иначе проверили бы не ту команду.
function decodeEntities(s: string): string {
  return s.replace(/&(quot|amp|lt|gt|#39);/g, (_, e: string) => ({ quot: '"', amp: '&', lt: '<', gt: '>', '#39': "'" })[e] ?? '')
}

function parseTag(text: string, line: number): RunTag | string {
  const m = RUN_TAG.exec(text)
  if (m === null) return 'строку <Run> не разобрать: нужен один тег на строке, атрибуты в двойных кавычках, в конце />'
  const attrs: Record<string, string> = {}
  for (const a of (m[1] ?? '').matchAll(ATTR)) {
    const [, name = '', value = ''] = a
    if (!KNOWN_ATTRS.has(name)) return `неизвестный атрибут ${name}`
    attrs[name] = decodeEntities(value)
  }
  const { dir, cmd, needs, before } = attrs
  if (dir === undefined || dir === '') return 'нет dir'
  if (cmd === undefined || cmd === '') return 'нет cmd'
  if (dir.startsWith('/') || dir.split('/').includes('..')) return `dir «${dir}» должен идти от корня репозитория`
  if (needs !== undefined && !NEEDS.has(needs)) return `needs="${needs}": допустимо только docker`
  return { line, dir, cmd, ...(needs === undefined ? {} : { needs }), ...(before === undefined ? {} : { before }) }
}

export function scanRunBlocks(markdown: string): RunScan {
  const scan: RunScan = { runs: [], machines: [], commandBlocks: [], headings: [], problems: [] }
  let open: string | null = null
  let openedAt = -1
  markdown.split('\n').forEach((raw, i) => {
    const line = raw.trim()
    const fence = FENCE.exec(line)?.[1]
    if (fence !== undefined) {
      if (open === null) {
        open = fence
        openedAt = i
      } else if (fence[0] === open[0] && fence.length >= open.length && line === fence) open = null
      return
    }
    if (open !== null) {
      const command = i === openedAt + 1 ? COMMAND_LINE.exec(raw)?.[1] : undefined
      if (command !== undefined) scan.commandBlocks.push({ line: openedAt + 1, command })
      return
    }
    const heading = HEADING.exec(raw)?.[1]
    if (heading !== undefined) scan.headings.push({ line: i + 1, level: heading.length })
    if (MACHINE.test(line)) scan.machines.push(i + 1)
    if (!RUN_START.test(line)) return
    const tag = parseTag(line, i + 1)
    if (typeof tag === 'string') scan.problems.push({ line: i + 1, message: tag })
    else scan.runs.push(tag)
  })
  return scan
}

// Прогон — всё от предыдущего абзаца «машина…» до текущего: <Run> ставится внутри него.
export function machinesWithoutRun(scan: RunScan): number[] {
  let prev = 0
  return scan.machines.filter((m) => {
    const covered = scan.runs.some((r) => r.line > prev && r.line < m)
    prev = m
    return !covered
  })
}

// Раздел блока — от ближайшего заголовка над ним до следующего заголовка того же или
// более высокого уровня (подразделы входят). Сессия из нескольких блоков делит один <Run>.
export function commandBlocksWithoutRun(scan: RunScan): CommandBlock[] {
  return scan.commandBlocks.filter((block) => {
    const heading = [...scan.headings].reverse().find((h) => h.line < block.line)
    const start = heading?.line ?? 0
    const level = heading?.level ?? 0
    const end = scan.headings.find((h) => h.line > block.line && h.level <= level)?.line ?? Number.POSITIVE_INFINITY
    return !scan.runs.some((r) => r.line > start && r.line < end)
  })
}

// region shell
interface Word {
  text: string
  // $, обратные кавычки, шаблоны и <заглушки> — значение известно только оболочке.
  dynamic: boolean
}

type Token = Word | { op: string }

const isOp = (t: Token): t is { op: string } => 'op' in t
const OPERATORS = ['&&', '||', '2>&1', '>>', ';', '|', '&', '>', '<']

// Минимальный разбор sh: слова, кавычки и операторы. Подстановки не раскрываются,
// слово с ними помечается dynamic, и проверка его пропускает.
function tokenize(cmd: string): Token[] {
  const tokens: Token[] = []
  let word = ''
  let dynamic = false
  let inWord = false
  const flush = (): void => {
    if (inWord) tokens.push({ text: word, dynamic })
    word = ''
    dynamic = false
    inWord = false
  }
  for (let i = 0; i < cmd.length; i++) {
    const c = cmd[i] ?? ''
    if (c === "'") {
      const end = cmd.indexOf("'", i + 1)
      const stop = end === -1 ? cmd.length : end
      word += cmd.slice(i + 1, stop)
      inWord = true
      i = stop
      continue
    }
    if (c === '"') {
      let j = i + 1
      while (j < cmd.length && cmd[j] !== '"') {
        if (cmd[j] === '\\' && j + 1 < cmd.length) j++
        if (cmd[j] === '$' || cmd[j] === '`') dynamic = true
        word += cmd[j]
        j++
      }
      inWord = true
      i = j
      continue
    }
    if (c === '\\' && i + 1 < cmd.length) {
      word += cmd[i + 1]
      inWord = true
      i++
      continue
    }
    if (/\s/.test(c)) {
      flush()
      continue
    }
    const op = OPERATORS.find((o) => cmd.startsWith(o, i))
    if (op !== undefined && !(op === '2>&1' && inWord)) {
      flush()
      tokens.push({ op })
      i += op.length - 1
      continue
    }
    if ('$`*?{['.includes(c)) dynamic = true
    word += c
    inWord = true
  }
  flush()
  return tokens
}

interface SimpleCommand {
  words: Word[]
  outputs: Word[]
}

function splitCommands(tokens: Token[]): SimpleCommand[] {
  const commands: SimpleCommand[] = []
  let current: SimpleCommand = { words: [], outputs: [] }
  for (let i = 0; i < tokens.length; i++) {
    const t = tokens[i] as Token
    if (!isOp(t)) {
      current.words.push(t)
      continue
    }
    if (t.op === '>' || t.op === '>>' || t.op === '<') {
      const target = tokens[i + 1]
      if (target !== undefined && !isOp(target)) {
        if (t.op !== '<') current.outputs.push(target)
        i++
      }
      continue
    }
    if (t.op === '2>&1') continue
    commands.push(current)
    current = { words: [], outputs: [] }
  }
  commands.push(current)
  return commands.filter((c) => c.words.length > 0 || c.outputs.length > 0)
}
// endregion shell

export type RefKind = 'dir' | 'package' | 'file'

export interface PathRef {
  kind: RefKind
  // Путь от корня репозитория.
  path: string
  // Что в команде на него указало — для сообщения об ошибке.
  via: string
}

export interface ComposeMounts {
  // Точки монтирования compose-файла: путь в контейнере → каталог от корня репозитория.
  mounts(composeFile: string): Map<string, string>
}

const ENV_ASSIGN = /^[A-Za-z_][A-Za-z0-9_]*=/
const SHELL_KEYWORDS = new Set(['do', 'done', 'then', 'fi', 'time', 'exec'])
const GO_PACKAGE_COMMANDS = new Set(['run', 'test', 'build', 'vet', 'install', 'generate'])
const LOCAL_PATH = /^(\.\.?\/|scripts\/)/
const PRODUCED_FLAG = /^-(?:o|\w*profile)=(.+)$/

interface Context {
  dir: string
  compose: string | null
  produced: Set<string>
  refs: PathRef[]
  mounts: ComposeMounts
}

const inRepo = (ctx: Context, p: string): string => posix.normalize(posix.join(ctx.dir, p)).replace(/\/$/, '')

function goPackages(ctx: Context, args: Word[], sub: string): void {
  for (let i = 0; i < args.length; i++) {
    const w = args[i] as Word
    if (w.dynamic) continue
    if (w.text === '-o') {
      const out = args[i + 1]
      if (out !== undefined && !out.dynamic) ctx.produced.add(inRepo(ctx, out.text))
      i++
      continue
    }
    if (w.text !== '.' && !/^\.\.?\//.test(w.text)) continue
    const pattern = w.text.endsWith('/...')
    const path = inRepo(ctx, pattern ? w.text.slice(0, -4) : w.text)
    ctx.refs.push({ kind: path.endsWith('.go') ? 'file' : pattern ? 'dir' : 'package', path, via: `go ${sub} ${w.text}` })
    // У go run после пакета идут аргументы самой программы.
    if (sub === 'run') return
  }
}

function composeFileRef(ctx: Context, file: string, via: string): void {
  const path = inRepo(ctx, file)
  ctx.compose = path
  ctx.refs.push({ kind: 'file', path, via })
}

function containerPath(ctx: Context, file: string): string | null {
  if (ctx.compose === null) return null
  for (const [inside, host] of ctx.mounts.mounts(ctx.compose)) {
    const prefix = inside.endsWith('/') ? inside : `${inside}/`
    if (file.startsWith(prefix)) return posix.join(host, file.slice(prefix.length))
  }
  return null
}

function analyzeCommand(ctx: Context, c: SimpleCommand): void {
  for (const out of c.outputs) if (!out.dynamic) ctx.produced.add(inRepo(ctx, out.text))
  const words = [...c.words]
  while (words.length > 0 && (ENV_ASSIGN.test(words[0]?.text ?? '') || SHELL_KEYWORDS.has(words[0]?.text ?? ''))) words.shift()
  const [head, ...rest] = words
  if (head === undefined) return
  for (const w of rest) {
    const m = PRODUCED_FLAG.exec(w.text)
    if (m !== null && !w.dynamic) ctx.produced.add(inRepo(ctx, m[1] ?? ''))
  }
  if (head.text === 'go' && GO_PACKAGE_COMMANDS.has(rest[0]?.text ?? '')) {
    goPackages(ctx, rest.slice(1), rest[0]?.text ?? '')
    return
  }
  for (let i = 0; i < rest.length; i++) {
    const w = rest[i] as Word
    const next = rest[i + 1]
    if ((head.text === 'sh' || head.text === 'bash' || w.text === 'sh' || w.text === 'bash') && next?.text === '-c' && rest[i + 2] !== undefined) {
      // Скрипт для sh -c разбираем как команду: в нём psql -f /lab/… из compose-файла выше.
      for (const inner of splitCommands(tokenize(rest[i + 2]?.text ?? ''))) analyzeCommand(ctx, inner)
      i += 2
      continue
    }
    if (w.text === '-f' && next !== undefined && !next.dynamic) {
      const isCompose = head.text === 'docker' && rest[0]?.text === 'compose'
      if (isCompose) composeFileRef(ctx, next.text, `docker compose -f ${next.text}`)
      else if (head.text === 'psql' && next.text.startsWith('/')) {
        const mapped = containerPath(ctx, next.text)
        if (mapped !== null) ctx.refs.push({ kind: 'file', path: mapped, via: `psql -f ${next.text}` })
      } else if (head.text === 'psql') ctx.refs.push({ kind: 'file', path: inRepo(ctx, next.text), via: `psql -f ${next.text}` })
      i++
      continue
    }
    if (head.text === 'cp' && i === rest.length - 1 && !w.dynamic) ctx.produced.add(inRepo(ctx, w.text))
  }
  for (const w of [head, ...rest]) {
    if (w.dynamic || !LOCAL_PATH.test(w.text)) continue
    if (head.text === 'cp' && w === rest[rest.length - 1]) continue
    ctx.refs.push({ kind: 'file', path: inRepo(ctx, w.text), via: w.text })
  }
}

function analyze(ctx: Context, cmd: string): void {
  for (const c of splitCommands(tokenize(cmd))) analyzeCommand(ctx, c)
}

const BEFORE_CODE = /`([^`]+)`/g

// Путь «подготовлен», если его создаёт сама команда (-o, >, cp) или before велит его
// создать: программу из урока кладут в labs/<имя>/main.go, и проверке она недоступна.
// В before путь пишется от корня репозитория; готовым считается он сам и его каталог.
function preparedByBefore(run: RunTag): Set<string> {
  const paths = new Set<string>()
  for (const m of (run.before ?? '').matchAll(BEFORE_CODE)) {
    const text = m[1] ?? ''
    if (/\s/.test(text) || !text.includes('/')) continue
    const path = posix.normalize(text)
    paths.add(path)
    paths.add(posix.dirname(path))
  }
  return paths
}

function prepared(path: string, produced: Set<string>, byBefore: Set<string>): boolean {
  if (byBefore.has(path)) return true
  for (const p of produced) if (path === p || path.startsWith(`${p}/`)) return true
  return false
}

// Пути, которые блок обещает: каталог запуска, пакеты Go и файлы из команды и подготовки.
// Подготовленные (см. prepared) отброшены.
export function runRefs(run: RunTag, mounts: ComposeMounts): PathRef[] {
  const ctx: Context = { dir: run.dir, compose: null, produced: new Set(), refs: [], mounts }
  for (const m of (run.before ?? '').matchAll(BEFORE_CODE)) analyze(ctx, m[1] ?? '')
  ctx.compose = null
  analyze(ctx, run.cmd)
  const refs: PathRef[] = [{ kind: 'dir', path: run.dir, via: `dir="${run.dir}"` }, ...ctx.refs]
  const byBefore = preparedByBefore(run)
  const seen = new Set<string>()
  return refs.filter((r) => {
    const key = `${r.kind}:${r.path}`
    if (seen.has(key) || prepared(r.path, ctx.produced, byBefore)) return false
    seen.add(key)
    return true
  })
}
