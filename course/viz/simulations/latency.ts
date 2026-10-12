import { formatNumber } from '../core/format'

export type LatencyKind = 'cpu' | 'memory' | 'disk' | 'network'

export interface LatencyItem {
  id: string
  label: string
  ns: number
  kind: LatencyKind
}

// Порядки величин по Colin Scott (оценки на 2020 г., продолжение таблицы Jeff Dean / Peter Norvig).
// Это ориентиры, а не измерения конкретного железа.
export const LATENCIES: readonly LatencyItem[] = [
  { id: 'l1', label: 'Обращение к кэшу L1 процессора', ns: 1, kind: 'cpu' },
  { id: 'l2', label: 'Обращение к кэшу L2', ns: 4, kind: 'cpu' },
  { id: 'ram', label: 'Обращение к оперативной памяти', ns: 100, kind: 'memory' },
  { id: 'ram-1mb', label: 'Последовательно прочитать 1\u00a0МБ из памяти', ns: 3_000, kind: 'memory' },
  { id: 'ssd-random', label: 'Случайное чтение 4\u00a0КБ с SSD', ns: 16_000, kind: 'disk' },
  { id: 'ssd-1mb', label: 'Последовательно прочитать 1\u00a0МБ с SSD', ns: 49_000, kind: 'disk' },
  { id: 'dc-rtt', label: 'Round trip внутри одного дата-центра', ns: 500_000, kind: 'network' },
  { id: 'hdd-1mb', label: 'Последовательно прочитать 1\u00a0МБ с HDD', ns: 825_000, kind: 'disk' },
  { id: 'hdd-seek', label: 'Позиционирование головки HDD', ns: 2_000_000, kind: 'disk' },
  { id: 'intercontinental', label: 'Round trip Калифорния → Нидерланды → Калифорния', ns: 150_000_000, kind: 'network' },
]

export const LATENCY_SOURCE = {
  title: 'Colin Scott, «Latency Numbers Every Programmer Should Know» (оценки на 2020\u00a0г.)',
  url: 'https://colin-scott.github.io/personal_website/research/interactive_latency.html',
} as const

// Ненулевая минимальная ширина, чтобы самая короткая полоса была видна.
export const MIN_BAR = 0.02

const logMin = Math.log10(LATENCIES[0]!.ns)
const logMax = Math.log10(LATENCIES.at(-1)!.ns)

export function barWidth(ns: number): number {
  const share = (Math.log10(ns) - logMin) / (logMax - logMin)
  return MIN_BAR + (1 - MIN_BAR) * Math.min(Math.max(share, 0), 1)
}

// «Человеческий масштаб»: 1 нс реального времени = 1 с ощущаемого.
const HUMAN_SECONDS_PER_NS = 1

export function humanSeconds(ns: number): number {
  return ns * HUMAN_SECONDS_PER_NS
}

// «в 4 раза», «в 1,7 раза», но «в 5 раз», «в 12 раз», «в 160 раз».
function times(factor: number): string {
  const digits = factor < 10 ? 1 : 0
  const scale = 10 ** digits
  const rounded = Math.round(factor * scale) / scale
  const n = rounded % 100
  const raza = !Number.isInteger(rounded) || (n % 10 >= 2 && n % 10 <= 4 && (n < 12 || n > 14))
  return `в ${formatNumber(rounded, digits)} ${raza ? 'раза' : 'раз'}`
}

export function compare(item: LatencyItem, baseline: LatencyItem): string {
  if (item.id === baseline.id) return 'это и есть точка отсчёта'
  const ratio = item.ns / baseline.ns
  return ratio >= 1 ? `${times(ratio)} медленнее` : `${times(1 / ratio)} быстрее`
}
