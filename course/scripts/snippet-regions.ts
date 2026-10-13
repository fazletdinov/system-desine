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

const escape = (s: string): string => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

export function hasRegion(source: string, region: string): boolean {
  const name = escape(region)
  const open = new RegExp(`^\\s*(?://|#|<!--)\\s*#region\\s+${name}\\s*(?:-->)?\\s*$`, 'm')
  const close = new RegExp(`^\\s*(?://|#|<!--)\\s*#endregion(?:\\s+${name})?\\s*(?:-->)?\\s*$`, 'm')
  const at = source.search(open)
  return at !== -1 && close.test(source.slice(at))
}
