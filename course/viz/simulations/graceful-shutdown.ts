import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Step } from '../core/types'

export type ShutdownVariant = 'exec-form' | 'wrapper-script'
export type ShutdownProcess = 'running' | 'draining' | 'shutting-down' | 'exited' | 'killed'

export interface ShutdownFrame {
  // Секунды от прихода SIGTERM.
  t: number
  process: ShutdownProcess
  readyz: 200 | 503 | null
  inLb: boolean
  inflight: number
  newRequests: 'served' | 'refused' | 'none'
  signalAt: 'pid1-app' | 'pid1-sh'
  exitCode: number | null
}

// Значения мессенджера и платформы: DRAIN_DELAY, SHUTDOWN_TIMEOUT, grace period, период пробы.
export const DRAIN_DELAY_S = 5
export const SHUTDOWN_TIMEOUT_S = 10
// Из stop_grace_period нашего compose; в Kubernetes - terminationGracePeriodSeconds. Не умолчание docker stop.
export const GRACE_PERIOD_S = 30
export const DOCKER_STOP_TIMEOUT_S = 10
export const LB_REMOVED_S = 1
export const TIMELINE_S = GRACE_PERIOD_S

const IN_FLIGHT = 3

const f = (p: Omit<ShutdownFrame, 'signalAt' | 'exitCode'> & Partial<Pick<ShutdownFrame, 'exitCode'>>, signalAt: ShutdownFrame['signalAt']): ShutdownFrame => ({
  exitCode: null,
  ...p,
  signalAt,
})

const EXEC: readonly Step<ShutdownFrame>[] = [
  {
    state: f({ t: 0, process: 'draining', readyz: 503, inLb: true, inflight: IN_FLIGHT, newRequests: 'served' }, 'pid1-app'),
    caption: [
      { text: 'ENTRYPOINT в exec-форме: приложение само ' },
      term('PID 1', 'pid-1'),
      { text: ', и SIGTERM от docker stop приходит прямо ему. Первое, что оно делает, — начинает ' },
      term('плавную остановку (graceful shutdown)', 'graceful-shutdown'),
      { text: ': /readyz сразу отвечает 503. Но балансировщик об этом ещё не знает, поэтому новые запросы пока обслуживаются как обычно.' },
    ],
  },
  {
    state: f({ t: 1, process: 'draining', readyz: 503, inLb: false, inflight: IN_FLIGHT, newRequests: 'none' }, 'pid1-app'),
    caption: [
      { text: 'Раз в секунду балансировщик опрашивает ' },
      term('readiness-проба', 'readiness-probe'),
      { text: '. Порог — одна неудача, так что на первом же 503 инстанс исключается из ротации. С этого момента новые запросы идут другим репликам, а у нас остаются только уже начатые.' },
    ],
  },
  {
    state: f({ t: 3, process: 'draining', readyz: 503, inLb: false, inflight: IN_FLIGHT, newRequests: 'none' }, 'pid1-app'),
    caption: 'Идёт пауза DRAIN_DELAY, пять секунд: приложение сознательно ничего не закрывает. Она нужна на случай, если балансировщик или kube-proxy узнали о недоступности позже нас, и последние «в пути» запросы всё ещё к нам доедут. Типичная остановка (около 6 секунд) укладывается и в 10 секунд, которые docker stop даёт по умолчанию, и в 30 секунд из stop_grace_period нашего compose (в Kubernetes то же задаёт terminationGracePeriodSeconds). Худший случай, 5 + 10 = 15 секунд, помещается только в 30.',
  },
  {
    state: f({ t: 5, process: 'shutting-down', readyz: 503, inLb: false, inflight: IN_FLIGHT, newRequests: 'refused' }, 'pid1-app'),
    caption: 'Дренаж закончился. Теперь http.Server.Shutdown закрывает слушающий сокет: новые соединения отклоняются, а начатые запросы получают до SHUTDOWN_TIMEOUT, 10 секунд, чтобы закончить. Три запроса ещё в работе.',
  },
  {
    state: f({ t: 5.5, process: 'shutting-down', readyz: 503, inLb: false, inflight: 1, newRequests: 'refused' }, 'pid1-app'),
    caption: 'Два запроса уже ответили, остался один. Обратите внимание: приложение не ждёт весь таймаут, а выходит, как только счётчик дошёл до нуля. Таймаут — потолок, а не обязательная пауза.',
  },
  {
    state: f({ t: 6, process: 'exited', readyz: null, inLb: false, inflight: 0, newRequests: 'none', exitCode: 0 }, 'pid1-app'),
    caption: 'Последний запрос получил ответ, процесс вышел с кодом 0 на шестой секунде. Ни один клиент не увидел ошибки. Docker засчитал остановку штатной. Шесть секунд помещаются и в 10 секунд docker stop по умолчанию, и в 30 секунд stop_grace_period.',
  },
]

const WRAPPER: readonly Step<ShutdownFrame>[] = [
  {
    state: f({ t: 0, process: 'running', readyz: 200, inLb: true, inflight: IN_FLIGHT, newRequests: 'served' }, 'pid1-sh'),
    caption: [
      { text: 'Тот же образ, но ENTRYPOINT [\"/start.sh\"], а скрипт запускает messenger без exec. Теперь ' },
      term('PID 1', 'pid-1'),
      { text: ' — это sh, а приложение — его ребёнок. SIGTERM от docker stop приходит в sh. У PID 1 ядро не применяет действие по умолчанию, поэтому sh без собственного обработчика сигнал игнорирует, и детям он не пересылается. Приложение ничего не знает: /readyz по-прежнему 200.' },
    ],
  },
  {
    state: f({ t: 5, process: 'running', readyz: 200, inLb: true, inflight: IN_FLIGHT, newRequests: 'served' }, 'pid1-sh'),
    caption: [
      { text: 'Пять секунд спустя ничего не изменилось: балансировщик исправно шлёт новые запросы, ' },
      term('readiness-проба', 'readiness-probe'),
      { text: ' зелёная, никакой ' },
      term('плавной остановки (graceful shutdown)', 'graceful-shutdown'),
      { text: ' нет. Docker терпеливо ждёт, а клиенты продолжают работать с инстансом, которого скоро не будет.' },
    ],
  },
  {
    state: f({ t: 10, process: 'killed', readyz: null, inLb: true, inflight: IN_FLIGHT, newRequests: 'refused', exitCode: 137 }, 'pid1-sh'),
    caption: 'docker stop по умолчанию ждёт 10 секунд и затем шлёт SIGKILL, который не перехватить. Код выхода 137 (128 + 9). Три запроса в работе оборваны, клиенты получили сброс соединения. Исправление — одна строка: exec messenger в скрипте, тогда приложение становится PID 1 и получает SIGTERM само.',
  },
]

export function gracefulShutdownScenario(variant: ShutdownVariant = 'exec-form'): readonly Step<ShutdownFrame>[] {
  return variant === 'exec-form' ? EXEC : WRAPPER
}
