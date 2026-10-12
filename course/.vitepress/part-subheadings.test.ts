import { describe, expect, it } from 'vitest'
import { createMarkdownRenderer } from 'vitepress'
import { partSubheadingsPlugin } from './part-subheadings'

async function render(src: string): Promise<string> {
  const md = await createMarkdownRenderer(process.cwd())
  partSubheadingsPlugin(md)
  return md.render(src)
}

function h3Classes(html: string): string[] {
  return [...html.matchAll(/<h3([^>]*)>/g)].map((m) => /class="([^"]*)"/.exec(m[1] ?? '')?.[1] ?? '')
}

describe('partSubheadingsPlugin', () => {
  it('подраздел получает цвет метки своего раздела', async () => {
    const html = await render(
      [
        '## <Part kind="theory" /> Теория',
        '### Первый',
        '### Второй',
        '## <Part kind="mistakes" /> Ошибки',
        '### Третий',
      ].join('\n\n'),
    )
    expect(h3Classes(html)).toEqual(['sub-theory', 'sub-theory', 'sub-mistakes'])
  })

  it('собственная метка подраздела важнее метки раздела', async () => {
    const html = await render(['## <Part kind="theory" /> Теория', '### <Part kind="practice" /> Ритуал'].join('\n\n'))
    expect(h3Classes(html)).toEqual(['sub-practice'])
  })

  it('раздел без метки сбрасывает цвет', async () => {
    const html = await render(['## <Part kind="theory" /> Теория', '## Без метки', '### Подраздел'].join('\n\n'))
    expect(h3Classes(html)).toEqual([''])
  })

  it('заголовки внутри блоков кода не трогает', async () => {
    const html = await render(['## <Part kind="theory" /> Теория', '```markdown\n### В коде\n```'].join('\n\n'))
    expect(html).not.toContain('<h3')
  })
})
