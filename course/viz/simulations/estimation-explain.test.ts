import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionText } from '../core/caption'
import { explanationTermIds, type Explanation } from '../core/explain'
import { formatBytes, formatNumber } from '../core/format'
import { INPUT_FIELDS, PRESETS, estimate, type EstimationInput } from './estimation'
import { ESTIMATION_SIMPLIFICATIONS, LAB_2A_MESSAGES, explainEstimation } from './estimation-explain'

const glossary = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
const allText = (e: Explanation): string =>
  [e.title, e.kind, ...e.sections.map((s) => captionText(s.body)), ...(e.here ? [captionText(e.here)] : []), ...(e.real ? [captionText(e.real)] : [])].join('\n')
const here = (id: string, input: EstimationInput = PRESETS.messenger): string => captionText(explainEstimation(id, input)!.here!)
const real = (id: string, input: EstimationInput = PRESETS.messenger): string => captionText(explainEstimation(id, input)!.real!)

const INPUT_IDS = INPUT_FIELDS.map((f) => `in:${f.key}`)
const STEP_IDS = estimate(PRESETS.default).map((s) => `step:${s.id}`)

describe('explainEstimation', () => {
  it('12 элементов: 6 входов и 6 результатов', () => {
    expect([...INPUT_IDS, ...STEP_IDS]).toHaveLength(12)
  })

  it.each(INPUT_IDS)('вход %s: что это, типичные значения, куда входит; «Здесь» и «В настоящей системе»', (id) => {
    const e = explainEstimation(id, PRESETS.default)!
    expect(e).not.toBeNull()
    expect(e.sections.map((s) => s.heading)).toEqual(['Что это', 'Типичные значения', 'В какие шаги входит'])
    expect(e.kind).toContain('допущение')
    expect(e.here).toBeDefined()
    expect(e.real).toBeDefined()
  })

  it.each(STEP_IDS)('результат %s: формула и ориентир для архитектуры; «Здесь» и «В настоящей системе»', (id) => {
    const e = explainEstimation(id, PRESETS.default)!
    expect(e).not.toBeNull()
    expect(e.sections.map((s) => s.heading)).toEqual(['Формула', 'Что это значит для архитектуры'])
    expect(e.kind).toContain('оценка')
    expect(e.here).toBeDefined()
    expect(e.real).toBeDefined()
  })

  it('все термины во всех объяснениях есть в глоссарии, разметка не просачивается', () => {
    const check = (e: Explanation | null): void => {
      expect(e).not.toBeNull()
      for (const t of explanationTermIds(e!)) expect(glossary.has(t), t).toBe(true)
      expect(allText(e!)).not.toMatch(/\[\[|\]\]|NaN|Infinity|undefined/)
    }
    check(ESTIMATION_SIMPLIFICATIONS)
    const zero: EstimationInput = { dau: 0, messagesPerUser: 0, messageBytes: 0, peakFactor: 0, retentionDays: 0, replicationFactor: 0 }
    for (const input of [PRESETS.default, PRESETS.messenger, zero]) for (const id of [...INPUT_IDS, ...STEP_IDS]) check(explainEstimation(id, input))
  })

  it('«Здесь» для средней нагрузки при пресете мессенджера — 46 296 RPS, как в сцене', () => {
    expect(here('step:avg-rps')).toContain(`${formatNumber(46_296)} RPS`)
    expect(here('step:avg-rps', PRESETS.default)).toContain(`${formatNumber(231)} RPS`)
  })

  it('«Здесь» пересчитывается от входов, а не берётся из пресета', () => {
    expect(here('step:messages-per-day', { ...PRESETS.messenger, dau: 1_000 })).toContain(formatNumber(40_000))
    expect(here('in:dau', { ...PRESETS.messenger, dau: 1_000 })).toContain(formatNumber(1_000))
  })

  it.each([
    ['step:peak-rps', PRESETS.default, 'один PostgreSQL'],
    ['step:peak-rps', PRESETS.messenger, 'шардирование'],
    ['step:avg-rps', PRESETS.messenger, 'на пределе одного сервера'],
    ['step:storage-total', PRESETS.default, 'помещается на один сервер'],
    ['step:storage-total', PRESETS.messenger, `${formatNumber(220)}–${formatNumber(440)} серверов`],
    ['step:peak-ingress', PRESETS.default, 'сеть не заметит'],
    ['step:peak-ingress', PRESETS.messenger, 'гигабитный порт'],
  ] as const)('ориентир в «Здесь»: %s → «%s»', (id, input, verdict) => {
    expect(here(id, input)).toContain(verdict)
  })

  it('одна копия между 10 и 20 ТБ — «на грани одного сервера», без противоречивого счёта серверов', () => {
    const h = here('step:storage-total', { ...PRESETS.default, retentionDays: 3_650, replicationFactor: 1 })
    expect(h).toContain(`одна копия — ${formatBytes(14.6e12)}`)
    expect(h).toContain('на грани одного сервера')
    expect(h).not.toContain('серверов только под хранение')
  })

  it('число и единица в текстах объяснений разделены неразрывным пробелом', () => {
    const plain = / (Б|КБ|МБ|ГБ|ТБ|ПБ|Мбит\/с|мс|дн\.|г\.)(?![а-яё])/
    const check = (t: string, id: string): void => expect(t.match(new RegExp(`\\d${plain.source}`)), id).toBeNull()
    for (const input of [PRESETS.default, PRESETS.messenger]) for (const id of [...INPUT_IDS, ...STEP_IDS]) check(allText(explainEstimation(id, input)!), id)
    check(allText(ESTIMATION_SIMPLIFICATIONS), 'упрощения')
  })

  it('хранилище: «без индексов» и замер лабы 2a — 135 Б таблицы и 169 Б индексов на сообщение', () => {
    for (const id of ['step:storage-per-day', 'step:storage-total', 'in:messageBytes']) {
      const r = real(id)
      expect(r, id).toContain('labs/2a/results/messenger-explain.txt')
      expect(r, id).toMatch(/135\u00A0Б таблицы и 169\u00A0Б трёх индексов на сообщение, вместе ≈ 303\u00A0Б/)
    }
    expect(captionText(explainEstimation('step:storage-total', PRESETS.messenger)!.sections[0]!.body)).toContain('без индексов')
  })

  it('числа лабы 2a совпадают с выводом §8 в файле результатов', () => {
    const out = readFileSync(fileURLToPath(new URL(`../../../${LAB_2A_MESSAGES.file}`, import.meta.url)), 'utf8')
    const section = out.slice(out.indexOf('=== 8.'))
    expect(section).toContain('на 10 000 сообщений чата 1 и 400 — чата 2')
    const bytes = (rel: string): number => Number(section.match(new RegExp(`^ ${rel}\\s+\\|\\s+(\\d+)`, 'm'))![1])
    expect(bytes('messages')).toBe(LAB_2A_MESSAGES.tableBytes)
    expect(bytes('messages_pkey') + bytes('messages_id_key') + bytes('messages_client_id_key')).toBe(LAB_2A_MESSAGES.indexBytes)
    expect(LAB_2A_MESSAGES.rows).toBe(10_400)
  })

  it('трафик: заголовки протоколов сверху и внутренний трафик реплик', () => {
    const r = real('step:peak-ingress')
    expect(r).toMatch(/120\u00A0Б/)
    expect(r).toMatch(/реплик/)
  })

  it('неизвестный id — null', () => {
    for (const id of ['foo', 'in:', 'in:zz', 'step:', 'step:zz', 'dau', 'row:ram']) expect(explainEstimation(id, PRESETS.default)).toBeNull()
  })
})

describe('ESTIMATION_SIMPLIFICATIONS', () => {
  it('только запись, равномерные сутки, логический размер, трафик без заголовков, реплики без трафика', () => {
    expect(ESTIMATION_SIMPLIFICATIONS.sections.map((s) => s.heading)).toEqual([
      'Только запись сообщений',
      'Сутки — 86 400 секунд и один коэффициент пика',
      'Логический размер, без индексов',
      'Трафик без заголовков',
      'Реплики не добавляют нагрузки',
    ])
  })
})
