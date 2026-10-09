import { formatBytes, formatNumber } from '../core/format'

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
  label: string
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

function nonNegative(value: number): number {
  return Number.isFinite(value) && value > 0 ? value : 0
}

export function sanitizeInput(raw: EstimationInput): EstimationInput {
  return {
    dau: nonNegative(raw.dau),
    messagesPerUser: nonNegative(raw.messagesPerUser),
    messageBytes: nonNegative(raw.messageBytes),
    peakFactor: Math.max(nonNegative(raw.peakFactor), 1),
    retentionDays: nonNegative(raw.retentionDays),
    replicationFactor: Math.max(nonNegative(raw.replicationFactor), 1),
  }
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
    { id: 'messages-per-day', label: 'Сообщений в день', formula: 'DAU × сообщений на пользователя', expression: `${n(i.dau)} × ${n(i.messagesPerUser)} = ${n(perDay)}`, value: perDay, unit: 'messages' },
    { id: 'avg-rps', label: 'Средняя нагрузка на запись', formula: 'сообщений в день ÷ 86 400 секунд', expression: `${n(perDay)} ÷ ${n(SECONDS_PER_DAY)} ≈ ${n(avgRps)}`, value: avgRps, unit: 'rps' },
    { id: 'peak-rps', label: 'Пиковая нагрузка на запись', formula: 'средняя × коэффициент пика', expression: `${n(avgRps)} × ${n(i.peakFactor)} ≈ ${n(peakRps)}`, value: peakRps, unit: 'rps' },
    { id: 'storage-per-day', label: 'Новых данных в день', formula: 'сообщений в день × размер сообщения', expression: `${n(perDay)} × ${n(i.messageBytes)} Б = ${formatBytes(storageDay)}`, value: storageDay, unit: 'bytes' },
    { id: 'storage-total', label: 'Хранилище за весь срок с репликами', formula: 'в день × дней хранения × коэффициент репликации', expression: `${formatBytes(storageDay)} × ${n(i.retentionDays)} × ${n(i.replicationFactor)} = ${formatBytes(storageTotal)}`, value: storageTotal, unit: 'bytes' },
    { id: 'peak-ingress', label: 'Входящий трафик в пик', formula: 'пиковый RPS × размер сообщения', expression: `${n(peakRps)} × ${n(i.messageBytes)} Б ≈ ${formatBytes(peakIngress)}/с`, value: peakIngress, unit: 'bytes-per-sec' },
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
