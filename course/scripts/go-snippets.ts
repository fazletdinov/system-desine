export interface GoSnippet {
  file: string
  line: number
  harness: string | null
  code: string
}

const MARKER = /^<!--\s*go-check(?::\s*harness=([a-z0-9-]+))?\s*-->\s*$/
const GO_FENCE = /^\s*(```|~~~)\s*go\b.*$/
const FENCE_END: Record<string, RegExp> = { '```': /^\s*```\s*$/, '~~~': /^\s*~~~\s*$/ }

interface Block {
  start: number
  code: string
  marker: { harness: string | null } | null
}

function goBlocks(markdown: string): Block[] {
  const lines = markdown.split(/\r?\n/)
  const blocks: Block[] = []
  for (let i = 0; i < lines.length; i++) {
    const open = GO_FENCE.exec(lines[i] ?? '')
    if (open === null) continue
    const closing = FENCE_END[open[1] ?? '```'] as RegExp
    let end = i + 1
    while (end < lines.length && !closing.test(lines[end] ?? '')) end++
    // Маркер — ближайшая непустая строка над блоком.
    let prev = i - 1
    while (prev >= 0 && (lines[prev] ?? '').trim() === '') prev--
    const m = prev >= 0 ? MARKER.exec((lines[prev] ?? '').trim()) : null
    blocks.push({ start: i + 1, code: lines.slice(i + 1, end).join('\n'), marker: m ? { harness: m[1] ?? null } : null })
    i = end
  }
  return blocks
}

export function extractGoSnippets(markdown: string, file: string): GoSnippet[] {
  return goBlocks(markdown).flatMap((b) => (b.marker === null ? [] : [{ file, line: b.start, harness: b.marker.harness, code: b.code }]))
}

export function findUnmarkedGoBlocks(markdown: string): number[] {
  return goBlocks(markdown).filter((b) => b.marker === null).map((b) => b.start)
}

const PLACEHOLDER = /^([ \t]*)\/\/ SNIPPET[ \t]*$/m

export function buildGoFile(snippet: GoSnippet, harnessTemplate: string | null): string {
  if (harnessTemplate === null) {
    const body = snippet.code.startsWith('package ') ? snippet.code : `package snippet\n\n${snippet.code}`
    return `${body}\n`
  }
  const m = PLACEHOLDER.exec(harnessTemplate)
  if (m === null) throw new Error(`${snippet.file}:${snippet.line}: в обвязке «${snippet.harness}» нет строки // SNIPPET`)
  const indent = m[1] ?? ''
  const indented = snippet.code.split('\n').map((l) => (l.trim() === '' ? '' : indent + l)).join('\n')
  // Функция-замена: в Go-коде может встретиться `$`, который строковая замена приняла бы за шаблон.
  return harnessTemplate.replace(PLACEHOLDER, () => indented)
}
