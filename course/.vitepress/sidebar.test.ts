import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import { buildSidebar, readTitle } from './sidebar'

describe('readTitle', () => {
  it.each([
    ['frontmatter с title', '---\ntitle: Урок 1\n---\n# x', 'Урок 1'],
    ['frontmatter без title', '---\nlayout: doc\n---\n# x', 'fallback'],
    ['без frontmatter', '# Заголовок', 'fallback'],
    ['CRLF-переводы строк', '---\r\ntitle: Win\r\n---\r\n', 'Win'],
    ['title не строка', '---\ntitle: 42\n---\n', 'fallback'],
  ])('%s', (_name, md, expected) => {
    expect(readTitle(md, 'fallback')).toBe(expected)
  })
})

function makeTree(): string {
  const root = mkdtempSync(join(tmpdir(), 'sidebar-'))
  const m1 = join(root, '01-go')
  const m0 = join(root, '00-foundation')
  mkdirSync(m1)
  mkdirSync(m0)
  writeFileSync(join(m0, 'index.md'), '---\ntitle: Модуль 0\n---\n')
  writeFileSync(join(m0, '02-b.md'), '---\ntitle: Урок B\n---\n')
  writeFileSync(join(m0, '01-a.md'), '---\ntitle: Урок A\n---\n')
  writeFileSync(join(m0, 'notes.md'), '# не урок\n')
  writeFileSync(join(m1, '01-x.md'), '# без title\n')
  return root
}

describe('buildSidebar', () => {
  it('сортирует модули и уроки по номеру, игнорирует файлы без номера', () => {
    const items = buildSidebar(makeTree())
    expect(items).toEqual([
      {
        text: 'Модуль 0',
        link: '/modules/00-foundation/',
        collapsed: true,
        items: [
          { text: 'Урок A', link: '/modules/00-foundation/01-a' },
          { text: 'Урок B', link: '/modules/00-foundation/02-b' },
        ],
      },
      {
        text: '01-go',
        link: '/modules/01-go/',
        collapsed: true,
        items: [{ text: '01-x.md', link: '/modules/01-go/01-x' }],
      },
    ])
  })

  it('сортирует модули по order из index.md, а не по имени папки', () => {
    const root = mkdtempSync(join(tmpdir(), 'sidebar-order-'))
    for (const [dir, order] of [['02a-b', 2.1], ['02.5-x', 2.5], ['02-a', 2]] as const) {
      mkdirSync(join(root, dir))
      writeFileSync(join(root, dir, 'index.md'), `---\ntitle: ${dir}\norder: ${order}\n---\n`)
    }
    expect(buildSidebar(root).map((m) => m.text)).toEqual(['02-a', '02a-b', '02.5-x'])
  })

  it('игнорирует папки, имя которых не начинается с двух цифр', () => {
    const root = makeTree()
    mkdirSync(join(root, 'drafts'))
    writeFileSync(join(root, 'drafts', '01-x.md'), '# черновик\n')
    expect(buildSidebar(root).map((m) => m.link)).toEqual(['/modules/00-foundation/', '/modules/01-go/'])
  })

  it('возвращает пустой список, если папки нет', () => {
    expect(buildSidebar('/nonexistent/path/xyz')).toEqual([])
  })
})
