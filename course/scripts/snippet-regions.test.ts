import { describe, expect, it } from 'vitest'
import { extractRegion, hasRegion, scanSnippetRefs, vitepressFindsRegion } from './snippet-regions'

const refs = (md: string) => scanSnippetRefs(md).refs

describe('scanSnippetRefs', () => {
  it('находит путь и регион, номер строки с 1', () => {
    const md = 'текст\n\n<<< @/../messenger/internal/chat/domain/chat.go#post{go}\n'
    expect(refs(md)).toEqual([{ path: '../messenger/internal/chat/domain/chat.go', region: 'post', line: 3 }])
  })
  it('ссылка без региона и без языка', () => {
    expect(refs('<<< @/../labs/0b/netem.sh')).toEqual([{ path: '../labs/0b/netem.sh', region: null, line: 1 }])
  })
  it('не трогает строки внутри блока кода', () => {
    expect(scanSnippetRefs('```md\n<<< @/../x.go#a\n<<< кривая\n```')).toEqual({ refs: [], unparseable: [] })
  })
  it('блок в ~~~ тоже блок кода', () => {
    expect(refs('~~~md\n<<< @/../x.go#a\n~~~\n<<< @/../y.go')).toEqual([{ path: '../y.go', region: null, line: 4 }])
  })
  it('``` внутри ```` не закрывает внешний блок', () => {
    const md = '````md\n```go\n```\n<<< @/../x.go#a\n````\n<<< @/../y.go'
    expect(refs(md)).toEqual([{ path: '../y.go', region: null, line: 6 }])
  })
  it('``` внутри ~~~ не закрывает внешний блок', () => {
    expect(refs('~~~\n```\n<<< @/../x.go\n~~~')).toEqual([])
  })
  it.each([
    ['<<<'],
    ['<<< ../x.go#a{go}'],
    ['<<<@/../x.go'],
    ['<<< @/../x.go#a{go} лишнее'],
  ])('строку с <<<, которую не разобрать, возвращает как unparseable: %j', (line) => {
    expect(scanSnippetRefs(`текст\n${line}\n`)).toEqual({ refs: [], unparseable: [{ text: line, line: 2 }] })
  })
})

describe('hasRegion', () => {
  const src = 'package x\n\n// #region post\nfunc A() {}\n// #endregion post\n'
  it('находит регион', () => expect(hasRegion(src, 'post')).toBe(true))
  it('не путает префикс', () => expect(hasRegion(src, 'pos')).toBe(false))
  it('требует закрывающую метку', () => expect(hasRegion('// #region a\n', 'a')).toBe(false))
  // Проверка должна видеть регион ровно так же, как VitePress: иначе урок покажет весь файл вместо фрагмента.
  it('закрывающая метка без имени не годится — VitePress её не узнаёт', () =>
    expect(hasRegion('// #region a\nx\n// #endregion\n', 'a')).toBe(false))
  it('YAML и shell: # #region не видит VitePress', () =>
    expect(hasRegion('# #region a\nx: 1\n# #endregion a\n', 'a')).toBe(false))
  it.each([
    ['#region a\nx: 1\n#endregion a\n'],
    ['  # region a\n  x: 1\n  # endregion a\n'],
    ['<!-- #region a -->\nx\n<!-- #endregion a -->\n'],
    ['//#region a\nx\n//#endregion a\n'],
  ])('находит регион в формате VitePress: %j', (src) => expect(hasRegion(src, 'a')).toBe(true))
})

describe('extractRegion', () => {
  const sql = [
    '-- шапка',
    '-- #region flat',
    'CREATE TABLE t (',
    '    id bigint',
    ');',
    '-- #endregion flat',
    '',
    '    -- #region outer',
    '    SELECT 1;',
    '    -- #region inner',
    '    SELECT 2;',
    '    -- #endregion inner',
    '    -- #endregion outer',
  ].join('\n')

  it('находит SQL-регион «--» и не включает метки', () =>
    expect(extractRegion(sql, 'flat')).toBe('CREATE TABLE t (\n    id bigint\n);'))
  it('вырезает вложенные метки того же формата и снимает общий отступ, как VitePress', () =>
    expect(extractRegion(sql, 'outer')).toBe('SELECT 1;\nSELECT 2;'))
  it('метки другого формата внутри региона оставляет — VitePress тоже их не трогает', () =>
    expect(extractRegion('-- #region a\n// #region b\nx\n-- #endregion a\n', 'a')).toBe('// #region b\nx'))
  it('нет региона — null', () => expect(extractRegion(sql, 'missing')).toBeNull())
  it('нет закрывающей метки — null', () => expect(extractRegion('-- #region a\nx\n', 'a')).toBeNull())
  it('CRLF не попадает в текст', () => expect(extractRegion('-- #region a\r\nx\r\n-- #endregion a\r\n', 'a')).toBe('x'))
})

describe('vitepressFindsRegion', () => {
  it('Go-регион VitePress находит сам', () => expect(vitepressFindsRegion('// #region a\nx\n// #endregion a\n', 'a')).toBe(true))
  it('SQL-регион «--» VitePress не находит — его вырезаем мы', () =>
    expect(vitepressFindsRegion('-- #region a\nx\n-- #endregion a\n', 'a')).toBe(false))
})
