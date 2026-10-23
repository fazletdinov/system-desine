import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import {
  ESTIMATION_HONESTY,
  ESTIMATION_QUIZ,
  INPUT_FIELDS,
  NUMERIC_RANGE,
  PRESETS,
  dauToSlider,
  estimate,
  estimationContext,
  formatStepValue,
  sanitizeInput,
  sliderToDau,
  type EstimationInput,
} from './estimation'

const glossary = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))

const valueOf = (input: EstimationInput, id: string) => estimate(input).find((s) => s.id === id)!.value

describe('estimate: пресет мессенджера', () => {
  const m = PRESETS.messenger
  it.each([
    ['messages-per-day', 4e9],
    ['avg-rps', 4e9 / 86_400],
    ['peak-rps', (4e9 / 86_400) * 3],
    ['storage-per-day', 8e11],
    ['storage-total', 8e11 * 1825 * 3],
    ['peak-ingress', (4e9 / 86_400) * 3 * 200],
  ])('%s = %s', (id, expected) => {
    expect(valueOf(m, id)).toBeCloseTo(expected, 3)
  })

  it('шаги идут в фиксированном порядке', () => {
    expect(estimate(m).map((s) => s.id)).toEqual(['messages-per-day', 'avg-rps', 'peak-rps', 'storage-per-day', 'storage-total', 'peak-ingress'])
  })

  it('выражение показывает подставленные числа', () => {
    expect(estimate(m)[0]!.expression).toBe('100 000 000 × 40 = 4 000 000 000')
  })
})

describe('estimate: пресет по умолчанию (числа урока 0.5)', () => {
  it.each([
    ['messages-per-day', 2e7],
    ['avg-rps', 231.481],
    ['peak-rps', 462.963],
    ['storage-per-day', 4e9],
    ['storage-total', 4.38e12],
    ['peak-ingress', 92_592.593],
  ])('%s ≈ %s', (id, expected) => {
    expect(valueOf(PRESETS.default, id)).toBeCloseTo(expected, 2)
  })
})

describe('sanitizeInput', () => {
  it('отрицательные, NaN и бесконечность → 0; коэффициенты пика и репликации не меньше 1', () => {
    expect(
      sanitizeInput({ dau: -5, messagesPerUser: Number.NaN, messageBytes: Number.POSITIVE_INFINITY, peakFactor: 0.5, retentionDays: -1, replicationFactor: 0 }),
    ).toEqual({ dau: 0, messagesPerUser: 0, messageBytes: 0, peakFactor: 1, retentionDays: 0, replicationFactor: 1 })
  })

  it('огромные введённые значения ограничены и дают конечные результаты', () => {
    const steps = estimate({ ...PRESETS.messenger, messagesPerUser: 1e308 })
    for (const step of steps) {
      expect(Number.isFinite(step.value)).toBe(true)
      expect(formatStepValue(step)).not.toMatch(/NaN|Infinity/)
    }
    const huge = sanitizeInput({ dau: 1e308, messagesPerUser: 1e308, messageBytes: 1e308, peakFactor: 1e308, retentionDays: 1e308, replicationFactor: 1e308 })
    expect(Object.values(huge).every((v) => Number.isFinite(v) && v <= 1e10)).toBe(true)
    for (const step of estimate(huge)) expect(Number.isFinite(step.value)).toBe(true)
  })

  it('нулевая аудитория даёт нули, а не NaN', () => {
    for (const step of estimate({ ...PRESETS.default, dau: 0 })) {
      expect(step.value).toBe(0)
      expect(formatStepValue(step)).not.toMatch(/NaN|Infinity/)
    }
  })
})

describe('слайдер DAU (логарифмический)', () => {
  it.each([
    [3, 1_000],
    [6, 1_000_000],
    [9, 1_000_000_000],
  ])('позиция %s → %s', (pos, dau) => {
    expect(sliderToDau(pos)).toBe(dau)
    expect(dauToSlider(dau)).toBeCloseTo(pos)
  })

  it('округляет до двух значащих цифр', () => {
    expect(sliderToDau(7.5)).toBe(32_000_000)
  })

  it.each([0, -5])('dauToSlider зажимает %s к минимуму', (dau) => {
    expect(dauToSlider(dau)).toBe(3)
  })
})

describe('formatStepValue', () => {
  it.each([
    ['messages-per-day', '4 000 000 000 сообщ./день'],
    ['avg-rps', '46 296 RPS'],
    ['storage-total', '4,4 ПБ'],
    ['peak-ingress', '28 МБ/с'],
  ])('%s → %s', (id, expected) => {
    const step = estimate(PRESETS.messenger).find((s) => s.id === id)!
    expect(formatStepValue(step)).toBe(expected)
  })
})

describe('подписи калькулятора по мерке', () => {
  it('шесть входов в порядке расчёта, у числовых — пределы ползунка', () => {
    expect(INPUT_FIELDS.map((f) => f.key)).toEqual(['dau', 'messagesPerUser', 'messageBytes', 'peakFactor', 'retentionDays', 'replicationFactor'])
    expect(Object.keys(NUMERIC_RANGE).sort()).toEqual(['messageBytes', 'messagesPerUser', 'peakFactor', 'replicationFactor', 'retentionDays'])
  })

  it('термины подписей (M4): DAU, метаданные, пик, срок хранения, репликация, RPS, салфетка — и все в глоссарии', () => {
    const captions = [
      estimationContext('default'),
      estimationContext('messenger'),
      ESTIMATION_HONESTY,
      ESTIMATION_QUIZ.question,
      ESTIMATION_QUIZ.answer,
      ...INPUT_FIELDS.map((f) => f.label),
      ...estimate(PRESETS.default).map((s) => s.label),
    ]
    const ids = captions.flatMap(captionTermIds)
    for (const t of ids) expect(glossary.has(t), t).toBe(true)
    for (const t of ['dau', 'metadata', 'peak-factor', 'retention', 'replication-factor', 'rps', 'back-of-envelope']) expect(ids, t).toContain(t)
    for (const c of captions) expect(captionText(c)).not.toMatch(/\[\[|\]\]/)
  })

  it('строка контекста (M1): что считаем и что не входит; у пресета мессенджера — отсылка к уроку 0.6', () => {
    const base = captionText(estimationContext('default'))
    expect(base).toContain('INSERT')
    expect(base).toMatch(/Чтение, статусы «доставлено\/прочитано» и вложения не входят/)
    expect(base).not.toContain('Пресет')
    expect(captionText(estimationContext('messenger'))).toMatch(/^Пресет: требования урока 0\.6\./)
  })

  it('честность (M5): входы — допущения, результат — оценка', () => {
    expect(captionText(ESTIMATION_HONESTY)).toBe('Входные числа — допущения (уроки 0.5, 0.6), результат — оценка порядка величины.')
  })

  it('вопрос (M6) — про коэффициент репликации 1', () => {
    expect(captionText(ESTIMATION_QUIZ.question)).toContain('коэффициент репликации 1')
    expect(captionText(ESTIMATION_QUIZ.answer)).toMatch(/реплик/)
  })
})
