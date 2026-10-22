import { createSSRApp, h } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { describe, expect, it } from 'vitest'
import Shot from './Shot.vue'
import { layoutMarks, markProblems, textSegments, type ShotMark } from './shot'

const MARKS: ShotMark[] = [
  { n: 2, x: 50, y: 50, text: 'Кружок без рамки' },
  { n: 1, x: 10, y: 20, w: 30, h: 10, text: 'Строка фильтра: `tcp.flags.syn == 1`' },
]

async function render(marks: ShotMark[] = MARKS): Promise<string> {
  const props = { src: '/shots/x.webp', alt: 'Окно Wireshark', width: 1000, height: 500, caption: 'Подпись', version: 'Wireshark 4.6.6', marks }
  return renderToString(createSSRApp({ render: () => h(Shot, props) }))
}

// Атрибуты элементов с data-mark в порядке документа: сначала пометки на снимке, потом пункты списка.
function marked(html: string, tag: string): string[] {
  return [...html.matchAll(new RegExp(`<${tag}\\b[^>]*data-mark="(\\d+)"[^>]*>`, 'g'))].map((m) => m[0])
}

describe('Shot', () => {
  it('рисует по пометке на каждый номер, по порядку', async () => {
    const html = await render()
    expect(marked(html, 'g').map((g) => g.match(/data-mark="(\d+)"/)?.[1])).toEqual(['1', '2'])
    expect(html.match(/<text[^>]*>(\d+)<\/text>/g)?.map((t) => t.replace(/<[^>]+>/g, ''))).toEqual(['1', '2'])
  })

  it('рамку — только у пометки с w и h', async () => {
    const html = await render()
    // Рамка рисуется дважды: ореол и сама линия.
    expect(html.match(/<rect class="shot-rect"/g)).toHaveLength(1)
    expect(html).toContain('x="97.75" y="97.75" width="304.5" height="54.5"')
  })

  it('номер пометки совпадает с номером пояснения, пометка ссылается на своё пояснение', async () => {
    const html = await render()
    const items = marked(html, 'li')
    expect(items.map((li) => li.match(/value="(\d+)"/)?.[1])).toEqual(['1', '2'])
    for (const g of marked(html, 'g')) {
      const n = g.match(/data-mark="(\d+)"/)?.[1]
      const target = g.match(/aria-describedby="([^"]+)"/)?.[1]
      const li = items.find((x) => x.includes(`id="${target}"`))
      expect(li, `пояснение для пометки ${n}`).toBeDefined()
      expect(li).toContain(`data-mark="${n}"`)
    }
  })

  it('код в пояснении — в <code>, версия программы — под снимком', async () => {
    const html = await render()
    expect(html).toMatch(/<code[^>]*>tcp\.flags\.syn == 1<\/code>/)
    expect(html).toContain('Wireshark 4.6.6')
  })

  it('ставит base сайта перед путём снимка', async () => {
    expect(await render()).toContain(`src="${import.meta.env.BASE_URL}shots/x.webp"`)
  })
})

describe('markProblems', () => {
  it('правильная разметка — без замечаний', () => expect(markProblems(MARKS)).toEqual([]))
  it.each([
    ['пропуск номера', [{ n: 1, x: 1, y: 1, text: 'а' }, { n: 3, x: 1, y: 1, text: 'б' }]],
    ['повтор номера', [{ n: 1, x: 1, y: 1, text: 'а' }, { n: 1, x: 2, y: 2, text: 'б' }]],
    ['кружок за краем', [{ n: 1, x: 101, y: 1, text: 'а' }]],
    ['рамка за краем', [{ n: 1, x: 80, y: 1, w: 30, h: 5, text: 'а' }]],
    ['рамка без высоты', [{ n: 1, x: 1, y: 1, w: 30, text: 'а' }]],
    ['пустое пояснение', [{ n: 1, x: 1, y: 1, text: ' ' }]],
    ['непарная кавычка', [{ n: 1, x: 1, y: 1, text: 'поле `seq' }]],
    ['сторона номера без рамки', [{ n: 1, x: 1, y: 1, at: 'below', text: 'а' }]],
    ['неизвестная сторона номера', [{ n: 1, x: 1, y: 1, w: 5, h: 5, at: 'top', text: 'а' }]],
  ] as [string, ShotMark[]][])('%s', (_, marks) => {
    expect(markProblems(marks)).toHaveLength(1)
  })
})

describe('layoutMarks', () => {
  it('переводит проценты в пиксели: рамка с запасом 0,15 радиуса, номер справа от неё по центру высоты', () => {
    // Ширина 1000: радиус 15, запас 2,25.
    expect(layoutMarks(MARKS, 1000, 500)).toEqual([
      { n: 1, cx: 420.25, cy: 125, rect: { x: 97.75, y: 97.75, w: 304.5, h: 54.5 } },
      { n: 2, cx: 500, cy: 250, rect: null },
    ])
  })
  it('у правого края номер встаёт слева от рамки', () => {
    const [m] = layoutMarks([{ n: 1, x: 90, y: 50, w: 9, h: 2, text: 'а' }], 1000, 500)
    expect(m?.cx).toBe(897.75 - 18)
  })
})

describe('layoutMarks: сторона номера задана явно', () => {
  // Рамка как у первой пометки MARKS: x 97,75…402,25, y 97,75…152,25; зазор до центра кружка — 18.
  it.each([
    ['right', 420.25, 125],
    ['left', 79.75, 125],
    ['above', 250, 79.75],
    ['below', 250, 170.25],
  ] as const)('%s', (at, cx, cy) => {
    const [m] = layoutMarks([{ n: 1, x: 10, y: 20, w: 30, h: 10, at, text: 'а' }], 1000, 500)
    expect([m?.cx, m?.cy]).toEqual([cx, cy])
  })
  it('явная сторона важнее автоматической: у правого края номер всё равно справа, но в пределах снимка', () => {
    const [m] = layoutMarks([{ n: 1, x: 90, y: 50, w: 9, h: 2, at: 'right', text: 'а' }], 1000, 500)
    expect(m?.cx).toBe(1000 - 15)
  })
})

describe('layoutMarks у края', () => {
  it('рамку обрезает по снимку, кружок сдвигает внутрь на радиус', () => {
    const [m] = layoutMarks([{ n: 1, x: 0, y: 99, w: 0.5, h: 1, text: 'а' }], 1000, 500)
    expect(m).toEqual({ n: 1, cx: 25.25, cy: 485, rect: { x: 0, y: 492.75, w: 7.25, h: 7.25 } })
  })
})

describe('textSegments', () => {
  it('делит текст на обычный и код', () => {
    expect(textSegments('поле `Seq=0` и `raw`')).toEqual([
      { text: 'поле ', code: false },
      { text: 'Seq=0', code: true },
      { text: ' и ', code: false },
      { text: 'raw', code: true },
    ])
  })
})
