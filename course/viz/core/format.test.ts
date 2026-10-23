import { describe, expect, it } from 'vitest'
import { formatBytes, formatMs, formatNs, formatNumber, formatSeconds, keepDigitGroups, plural } from './format'

const NBSP = '\u00A0'

describe('formatNumber', () => {
  it.each<[number, number | undefined, string]>([
    [0, undefined, '0'],
    [1234567, undefined, `1${NBSP}234${NBSP}567`],
    [46296.296, 1, `46${NBSP}296,3`],
    [0.5, 1, '0,5'],
  ])('%s (%s знаков) → %s', (value, digits, expected) => {
    expect(formatNumber(value, digits)).toBe(expected)
  })
})

describe('formatMs', () => {
  it.each([
    [0, `0${NBSP}мс`],
    [0.5, `0,5${NBSP}мс`],
    [2, `2${NBSP}мс`],
    [144, `144${NBSP}мс`],
  ])('%s → %s', (ms, expected) => expect(formatMs(ms)).toBe(expected))
})

describe('formatNs', () => {
  it.each([
    [1, `1${NBSP}нс`],
    [100, `100${NBSP}нс`],
    [3_000, `3${NBSP}мкс`],
    [16_000, `16${NBSP}мкс`],
    [500_000, `500${NBSP}мкс`],
    [2_000_000, `2${NBSP}мс`],
    [150_000_000, `150${NBSP}мс`],
    [2_500_000_000, `2,5${NBSP}с`],
  ])('%s → %s', (ns, expected) => expect(formatNs(ns)).toBe(expected))
})

describe('formatBytes', () => {
  it.each([
    [0, `0${NBSP}Б`],
    [999, `999${NBSP}Б`],
    [1_500, `1,5${NBSP}КБ`],
    [4e11, `400${NBSP}ГБ`],
    [4.38e15, `4,4${NBSP}ПБ`],
    [2.78e7, `28${NBSP}МБ`],
  ])('%s → %s', (bytes, expected) => expect(formatBytes(bytes)).toBe(expected))
})

describe('formatSeconds', () => {
  it.each([
    [1, `1${NBSP}с`],
    [100, `1,7${NBSP}мин`],
    [16_000, `4,4${NBSP}ч`],
    [2_000_000, `23${NBSP}дн.`],
    [150_000_000, `4,8${NBSP}г.`],
  ])('%s → %s', (s, expected) => expect(formatSeconds(s)).toBe(expected))
})

describe('plural', () => {
  it.each<[number, string]>([
    [0, 'листьев'],
    [1, 'лист'],
    [2, 'листа'],
    [4, 'листа'],
    [5, 'листьев'],
    [11, 'листьев'],
    [12, 'листьев'],
    [14, 'листьев'],
    [21, 'лист'],
    [22, 'листа'],
    [101, 'лист'],
    [111, 'листьев'],
    [112, 'листьев'],
  ])('%s → %s', (n, expected) => {
    expect(plural(n, 'лист', 'листа', 'листьев')).toBe(expected)
  })
})

// CaptionText применяет keepDigitGroups к каждому сегменту отдельно, поэтому число, разрезанное
// границей термина («1 [[x|000]]»), не склеивается: так авторские тексты не пишут, и склейка через
// сегменты не делается намеренно.
describe('keepDigitGroups', () => {
  it.each([
    ['Те же 16 384 строки', 'Те же 16\u00A0384 строки'],
    ['≈8 800 км и 200 000 км/с', '≈8\u00A0800 км и 200\u00A0000 км/с'],
    ['1 и 400 — чата 2', '1 и 400 — чата 2'],
    ['шаг 1 из 15', 'шаг 1 из 15'],
    ['число 12 3456', 'число 12 3456'],
    // Два года подряд — не разряды одного числа: группа после пробела из четырёх цифр не склеивается.
    ['2009 2014', '2009 2014'],
    ['сообщений в день ÷ 86 400 секунд', 'сообщений в день ÷ 86\u00A0400 секунд'],
  ])('«%s» → без переноса внутри числа', (src, expected) => {
    expect(keepDigitGroups(src)).toBe(expected)
  })
})
