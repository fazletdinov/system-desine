const NBSP = ' '

// ICU в разных версиях Node использует для разрядов то U+00A0, то U+202F.
// Приводим к одному символу, чтобы вывод был стабильным.
export function formatNumber(value: number, fractionDigits = 0): string {
  return new Intl.NumberFormat('ru-RU', { maximumFractionDigits: fractionDigits })
    .format(value)
    .replace(/[  ]/g, NBSP)
}

// Один знак после запятой для значений меньше 10: «2,5 с» информативнее «3 с»,
// а «144,3 мс» — ложная точность.
function compact(value: number): string {
  return formatNumber(value, Math.abs(value) < 10 ? 1 : 0)
}

interface Unit {
  size: number
  label: string
}

function inUnits(value: number, units: readonly Unit[]): string {
  let chosen = units[0] as Unit
  for (const unit of units) if (Math.abs(value) >= unit.size) chosen = unit
  return `${compact(value / chosen.size)}${NBSP}${chosen.label}`
}

const NS_UNITS: readonly Unit[] = [
  { size: 1, label: 'нс' },
  { size: 1e3, label: 'мкс' },
  { size: 1e6, label: 'мс' },
  { size: 1e9, label: 'с' },
]

// Десятичные единицы (1 КБ = 1000 Б): для оценок на салфетке это проще,
// а разница с двоичными (КиБ) — меньше 10% на любом порядке до ТБ.
const BYTE_UNITS: readonly Unit[] = ['Б', 'КБ', 'МБ', 'ГБ', 'ТБ', 'ПБ', 'ЭБ'].map((label, i) => ({ size: 1000 ** i, label }))

const SECOND_UNITS: readonly Unit[] = [
  { size: 1, label: 'с' },
  { size: 60, label: 'мин' },
  { size: 3_600, label: 'ч' },
  { size: 86_400, label: 'дн.' },
  { size: 31_536_000, label: 'г.' },
]

export function formatMs(ms: number): string {
  return `${compact(ms)}${NBSP}мс`
}

export function formatNs(ns: number): string {
  return inUnits(ns, NS_UNITS)
}

export function formatBytes(bytes: number): string {
  return inUnits(bytes, BYTE_UNITS)
}

export function formatSeconds(seconds: number): string {
  return inUnits(seconds, SECOND_UNITS)
}
