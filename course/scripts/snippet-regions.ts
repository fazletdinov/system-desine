// Разбор ссылок VitePress `<<< @/путь#регион{язык}`. `@/` — каталог course/,
// поэтому путь возвращается относительно него.
export interface SnippetRef {
  path: string
  region: string | null
  line: number
}

const REF = /^<<<\s+@\/(\S+?)(?:#([\w-]+))?(?:\{[^}]*\})?\s*$/

export function findSnippetRefs(markdown: string): SnippetRef[] {
  const refs: SnippetRef[] = []
  let inFence = false
  markdown.split('\n').forEach((raw, i) => {
    const line = raw.trim()
    if (line.startsWith('```')) {
      inFence = !inFence
      return
    }
    if (inFence) return
    const m = REF.exec(line)
    if (m !== null) refs.push({ path: m[1] ?? '', region: m[2] ?? null, line: i + 1 })
  })
  return refs
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
