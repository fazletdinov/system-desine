import { term } from '../core/caption'
import type { Step } from '../core/types'

export interface MutexFrame {
  // Миллисекунды от начала сценария.
  t: number
  mode: 'normal' | 'starving'
  owner: string | null
  waiters: { g: string; since: number }[]
  arriving: string | null
  // barging — новичок на CPU может перехватить замок; direct — замок передаётся первому в очереди.
  handoff: 'barging' | 'direct' | null
}

// sync/mutex.go: starvationThresholdNs = 1e6 — ждущий дольше 1 мс переводит Mutex в режим голодания.
export const STARVATION_MS = 1

const w = (g: string, since: number): MutexFrame['waiters'][number] => ({ g, since })

const MUTEX = term('мьютекс (mutex)', 'mutex')
const STARVATION = term('режим голодания (starvation mode)', 'starvation-mode')

export function mutexModesScenario(): readonly Step<MutexFrame>[] {
  return [
    {
      state: { t: 0, mode: 'normal', owner: 'G1', waiters: [], arriving: null, handoff: null },
      caption: [{ text: 'Нормальный режим ' }, MUTEX, { text: ': G1 держит замок. Если никто не ждёт, Lock и Unlock — это одна атомарная операция в пространстве пользователя, без ядра и без планировщика.' }],
    },
    {
      state: { t: 0.2, mode: 'normal', owner: 'G1', waiters: [w('G2', 0.2)], arriving: null, handoff: null },
      caption: 'На t = 0,2 мс G2 пытается взять замок. Несколько раз покрутившись на месте (спиннинг, пока замок вот-вот освободится), она встаёт в очередь на семафоре и засыпает. Запоминается момент, когда она начала ждать: это отсчёт для порога в 1 мс.',
    },
    {
      state: { t: 0.5, mode: 'normal', owner: null, waiters: [w('G2', 0.2)], arriving: 'G3', handoff: null },
      caption: 'На t = 0,5 мс G1 отпускает замок и будит первого в очереди, G2. Но просыпающейся горутине нужно время: её надо поставить на P и дождаться запуска. А в этот же момент на CPU уже работает G3, и она как раз зовёт Lock.',
    },
    {
      state: { t: 0.5, mode: 'normal', owner: 'G3', waiters: [w('G2', 0.2)], arriving: null, handoff: 'barging' },
      caption: 'G3 хватает замок раньше проснувшейся G2. Это barging («влезание без очереди»), и в нормальном режиме оно сознательно разрешено: G3 уже горячая, ей не нужно переключение контекста, и суммарная пропускная способность выше. G2 проснулась, увидела занятый замок и вернулась в голову очереди, не потеряв отметку времени 0,2 мс. Ожидание 0,3 мс не больше порога.',
    },
    {
      state: { t: 1, mode: 'normal', owner: null, waiters: [w('G2', 0.2)], arriving: 'G4', handoff: null },
      caption: 'На t = 1,0 мс G3 отпускает замок, G2 снова будят, а у двери опять свежая G4. Ожидание G2 — 0,8 мс, порог пока не пройден.',
    },
    {
      state: { t: 1, mode: 'normal', owner: 'G4', waiters: [w('G2', 0.2)], arriving: null, handoff: 'barging' },
      caption: 'G4 снова перехватывает замок. Так G2 может проигрывать раз за разом: нормальный режим быстр, но несправедлив, и при плотном потоке новичков ждущий рискует не дождаться вовсе.',
    },
    {
      state: { t: 1.4, mode: 'normal', owner: null, waiters: [w('G2', 0.2)], arriving: 'G5', handoff: null },
      caption: 'На t = 1,4 мс G4 отпускает замок. G2 будят в третий раз, новичок G5 снова уже на CPU.',
    },
    {
      state: { t: 1.4, mode: 'normal', owner: 'G5', waiters: [w('G2', 0.2)], arriving: null, handoff: 'barging' },
      caption: 'G5 перехватывает замок в третий раз. G2 вот-вот проснётся, и вот что она увидит.',
    },
    {
      state: { t: 1.5, mode: 'starving', owner: 'G5', waiters: [w('G2', 0.2)], arriving: 'G6', handoff: 'direct' },
      caption: [
        { text: 'На t = 1,5 мс G2 проснулась, замок занят, а ждёт она уже 1,3 мс — больше порога в ' },
        { text: `${STARVATION_MS} мс. G2 переводит Mutex в ` },
        STARVATION,
        { text: ': отныне при Unlock замок передаётся напрямую первому в очереди, а новичков не пускают к нему даже на спиннинг. Пришедшая G6 в борьбе уже не участвует.' },
      ],
    },
    {
      state: { t: 1.5, mode: 'starving', owner: 'G5', waiters: [w('G2', 0.2), w('G6', 1.5)], arriving: null, handoff: 'direct' },
      caption: 'G6 встала в хвост очереди и засыпает: перехватить замок у стоящих впереди она не может. Справедливость куплена задержкой: переключения контекста на каждой передаче замка делают режим заметно медленнее нормального.',
    },
    {
      state: { t: 1.9, mode: 'starving', owner: 'G2', waiters: [w('G6', 1.5)], arriving: 'G7', handoff: 'direct' },
      caption: 'На t = 1,9 мс G5 вызывает Unlock, и замок отдаётся G2 напрямую, мимо всех. Он остаётся занятым на всём протяжении передачи, поэтому новичок G7 не может им воспользоваться. G2 выходить из режима не спешит: она ждала дольше порога, а в очереди ещё есть G6.',
    },
    {
      state: { t: 2.1, mode: 'normal', owner: 'G6', waiters: [w('G7', 1.9)], arriving: null, handoff: 'direct' },
      caption: 'На t = 2,1 мс Unlock передаёт замок G6. Она ждала всего 0,6 мс, меньше порога, поэтому Mutex возвращается в нормальный режим (так же происходит, если очередь опустела). G7, пришедшая раньше, стоит в очереди с отметкой 1,9 мс.',
    },
    {
      state: { t: 2.4, mode: 'normal', owner: 'G7', waiters: [], arriving: null, handoff: null },
      caption: 'G6 отпускает замок, G7 берёт его, очередь пуста. Mutex снова в быстром нормальном режиме. Итог: режим голодания — страховка от обгона для самых невезучих, а не штатный режим работы.',
    },
  ]
}
