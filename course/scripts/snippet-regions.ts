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
// Подмножество меток, которые понимает findRegion из snippet-плагина VitePress 1.6:
// Go/JS, CSS, HTML/Markdown и комментарии на «#». Регион, которого VitePress не узнает,
// он молча заменит всем файлом, поэтому для этих форматов проверка видит регионы так же,
// как он; остальные его форматы (#pragma, VB, bat) в курсе не используются.
const REGION_MARKERS: readonly RegExp[] = [
  /^\/\/ ?#?((?:end)?region) ([\w*-]+)$/, // Go, JS, TS
  /^\/\* ?#((?:end)?region) ([\w*-]+) ?\*\/$/, // CSS
  /^<!-- #?((?:end)?region) ([\w*-]+) -->$/, // HTML, Markdown
  /^# ?((?:end)?region) ([\w*-]+)$/, // YAML, shell, Python
]

function markerAt(line: string, re: RegExp, region: string, end: boolean): boolean {
  const m = re.exec(line.trim())
  return m !== null && m[2] === region && (end ? /^end ?region$/i : /^region$/i).test(m[1] ?? '')
}

export function hasRegion(source: string, region: string): boolean {
  const lines = source.split(/\r?\n/)
  for (let i = 0; i < lines.length; i++) {
    const re = REGION_MARKERS.find((r) => markerAt(lines[i] ?? '', r, region, false))
    if (re === undefined) continue
    return lines.slice(i + 1).some((l) => markerAt(l, re, region, true))
  }
  return false
}
