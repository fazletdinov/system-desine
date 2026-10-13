import { describe, expect, it } from 'vitest'
import { findSnippetRefs, hasRegion } from './snippet-regions'

describe('findSnippetRefs', () => {
  it('находит путь и регион, номер строки с 1', () => {
    const md = 'текст\n\n<<< @/../messenger/internal/chat/domain/chat.go#post{go}\n'
    expect(findSnippetRefs(md)).toEqual([{ path: '../messenger/internal/chat/domain/chat.go', region: 'post', line: 3 }])
  })
  it('ссылка без региона и без языка', () => {
    expect(findSnippetRefs('<<< @/../labs/0b/netem.sh')).toEqual([{ path: '../labs/0b/netem.sh', region: null, line: 1 }])
  })
  it('не трогает строки внутри блока кода', () => {
    expect(findSnippetRefs('```md\n<<< @/../x.go#a\n```')).toEqual([])
  })
})

describe('hasRegion', () => {
  const src = 'package x\n\n// #region post\nfunc A() {}\n// #endregion post\n'
  it('находит регион', () => expect(hasRegion(src, 'post')).toBe(true))
  it('не путает префикс', () => expect(hasRegion(src, 'pos')).toBe(false))
  it('требует закрывающую метку', () => expect(hasRegion('// #region a\n', 'a')).toBe(false))
})
