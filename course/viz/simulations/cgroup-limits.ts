import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Step } from '../core/types'

export type CgroupVariant = 'memory' | 'cpu'

export interface MemoryFrame {
  kind: 'memory'
  usageMiB: number
  maxMiB: number
  reclaimed: boolean
  oomKilled: boolean
  events: { oom: number; oom_kill: number }
}

export interface CpuPeriod {
  used: number
  throttled: boolean
}

export interface CpuFrame {
  kind: 'cpu'
  quotaUs: number
  periodUs: number
  periods: CpuPeriod[]
  // cpu.stat показывается только в последнем кадре.
  stat?: { nr_periods: number; nr_throttled: number; throttled_usec: number }
}

export type CgroupFrame = MemoryFrame | CpuFrame

export const MEMORY_MAX_MIB = 64
export const CPU_QUOTA_US = 50_000
export const CPU_PERIOD_US = 100_000

// Доля процессорного времени, которую процесс реально получил.
export function cpuShare(f: CpuFrame): number {
  const used = f.periods.reduce((sum, p) => sum + p.used, 0)
  return used / (f.periods.length * f.periodUs)
}

const mem = (usageMiB: number, p: { reclaimed?: boolean; killed?: boolean; oom?: number }): MemoryFrame => ({
  kind: 'memory',
  usageMiB,
  maxMiB: MEMORY_MAX_MIB,
  reclaimed: p.reclaimed ?? false,
  oomKilled: p.killed ?? false,
  events: { oom: p.oom ?? 0, oom_kill: p.killed ? 1 : 0 },
})

const MEMORY: readonly Step<CgroupFrame>[] = [
  {
    state: mem(16, {}),
    caption: [
      { text: 'Лимит памяти — файл memory.max в ' },
      term('cgroup', 'cgroup'),
      { text: ' контейнера: здесь 64 МиБ. Процесс стартует и занимает 16 МиБ. Пока всё ниже лимита, ядро просто выдаёт страницы.' },
    ],
  },
  { state: mem(32, {}), caption: 'Процесс продолжает выделять память: 32 МиБ из 64. Счётчик memory.current растёт вместе с ним. Пока есть запас, ядро ничего не предпринимает.' },
  { state: mem(48, {}), caption: '48 МиБ из 64. Свободного места остаётся четверть. Если бы память росла медленнее, процесс мог бы так жить годами.' },
  { state: mem(60, {}), caption: '60 МиБ: до стены четыре. Ядро ещё не вмешивается, но следующее выделение может упереться в memory.max.' },
  {
    state: mem(60, { reclaimed: true }),
    caption: 'Не хватает страниц, и ядро сначала пытается освободить, что можно: вытеснить кэш страниц файлов. Но процесс держит анонимную память (кучу), её нельзя отдать без записи на диск, а подкачки в контейнере нет. Освобождать нечего.',
  },
  {
    state: mem(64, { reclaimed: true, oom: 1 }),
    caption: 'Лимит достигнут, освободить ничего не удалось: в memory.events счётчик oom стал 1. Ядро должно либо отказать в выделении, либо убить кого-то из этого cgroup. Для анонимной памяти отказать нельзя, остаётся убить.',
  },
  {
    state: { ...mem(0, { reclaimed: true, oom: 1, killed: true }) },
    caption: [
      { text: 'Сработал ' },
      term('oom-kill', 'oom-kill'),
      { text: ': ядро послало процессу SIGKILL, в memory.events oom_kill стал 1, а cgroup опустел. Docker увидит код выхода 137 (128 + 9, сигнал SIGKILL). Хост и соседние контейнеры продолжают работать: лимит защищает как раз их.' },
    ],
  },
]

const cpuPeriod: CpuPeriod = { used: CPU_QUOTA_US, throttled: true }

const cpuFrame = (n: number, withStat: boolean): CpuFrame => ({
  kind: 'cpu',
  quotaUs: CPU_QUOTA_US,
  periodUs: CPU_PERIOD_US,
  periods: Array.from({ length: n }, () => ({ ...cpuPeriod })),
  ...(withStat ? { stat: { nr_periods: n, nr_throttled: n, throttled_usec: n * (CPU_PERIOD_US - CPU_QUOTA_US) } } : {}),
})

const CPU: readonly Step<CgroupFrame>[] = [
  {
    state: cpuFrame(1, false),
    caption: [
      { text: 'Лимит CPU — файл cpu.max в ' },
      term('cgroup', 'cgroup'),
      { text: ': «50000 100000» значит 50 мс процессорного времени на каждые 100 мс. Процесс хочет 100 % одного ядра и в первом периоде честно работает свои 50 мс — квота кончилась на середине периода.' },
    ],
  },
  {
    state: cpuFrame(2, false),
    caption: [
      { text: 'Остаток первого периода процесс стоит: планировщик его не запускает, хотя ядро свободно. Это ' },
      term('CPU throttling', 'cpu-throttling'),
      { text: '. Потом начинается второй период, квота обновляется, процесс снова работает 50 мс и снова упирается в лимит.' },
    ],
  },
  { state: cpuFrame(3, false), caption: 'Третий период, то же самое: 50 мс работы, 50 мс ожидания. Процесс не убит и не получил ошибку; он просто стал в два раза медленнее. Для сервера это означает скачки задержки: запрос, пришедший во время паузы, ждёт до конца периода.' },
  { state: cpuFrame(4, false), caption: 'Четвёртый период. Заметьте: лимит считается за период, а не «в среднем за минуту». Короткий всплеск нагрузки упирается в стену, даже если в среднем процесс далёк от лимита.' },
  {
    state: cpuFrame(5, true),
    caption: 'Итог за пять периодов: процессорное время ровно половина от желаемого, 50 %. В файле cpu.stat это видно как nr_periods 5, nr_throttled 5 и throttled_usec 250000 (пять раз по 50 мс простоя). Если nr_throttled растёт быстрее, чем вы ожидали, лимит CPU слишком тесен.',
  },
]

export function cgroupScenario(variant: CgroupVariant = 'memory'): readonly Step<CgroupFrame>[] {
  return variant === 'memory' ? MEMORY : CPU
}
