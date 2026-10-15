import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Step } from '../core/types'

export type CiVariant = 'aggregate' | 'on-paths'
export type JobStatus = 'pending' | 'running' | 'success' | 'failure' | 'skipped'
export type PrState = 'waiting' | 'mergeable' | 'blocked'

export interface CiJob {
  id: string
  status: JobStatus
}

export interface CiFrame {
  jobs: CiJob[]
  required: string[]
  pr: PrState
}

// Джобы ci.yml; порядок = порядок слоёв графа: changes, ворота, ci-ok.
export const JOB_IDS = [
  'changes',
  'messenger-lint',
  'messenger-test',
  'messenger-generate',
  'messenger-vuln',
  'course',
  'labs',
  'image',
  'workflows',
  'ci-ok',
] as const

export const GATE_IDS: readonly string[] = JOB_IDS.filter((id) => id !== 'changes' && id !== 'ci-ok')
const MESSENGER_JOBS: readonly string[] = JOB_IDS.filter((id) => id.startsWith('messenger-'))

type Overrides = Partial<Record<(typeof JOB_IDS)[number], JobStatus>>

// Пропущенные джобы, когда PR трогает только course/**.
const SKIPPED: Overrides = {
  'messenger-lint': 'skipped',
  'messenger-test': 'skipped',
  'messenger-generate': 'skipped',
  'messenger-vuln': 'skipped',
  labs: 'skipped',
  image: 'skipped',
  workflows: 'skipped',
}

const frame = (o: Overrides, required: string[], pr: PrState): CiFrame => ({
  jobs: JOB_IDS.map((id) => ({ id, status: o[id] ?? 'pending' })),
  required,
  pr,
})

const AGGREGATE: readonly Step<CiFrame>[] = [
  {
    state: frame({}, ['ci-ok'], 'waiting'),
    caption: [
      { text: 'PR меняет один урок, файлы только в course/. В ' },
      term('монорепозитории', 'monorepo'),
      { text: ' живут и мессенджер, и курс, и лабы; запускать всё на каждую правку незачем. Workflow стартует всегда, а вот отдельные джобы (' },
      term('CI-джоба', 'ci-job'),
      { text: ') решают, нужны ли они. Обязательная проверка одна — ci-ok.' },
    ],
  },
  {
    state: frame({ changes: 'running' }, ['ci-ok'], 'waiting'),
    caption: 'Первой идёт джоба changes: dorny/paths-filter сверяет изменённые файлы PR с фильтрами и выдаёт флаги — что тронуто. От результата зависят все остальные джобы.',
  },
  {
    state: frame({ ...SKIPPED, changes: 'success', course: 'running' }, ['ci-ok'], 'waiting'),
    caption: 'changes сообщил: изменён только course. Джобы мессенджера, labs, image и workflows пропущены по условию if:. Джоба, пропущенная так, отчитывается в GitHub как Success, а не как «нет статуса». Запущена только course.',
  },
  {
    state: frame({ ...SKIPPED, changes: 'success', course: 'success' }, ['ci-ok'], 'waiting'),
    caption: 'Джоба course прошла. Все ворота либо успешны, либо пропущены, но PR ещё ждёт: обязательная проверка ci-ok не отчиталась.',
  },
  {
    state: frame({ ...SKIPPED, changes: 'success', course: 'success', 'ci-ok': 'running' }, ['ci-ok'], 'waiting'),
    caption: 'ci-ok стартует с условием always(), даже если часть зависимостей пропущена, и смотрит на их итоги: если среди них есть failure или cancelled, проваливается сам. Это и есть смысл агрегирующей джобы — одна стабильная точка вместо списка из девяти имён.',
  },
  {
    state: frame({ ...SKIPPED, changes: 'success', course: 'success', 'ci-ok': 'success' }, ['ci-ok'], 'mergeable'),
    caption: [
      { text: 'ci-ok зелёная, ' },
      term('обязательная проверка (required check)', 'required-check'),
      { text: ' выполнена, PR можно сливать. Добавите завтра десятую джобу или переименуете одну из них — настройки ветки менять не придётся: в защите стоит только имя ci-ok.' },
    ],
  },
]

const ON_PATHS_REQUIRED = [...MESSENGER_JOBS]

const ON_PATHS: readonly Step<CiFrame>[] = [
  {
    state: frame({}, ON_PATHS_REQUIRED, 'waiting'),
    caption: [
      { text: 'Другая схема: фильтр на уровне всего workflow, on.pull_request.paths: [\'messenger/**\'], а обязательными в защите ветки сделаны джобы мессенджера. PR меняет только урок. Такая схема для ' },
      term('монорепозитория', 'monorepo'),
      { text: ' выглядит логично, пока не дойдёт до слияния.' },
    ],
  },
  {
    state: frame({}, ON_PATHS_REQUIRED, 'waiting'),
    caption: [
      { text: 'Файлы PR не подходят под paths, поэтому workflow не запускается вообще. Ни одной ' },
      term('CI-джобы', 'ci-job'),
      { text: ' не создано, никто не отчитался. Это принципиально отличается от пропуска через if:: там джоба отчитывается как Success, а здесь отчитываться некому.' },
    ],
  },
  {
    state: frame({}, ON_PATHS_REQUIRED, 'waiting'),
    caption: [
      { text: 'Для GitHub ' },
      term('обязательные проверки (required checks)', 'required-check'),
      { text: ' остаются в состоянии Pending: «Expected — Waiting for status to be reported». PR невозможно слить, хотя ни одна проверка не упала и ничего не запущено. Вот почему в нашем ci.yml фильтр стоит на уровне джоб (paths-filter и if:), а не в on.paths.' },
    ],
  },
]

export function ciGraphScenario(variant: CiVariant = 'aggregate'): readonly Step<CiFrame>[] {
  return variant === 'aggregate' ? AGGREGATE : ON_PATHS
}
