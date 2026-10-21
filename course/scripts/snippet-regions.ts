// Разбор ссылок VitePress `<<< @/путь#регион{язык}`. `@/` — каталог course/,
// поэтому путь возвращается относительно него.
export interface SnippetRef {
  path: string
  region: string | null
  line: number
}

// Строка вне блока кода, которая начинается с `<<<`, но не разбирается как ссылка.
// VitePress такую строку не вставит как код, а выведет текстом — урок молча сломается.
export interface UnparseableRef {
  text: string
  line: number
}

export interface SnippetScan {
  refs: SnippetRef[]
  unparseable: UnparseableRef[]
}

const REF = /^<<<\s+@\/(\S+?)(?:#([\w-]+))?(?:\{[^}]*\})?\s*$/
// Ограда блока кода по CommonMark: три и больше «`» или «~». Закрывает блок только
// ограда из того же символа и не короче открывающей — поэтому ``` внутри ~~~~ не в счёт.
const FENCE = /^(`{3,}|~{3,})/

export function scanSnippetRefs(markdown: string): SnippetScan {
  const scan: SnippetScan = { refs: [], unparseable: [] }
  let open: string | null = null
  markdown.split('\n').forEach((raw, i) => {
    const line = raw.trim()
    const fence = FENCE.exec(line)?.[1]
    if (fence !== undefined) {
      if (open === null) open = fence
      else if (fence[0] === open[0] && fence.length >= open.length && line === fence) open = null
      return
    }
    if (open !== null || !line.startsWith('<<<')) return
    const m = REF.exec(line)
    if (m === null) scan.unparseable.push({ text: line, line: i + 1 })
    else scan.refs.push({ path: m[1] ?? '', region: m[2] ?? null, line: i + 1 })
  })
  return scan
}
// Метки, которые понимает findRegion из snippet-плагина VitePress 1.6.4, — все семь, в его
// порядке: регион, который VitePress не узнает, он молча заменит всем файлом, поэтому
// проверка и fence-обёртка видят регионы ровно так же, как он.
const VITEPRESS_MARKERS: readonly RegExp[] = [
  /^\/\/ ?#?((?:end)?region) ([\w*-]+)$/, // Go, JS, TS
  /^\/\* ?#((?:end)?region) ([\w*-]+) ?\*\/$/, // CSS
  /^#pragma ((?:end)?region) ([\w*-]+)$/, // C, C++
  /^<!-- #?((?:end)?region) ([\w*-]+) -->$/, // HTML, Markdown
  /^#((?:End )Region) ([\w*-]+)$/, // Visual Basic
  /^::#((?:end)region) ([\w*-]+)$/, // bat
  /^# ?((?:end)?region) ([\w*-]+)$/, // YAML, shell, Python
]

// Метки «--» (SQL, Lua) VitePress не знает. Блочный /* */ в SQL не годится: psql отправляет
// его вместе со следующим запросом, и \set ECHO queries печатает метку в выводе лабы.
// Такие регионы вырезает fence-обёртка из .vitepress/sql-regions.ts той же функцией extractRegion.
const DASH_MARKER = /^-- ?#?((?:end)?region) ([\w*-]+)$/

const REGION_MARKERS: readonly RegExp[] = [...VITEPRESS_MARKERS, DASH_MARKER]

function markerAt(line: string, re: RegExp, region: string, end: boolean): boolean {
  const m = re.exec(line.trim())
  return m !== null && m[2] === region && (end ? /^end ?region$/i : /^region$/i).test(m[1] ?? '')
}

interface RegionBounds {
  start: number // первая строка после открывающей метки
  end: number // строка закрывающей метки
  marker: RegExp
}

// Тот же порядок, что у findRegion VitePress: первая открывающая метка любого формата,
// затем закрывающая метка того же формата и с тем же именем.
function findBounds(lines: readonly string[], region: string, markers: readonly RegExp[]): RegionBounds | null {
  for (let i = 0; i < lines.length; i++) {
    const marker = markers.find((r) => markerAt(lines[i] ?? '', r, region, false))
    if (marker === undefined) continue
    for (let j = i + 1; j < lines.length; j++) {
      if (markerAt(lines[j] ?? '', marker, region, true)) return { start: i + 1, end: j, marker }
    }
    return null
  }
  return null
}

// Копия dedent из VitePress 1.6: общий отступ снимается со всех строк, считая пустые
// (у пустой строки отступа нет, поэтому она не мешает).
function dedent(text: string): string {
  const lines = text.split('\n')
  const min = lines.reduce((acc, line) => {
    for (let i = 0; i < line.length; i++) if (line[i] !== ' ' && line[i] !== '\t') return Math.min(i, acc)
    return acc
  }, Infinity)
  return min < Infinity ? lines.map((l) => l.slice(min)).join('\n') : text
}

function split(source: string): string[] {
  return source.replace(/\r\n/g, '\n').split('\n')
}

// Регион понимает сам VitePress — тогда вставку оставляем ему.
export function vitepressFindsRegion(source: string, region: string): boolean {
  return findBounds(split(source), region, VITEPRESS_MARKERS) !== null
}

// Текст региона так, как его вставил бы VitePress: без строк-меток того же формата
// (вложенные регионы вырезаются, как у него) и с общим отступом, снятым dedent.
// null — региона нет.
export function extractRegion(source: string, region: string): string | null {
  const lines = split(source)
  const b = findBounds(lines, region, REGION_MARKERS)
  if (b === null) return null
  return dedent(
    lines
      .slice(b.start, b.end)
      .filter((l) => !b.marker.test(l.trim()))
      .join('\n'),
  )
}

export function hasRegion(source: string, region: string): boolean {
  return extractRegion(source, region) !== null
}
