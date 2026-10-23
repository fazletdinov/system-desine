import { markedCaption, type Caption } from '../core/caption'
import { formatBytes, formatNumber } from '../core/format'
import type { StepQuiz } from '../core/types'

export interface EstimationInput {
  dau: number
  messagesPerUser: number
  messageBytes: number
  peakFactor: number
  retentionDays: number
  replicationFactor: number
}

export type StepId = 'messages-per-day' | 'avg-rps' | 'peak-rps' | 'storage-per-day' | 'storage-total' | 'peak-ingress'
export type StepUnit = 'messages' | 'rps' | 'bytes' | 'bytes-per-sec'

export interface EstimationStep {
  id: StepId
  label: Caption
  formula: string
  expression: string
  value: number
  unit: StepUnit
}

export const SECONDS_PER_DAY = 86_400

export type PresetId = 'default' | 'messenger'

export const PRESETS: Record<PresetId, EstimationInput> = {
  default: { dau: 1_000_000, messagesPerUser: 20, messageBytes: 200, peakFactor: 2, retentionDays: 365, replicationFactor: 3 },
  // Обоснование каждого числа — в уроке 0.6.
  messenger: { dau: 100_000_000, messagesPerUser: 40, messageBytes: 200, peakFactor: 3, retentionDays: 1825, replicationFactor: 3 },
}

// Потолки для значений, введённых руками: без них 1e308 в поле превращает
// произведения в Infinity. Пределы заведомо шире любой реальной системы
// (10 млрд DAU — больше населения Земли), но держат все шаги конечными.
export const INPUT_LIMITS: Readonly<Record<keyof EstimationInput, number>> = {
  dau: 1e10,
  messagesPerUser: 1e4,
  messageBytes: 1e7,
  peakFactor: 100,
  retentionDays: 36_500,
  replicationFactor: 10,
}

function clamp(value: number, max: number): number {
  return Number.isFinite(value) && value > 0 ? Math.min(value, max) : 0
}

export function sanitizeInput(raw: EstimationInput): EstimationInput {
  return {
    dau: clamp(raw.dau, INPUT_LIMITS.dau),
    messagesPerUser: clamp(raw.messagesPerUser, INPUT_LIMITS.messagesPerUser),
    messageBytes: clamp(raw.messageBytes, INPUT_LIMITS.messageBytes),
    peakFactor: Math.max(clamp(raw.peakFactor, INPUT_LIMITS.peakFactor), 1),
    retentionDays: clamp(raw.retentionDays, INPUT_LIMITS.retentionDays),
    replicationFactor: Math.max(clamp(raw.replicationFactor, INPUT_LIMITS.replicationFactor), 1),
  }
}

// Подписи входов и результатов. Термин подчёркивается один раз на сцену: общий seen,
// порядок вызовов = порядок на экране (входы, затем шаги).
const sceneTerms = new Set<string>()
const label = (src: string): Caption => markedCaption(src, sceneTerms)

export interface InputField {
  key: keyof EstimationInput
  label: Caption
  hint: string
}

export const INPUT_FIELDS: readonly InputField[] = [
  { key: 'dau', label: label('[[dau|Активных пользователей в день (DAU)]]'), hint: 'Шкала логарифмическая: от тысячи до миллиарда.' },
  { key: 'messagesPerUser', label: label('Сообщений на пользователя в день'), hint: 'Сколько сообщений в среднем отправляет один активный пользователь.' },
  { key: 'messageBytes', label: label('Размер сообщения с [[metadata|метаданными]], байт'), hint: 'Текст плюс id, автор, чат, время, статус.' },
  { key: 'peakFactor', label: label('[[peak-factor|Коэффициент пика]]'), hint: 'Во сколько раз пик выше среднего.' },
  { key: 'retentionDays', label: label('[[retention|Срок хранения]], дней'), hint: '365 — год, 1825 — пять лет.' },
  { key: 'replicationFactor', label: label('[[replication-factor|Коэффициент репликации]]'), hint: 'Сколько копий данных храним.' },
]

// Пределы ползунков; DAU — отдельный логарифмический ползунок (DAU_SLIDER).
export const NUMERIC_RANGE: Readonly<Record<Exclude<keyof EstimationInput, 'dau'>, { min: number; max: number; step: number }>> = {
  messagesPerUser: { min: 1, max: 200, step: 1 },
  messageBytes: { min: 10, max: 5_000, step: 10 },
  peakFactor: { min: 1, max: 10, step: 0.5 },
  retentionDays: { min: 1, max: 3_650, step: 1 },
  replicationFactor: { min: 1, max: 5, step: 1 },
}

const STEP_LABEL: Readonly<Record<StepId, Caption>> = {
  'messages-per-day': label('Сообщений в день'),
  'avg-rps': label('Средняя нагрузка на запись ([[rps|RPS]])'),
  'peak-rps': label('Пиковая нагрузка на запись'),
  'storage-per-day': label('Новых данных в день'),
  'storage-total': label('Хранилище за весь срок с репликами'),
  'peak-ingress': label('Входящий трафик в пик'),
}

const n = (value: number): string => formatNumber(value, value < 10 ? 1 : 0)

export function estimate(raw: EstimationInput): EstimationStep[] {
  const i = sanitizeInput(raw)
  const perDay = i.dau * i.messagesPerUser
  const avgRps = perDay / SECONDS_PER_DAY
  const peakRps = avgRps * i.peakFactor
  const storageDay = perDay * i.messageBytes
  const storageTotal = storageDay * i.retentionDays * i.replicationFactor
  const peakIngress = peakRps * i.messageBytes
  return [
    { id: 'messages-per-day', label: STEP_LABEL['messages-per-day'], formula: 'DAU × сообщений на пользователя', expression: `${n(i.dau)} × ${n(i.messagesPerUser)} = ${n(perDay)}`, value: perDay, unit: 'messages' },
    { id: 'avg-rps', label: STEP_LABEL['avg-rps'], formula: 'сообщений в день ÷ 86 400 секунд', expression: `${n(perDay)} ÷ ${n(SECONDS_PER_DAY)} ≈ ${n(avgRps)}`, value: avgRps, unit: 'rps' },
    { id: 'peak-rps', label: STEP_LABEL['peak-rps'], formula: 'средняя × коэффициент пика', expression: `${n(avgRps)} × ${n(i.peakFactor)} ≈ ${n(peakRps)}`, value: peakRps, unit: 'rps' },
    { id: 'storage-per-day', label: STEP_LABEL['storage-per-day'], formula: 'сообщений в день × размер сообщения', expression: `${n(perDay)} × ${n(i.messageBytes)}\u00A0Б = ${formatBytes(storageDay)}`, value: storageDay, unit: 'bytes' },
    { id: 'storage-total', label: STEP_LABEL['storage-total'], formula: 'в день × дней хранения × коэффициент репликации', expression: `${formatBytes(storageDay)} × ${n(i.retentionDays)} × ${n(i.replicationFactor)} = ${formatBytes(storageTotal)}`, value: storageTotal, unit: 'bytes' },
    { id: 'peak-ingress', label: STEP_LABEL['peak-ingress'], formula: 'пиковый RPS × размер сообщения', expression: `${n(peakRps)} × ${n(i.messageBytes)}\u00A0Б ≈ ${formatBytes(peakIngress)}/с`, value: peakIngress, unit: 'bytes-per-sec' },
  ]
}

export function formatStepValue(step: EstimationStep): string {
  switch (step.unit) {
    case 'messages':
      return `${formatNumber(step.value)} сообщ./день`
    case 'rps':
      return `${formatNumber(step.value, step.value < 10 ? 1 : 0)} RPS`
    case 'bytes':
      return formatBytes(step.value)
    case 'bytes-per-sec':
      return `${formatBytes(step.value)}/с`
  }
}

// Аудитория меняется на порядки (тысяча … миллиард), поэтому слайдер логарифмический:
// позиция — степень десяти от 3 до 9.
export const DAU_SLIDER = { min: 3, max: 9, step: 0.1 } as const

export function sliderToDau(position: number): number {
  return Number((10 ** position).toPrecision(2))
}

export function dauToSlider(dau: number): number {
  return dau > 0 ? Math.min(Math.max(Math.log10(dau), DAU_SLIDER.min), DAU_SLIDER.max) : DAU_SLIDER.min
}

// M1: что именно считает калькулятор. Пресет мессенджера стоит в уроке 0.6 — там видно, откуда числа.
const CONTEXT =
  'Оценка [[back-of-envelope|на салфетке]]: считаем запись сообщений мессенджера — сколько INSERT в секунду и сколько байт на дисках. Чтение, статусы «доставлено/прочитано» и вложения не входят (урок 0.5, блок «нагрузка на чтение»). Нажмите на вход или результат — объяснение.'

export function estimationContext(preset: PresetId): Caption {
  return markedCaption(preset === 'messenger' ? `Пресет: требования урока 0.6. ${CONTEXT}` : CONTEXT, new Set())
}

// M5: входы — допущения, результат — оценка, а не прогноз.
export const ESTIMATION_HONESTY: Caption = 'Входные числа — допущения (уроки 0.5, 0.6), результат — оценка порядка величины.'

// M6: вопрос из инвентаризации 1.4; ответ — урок 0.5, задание 3.
export const ESTIMATION_QUIZ: StepQuiz = {
  question: 'Поставьте коэффициент репликации 1. Почему RPS и трафик в калькуляторе не изменились, а в реальной системе изменятся?',
  answer: markedCaption(
    'Калькулятор считает только то, что приходит от пользователей, а копии он учитывает одним множителем места в шаге 5. В реальной системе каждую запись нужно ещё доставить на реплики: при трёх копиях основной сервер отправляет каждое сообщение ещё двум репликам, и весь трафик записи растёт втрое (входящий плюс две копии), а каждая реплика выполняет ту же запись на своём диске. С одной копией этого трафика нет — но и данные умирают вместе с диском. Подробно о репликах — в модуле 8.',
    new Set(),
  ),
}
