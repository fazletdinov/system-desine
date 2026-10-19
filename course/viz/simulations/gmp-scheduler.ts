import { term } from '../core/caption'
import type { Step } from '../core/types'

export type GmpNote = 'spawn' | 'steal' | 'syscall-handoff' | 'preempt' | 'syscall-return' | 'idle'
export type MState = 'running' | 'spinning' | 'syscall' | 'idle'

export interface GmpP {
  id: 'P0' | 'P1'
  m: string | null
  running: string | null
  runnext: string | null
  local: string[]
}

export interface GmpFrame {
  ps: GmpP[]
  global: string[]
  ms: { id: string; state: MState }[]
  note: GmpNote
}

// Из runtime/runtime2.go: p.runq — кольцо на 256 G; runtime/proc.go: sysmon вытесняет G, занявшую P дольше 10 мс.
export const LOCAL_QUEUE_CAP = 256
export const PREEMPT_AFTER_MS = 10

interface PInit {
  m?: string | null
  running?: string | null
  runnext?: string | null
  local?: string[]
}

const p = (id: GmpP['id'], v: PInit): GmpP => ({ id, m: null, running: null, runnext: null, ...v, local: v.local ?? [] })

const frame = (note: GmpNote, p0: PInit, p1: PInit, ms: GmpFrame['ms'], global: string[] = []): GmpFrame => ({
  ps: [p('P0', p0), p('P1', p1)],
  global,
  ms,
  note,
})

export function gmpSchedulerScenario(): readonly Step<GmpFrame>[] {
  return [
    {
      state: frame('idle', { m: 'M0', running: 'G1' }, {}, [{ id: 'M0', state: 'running' }, { id: 'M1', state: 'idle' }]),
      caption: [
        { text: 'Модель ' },
        term('GMP', 'gmp'),
        { text: ': G — горутина, M — поток ОС, P — право выполнять Go-код (их ровно GOMAXPROCS, здесь два). Горутина G1 бежит на потоке M0, у которого есть P0. P1 свободен, а его поток M1 спит: делать нечего. Очередь готовых G у каждого P своя — это ' },
        term('локальная очередь (run queue)', 'run-queue'),
        { text: `, она вмещает ${LOCAL_QUEUE_CAP} горутин.` },
      ],
    },
    {
      state: frame('spawn', { m: 'M0', running: 'G1', runnext: 'G2' }, {}, [{ id: 'M0', state: 'running' }, { id: 'M1', state: 'idle' }]),
      caption: 'G1 выполняет go G2. Новая горутина не идёт в хвост очереди, а занимает особый слот runnext: она, скорее всего, связана с текущей (например, получит от неё данные), поэтому ей отдают следующий запуск на этом же P, пока кэш процессора тёплый.',
    },
    {
      state: frame('spawn', { m: 'M0', running: 'G1', runnext: 'G3', local: ['G2'] }, {}, [{ id: 'M0', state: 'running' }, { id: 'M1', state: 'idle' }]),
      caption: 'go G3. Слот runnext один, поэтому G2 вытесняется из него в хвост локальной очереди, а G3 занимает слот. Очередь работает по принципу FIFO, слот — «последний пришёл, первый ушёл».',
    },
    {
      state: frame('spawn', { m: 'M0', running: 'G1', runnext: 'G4', local: ['G2', 'G3'] }, {}, [{ id: 'M0', state: 'running' }, { id: 'M1', state: 'idle' }]),
      caption: 'go G4. Теперь в runnext стоит G4, а в очереди ждут G2 и G3. Работы у P0 много, у P1 — нет совсем: нагрузка перекошена, и пока P1 простаивает, процессорное ядро теряется впустую.',
    },
    {
      state: frame('steal', { m: 'M0', running: 'G1', runnext: 'G4', local: ['G3'] }, { m: 'M1', running: 'G2' }, [{ id: 'M0', state: 'running' }, { id: 'M1', state: 'running' }]),
      caption: [
        { text: 'Свободный P не ждёт, а ворует: M1 просыпается, получает P1 и ' },
        term('крадёт работу (work-stealing)', 'work-stealing'),
        { text: ' у соседа — половину его локальной очереди, с головы. Из двух готовых G забрана одна, G2, и сразу запущена. Очереди выровнялись: в P0 осталась G3 (и G4 в runnext), в P1 пусто. Слот runnext чужой P трогает в последнюю очередь, чтобы не рвать связку «создатель — созданная».' },
      ],
    },
    {
      state: frame('spawn', { m: 'M0', running: 'G1', runnext: 'G4', local: ['G3'] }, { m: 'M1', running: 'G5' }, [{ id: 'M0', state: 'running' }, { id: 'M1', state: 'running' }]),
      caption: 'G2 запустила go G5 и закончилась; G5 из runnext её P1 запускает следующей. Ничего необычного: на каждом P одновременно выполняется ровно одна горутина.',
    },
    {
      state: frame('syscall-handoff', { m: 'M0', running: 'G1', runnext: 'G4', local: ['G3'] }, { m: 'M2', running: null }, [{ id: 'M0', state: 'running' }, { id: 'M1', state: 'syscall' }, { id: 'M2', state: 'spinning' }]),
      caption: [
        { text: 'G5 вызывает блокирующий системный вызов (например, читает обычный файл). Поток M1 уходит в ядро и вместе с ним застряла G5. Пока ' },
        term('sysmon', 'sysmon'),
        { text: ' — фоновый поток без P — не заметил, P1 простаивает вместе с M1. На ближайшем проходе он видит P в системном вызове при том, что свободных P нет, отнимает P1 у M1 и отдаёт его потоку M2 (если свободного M нет, создаёт новый). Так блокировка одной горутины не отнимает ядро у остальных.' },
      ],
    },
    {
      state: frame('syscall-handoff', { m: 'M0', running: 'G1', runnext: 'G4' }, { m: 'M2', running: 'G3' }, [{ id: 'M0', state: 'running' }, { id: 'M1', state: 'syscall' }, { id: 'M2', state: 'running' }]),
      caption: 'M2 с P1 в руках осматривается, находит в очереди P0 горутину G3, крадёт её и запускает. Ядер два, и оба заняты: G1 на P0, G3 на P1. G5 по-прежнему ждёт ответа от ядра ОС на M1, к P она больше не привязана.',
    },
    {
      state: frame('preempt', { m: 'M0', running: 'G4' }, { m: 'M2', running: 'G3' }, [{ id: 'M0', state: 'running' }, { id: 'M1', state: 'syscall' }, { id: 'M2', state: 'running' }], ['G1']),
      caption: [
        { text: `G1 крутит цикл без вызовов функций дольше ${PREEMPT_AFTER_MS} мс, и остальные горутины её P ждут. Раньше такая G вешала планировщик: переключение происходило только в прологе функции. Теперь sysmon шлёт потоку сигнал SIGURG, обработчик сигнала прерывает G1 в безопасной точке — это ` },
        term('асинхронное вытеснение (preemption)', 'preemption'),
        { text: ' (Go 1.14+). G1 снята с P0 и поставлена в конец глобальной очереди; P0 берёт следующую — G4 из runnext.' },
      ],
    },
    {
      state: frame('syscall-return', { m: 'M0', running: 'G4' }, { m: 'M2', running: 'G3' }, [{ id: 'M0', state: 'running' }, { id: 'M1', state: 'idle' }, { id: 'M2', state: 'running' }], ['G1', 'G5']),
      caption: 'Системный вызов G5 завершился. M1 пытается вернуть себе P, но оба заняты. Тогда G5 кладётся в глобальную очередь (её просматривают, когда локальная пуста, и ещё раз в 61 тик планировщика, чтобы она не голодала), а M1, оставшись без P, уходит спать. Количество M в системе не ограничено, а количество P — ограничено: это и держит число одновременно работающих горутин на уровне GOMAXPROCS.',
    },
  ]
}
