import { describe, expect, it } from 'vitest'
import { buildGoFile, extractGoSnippets, findUnmarkedGoBlocks } from './go-snippets'

const md = [
  'Текст',
  '<!-- go-check -->',
  '',
  '```go',
  'package main',
  '',
  'func main() {}',
  '```',
  '',
  '<!-- go-check: harness=sql-pool -->',
  '```go',
  'db.SetMaxOpenConns(80)',
  '```',
  '',
  '```go',
  'x := 1',
  '```',
].join('\n')

describe('extractGoSnippets', () => {
  it('находит помеченные блоки, обвязку и строку начала', () => {
    expect(extractGoSnippets(md, 'a.md')).toEqual([
      { file: 'a.md', line: 4, harness: null, code: 'package main\n\nfunc main() {}' },
      { file: 'a.md', line: 11, harness: 'sql-pool', code: 'db.SetMaxOpenConns(80)' },
    ])
  })
})

describe('findUnmarkedGoBlocks', () => {
  it('возвращает строки блоков go без маркера', () => {
    expect(findUnmarkedGoBlocks(md)).toEqual([15])
  })
})

describe('buildGoFile', () => {
  const snip = (code: string, harness: string | null = null) => ({ file: 'a.md', line: 1, harness, code })

  it('полная программа остаётся как есть', () => {
    expect(buildGoFile(snip('package main\n\nfunc main() {}'), null)).toBe('package main\n\nfunc main() {}\n')
  })

  it('без package и без обвязки получает package snippet', () => {
    expect(buildGoFile(snip('func f() {}'), null)).toBe('package snippet\n\nfunc f() {}\n')
  })

  it('вставляет фрагмент на место // SNIPPET с отступом этой строки', () => {
    const tmpl = 'package snippet\n\nfunc f() {\n\t// SNIPPET\n}\n'
    expect(buildGoFile(snip('a := 1\n\nb := a', 'x'), tmpl)).toBe('package snippet\n\nfunc f() {\n\ta := 1\n\n\tb := a\n}\n')
  })

  it('обвязка без // SNIPPET — понятная ошибка с файлом и строкой', () => {
    expect(() => buildGoFile(snip('x', 'x'), 'package snippet\n')).toThrow(/a\.md:1.*SNIPPET/)
  })
})
