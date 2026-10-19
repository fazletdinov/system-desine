import { term } from '../core/caption'
import type { Step } from '../core/types'

export type LifecycleVariant = 'signal' | 'component-failure' | 'second-signal'
export type HttpState = 'serving' | 'draining' | 'shutting-down' | 'stopped'
export type BackState = 'running' | 'stopping' | 'stopped' | 'failed'

export interface LifecycleFrame {
  // Секунды от начала сценария (SIGTERM или сбоя).
  t: number
  http: HttpState
  readyz: 200 | 503
  back: { name: 'idempotency-sweeper'; state: BackState }[]
  hashQueue: { running: number; waiting: number; rejected: number }
  exit: number | null
  note: string
}

// Значения мессенджера (cmd/messenger, platform/lifecycle, auth/adapters/hashlimit).
export const DRAIN_DELAY_S = 5
export const SHUTDOWN_TIMEOUT_S = 10
export const BACK_STOP_TIMEOUT_S = 2
export const HASH_CONCURRENCY = 2
export const HASH_QUEUE = 8
export const TIMELINE_S = 20

const SWEEPER = 'idempotency-sweeper' as const
const COMPONENT = term('компонент с жизненным циклом (component lifecycle)', 'component-lifecycle')
const ERRGROUP = term('errgroup', 'errgroup')
const GRACEFUL = term('плавная остановка (graceful shutdown)', 'graceful-shutdown')
const CRASH_ONLY = term('crash-only', 'crash-only')

const fr = (t: number, http: HttpState, readyz: 200 | 503, back: BackState, hashQueue: LifecycleFrame['hashQueue'], exit: number | null, note: string): LifecycleFrame => ({
  t,
  http,
  readyz,
  back: [{ name: SWEEPER, state: back }],
  hashQueue,
  exit,
  note,
})

const BUSY = { running: HASH_CONCURRENCY, waiting: 3, rejected: 0 }
const NONE = { running: 0, waiting: 0, rejected: 0 }

const SIGNAL: readonly Step<LifecycleFrame>[] = [
  {
    state: fr(0, 'serving', 200, 'running', BUSY, null, 'работа'),
    caption: [
      { text: 'Обычная работа. HTTP-сервер и фоновая очистка ключей идемпотентности — два ' },
      COMPONENT,
      { text: ' с общим контрактом Run(ctx) error, запущенные под одним ' },
      ERRGROUP,
      { text: `. Очистка срабатывает по таймеру. В очереди хеширования паролей Argon2id выполняются ${HASH_CONCURRENCY} хеша (HASH_CONCURRENCY), ещё 3 ждут места из ${HASH_QUEUE} (HASH_QUEUE); девятый запрос получил бы 503 сразу.` },
    ],
  },
  {
    state: fr(0, 'draining', 503, 'running', BUSY, null, 'SIGTERM'),
    caption: 'Пришёл SIGTERM. Первое, что делает приложение: /readyz отвечает 503. Сервер при этом продолжает принимать запросы, ведь балансировщик ещё не знает, что инстанс уходит. Фоновая очистка работает: ей отмену пока не посылали.',
  },
  {
    state: fr(DRAIN_DELAY_S, 'shutting-down', 503, 'running', BUSY, null, 'конец дренажа'),
    caption: [
      { text: `Прошла пауза DRAIN_DELAY, ${DRAIN_DELAY_S} с: балансировщик давно убрал инстанс. Начинается ` },
      GRACEFUL,
      { text: `: http.Server.Shutdown закрывает слушающий сокет и ждёт начатые запросы до SHUTDOWN_TIMEOUT, ${SHUTDOWN_TIMEOUT_S} с. Хеши, которые ещё в очереди, — это тоже начатые запросы, их тоже дождутся.` },
    ],
  },
  {
    state: fr(6, 'shutting-down', 503, 'running', { running: 1, waiting: 0, rejected: 0 }, null, 'запросы заканчиваются'),
    caption: 'Хеши досчитаны, один ещё считается. Сервер не ждёт весь таймаут: он выйдет, как только счётчик начатых запросов дойдёт до нуля. Фоновая очистка всё ещё работает: запросам могла понадобиться она, обратное неверно.',
  },
  {
    state: fr(6.4, 'stopped', 503, 'running', NONE, null, 'HTTP остановлен'),
    caption: 'HTTP-сервер остановился: все начатые запросы закончены. Только теперь фоновые компоненты получают отмену. Порядок «сначала фронт, потом фон» гарантирует, что ни один запрос не остался без своих зависимостей.',
  },
  {
    state: fr(6.4, 'stopped', 503, 'stopping', NONE, null, 'остановка фона'),
    caption: `Отмена контекста дошла до idempotency-sweeper, он завершает итерацию. На всё про всё у фона ${BACK_STOP_TIMEOUT_S} с (backStopTimeout): компонент, игнорирующий отмену, не должен вешать выход процесса.`,
  },
  {
    state: fr(6.9, 'stopped', 503, 'stopped', NONE, null, 'фон остановлен'),
    caption: 'Очистка вышла за полсекунды и вернула nil. Все компоненты остановлены, Run возвращается без ошибки.',
  },
  {
    state: fr(6.9, 'stopped', 503, 'stopped', NONE, 0, 'выход'),
    caption: [
      { text: 'Процесс завершается с кодом 0: штатная ' },
      GRACEFUL,
      { text: ' уложилась в 6,9 с. Ни один клиент не увидел ошибки. Худший случай — DRAIN_DELAY + SHUTDOWN_TIMEOUT + backStopTimeout = 17 с, он по-прежнему в пределах 30 с, которые оркестратор даёт до SIGKILL. Всё остальное — аварийный путь по принципу ' },
      CRASH_ONLY,
      { text: ': если выйти чисто не вышло, лучше упасть и дать оркестратору перезапустить.' },
    ],
  },
]

const FAILURE: readonly Step<LifecycleFrame>[] = [
  {
    state: fr(0, 'serving', 200, 'running', BUSY, null, 'работа'),
    caption: [
      { text: 'Та же картина: HTTP и idempotency-sweeper — ' },
      COMPONENT,
      { text: ' под одним ' },
      ERRGROUP,
      { text: ', очередь хешей загружена. Но на этот раз никто не посылал SIGTERM.' },
    ],
  },
  {
    state: fr(3, 'draining', 503, 'failed', BUSY, null, 'сбой компонента'),
    caption: 'На третьей секунде idempotency-sweeper вернул ошибку, не дожидаясь отмены. errgroup отменяет общий контекст: сервер идёт тем же путём, что и по сигналу, — /readyz 503, дренаж. Продолжать работать без очистки молча было бы хуже: ключи копились бы, а никто бы не знал. Потратить 5 секунд на аккуратный уход дешевле, чем оборвать начатые запросы из-за упавшей очистки.',
  },
  {
    state: fr(3 + DRAIN_DELAY_S, 'shutting-down', 503, 'failed', BUSY, null, 'Shutdown'),
    caption: [
      { text: `Пауза DRAIN_DELAY, ${DRAIN_DELAY_S} с, закончилась, Shutdown ждёт начатые запросы: ` },
      GRACEFUL,
      { text: ' не зависит от того, почему мы уходим. Упавший компонент уже не работает, остальным фоновым отмена придёт позже, после HTTP.' },
    ],
  },
  {
    state: fr(8.6, 'stopped', 503, 'failed', NONE, null, 'HTTP остановлен'),
    caption: 'Все запросы закончены, HTTP остановлен. Остальные фоновые компоненты (если бы они были) получили бы отмену сейчас; упавшему останавливаться нечему.',
  },
  {
    state: fr(8.6, 'stopped', 503, 'failed', NONE, 1, 'выход с кодом 1'),
    caption: [
      { text: 'Run вернул ошибку с именем упавшего компонента, процесс вышел с кодом 1 — оркестратор увидит сбой и перезапустит инстанс. Это философия ' },
      CRASH_ONLY,
      { text: ': не пытаться жить с повреждённым состоянием, а упасть громко и начать заново. Паника в фоновой горутине не перехватывается и тоже роняет процесс, но с кодом 2 и без дренажа.' },
    ],
  },
]

const SECOND: readonly Step<LifecycleFrame>[] = [
  {
    state: fr(0, 'serving', 200, 'running', BUSY, null, 'работа'),
    caption: [
      { text: 'Обычная работа: HTTP и idempotency-sweeper — ' },
      COMPONENT,
      { text: ' под одним ' },
      ERRGROUP,
      { text: ', в очереди хешей 2 + 3.' },
    ],
  },
  {
    state: fr(0, 'draining', 503, 'running', BUSY, null, 'SIGTERM'),
    caption: [
      { text: 'Первый SIGTERM запустил ' },
      GRACEFUL,
      { text: ': /readyz 503, дренаж. Впереди пять секунд паузы, а оператор не хочет ждать, или что-то пошло не так.' },
    ],
  },
  {
    state: fr(2, 'draining', 503, 'running', BUSY, 1, 'второй SIGTERM'),
    caption: [
      { text: 'На второй секунде приходит второй сигнал. Он не запускает ничего нового: функция exitOnSecondSignal пишет в журнал «второй сигнал: выхожу немедленно» и вызывает os.Exit(1). HTTP не дошёл до остановки, defer-ы не выполняются, начатые запросы оборваны. Это осознанная цена: так делает и Kubernetes. Принцип ' },
      CRASH_ONLY,
      { text: ' — безопасно упасть всегда; состояние, которое нельзя бросить, не должно жить в памяти процесса.' },
    ],
  },
]

export function appLifecycleScenario(variant: LifecycleVariant = 'signal'): readonly Step<LifecycleFrame>[] {
  return variant === 'signal' ? SIGNAL : variant === 'component-failure' ? FAILURE : SECOND
}
