import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionText } from '../core/caption'
import { explanationTermIds, type Explanation } from '../core/explain'
import { LATENCIES, compare, type LatencyMode } from './latency'
import { LATENCY_SIMPLIFICATIONS, explainLatency } from './latency-explain'

const byId = (id: string) => LATENCIES.find((l) => l.id === id)!
const glossary = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
const allText = (e: Explanation): string =>
  [e.title, e.kind, ...e.sections.map((s) => captionText(s.body)), ...(e.here ? [captionText(e.here)] : []), ...(e.real ? [captionText(e.real)] : [])].join('\n')
const MODES: LatencyMode[] = ['real', 'human']

describe('explainLatency', () => {
  it.each(LATENCIES.map((l) => l.id))('строка %s: разделы по мерке, «Здесь» и «В настоящей системе»', (id) => {
    const e = explainLatency(`row:${id}`, { mode: 'real', baselineId: 'ram' })!
    expect(e).not.toBeNull()
    expect(e.sections.map((s) => s.heading)).toEqual(['Что это', 'Почему столько', 'В мессенджере'])
    expect(e.here).toBeDefined()
    expect(e.real).toBeDefined()
  })

  it('все термины во всех объяснениях есть в глоссарии, разметка не просачивается', () => {
    const check = (e: Explanation | null): void => {
      expect(e).not.toBeNull()
      for (const t of explanationTermIds(e!)) expect(glossary.has(t), t).toBe(true)
      expect(allText(e!)).not.toMatch(/\[\[|\]\]/)
    }
    check(LATENCY_SIMPLIFICATIONS)
    for (const mode of MODES) for (const l of LATENCIES) for (const b of LATENCIES) check(explainLatency(`row:${l.id}`, { mode, baselineId: b.id }))
  })

  it('«Здесь» для RAM при точке отсчёта L1 — отношение из compare()', () => {
    const e = explainLatency('row:ram', { mode: 'real', baselineId: 'l1' })!
    expect(captionText(e.here!)).toContain(compare(byId('ram'), byId('l1')))
    expect(captionText(e.here!)).toContain('в 65 раз медленнее')
  })

  it('«Здесь» показывает значение в выбранном масштабе', () => {
    expect(captionText(explainLatency('row:ram', { mode: 'real', baselineId: 'ram' })!.here!)).toContain('100 нс')
    expect(captionText(explainLatency('row:ram', { mode: 'human', baselineId: 'ram' })!.here!)).toContain('1,7 мин')
  })

  it('«В мессенджере» повторяет подпись строки', () => {
    for (const l of LATENCIES) {
      const s = explainLatency(`row:${l.id}`, { mode: 'real', baselineId: 'ram' })!.sections.find((x) => x.heading === 'В мессенджере')!
      expect(captionText(s.body).toLowerCase()).toContain(l.messenger.toLowerCase())
    }
  })

  it('«В настоящей системе» называет число модели как экстраполяцию; оценки — словом «Оценка»', () => {
    for (const l of LATENCIES) {
      const real = captionText(explainLatency(`row:${l.id}`, { mode: 'real', baselineId: 'ram' })!.real!)
      expect(real, l.id).toMatch(/^Модель Scott: /)
      expect(real, l.id).toContain(l.range!.estimate ? 'Оценка: ' : 'Реально: ')
      if (l.range!.links) expect(real, l.id).toContain('ссылки — под шкалой')
    }
  })

  it('измеренные строки называют файл лабы, SSD — ещё и p99', () => {
    for (const l of LATENCIES.filter((x) => x.author !== undefined))
      expect(captionText(explainLatency(`row:${l.id}`, { mode: 'real', baselineId: 'ram' })!.real!)).toContain(l.author!.file)
    expect(captionText(explainLatency('row:ssd-random', { mode: 'real', baselineId: 'ram' })!.real!)).toMatch(/p99 86 мкс/)
    expect(captionText(explainLatency('row:ssd-1mb', { mode: 'real', baselineId: 'ram' })!.real!)).toMatch(/p99 791 мкс/)
  })

  it('RTT в ЦОД не выдаёт замер на одной машине за ЦОД', () => {
    const real = captionText(explainLatency('row:dc-rtt', { mode: 'real', baselineId: 'ram' })!.real!)
    expect(real).toContain('Дина')
    expect(real).toContain('labs/0-4/results/net-rtt.txt')
    expect(real).toMatch(/это не ЦОД/)
  })

  it('неизвестный id — null', () => {
    for (const id of ['foo', 'row:', 'row:zz', 'l1']) expect(explainLatency(id, { mode: 'real', baselineId: 'ram' })).toBeNull()
  })

  it('неизвестная точка отсчёта не роняет объяснение: берётся первая строка', () => {
    expect(captionText(explainLatency('row:ram', { mode: 'real', baselineId: 'zz' })!.here!)).toContain('в 65 раз медленнее')
  })
})

describe('LATENCY_SIMPLIFICATIONS', () => {
  it('экстраполяция, константы сети, очередь, одна машина', () => {
    expect(LATENCY_SIMPLIFICATIONS.sections.map((s) => s.heading)).toEqual(['Экстраполяция, а не замер', 'Сеть — константы', 'Очередь и параллельность', 'Одна машина автора'])
  })
})
