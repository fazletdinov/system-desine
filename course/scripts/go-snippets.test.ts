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
  '<!-- go-check: harness=pgx-pool -->',
  '```go',
  'config.MaxConns = 80',
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
      { file: 'a.md', line: 11, harness: 'pgx-pool', code: 'config.MaxConns = 80' },
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

describe('варианты ограждения', () => {
  it('```go{2} с маркером извлекается', () => {
    const m = '<!-- go-check -->\n```go{2}\nx := 1\n```'
    expect(extractGoSnippets(m, 'a.md')).toEqual([{ file: 'a.md', line: 2, harness: null, code: 'x := 1' }])
  })

  it('```go:line-numbers без маркера попадает в отчёт', () => {
    expect(findUnmarkedGoBlocks('```go:line-numbers\nx := 1\n```')).toEqual([1])
  })

  it('блок с отступом (в списке) без маркера попадает в отчёт', () => {
    expect(findUnmarkedGoBlocks('- пункт\n\n  ```go\n  x := 1\n  ```')).toEqual([3])
  })

  it('~~~go без маркера попадает в отчёт', () => {
    expect(findUnmarkedGoBlocks('~~~go\nx := 1\n~~~')).toEqual([1])
  })

  it('CRLF не ломает маркер и код', () => {
    const m = '<!-- go-check -->\r\n```go\r\nx := 1\r\n```\r\n'
    expect(extractGoSnippets(m, 'a.md')).toEqual([{ file: 'a.md', line: 2, harness: null, code: 'x := 1' }])
  })
})
