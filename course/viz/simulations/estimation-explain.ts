// Объяснения входов и результатов калькулятора: что это, откуда типичное значение, куда входит,
// что число значит для архитектуры, что сейчас на экране и чего модель не видит.
// Чистая функция от id элемента и текущих входов: панель пересчитывается вместе с калькулятором.
import { captionText, markedCaption, type Caption } from '../core/caption'
import type { Explanation } from '../core/explain'
import { formatBytes, formatNumber } from '../core/format'
import { INPUT_FIELDS, SECONDS_PER_DAY, estimate, formatStepValue, sanitizeInput, type EstimationInput, type EstimationStep, type StepId } from './estimation'

// Замер лабы 2a, §8: chat.messages и три её индекса на 10 400 коротких сообщениях
// (PostgreSQL 18.6, машина автора). После перепрогона лабы тест сверит байты с файлом.
export const LAB_2A_MESSAGES = {
  file: 'labs/2a/results/messenger-explain.txt',
  rows: 10_400,
  tableBytes: 1_400_832,
  indexBytes: 1_753_088,
} as const

const perRow = (bytes: number): number => Math.round(bytes / LAB_2A_MESSAGES.rows)
const LAB_ROW = perRow(LAB_2A_MESSAGES.tableBytes)
const LAB_INDEX = perRow(LAB_2A_MESSAGES.indexBytes)
// Сумма — от байтов, а не от округлённых слагаемых: 303, а не 135 + 169 = 304.
const LAB_TOTAL = perRow(LAB_2A_MESSAGES.tableBytes + LAB_2A_MESSAGES.indexBytes)

// Одно знание — в одном месте: на это предложение ссылаются вход «размер» и оба шага хранилища.
const LAB_SENTENCE = `На реальной схеме модуля 2a индексы весят больше самой таблицы: ${LAB_ROW}\u00A0Б таблицы и ${LAB_INDEX}\u00A0Б трёх индексов на сообщение, вместе ≈ ${LAB_TOTAL}\u00A0Б (${LAB_2A_MESSAGES.file}, §8: ${formatNumber(LAB_2A_MESSAGES.rows)} сообщений с коротким текстом).`

// Ориентиры для архитектуры — оценки уроков 0.4–0.6, а не измеренные пределы.
const ORIENT = {
  // fsync ≈ 2 мс → около 500 одиночных подтверждений в секунду (урок 0.4).
  singleFsyncRps: 500,
  // «единицы, в лучшем случае десятки тысяч простых вставок в секунду» на один PostgreSQL (урок 0.6).
  groupCommitRps: 10_000,
  oneServerRps: 50_000,
  // полезного диска на одном сервере — допущение урока 0.6.
  serverDiskLo: 10e12,
  serverDiskHi: 20e12,
  // меньше мегабайта в секунду — меньше 8 Мбит/с (урок 0.5).
  quietNetwork: 1e6,
  // полезная полоса гигабитного порта после заголовков (урок 0.5).
  gigabitPort: 117e6,
  // заголовки WebSocket, TLS, TCP/IP и Ethernet на отдельное мелкое сообщение (урок 0.5).
  wireOverheadBytes: 120,
} as const

const num = (v: number): string => formatNumber(v, v < 10 ? 1 : 0)
const mbit = (bytesPerSec: number): string => `${num((bytesPerSec * 8) / 1e6)} Мбит/с`

function writeVerdict(rps: number): string {
  if (rps <= ORIENT.singleFsyncRps) return 'один PostgreSQL справится даже с одиночными транзакциями (≈ 500 fsync в секунду)'
  if (rps <= ORIENT.groupCommitRps) return 'один PostgreSQL, но выше ≈ 500 одиночных fsync в секунду: нужна групповая фиксация'
  if (rps <= ORIENT.oneServerRps) return 'на пределе одного сервера: тут нужен замер на своём железе, а не оценка'
  return 'один сервер не выдержит: запись делят между серверами — шардирование, модуль 9'
}

// Две значащие цифры: «≈ 220–440», как в уроке 0.6, — точнее оценка не бывает.
const servers = (count: number): string => formatNumber(Number(count.toPrecision(2)))

function storageVerdict(total: number, replicas: number): string {
  const copy = total / replicas
  if (copy <= ORIENT.serverDiskLo) return `одна копия — ${formatBytes(copy)}: помещается на один сервер, каждая копия — на своём`
  if (copy <= ORIENT.serverDiskHi) return `одна копия — ${formatBytes(copy)}: на грани одного сервера (10–20\u00A0ТБ полезного диска), запаса на рост нет`
  return `≈ ${servers(total / ORIENT.serverDiskHi)}–${servers(total / ORIENT.serverDiskLo)} серверов только под хранение (по 10–20\u00A0ТБ на сервер): данные разбивают по серверам`
}

function trafficVerdict(bytesPerSec: number): string {
  if (bytesPerSec < ORIENT.quietNetwork) return 'сеть не заметит'
  if (bytesPerSec <= ORIENT.gigabitPort) return 'помещается в один гигабитный порт (≈ 117\u00A0МБ/с полезных данных)'
  return 'больше гигабитного порта: нужны порты 10 Гбит/с или несколько входных узлов'
}

type Values = Record<StepId, EstimationStep>

interface InputText {
  what: string
  typical: string
  usedIn: string
  here: (i: EstimationInput, v: Values) => string
  real: string
}

const INPUTS: Record<keyof EstimationInput, InputText> = {
  dau: {
    what: '[[dau|DAU (daily active users)]] — сколько разных людей хоть раз за сутки пользовались продуктом. Главное число требований: от него считается всё остальное.',
    typical: '1 млн — вопрос тимлида в уроке 0.5; 100 млн — требования мессенджера в уроке 0.6. Ползунок логарифмический: каждое деление — умножение, поэтому на нём помещаются и тысяча, и миллиард.',
    usedIn: 'Во все шесть шагов, множителем: DAU ×10 — каждый результат ×10 (урок 0.5, задание 1).',
    here: (i, v) => `Сейчас ${formatNumber(i.dau)} — допущение. Это ${formatStepValue(v['messages-per-day'])} и ${formatStepValue(v['peak-rps'])} в пик.`,
    real: 'Берут из аналитики продукта. DAU — разные люди за сутки, а не одновременно подключённые: в пик онлайн только часть из них.',
  },
  messagesPerUser: {
    what: 'Сколько сообщений в среднем отправляет за сутки один активный пользователь.',
    typical: '20 в уроке 0.5, 40 в уроке 0.6. Это наше предположение и главный источник неопределённости: публичной статистики в таком разрезе мессенджеры обычно не дают.',
    usedIn: 'Во все шесть шагов, множителем — как и DAU.',
    here: (i, v) => `Сейчас ${num(i.messagesPerUser)} — допущение: ${formatStepValue(v['messages-per-day'])}.`,
    real: 'Распределение неравномерное: часть людей пишет сотни сообщений, часть только читает (урок 0.6). Среднее берут из метрик, когда они появятся.',
  },
  messageBytes: {
    what: 'Логический размер одного сообщения: текст плюс [[metadata|метаданные]] — идентификаторы сообщения, чата и автора, время, статус.',
    typical: '200\u00A0Б: около 100\u00A0Б текста (≈ 50 букв кириллицей по 2 байта в UTF-8) и около 100\u00A0Б метаданных (урок 0.6). Вложения хранятся отдельно, здесь только ссылка на файл.',
    usedIn: 'В шаги 4, 5 и 6: объём и трафик. На нагрузку в RPS не влияет.',
    here: (i, v) => `Сейчас ${formatNumber(i.messageBytes)}\u00A0Б — допущение: ${formatStepValue(v['storage-per-day'])} в день и ${formatStepValue(v['peak-ingress'])} в пик.`,
    real: `На диске больше: у каждой строки PostgreSQL заголовок 24\u00A0Б и указатель 4\u00A0Б, сверху — индексы (урок 0.5). ${LAB_SENTENCE} В сети к мелкому сообщению добавляется ≈ ${ORIENT.wireOverheadBytes}\u00A0Б заголовков.`,
  },
  peakFactor: {
    what: '[[peak-factor|Коэффициент пика]] — во сколько раз пиковая нагрузка выше средней за сутки. Ночью почти тишина, вечером и в праздники — шквал, а выдержать система должна именно шквал.',
    typical: '×2 — у Сюя в примере с Twitter (гл. 2); ×3 — мессенджер урока 0.6 (вечерний пик и праздники); ×5 — рабочий мессенджер, где почти всё пишут за 8 рабочих часов (задача про Slack в уроке 0.6). Обычно 2–5 — оценка, а не закон.',
    usedIn: 'В шаги 3 и 6: пиковая нагрузка и трафик в пик. Средняя нагрузка и хранилище от него не зависят.',
    here: (i, v) => `Сейчас ×${num(i.peakFactor)} — допущение: ${formatStepValue(v['peak-rps'])} в пик против ${formatStepValue(v['avg-rps'])} в среднем.`,
    real: 'Когда есть метрики, коэффициент берут из них: максимальный RPS за неделю делят на средний (урок 0.5).',
  },
  retentionDays: {
    what: '[[retention|Срок хранения]] — сколько дней сообщение живёт в системе до удаления или переноса в архив. Это продуктовое или юридическое решение, а не техническое.',
    typical: '365 дней в уроке 0.5, 5 лет (1 825 дней) в уроке 0.6. Многие мессенджеры хранят историю бессрочно; ползунок ограничен 10 годами.',
    usedIn: 'Только в шаг 5: хранилище растёт прямо пропорционально сроку. RPS и трафик от срока не зависят.',
    here: (i, v) => `Сейчас ${formatNumber(i.retentionDays)}\u00A0дн. ≈ ${num(i.retentionDays / 365)}\u00A0г. — допущение: ${formatStepValue(v['storage-total'])} с копиями.`,
    real: 'Старые сообщения читают редко, поэтому их часто переносят на дешёвое медленное хранилище — горячие и холодные данные (урок 0.6, модуль 9.5).',
  },
  replicationFactor: {
    what: '[[replication-factor|Коэффициент репликации]] — сколько копий данных держит система. Одна копия умирает вместе с диском.',
    typical: '3 — по умолчанию в HDFS и типичная продакшен-настройка в Kafka и Cassandra; для PostgreSQL — основной сервер и две реплики (урок 0.5).',
    usedIn: 'Только в шаг 5. RPS и трафик в калькуляторе от него не меняются — почему, разбирает вопрос под калькулятором.',
    here: (i, v) => `Сейчас ${num(i.replicationFactor)} — допущение: ${formatStepValue(v['storage-total'])} всего, одна копия — ${formatBytes(v['storage-total'].value / i.replicationFactor)}.`,
    real: 'Копии стоят не только места: каждую запись основной сервер ещё отправляет репликам, и каждая копия несёт те же индексы (модуль 8).',
  },
}

interface StepText {
  formula: string
  meaning: string
  here: (i: EstimationInput, v: Values) => string
  real: (i: EstimationInput, v: Values) => string
}

const STEPS: Record<StepId, StepText> = {
  'messages-per-day': {
    formula: 'DAU × сообщений на пользователя. Это сырьё для всех следующих шагов: делим на секунды — получаем нагрузку, умножаем на байты — объём.',
    meaning: 'Само число ничего не решает, его переводят в секунды (шаг 2) и байты (шаг 4). 20 млн в день (урок 0.5) и 4 млрд (урок 0.6) различаются в 200 раз — и это две разные архитектуры.',
    here: (_i, v) => `Сейчас ${formatStepValue(v['messages-per-day'])}: ${v['messages-per-day'].expression}.`,
    real: () => 'Сообщения — не единственная запись: статусы «доставлено» и «прочитано» пишутся на каждого получателя, и без объединения в пачки их может быть больше, чем самих сообщений (урок 0.6).',
  },
  'avg-rps': {
    formula: `Сообщений в день ÷ ${formatNumber(SECONDS_PER_DAY)} секунд в сутках. [[rps|RPS]] — запросов в секунду; здесь каждый запрос — один INSERT сообщения.`,
    meaning: 'Средняя нагрузка — для бюджета, а выдержать система должна пик (шаг 3). Ориентиры по записи (оценка уроков 0.4 и 0.6): до ≈ 500 в секунду хватает одиночных транзакций с [[fsync|fsync]] за 2\u00A0мс; до единиц, в лучшем случае десятков тысяч вставок — один PostgreSQL с групповой фиксацией (одним fsync подтверждают сразу пачку транзакций); выше — [[sharding|шардирование]], модуль 9.',
    here: (_i, v) => `Сейчас ${formatStepValue(v['avg-rps'])}: ${v['avg-rps'].expression}. Ориентир: ${writeVerdict(v['avg-rps'].value)}.`,
    real: () => 'Чтение не входит, а мессенджер в основном читает: каждое сообщение доставляют получателям и перечитывают в истории. У Twitter в 2012 году на 4,6 тыс. записей в секунду приходилось около 300 тыс. чтений лент (Клеппман, урок 0.6).',
  },
  'peak-rps': {
    formula: 'Средняя нагрузка × коэффициент пика. Под это число подбирают серверы: в пик система должна работать, а не только в среднем.',
    meaning: 'Те же ориентиры, что у шага 2, но применяют их к пику: ≈ 500 одиночных fsync в секунду, единицы–десятки тысяч вставок на один PostgreSQL с групповой фиксацией, выше — шардирование (модуль 9).',
    here: (_i, v) => `Сейчас ${formatStepValue(v['peak-rps'])}: ${v['peak-rps'].expression}. Ориентир: ${writeVerdict(v['peak-rps'].value)}.`,
    real: () => 'Каждую запись ещё повторяют реплики, а статусы доставки добавляют своих записей. Коэффициент пика в реальной системе берут из метрик: максимум RPS за неделю к среднему (урок 0.5).',
  },
  'storage-per-day': {
    formula: 'Сообщений в день × размер сообщения. Одна копия, логический размер — без индексов и служебных данных базы.',
    meaning: 'Ориентир — скорость роста: сколько дисков докупать. 4\u00A0ГБ в день (урок 0.5) незаметны; 800\u00A0ГБ в день (урок 0.6) — почти диск в сутки.',
    here: (i, v) => `Сейчас ${formatStepValue(v['storage-per-day'])} в день: ${v['storage-per-day'].expression}. За год одна копия — ${formatBytes(v['storage-per-day'].value * 365)}, с копиями — ${formatBytes(v['storage-per-day'].value * 365 * i.replicationFactor)}.`,
    real: () => `Модель считает логические байты. ${LAB_SENTENCE}`,
  },
  'storage-total': {
    formula: 'Новых данных в день × дней хранения × [[replication-factor|коэффициент репликации]]. Логический размер, без индексов и служебных данных базы.',
    meaning: 'Ориентир (допущение урока 0.6): на одном сервере 10–20\u00A0ТБ полезного диска. Помещается одна копия — каждая копия живёт на своём сервере (основной и реплики). Не помещается — данные разбивают по серверам (шардирование, модуль 9) и делят на горячие и холодные.',
    here: (i, v) => `Сейчас ${formatStepValue(v['storage-total'])}: ${v['storage-total'].expression}. Ориентир: ${storageVerdict(v['storage-total'].value, i.replicationFactor)}.`,
    real: (_i, v) =>
      `Индексы и служебные данные увеличивают объём в 1,5–2 раза (эвристика урока 0.5): ${formatBytes(v['storage-total'].value * 1.5)}–${formatBytes(v['storage-total'].value * 2)}. ${LAB_SENTENCE} Сверху — архив WAL, если его хранят для восстановления на любой момент.`,
  },
  'peak-ingress': {
    formula: 'Пиковый RPS × размер сообщения. Только входящий трафик и только полезные данные.',
    meaning: 'Ориентиры (урок 0.5): меньше 1\u00A0МБ/с — меньше 8\u00A0Мбит/с, сеть не заметит; гигабитный порт даёт ≈ 117\u00A0МБ/с полезных данных. Текст сообщений редко упирается в сеть — упираются вложения и исходящий трафик (модуль 11).',
    here: (_i, v) => `Сейчас ${formatStepValue(v['peak-ingress'])} ≈ ${mbit(v['peak-ingress'].value)}: ${v['peak-ingress'].expression}. Ориентир: ${trafficVerdict(v['peak-ingress'].value)}.`,
    real: (i, v) =>
      `Если каждое сообщение уходит отдельным пакетом по [[websocket|WebSocket]], сверху ≈ ${ORIENT.wireOverheadBytes}\u00A0Б заголовков WebSocket, TLS, TCP/IP и Ethernet (урок 0.5): ≈ ${formatBytes(v['peak-rps'].value * (i.messageBytes + ORIENT.wireOverheadBytes))}/с на проводе. Исходящий трафик больше входящего: сообщение уходит получателям ([[fan-out|fan-out]]). И каждую запись основной сервер шлёт ещё репликам: при ${num(i.replicationFactor)} копиях внутренний трафик — ещё ${num(i.replicationFactor - 1)} × входящий.`,
  },
}

export function explainEstimation(id: string, raw: EstimationInput): Explanation | null {
  const [kind, key] = id.split(':')
  if (key === undefined) return null
  const input = sanitizeInput(raw)
  const steps = estimate(input)
  const values = Object.fromEntries(steps.map((s) => [s.id, s])) as Values
  const seen = new Set<string>()
  const text = (src: string): Caption => markedCaption(src, seen)

  if (kind === 'in') {
    const field = INPUT_FIELDS.find((f) => f.key === key)
    if (field === undefined) return null
    const t = INPUTS[field.key]
    return {
      title: captionText(field.label),
      kind: 'вход модели · допущение',
      sections: [
        { heading: 'Что это', body: text(t.what) },
        { heading: 'Типичные значения', body: text(t.typical) },
        { heading: 'В какие шаги входит', body: text(t.usedIn) },
      ],
      here: text(t.here(input, values)),
      real: text(t.real),
    }
  }
  if (kind === 'step') {
    const index = steps.findIndex((s) => s.id === key)
    const step = steps[index]
    if (step === undefined) return null
    const t = STEPS[step.id]
    return {
      title: `Шаг ${index + 1}. ${captionText(step.label)}`,
      kind: 'результат · оценка порядка величины',
      sections: [
        { heading: 'Формула', body: text(t.formula) },
        { heading: 'Что это значит для архитектуры', body: text(t.meaning) },
      ],
      here: text(t.here(input, values)),
      real: text(t.real(input, values)),
    }
  }
  return null
}

// Не привязана к элементу: панель открывает её кнопкой «Что в модели упрощено».
export const ESTIMATION_SIMPLIFICATIONS: Explanation = {
  title: 'Что в модели упрощено',
  kind: 'чтобы отличать оценку от прогноза',
  sections: [
    {
      heading: 'Только запись сообщений',
      body: 'Калькулятор считает поток POST /messages → INSERT. Чтение истории, доставку получателям (fan-out), статусы «доставлено/прочитано», поиск и вложения оценивают тем же способом, отдельными строками (урок 0.5, блок «нагрузка на чтение»).',
    },
    {
      heading: 'Сутки — 86 400 секунд и один коэффициент пика',
      body: 'Калькулятор делит на 86 400 без округления до 10⁵, но точность здесь мнимая: ошибка во входах больше 15% разницы. Вся неравномерность суток и недели сжата в один множитель пика.',
    },
    {
      heading: 'Логический размер, без индексов',
      body: `Хранилище — байты сообщений, без заголовков строк, индексов, свободного места в страницах и WAL. Эвристика — ×1,5–2 (урок 0.5); замер на схеме модуля 2a — ${LAB_ROW} + ${LAB_INDEX}\u00A0Б на сообщение.`,
    },
    { heading: 'Трафик без заголовков', body: `Шаг 6 — полезные данные. На проводе к мелкому сообщению добавляется ≈ ${ORIENT.wireOverheadBytes}\u00A0Б заголовков (+60% к 200\u00A0Б), а исходящий трафик не считается совсем.` },
    {
      heading: 'Реплики не добавляют нагрузки',
      body: 'Коэффициент репликации умножает только место. В реальной системе каждую запись ещё доставляют на реплики и выполняют там же: растут внутренний трафик и запись на дисках (урок 0.5, задание 3).',
    },
  ],
}
