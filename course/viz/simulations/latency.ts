import { markedCaption, type Caption } from '../core/caption'
import { formatNs, formatNumber, formatSeconds } from '../core/format'
import type { StepQuiz } from '../core/types'
import { LAB_04, type LabValue } from './latency-lab'

export type LatencyKind = 'cpu' | 'memory' | 'disk' | 'network'
export type LatencyMode = 'real' | 'human'

export interface LatencyItem {
  id: string
  label: Caption
  // Где эта операция встречается в мессенджере курса (мерка M1).
  messenger: string
  kind: LatencyKind
  // Число модели Scott: по нему строятся полосы и «в N раз». Ответы заданий урока опираются на него.
  typical: { ns: number; source: string }
  // Диапазон на настоящем железе 2025–2026 с источником: здесь модель и железо честно расходятся.
  range?: { lo: number; hi: number; source: string }
  // Замер лабы 0.4 на машине автора; нет — значит, не мерили (HDD нет, второго сервера нет).
  author?: LabValue
}

export const KIND_LABEL: Record<LatencyKind, string> = { cpu: 'процессор', memory: 'память', disk: 'диск', network: 'сеть' }

const SCOTT = 'модель Scott, 2020'
// Строки, которые лаба не мерила: число модели и есть всё, что у нас для них есть.
const SCOTT_UNMEASURED = 'модель Scott / Дин 2009, экстраполяция'
const SEAGATE = 'даташит Seagate Exos X20 (ссылка — в labs/0-4/README.md)'

// Термин подчёркивается в первой строке, где встретился: подписи читаются сверху вниз.
const seen = new Set<string>()
const label = (src: string): Caption => markedCaption(src, seen)

// Порядки величин по Colin Scott (оценки на 2020 г., продолжение таблицы Jeff Dean / Peter Norvig).
// Это экстраполяции трендов, а не измерения конкретного железа; рядом — реальный диапазон и замер автора.
export const LATENCIES: readonly LatencyItem[] = [
  {
    id: 'l1',
    label: label('Обращение к [[cpu-cache|кэшу L1 процессора]]'),
    messenger: 'Go-сервис разбирает JSON запроса и проверяет токен (урок 0.3, шаг 10)',
    kind: 'cpu',
    typical: { ns: 1, source: SCOTT },
    range: { lo: 0.8, hi: 1.7, source: '4–5 тактов на частоте 3–5 ГГц (Intel 64 and IA-32 Architectures Optimization Reference Manual: ядро Golden Cove — 5 тактов)' },
    author: LAB_04.l1,
  },
  {
    id: 'l2',
    label: label('Обращение к кэшу L2'),
    messenger: 'тот же разбор JSON и проверка токена: данные, которые не поместились в L1',
    kind: 'cpu',
    typical: { ns: 4, source: SCOTT },
    range: { lo: 2.8, hi: 5.3, source: '14–16 тактов на частоте 3–5 ГГц (Intel Optimization Reference Manual: Golden Cove — 15 тактов)' },
    author: LAB_04.l2,
  },
  {
    id: 'ram',
    label: label('Обращение к [[ram|оперативной памяти]]'),
    messenger: 'кэш в процессе: map под мьютексом в Go-сервисе (урок 0.4)',
    kind: 'memory',
    typical: { ns: 100, source: SCOTT },
    range: { lo: 70, hi: 120, source: 'промах всех кэшей на DDR4/DDR5 (Drepper, «What Every Programmer Should Know About Memory», разд. 2–3)' },
    author: LAB_04.ram,
  },
  {
    id: 'ram-1mb',
    label: label('Последовательно прочитать 1 МБ из памяти'),
    messenger: 'страница истории чата, которая уже лежит в памяти сервиса',
    kind: 'memory',
    typical: { ns: 3_000, source: SCOTT },
    range: { lo: 35_000, hi: 100_000, source: 'одно ядро DDR4/DDR5 читает ~10–30 ГБ/с (Drepper, разд. 3)' },
    author: LAB_04.ram1mb,
  },
  {
    id: 'ssd-random',
    label: label('Случайное чтение 4 КБ с [[ssd|SSD]]'),
    messenger: 'PostgreSQL читает страницу 8 КБ, которой нет в кэше',
    kind: 'disk',
    typical: { ns: 16_000, source: SCOTT },
    range: { lo: 45_000, hi: 100_000, source: 'массовые NVMe на TLC при глубине очереди 1; даташит Samsung 990 PRO: 22K IOPS при QD 1, ≈45 мкс на чтение' },
    author: LAB_04.ssdRandP50,
  },
  {
    id: 'ssd-1mb',
    label: label('Последовательно прочитать 1 МБ с SSD'),
    messenger: 'запись WAL, бэкап базы',
    kind: 'disk',
    typical: { ns: 49_000, source: SCOTT },
    range: { lo: 70_000, hi: 290_000, source: 'по паспортной полосе NVMe: PCIe 5.0 ~14 ГБ/с, PCIe 4.0 — 7 450 МБ/с (даташит Samsung 990 PRO), PCIe 3.0 ~3,5 ГБ/с' },
    author: LAB_04.ssdSeqP50,
  },
  {
    id: 'dc-rtt',
    label: label('[[rtt|Круговой путь (RTT)]] внутри одного [[data-center|дата-центра]]'),
    messenger: 'Go-сервис → PostgreSQL или Redis в том же ЦОД (internalHop в уроке 0.3)',
    kind: 'network',
    typical: { ns: 500_000, source: SCOTT_UNMEASURED },
    range: { lo: 20_000, hi: 500_000, source: 'соседние серверы в современном ЦОД; оценка без замера, для замера нужен второй сервер' },
  },
  {
    id: 'hdd-1mb',
    label: label('Последовательно прочитать 1 МБ с [[hdd|HDD]]'),
    messenger: 'архив и холодное хранение: старые вложения (урок 0.6)',
    kind: 'disk',
    typical: { ns: 825_000, source: SCOTT_UNMEASURED },
    range: { lo: 3_500_000, hi: 7_000_000, source: `${SEAGATE}: до 285 МБ/с на внешних дорожках, на внутренних примерно вдвое медленнее` },
  },
  {
    id: 'hdd-seek',
    label: label('Позиционирование головки HDD'),
    messenger: 'случайное чтение из архива на HDD',
    kind: 'disk',
    typical: { ns: 2_000_000, source: SCOTT_UNMEASURED },
    range: { lo: 4_000_000, hi: 10_000_000, source: 'урок 0.4 («несколько мс, до ~10»); к ним добавляется в среднем пол-оборота, 4,16 мс по даташиту Seagate Exos X20' },
  },
  {
    id: 'intercontinental',
    label: label('Круговой путь Калифорния → Нидерланды → Калифорния'),
    messenger: 'телефон → ЦОД на другом континенте (урок 0.4: 144 → 474 мс)',
    kind: 'network',
    typical: { ns: 150_000_000, source: SCOTT_UNMEASURED },
    range: { lo: 88_000_000, hi: 150_000_000, source: 'нижняя граница: свет в оптоволокне (~200 000 км/с) по ~8 800 км дуги туда и обратно; верхняя: оценка модели' },
  },
]

// Строка контекста над шкалой (M1): шкала — это цены операций одного запроса мессенджера.
export const LATENCY_CONTEXT: Caption =
  'Сколько стоит каждая операция за одним сообщением в мессенджере: от разбора JSON в Go-сервисе до чтения страницы PostgreSQL с диска и пути телефона к ЦОД на другом континенте. Крупное число и полоса — модель Scott (экстраполяция трендов на 2020 г., не замер); строкой ниже — реальный диапазон и замер на машине автора. Нажмите на строку — объяснение.'

export const LATENCY_NOTE: Caption = markedCaption(
  'Длина полос — в [[log-scale|логарифмической шкале]]: каждые ~12% ширины означают ×10. Смотрите на подписи и «в N раз», а не на длину.',
  new Set(),
)

// Вопрос под шкалой (M6): тот самый компромисс из урока 0.4 про кэш в процессе.
export const LATENCY_QUIZ: StepQuiz = {
  question: 'Redis на соседнем сервере или map в процессе Go-сервиса: во сколько раз дороже одно обращение к Redis и чем платит map?',
  answer: markedCaption(
    'По модели примерно в 5 000 раз: [[rtt|круговой путь]] в ЦОД 0,5 мс против 100 нс обращения к памяти. Цена map — у каждого экземпляра сервиса своя копия кэша, и инвалидировать её сложно: устаревшую копию надо сбросить у каждого экземпляра (модуль 4).',
    new Set(),
  ),
}

export const LATENCY_SOURCE = {
  title: 'Colin Scott, «Latency Numbers Every Programmer Should Know» (оценки на 2020 г.)',
  url: 'https://colin-scott.github.io/personal_website/research/interactive_latency.html',
} as const

// Ненулевая минимальная ширина, чтобы самая короткая полоса была видна.
export const MIN_BAR = 0.02

const logMin = Math.log10(LATENCIES[0]!.typical.ns)
const logMax = Math.log10(LATENCIES.at(-1)!.typical.ns)

export function barWidth(ns: number): number {
  const share = (Math.log10(ns) - logMin) / (logMax - logMin)
  return MIN_BAR + (1 - MIN_BAR) * Math.min(Math.max(share, 0), 1)
}

// «Человеческий масштаб»: 1 нс реального времени = 1 с ощущаемого.
const HUMAN_SECONDS_PER_NS = 1

export function humanSeconds(ns: number): number {
  return ns * HUMAN_SECONDS_PER_NS
}

export const MODE_LABEL: Record<LatencyMode, string> = { real: 'Реальное время', human: 'Человеческий масштаб: 1 нс = 1 с' }

// Одно правило показа для строки шкалы и для объяснения: значение в выбранном масштабе.
export function latencyValue(ns: number, mode: LatencyMode): string {
  return mode === 'real' ? formatNs(ns) : formatSeconds(humanSeconds(ns))
}

// «0,8–1,7 нс», «3,5–7 мс»: единица одна, по верхней границе, чтобы диапазон читался как одно число.
export function latencyRange(range: { lo: number; hi: number }, mode: LatencyMode): string {
  const [lo, hi] = [latencyValue(range.lo, mode), latencyValue(range.hi, mode)]
  const [loNum, loUnit] = lo.split(' ')
  const hiUnit = hi.split(' ')[1]
  return loUnit === hiUnit ? `${loNum}–${hi}` : `${lo} – ${hi}`
}

export function baselineOf(id: string): LatencyItem {
  return LATENCIES.find((l) => l.id === id) ?? LATENCIES[0]!
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
  const ratio = item.typical.ns / baseline.typical.ns
  return ratio >= 1 ? `${times(ratio)} медленнее` : `${times(1 / ratio)} быстрее`
}
