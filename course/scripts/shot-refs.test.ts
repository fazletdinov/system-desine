import { describe, expect, it } from 'vitest'
import { scanShots, webpSize } from './shot-refs'

describe('scanShots', () => {
  it('разбирает многострочный тег: путь, размеры и пометки, номер строки с 1', () => {
    const md = [
      'текст',
      '',
      '<Shot',
      '  src="/shots/a.webp"',
      '  alt="Окно"',
      '  :width="1400" :height="800"',
      '  caption="Подпись" version="Wireshark 4.6.6"',
      "  :marks=\"[{ n: 1, x: 10, y: 5, w: 20, h: 3, text: 'Фильтр `tcp`' }]\"",
      '/>',
    ].join('\n')
    expect(scanShots(md)).toEqual({
      refs: [{ line: 3, src: '/shots/a.webp', width: 1400, height: 800, marks: [{ n: 1, x: 10, y: 5, w: 20, h: 3, text: 'Фильтр `tcp`' }] }],
      unparseable: [],
    })
  })

  it.each([
    ['без размеров', '<Shot src="/shots/a.webp" :marks="[]" />'],
    ['сломанное выражение', '<Shot src="/shots/a.webp" :width="1" :height="1" :marks="[{ n: 1," />'],
  ])('тег, который не разобрать, возвращает как unparseable: %s', (_, md) => {
    expect(scanShots(md).unparseable).toHaveLength(1)
  })
})

// Минимальные заголовки трёх видов WebP — ровно те байты, которые читает webpSize.
function webp(chunk: string, body: number[]): Buffer {
  const buf = Buffer.alloc(40)
  buf.write('RIFF', 0, 'ascii')
  buf.write('WEBP', 8, 'ascii')
  buf.write(chunk, 12, 'ascii')
  Buffer.from(body).copy(buf, 20)
  return buf
}

describe('webpSize', () => {
  it('VP8X: ширина и высота минус один, по три байта', () => {
    // 1399 = 0x000577, 799 = 0x00031f
    // Между заголовком VP8X и размерами — 4 байта флагов.
    expect(webpSize(webp('VP8X', [0, 0, 0, 0, 0x77, 0x05, 0x00, 0x1f, 0x03, 0x00]))).toEqual({ width: 1400, height: 800 })
  })
  it('VP8: 14 бит после сигнатуры кадра', () => {
    expect(webpSize(webp('VP8 ', [0, 0, 0, 0x9d, 0x01, 0x2a, 0x78, 0x05, 0x20, 0x03]))).toEqual({ width: 1400, height: 800 })
  })
  it('VP8L: 14 + 14 бит после байта сигнатуры', () => {
    const bits = 1399 | (799 << 14)
    expect(webpSize(webp('VP8L', [0x2f, bits & 0xff, (bits >> 8) & 0xff, (bits >> 16) & 0xff, (bits >> 24) & 0xff]))).toEqual({ width: 1400, height: 800 })
  })
  it('не WebP — null', () => {
    expect(webpSize(Buffer.from('\x89PNG\r\n\x1a\n'.padEnd(40, '\0'), 'binary'))).toBeNull()
  })
})
