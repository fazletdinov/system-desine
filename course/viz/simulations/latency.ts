import { markedCaption, type Caption } from '../core/caption'
import { formatNs, formatNumber, formatSeconds } from '../core/format'
import type { StepQuiz } from '../core/types'
import { LAB_04, type LabValue } from './latency-lab'

export type LatencyKind = 'cpu' | 'memory' | 'disk' | 'network'
export type LatencyMode = 'real' | 'human'

export interface SourceLink {
  title: string
  url: string
}

export interface LatencyRange {
  lo: number
  hi: number
  source: string
  // true — диапазон без проверяемого источника (или с ним лишь для одной границы): в строке пишется «оценка», а не «реально».
  estimate?: boolean
  // Документы, где стоят числа диапазона; сцена выводит их ссылками под шкалой.
  links?: readonly SourceLink[]
}

export interface LatencyItem {
  id: string
  label: Caption
  // Где эта операция встречается в мессенджере курса (мерка M1).
  messenger: string
  kind: LatencyKind
  // Главное число строки: замер автора, где он есть, иначе модель. По нему крупное число, полоса и «в N раз».
  typical: { ns: number; source: string }
  // Число модели Scott — для сравнения в объяснении.
  model: { ns: number; source: string }
  range?: LatencyRange
  // Замер лабы 0.4 на машине автора; нет — значит, не мерили (HDD нет, второго сервера нет).
  author?: LabValue
}

export const KIND_LABEL: Record<LatencyKind, string> = { cpu: 'процессор', memory: 'память', disk: 'диск', network: 'сеть' }

const SCOTT = 'модель Scott, 2020, экстраполяция'
// Строки, которые лаба не мерила: число модели и есть главное число строки.
const SCOTT_UNMEASURED = 'модель Scott / Дин 2009, экстраполяция'
const AUTHOR = 'у автора, лаба 0.4'

// Каждый URL открыт и сверен с числом в тексте диапазона (2026-10-23).
const CHIPS_GOLDEN_COVE: SourceLink = {
  title: 'Chips and Cheese, «Going Armchair Quarterback on Golden Cove’s Caches»: L1D — 5 тактов, L2 — около 15 тактов (≈2,9 нс на 5,2 ГГц)',
  url: 'https://old.chipsandcheese.com/2022/02/11/going-armchair-quarterback-on-golden-coves-caches/',
}
const SAMSUNG_990: SourceLink = {
  title: 'Даташит Samsung 990 PRO: 7 450 МБ/с подряд, 22K IOPS случайного чтения при QD 1',
  url: 'https://download.semiconductor.samsung.com/resources/data-sheet/Samsung_NVMe_SSD_990_PRO_Datasheet_Rev.1.0_10129514072296.pdf',
}
const SAMSUNG_970: SourceLink = {
  title: 'Брошюра Samsung 970 PRO | EVO Plus (PCIe 3.0): до 3 500 МБ/с подряд, 15K–19K IOPS при QD 1',
  url: 'https://download.semiconductor.samsung.com/resources/brochure/SSD_970_PRO_EVO_Plus_Brochure_210712.pdf',
}
const SAMSUNG_9100: SourceLink = {
  title: 'Даташит Samsung 9100 PRO (PCIe 5.0): 14 700 МБ/с подряд у моделей 1 и 2 ТБ',
  url: 'https://download.semiconductor.samsung.com/resources/data-sheet/Samsung_NVMe_SSD_9100_PRO_Datasheet_Rev.2.0.pdf',
}
const SEAGATE_X20: SourceLink = {
  title: 'Даташит Seagate Exos X20: 7200 об/мин, до 285 МБ/с на внешних дорожках, средняя задержка вращения 4,16 мс',
  url: 'https://www.seagate.com/content/dam/seagate/migrated-assets/www-content/datasheets/pdfs/exos-x22-channel-DS2080-2111US-en_US.pdf',
}
const GRIGORIK: SourceLink = {
  title: 'Grigorik, «High Performance Browser Networking», гл. 1: свет в оптоволокне ~200 000 км/с',
  url: 'https://hpbn.co/primer-on-latency-and-bandwidth/',
}

interface RowSpec extends Omit<LatencyItem, 'typical' | 'model'> {
  modelNs: number
}

// Главное число выводится из замера, а не пишется руками: так оно не разойдётся с лабой.
function row({ modelNs, ...spec }: RowSpec): LatencyItem {
  const model = { ns: modelNs, source: spec.author ? SCOTT : SCOTT_UNMEASURED }
  return { ...spec, model, typical: spec.author ? { ns: spec.author.ns, source: AUTHOR } : model }
}

// Термин подчёркивается в первой строке, где встретился: подписи читаются сверху вниз.
const seen = new Set<string>()
const label = (src: string): Caption => markedCaption(src, seen)

// Модель — Colin Scott (оценки на 2020 г., продолжение таблицы Jeff Dean / Peter Norvig): экстраполяции
// трендов, а не измерения. Замеры — лаба 0.4 на машине автора (latency-lab.ts).
export const LATENCIES: readonly LatencyItem[] = [
  row({
    id: 'l1',
    label: label('Обращение к [[cpu-cache|кэшу L1 процессора]]'),
    messenger: 'Go-сервис разбирает JSON запроса и проверяет токен (урок 0.3, шаг 10)',
    kind: 'cpu',
    modelNs: 1,
    range: { lo: 0.96, hi: 1.67, source: '5 тактов на частоте 3–5,2 ГГц, ядро Golden Cove (Chips and Cheese)', links: [CHIPS_GOLDEN_COVE] },
    author: LAB_04.l1,
  }),
  row({
    id: 'l2',
    label: label('Обращение к кэшу L2'),
    messenger: 'тот же разбор JSON и проверка токена: данные, которые не поместились в L1',
    kind: 'cpu',
    modelNs: 4,
    range: { lo: 2.9, hi: 5, source: '≈2,9 нс (15 тактов на 5,2 ГГц) и 5 нс (на 3 ГГц), ядро Golden Cove (по Chips and Cheese)', links: [CHIPS_GOLDEN_COVE] },
    author: LAB_04.l2,
  }),
  row({
    id: 'ram',
    label: label('Обращение к [[ram|оперативной памяти]]'),
    messenger: 'кэш в процессе: map под мьютексом в Go-сервисе (урок 0.4)',
    kind: 'memory',
    modelNs: 100,
    range: { lo: 70, hi: 120, source: 'промах всех кэшей на настольной DDR4/DDR5; оценка без источника', estimate: true },
    author: LAB_04.ram,
  }),
  row({
    id: 'ram-1mb',
    label: label('Последовательно прочитать 1 МБ из памяти'),
    messenger: 'страница истории чата, которая уже лежит в памяти сервиса',
    kind: 'memory',
    modelNs: 3_000,
    range: { lo: 35_000, hi: 100_000, source: 'одно ядро DDR4/DDR5 читает ~10–30 ГБ/с; оценка без источника', estimate: true },
    author: LAB_04.ram1mb,
  }),
  row({
    id: 'ssd-random',
    label: label('Случайное чтение 4 КБ с [[ssd|SSD]]'),
    messenger: 'PostgreSQL читает страницу 8 КБ, которой нет в кэше',
    kind: 'disk',
    modelNs: 16_000,
    range: { lo: 45_000, hi: 67_000, source: 'по паспорту при глубине очереди 1: Samsung 990 PRO — 22K IOPS (≈45 мкс), 970 PRO — 15K IOPS (≈67 мкс)', links: [SAMSUNG_990, SAMSUNG_970] },
    author: LAB_04.ssdRandP50,
  }),
  row({
    id: 'ssd-1mb',
    label: label('Последовательно прочитать 1 МБ с SSD'),
    messenger: 'запись WAL, бэкап базы',
    kind: 'disk',
    modelNs: 49_000,
    range: {
      lo: 68_000,
      hi: 286_000,
      source: 'по паспортной полосе (она достигается при глубокой очереди): PCIe 5.0, Samsung 9100 PRO — 14 700 МБ/с (≈68 мкс); PCIe 4.0, 990 PRO — 7 450 МБ/с (≈134 мкс); PCIe 3.0, 970 EVO Plus — 3 500 МБ/с (≈286 мкс)',
      links: [SAMSUNG_9100, SAMSUNG_990, SAMSUNG_970],
    },
    author: LAB_04.ssdSeqP50,
  }),
  row({
    id: 'dc-rtt',
    label: label('[[rtt|Круговой путь (RTT)]] внутри одного [[data-center|дата-центра]]'),
    messenger: 'Go-сервис → PostgreSQL или Redis в том же ЦОД (internalHop в уроке 0.3)',
    kind: 'network',
    modelNs: 500_000,
    range: { lo: 20_000, hi: 500_000, source: 'соседние серверы в современном ЦОД; оценка без источника и без замера, для замера нужен второй сервер', estimate: true },
  }),
  row({
    id: 'hdd-1mb',
    label: label('Последовательно прочитать 1 МБ с [[hdd|HDD]]'),
    messenger: 'архив и холодное хранение: старые вложения (урок 0.6)',
    kind: 'disk',
    modelNs: 825_000,
    range: {
      lo: 3_500_000,
      hi: 7_000_000,
      source: 'нижняя граница — 285 МБ/с на внешних дорожках по даташиту Seagate Exos X20; верхняя (внутренние дорожки примерно вдвое медленнее) — оценка',
      estimate: true,
      links: [SEAGATE_X20],
    },
  }),
  row({
    id: 'hdd-seek',
    label: label('Позиционирование головки HDD'),
    messenger: 'случайное чтение из архива на HDD',
    kind: 'disk',
    modelNs: 2_000_000,
    range: {
      lo: 4_000_000,
      hi: 10_000_000,
      source: 'оценка без источника: несколько мс, до ~10; к ним добавляется в среднем пол-оборота — 4,16 мс по даташиту Seagate Exos X20',
      estimate: true,
      links: [SEAGATE_X20],
    },
  }),
  row({
    id: 'intercontinental',
    label: label('Круговой путь Калифорния → Нидерланды → Калифорния'),
    messenger: 'телефон → ЦОД на другом континенте (урок 0.4: 144 → 474 мс)',
    kind: 'network',
    modelNs: 150_000_000,
    range: {
      lo: 88_000_000,
      hi: 150_000_000,
      source: 'нижняя граница — расчёт: свет в оптоволокне ~200 000 км/с (Grigorik) по ~8 800 км дуги туда и обратно; верхняя — модель',
      estimate: true,
      links: [GRIGORIK],
    },
  }),
]

// Все документы диапазонов по одному разу, в порядке строк: сцена выводит их ссылками под шкалой.
export const RANGE_LINKS: readonly SourceLink[] = [...new Map(LATENCIES.flatMap((l) => l.range?.links ?? []).map((k) => [k.url, k])).values()]

// Строка контекста над шкалой (M1): шкала — это цены операций одного запроса мессенджера.
export const LATENCY_CONTEXT: Caption =
  'Сколько стоит каждая операция за одним сообщением в мессенджере: от разбора JSON в Go-сервисе до чтения страницы PostgreSQL с диска и пути телефона к ЦОД на другом континенте. Крупное число и полоса — замер на машине автора (лаба 0.4), где он есть; у HDD и сети замера нет, там — модель Scott / Дин (экстраполяция, не замер). Строкой ниже — модель и реальный диапазон или оценка. Нажмите на строку — объяснение.'

export const LATENCY_NOTE: Caption = markedCaption(
  'Длина полос — в [[log-scale|логарифмической шкале]]: каждые ~12% ширины означают ×10. Смотрите на подписи и «в N раз», а не на длину.',
  new Set(),
)

// Вопрос под шкалой (M6): тот самый компромисс из урока 0.4 про кэш в процессе.
export const LATENCY_QUIZ: StepQuiz = {
  question: 'Redis на соседнем сервере или map в процессе Go-сервиса: во сколько раз дороже одно обращение к Redis и чем платит map?',
  answer: markedCaption(
    'По модели примерно в 5 000 раз: [[rtt|круговой путь]] в ЦОД 0,5 мс против 100 нс обращения к памяти. С оценкой RTT в современном ЦОД 20–500 мкс выходит в 200–5 000 раз — всё равно на порядки. Цена map — у каждого экземпляра сервиса своя копия кэша, и инвалидировать её сложно: устаревшую копию надо сбросить у каждого экземпляра (модуль 4).',
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
