import { describe, expect, it } from 'vitest'
import { captionSegments, captionTermIds, captionText } from './caption'

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
