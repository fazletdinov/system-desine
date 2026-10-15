import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Step } from '../core/types'

export type MemLimitVariant = 'without' | 'with'

export interface MemFrame {
  t: number
  heapLiveMiB: number
  heapGoalMiB: number
  rssMiB: number
  limitMiB: number
  gcRuns: number
  killed: boolean
}

// Лимит контейнера (memory.max) и мягкий лимит GC с запасом под не-кучу.
export const MEM_LIMIT_MIB = 256
export const GOMEMLIMIT_MIB = 230

// Параметры упрощённой модели: GOGC=100 и постоянный поток мусора.
const GOGC_FACTOR = 2
const OVERHEAD_OF_LIVE = 0.1
const ALLOC_MIB_PER_S = 120
const TICK_S = 2

// Живая куча по кадрам: рост до 120, плато, всплеск до 135 (пик нагрузки) и возврат.
const TIMELINE: readonly { t: number; live: number }[] = [
  { t: 0, live: 30 },
  { t: 2, live: 60 },
  { t: 4, live: 90 },
  { t: 6, live: 120 },
  { t: 8, live: 120 },
  { t: 10, live: 135 },
  { t: 12, live: 135 },
  { t: 14, live: 120 },
]

const goal = (live: number, withLimit: boolean): number => {
  const natural = live * GOGC_FACTOR
  return withLimit ? Math.min(natural, GOMEMLIMIT_MIB) : natural
}

function model(withLimit: boolean): MemFrame[] {
  let runs = 0
  const frames: MemFrame[] = []
  for (const { t, live } of TIMELINE) {
    const heapGoalMiB = goal(live, withLimit)
    // Сколько мусора помещается между сборками, столько циклов GC за такт.
    if (t > 0) runs += (ALLOC_MIB_PER_S * TICK_S) / (heapGoalMiB - live)
    const rssMiB = Math.round(heapGoalMiB + live * OVERHEAD_OF_LIVE)
    const killed = rssMiB > MEM_LIMIT_MIB
    frames.push({ t, heapLiveMiB: live, heapGoalMiB, rssMiB, limitMiB: MEM_LIMIT_MIB, gcRuns: Math.floor(runs), killed })
    if (killed) break
  }
  return frames
}

const WITHOUT = model(false)
const WITH = model(true)

const captionsWithout: readonly Caption[] = [
  'Упрощённая модель, числа круглые, но механизм настоящий. Контейнеру выдано 256 МиБ, GOMEMLIMIT не задан. Приложение только что стартовало: живая куча 30 МиБ. Сборщик мусора устроен так, что цель кучи — живые данные, умноженные на два (GOGC=100), а RSS (память процесса в ядре) идёт к этой цели плюс накладные расходы около 10 % от живой кучи.',
  'Живая куча растёт, цель GC растёт вместе с ней: 60 МиБ живых — цель 120. Go не знает, что вокруг есть стена в 256 МиБ: он видит только свой GOGC. Пока места много, это незаметно.',
  'Живая куча 90 МиБ, цель 180, RSS около 189. Запас до лимита ещё есть, GC запускается редко: между сборками умещается много мусора. Это эффективно, но память берётся «про запас».',
  'Живая куча достигла 120 МиБ и дальше держится. Цель GC — 240, RSS — 252 МиБ из 256. Вроде бы жить можно, но стена в четырёх мегабайтах, а все расчёты про «в среднем» держатся только пока нагрузка ровная.',
  'Плато. Процесс стабильно работает в 252 МиБ из 256. Любое отклонение вверх станет последним: у сборщика нет причин сдерживаться, потому что лимита он не видит.',
  [
    { text: 'Пик нагрузки: живая куча 135 МиБ (в обработке много больших запросов сразу). Цель GC — 270, больше самого лимита, RSS ~284. Ядро упирается в memory.max, и срабатывает ' },
    term('oom-kill', 'oom-kill'),
    { text: ': процесс убит, код выхода 137. Обратите внимание: живых данных всего 135 МиБ, то есть приложению памяти хватало, убила его не утечка, а политика «цель = живые × 2». Включите ' },
    term('GOMEMLIMIT', 'gomemlimit'),
    { text: ' в соседней вкладке.' },
  ],
]

const captionsWith: readonly Caption[] = [
  [
    { text: 'Упрощённая модель, числа круглые, но механизм настоящий. Тот же контейнер на 256 МиБ, но теперь ' },
    term('GOMEMLIMIT', 'gomemlimit'),
    { text: ' = 230 МиБ. Это мягкий лимит всей памяти Go-процесса: когда она подходит к нему, сборщик сам собирается чаще и не даёт цели вырасти. Запас в 26 МиБ нужен под всё, что рантайм не считает.' },
  ],
  'Пока живых данных мало, цель по-прежнему «живые × 2»: 60 МиБ живых — цель 120. Лимит ещё не работает, поведение такое же, как без него. Эта функция включается только у стены.',
  'Живая куча 90 МиБ, естественная цель была бы 180, ниже лимита, поэтому она не меняется. Разница с предыдущим вариантом появится, когда цель дорастёт до 230.',
  'Живая куча 120 МиБ. Естественная цель — 240, но лимит режет её до 230: сборщик собирает, не дожидаясь запаса. Мусора между сборками помещается меньше, поэтому GC запускается чаще: это цена за безопасность.',
  'Плато. RSS около 242 МиБ вместо 252: запас до стены увеличился, а счётчик запусков GC растёт чуть быстрее. Процессорного времени уходит немного больше, зато память под контролем.',
  [
    { text: 'Тот же пик нагрузки, 135 МиБ живых. Без лимита цель была бы 270 и ' },
    term('oom-kill', 'oom-kill'),
    { text: ' убил бы процесс. Сейчас цель по-прежнему 230, RSS ~244, процесс жив. GC работает чаще всего именно сейчас: запусков заметно больше, чем в варианте без лимита на той же секунде.' },
  ],
  'Пик продолжается. Цель так и остаётся 230, GC справляется, RSS держится ниже 256. Если бы живых данных стало больше 230, лимит бы не спас: GC начал бы собираться почти непрерывно («спираль смерти»), для этого Go ограничивает долю процессора, которую GC может занять.',
  'Нагрузка спала, живая куча вернулась к 120 МиБ. Процесс пережил пик без перезапуска. GOMEMLIMIT не заменяет лимит контейнера: он должен стоять ниже него, чтобы Go договаривался с самим собой раньше, чем вмешается ядро.',
]

function build(frames: MemFrame[], captions: readonly Caption[]): Step<MemFrame>[] {
  return frames.map((state, i) => ({ state, caption: captions[i] as Caption }))
}

export function gomemlimitScenario(variant: MemLimitVariant = 'without'): readonly Step<MemFrame>[] {
  return variant === 'without' ? build(WITHOUT, captionsWithout) : build(WITH, captionsWith)
}
