import { term } from '../core/caption'
import type { Step } from '../core/types'

export type HbVariant = 'none' | 'channel' | 'mutex'

export interface HbEvent {
  id: string
  op: string
  // Момент по «часам стенки» — только для подписи на шкале; порядок гарантируют рёбра, а не t.
  t: number
}

export interface HbFrame {
  g1: HbEvent[]
  g2: HbEvent[]
  edges: { from: string; to: string }[]
  outcome: 'any' | 'guaranteed' | null
  raceReport: boolean
}

type EventDef = [goroutine: 1 | 2, event: HbEvent]
type Edge = HbFrame['edges'][number]

// Кадр k — первые k событий и рёбра, чьи оба конца уже показаны.
function build(events: readonly EventDef[], edges: readonly Edge[], captions: readonly Step<HbFrame>['caption'][], outcomeAt: HbFrame['outcome'], race: boolean): readonly Step<HbFrame>[] {
  return captions.map((caption, i) => {
    const shown = events.slice(0, i + 1)
    const ids = new Set(shown.map(([, e]) => e.id))
    const last = i === captions.length - 1
    return {
      state: {
        g1: shown.filter(([g]) => g === 1).map(([, e]) => e),
        g2: shown.filter(([g]) => g === 2).map(([, e]) => e),
        edges: edges.filter((e) => ids.has(e.from) && ids.has(e.to)),
        outcome: last ? outcomeAt : null,
        raceReport: last && race,
      },
      caption,
    }
  })
}

const ev = (id: string, op: string, t: number): HbEvent => ({ id, op, t })

const HB = term('«происходит раньше» (happens-before)', 'happens-before')
const MM = term('модель памяти (memory model)', 'memory-model')
const RACE = term('гонка данных (data race)', 'data-race')

const NONE = build(
  [
    [1, ev('write', 'data = 42', 1)],
    [1, ev('flag', 'done = true', 2)],
    [2, ev('wait', 'if done { … }', 3)],
    [2, ev('read', 'print(data)', 4)],
  ],
  [],
  [
    [{ text: 'G1 пишет data = 42. Пока это обычная запись в память: никто её не видит и ничего не обещает. Правила, что увидит другая горутина, задаёт ' }, MM, { text: ' Go.' }],
    'G1 поднимает флаг done = true. Кажется естественным: «сначала данные, потом флаг, значит, увидевший флаг увидит и данные». Но компилятор и процессор вправе переставить две независимые записи, а запись из буфера ядра может дойти до другого ядра в другом порядке.',
    'G2 на другом ядре читает done. Даже если она увидела true, между записью в G1 и чтением в G2 нет ни одного синхронизирующего события, то есть нет цепочки, которая связала бы их.',
    [
      { text: 'G2 читает data. В графе нет ни одного ребра между горутинами: запись ' },
      { text: 'не «происходит раньше» чтения (' },
      HB,
      { text: ' не установлено), а значит, это ' },
      RACE,
      { text: ': результат может быть 42, 0, а для строки или среза — «рваным» значением. Детектор выведет: WARNING: DATA RACE — Read at 0x… by goroutine 8 / Previous write at 0x… by goroutine 7. Гарантий нет.' },
    ],
  ],
  'any',
  true,
)

const CHANNEL = build(
  [
    [1, ev('write', 'data = 42', 1)],
    [1, ev('send', 'ch <- struct{}{}', 2)],
    [2, ev('recv', '<-ch', 3)],
    [2, ev('read', 'print(data)', 4)],
  ],
  [{ from: 'send', to: 'recv' }],
  [
    [{ text: 'G1 пишет data = 42, пока ничего не публикуя. Теперь добавим синхронизацию, а не флаг: после записи горутина отправит значение в канал. По ' }, MM, { text: ' запись, стоящая раньше отправки в программе, «происходит раньше» самой отправки (порядок внутри одной горутины).' }],
    'G1 отправляет в канал ch. Это синхронизирующее событие: по правилу модели памяти отправка в канал синхронизирована с завершением соответствующего получения.',
    [{ text: 'G2 завершила получение из ch. Появилось ребро: отправка → завершение получения. Теперь действует ' }, HB, { text: ': всё, что G1 сделала до отправки, видно G2 после получения. Для небуферизованного канала верно и обратное направление: получение «происходит раньше» завершения отправки.' }],
    [{ text: 'G2 читает data. Цепочка запись → отправка → получение → чтение по программному порядку и ребру замыкается, поэтому чтение гарантированно видит 42 и ' }, RACE, { text: ' нет: детектор молчит. Тот же приём лежит в основе «передавай данные по каналу, а не делай их общими».' }],
  ],
  'guaranteed',
  false,
)

const MUTEX = build(
  [
    [1, ev('lock1', 'mu.Lock()', 1)],
    [1, ev('write', 'data = 42', 2)],
    [1, ev('unlock1', 'mu.Unlock()', 3)],
    [2, ev('lock2', 'mu.Lock()', 4)],
    [2, ev('read', 'print(data)', 5)],
  ],
  [{ from: 'unlock1', to: 'lock2' }],
  [
    [{ text: 'G1 берёт замок и входит в критическую секцию. Сам по себе замок защищает только тем, что в секции в каждый момент одна горутина; а вот что увидит следующая, определяет ' }, MM, { text: '.' }],
    'G1 пишет data = 42 внутри секции. Это обычная запись, но она оформлена между Lock и Unlock.',
    'G1 отпускает замок. По модели памяти это событие — первое в паре: n-й вызов Unlock синхронизирован перед возвратом (n+1)-го вызова Lock.',
    [{ text: 'G2 берёт тот же замок, и Lock возвращается. Появилось ребро Unlock № 1 → Lock № 2. Установлено ' }, HB, { text: ': всё, что G1 делала до Unlock, видно G2 после Lock.' }],
    [{ text: 'G2 читает data и гарантированно видит 42. ' }, RACE, { text: ' нет, потому что оба обращения упорядочены. Если бы G2 прочитала data без Lock, ребра не было бы, и мы вернулись бы к первому сценарию.' }],
  ],
  'guaranteed',
  false,
)

export function happensBeforeScenario(variant: HbVariant = 'none'): readonly Step<HbFrame>[] {
  return variant === 'none' ? NONE : variant === 'channel' ? CHANNEL : MUTEX
}
