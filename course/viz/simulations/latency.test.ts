import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { LATENCIES, LATENCY_CONTEXT, LATENCY_NOTE, LATENCY_QUIZ, MIN_BAR, RANGE_LINKS, barWidth, compare, humanSeconds } from './latency'
import { LAB_04, type LabValue } from './latency-lab'

const byId = (id: string) => LATENCIES.find((l) => l.id === id)!
const repoRoot = new URL('../../../', import.meta.url)
const glossary = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))

// Формат строки результата лабы 0.4 (labs/0-4/README.md): «RESULT <ключ> <значение> <единица>».
const UNIT_NS: Record<string, number> = { ns: 1, us: 1e3, ms: 1e6 }
function resultNs(file: string, key: string): number {
  const text = readFileSync(fileURLToPath(new URL(file, repoRoot)), 'utf8')
  const m = new RegExp(`^RESULT ${key} ([0-9.]+) (\\w+)$`, 'm').exec(text)
  if (!m) throw new Error(`в ${file} нет строки RESULT ${key}`)
  const unit = UNIT_NS[m[2]!]
  if (unit === undefined) throw new Error(`неизвестная единица ${m[2]} у ${key}`)
  return Number(m[1]) * unit
}

describe('LATENCIES', () => {
  it('отсортированы по возрастанию, id уникальны', () => {
    const ns = LATENCIES.map((l) => l.typical.ns)
    expect(ns).toEqual([...ns].sort((a, b) => a - b))
    expect(new Set(LATENCIES.map((l) => l.id)).size).toBe(LATENCIES.length)
  })

  it('порядок строк прежний', () => {
    expect(LATENCIES.map((l) => l.id)).toEqual(['l1', 'l2', 'ram', 'ram-1mb', 'ssd-random', 'ssd-1mb', 'dc-rtt', 'hdd-1mb', 'hdd-seek', 'intercontinental'])
  })

  it('главное число — замер автора, где он есть; число модели Scott — отдельно и прежнее', () => {
    expect(LATENCIES.map((l) => l.model.ns)).toEqual([1, 4, 100, 3_000, 16_000, 49_000, 500_000, 825_000, 2_000_000, 150_000_000])
    for (const l of LATENCIES) {
      if (l.author) expect(l.typical, l.id).toEqual({ ns: l.author.ns, source: 'у автора, лаба 0.4' })
      else expect(l.typical, l.id).toEqual(l.model)
      expect(l.model.source, l.id).toMatch(/^модель Scott/)
    }
  })

  it('диапазон без ссылки на документ подписан как оценка; ссылки — https и без повторов в RANGE_LINKS', () => {
    for (const l of LATENCIES) {
      const r = l.range!
      if (!r.estimate) expect(r.links?.length ?? 0, l.id).toBeGreaterThan(0)
      if (r.estimate) expect(r.source, l.id).toMatch(/оценка|расчёт/)
      for (const k of r.links ?? []) expect(k.url).toMatch(/^https:\/\//)
    }
    expect(new Set(RANGE_LINKS.map((k) => k.url)).size).toBe(RANGE_LINKS.length)
    expect(RANGE_LINKS.length).toBe(6)
  })

  it('у каждой строки — источник типичного значения, реальный диапазон и место в мессенджере', () => {
    for (const l of LATENCIES) {
      expect(l.typical.source, l.id).not.toBe('')
      expect(l.messenger, l.id).not.toBe('')
      expect(l.range, l.id).toBeDefined()
      expect(l.range!.lo, l.id).toBeLessThan(l.range!.hi)
      expect(l.range!.source, l.id).not.toBe('')
    }
  })

  it('не мерили HDD, ЦОД и океан: у них нет числа автора и подпись «модель Scott / Дин 2009, экстраполяция»', () => {
    const unmeasured = LATENCIES.filter((l) => l.author === undefined).map((l) => l.id)
    expect(unmeasured).toEqual(['dc-rtt', 'hdd-1mb', 'hdd-seek', 'intercontinental'])
    for (const id of unmeasured) expect(byId(id).typical.source).toBe('модель Scott / Дин 2009, экстраполяция')
  })

  it('подписи, контекст, примечание и вопрос: термины есть в глоссарии', () => {
    const captions = [LATENCY_CONTEXT, LATENCY_NOTE, LATENCY_QUIZ.question, LATENCY_QUIZ.answer, ...LATENCIES.map((l) => l.label)]
    for (const c of captions) for (const t of captionTermIds(c)) expect(glossary.has(t), t).toBe(true)
    const ids = LATENCIES.flatMap((l) => captionTermIds(l.label))
    for (const t of ['cpu-cache', 'ram', 'ssd', 'hdd', 'rtt', 'data-center']) expect(ids).toContain(t)
    expect(captionTermIds(LATENCY_NOTE)).toContain('log-scale')
  })

  it('«Round trip» заменён на «Круговой путь (RTT)»', () => {
    const labels = LATENCIES.map((l) => captionText(l.label)).join('\n')
    expect(labels).not.toMatch(/round trip/i)
    expect(captionText(byId('dc-rtt').label)).toMatch(/^Круговой путь \(RTT\)/)
  })
})

describe('числа автора совпадают с результатами лабы 0.4', () => {
  it.each(LATENCIES.filter((l) => l.author !== undefined).map((l) => [l.id, l.author!] as const))('%s', (_id, author) => {
    expect(author.file).toMatch(/^labs\/0-4\/results\/[\w-]+\.txt$/)
    expect(author.ns).toBeCloseTo(resultNs(author.file, (author as LabValue).key), 6)
  })

  it.each(Object.entries(LAB_04))('константа %s', (_name, v) => {
    expect(v.ns).toBeCloseTo(resultNs(v.file, v.key), 6)
  })

  it('строки берут ключи по решению: lat для SSD, 256 КиБ для L2', () => {
    expect((byId('l2').author as LabValue).key).toBe('l2_random_ns')
    expect((byId('ssd-random').author as LabValue).key).toBe('ssd_rand4k_lat_p50_us')
    expect((byId('ssd-1mb').author as LabValue).key).toBe('ssd_seq1m_lat_p50_us')
  })
})

describe('barWidth', () => {
  it('минимум для самого быстрого, 1 для самого медленного, монотонно растёт', () => {
    const widths = LATENCIES.map((l) => barWidth(l.typical.ns))
    expect(widths[0]).toBe(MIN_BAR)
    expect(widths.at(-1)).toBe(1)
    for (let i = 1; i < widths.length; i++) expect(widths[i]!).toBeGreaterThan(widths[i - 1]!)
  })
})

describe('humanSeconds', () => {
  it('1 нс превращается в 1 с', () => {
    expect(humanSeconds(1)).toBe(1)
    expect(humanSeconds(150_000_000)).toBe(150_000_000)
  })
})

describe('compare', () => {
  it.each([
    // Замеры автора: SSD в ~640 раз медленнее RAM, а не в 160 (модель).
    ['ssd-random', 'ram', 'в 636 раз медленнее'],
    ['l1', 'ram', 'в 65 раз быстрее'],
    ['ram', 'ram', 'это и есть точка отсчёта'],
    // Обе сетевые строки — модель: отношение прежнее.
    ['intercontinental', 'dc-rtt', 'в 300 раз медленнее'],
    // «раза» после 2–4 и дробей, но «раз» после 12–14 и остальных
    ['hdd-seek', 'dc-rtt', 'в 4 раза медленнее'],
    ['hdd-1mb', 'dc-rtt', 'в 1,7 раза медленнее'],
    ['l2', 'ram', 'в 25 раз быстрее'],
    ['ram-1mb', 'hdd-seek', 'в 34 раза быстрее'],
    ['ssd-random', 'ssd-1mb', 'в 4,1 раза быстрее'],
    ['dc-rtt', 'ram', 'в 4\u00a0992 раза медленнее'],
  ])('%s относительно %s → %s', (a, b, expected) => {
    expect(compare(byId(a), byId(b))).toBe(expected)
  })
})
