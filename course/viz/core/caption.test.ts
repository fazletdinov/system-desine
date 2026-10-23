import { describe, expect, it } from 'vitest'
import { captionSegments, captionTermIds, captionText, markedCaption, stripTerms } from './caption'

describe('caption', () => {
  it('строка — один сегмент без термина', () => {
    expect(captionSegments('Привет')).toEqual([{ text: 'Привет' }])
    expect(captionText('Привет')).toBe('Привет')
    expect(captionTermIds('Привет')).toEqual([])
  })

  it('сегменты склеиваются в текст, термины собираются по порядку', () => {
    const c = [{ text: 'Прошёл один ' }, { text: 'RTT', termId: 'rtt' }, { text: ', а потом ' }, { text: 'TLS', termId: 'tls' }]
    expect(captionText(c)).toBe('Прошёл один RTT, а потом TLS')
    expect(captionTermIds(c)).toEqual(['rtt', 'tls'])
  })
})

describe('markedCaption', () => {
  it('«[[id|текст]]» → термин; повтор id из seen — обычный текст', () => {
    const seen = new Set<string>()
    expect(markedCaption('Чтение с [[ssd|SSD]]', seen)).toEqual([{ text: 'Чтение с ' }, { text: 'SSD', termId: 'ssd' }])
    expect(markedCaption('Ещё раз [[ssd|SSD]]', seen)).toEqual([{ text: 'Ещё раз ' }, { text: 'SSD' }])
    expect(markedCaption('без терминов', seen)).toBe('без терминов')
  })

  it('битая разметка — ошибка автора', () => {
    expect(() => markedCaption('[[ssd SSD]]', new Set())).toThrow(/битая разметка/)
    expect(stripTerms('[[ram|память]] и [[hdd|HDD]]')).toBe('память и HDD')
  })
})
