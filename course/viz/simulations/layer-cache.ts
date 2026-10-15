import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Simulation, Step } from '../core/types'

// Модель кэша BuildKit, достаточная для урока: ключ слоя = инструкция + ключ
// предыдущего слоя + (для COPY) содержимое скопированных файлов. Слой берётся из кэша,
// только если совпал ключ его и всех слоёв до него.

export type FileId = 'gomod' | 'source' | 'readme'
export type LayerId = 'base' | 'workdir' | 'copy-mod' | 'download' | 'copy-src' | 'build' | 'runtime' | 'copy-bin'
export type Order = 'correct' | 'naive'
export type LayerStatus = 'built' | 'cached'

export interface Layer {
  id: LayerId
  instruction: string
  stage: 'build' | 'runtime'
  status: LayerStatus
  seconds: number
}

export interface LayerState {
  order: Order
  dockerignore: boolean
  cacheMount: boolean
  multiStage: boolean
  versions: Readonly<Record<FileId, number>> // номер правки каждого файла
  builtVersions: Readonly<Record<FileId, number>> // версии файлов на момент последней сборки
  cache: ReadonlySet<string> // ключи слоёв, уже лежащих в кэше
  lastBuild: readonly Layer[]
  imageMB: number
  builds: number
  last: LayerAction['type'] | 'start'
}

export type LayerAction =
  | { type: 'build' }
  | { type: 'edit'; file: FileId }
  | { type: 'order'; order: Order }
  | { type: 'dockerignore'; on: boolean }
  | { type: 'cacheMount'; on: boolean }
  | { type: 'stage'; multi: boolean }
  | { type: 'reset' }

interface Instr {
  id: LayerId
  text: string
  stage: 'build' | 'runtime'
  copies: readonly FileId[] // какие файлы входят в ключ слоя
  seconds: number // время сборки шага без кэша
  mb: number // вклад в размер, если слой попадёт в итоговый образ
}

// Время и размеры — порядок величин для Go-сервиса средних размеров, не замер.
const BASE: Instr = { id: 'base', text: 'FROM golang:1.26 AS build', stage: 'build', copies: [], seconds: 20, mb: 850 }
const WORKDIR: Instr = { id: 'workdir', text: 'WORKDIR /src', stage: 'build', copies: [], seconds: 0.1, mb: 0 }
const COPY_MOD: Instr = { id: 'copy-mod', text: 'COPY go.mod go.sum ./', stage: 'build', copies: ['gomod'], seconds: 0.1, mb: 0 }
const DOWNLOAD: Instr = { id: 'download', text: 'RUN go mod download', stage: 'build', copies: [], seconds: 25, mb: 120 }
const BUILD: Instr = { id: 'build', text: 'RUN go build -o /out/messenger ./cmd/messenger', stage: 'build', copies: [], seconds: 40, mb: 300 }
const RUNTIME: Instr = { id: 'runtime', text: 'FROM gcr.io/distroless/static-debian13:nonroot', stage: 'runtime', copies: [], seconds: 3, mb: 2 }
const COPY_BIN: Instr = { id: 'copy-bin', text: 'COPY --from=build /out/messenger /messenger', stage: 'runtime', copies: [], seconds: 0.2, mb: 15 }

const copySrc = (ignore: boolean): Instr => ({
  id: 'copy-src',
  text: 'COPY . .',
  stage: 'build',
  copies: ignore ? ['gomod', 'source'] : ['gomod', 'source', 'readme'],
  seconds: 0.5,
  mb: 0,
})

function program(s: Pick<LayerState, 'order' | 'dockerignore' | 'multiStage'>): Instr[] {
  const src = copySrc(s.dockerignore)
  const build = s.order === 'correct' ? [BASE, WORKDIR, COPY_MOD, DOWNLOAD, src, BUILD] : [BASE, WORKDIR, src, DOWNLOAD, BUILD]
  return s.multiStage ? [...build, RUNTIME, COPY_BIN] : build
}

// Пересобранный go build с cache mount переиспользует кэш компилятора: пакеты, которые
// не менялись, не компилируются заново. Слой всё равно новый — меняется его результат.
const CACHE_MOUNT_SPEEDUP = 0.15

function build(s: LayerState): LayerState {
  const cache = new Set(s.cache)
  let parent = ''
  const layers: Layer[] = []
  for (const ins of program(s)) {
    // Новая стадия начинается со своего FROM: её ключ не зависит от предыдущей стадии.
    if (ins.id === 'runtime') parent = ''
    const copied = ins.copies.map((f) => `${f}@${s.versions[f]}`).join(',')
    // COPY --from сравнивает содержимое копируемого файла. Бинарь определяют go.mod и
    // исходники (сборка с -trimpath воспроизводима), а не то, пересобирался ли слой.
    const bin = ins.id === 'copy-bin' ? `|bin@${s.versions.gomod}.${s.versions.source}` : ''
    // Ключ включает ключ родителя: промах в любом слое выше даёт промах во всех ниже.
    const key = `${parent}|${ins.text}|${copied}${bin}`
    const hit = cache.has(key)
    cache.add(key)
    const seconds = hit ? 0 : ins.id === 'build' && s.cacheMount && s.builds > 0 ? ins.seconds * CACHE_MOUNT_SPEEDUP : ins.seconds
    layers.push({ id: ins.id, instruction: ins.text, stage: ins.stage, status: hit ? 'cached' : 'built', seconds })
    parent = key
  }
  const finalStage = s.multiStage ? program(s).filter((i) => i.stage === 'runtime') : program(s)
  const imageMB = Math.round(finalStage.reduce((sum, i) => sum + i.mb, 0))
  return { ...s, cache, builtVersions: s.versions, lastBuild: layers, imageMB, builds: s.builds + 1, last: 'build' }
}

const start = (): LayerState => ({
  order: 'correct',
  dockerignore: true,
  cacheMount: true,
  multiStage: true,
  versions: { gomod: 0, source: 0, readme: 0 },
  builtVersions: { gomod: 0, source: 0, readme: 0 },
  cache: new Set(),
  lastBuild: [],
  imageMB: 0,
  builds: 0,
  last: 'start',
})

export const layerCacheSimulation: Simulation<LayerState, LayerAction> = {
  initial: start,
  apply: (s, a) => {
    switch (a.type) {
      case 'reset':
        return start()
      case 'build':
        return build(s)
      case 'edit':
        return { ...s, versions: { ...s.versions, [a.file]: s.versions[a.file] + 1 }, last: 'edit' }
      case 'order':
        return { ...s, order: a.order, last: 'order' }
      case 'dockerignore':
        return { ...s, dockerignore: a.on, last: 'dockerignore' }
      case 'cacheMount':
        return { ...s, cacheMount: a.on, last: 'cacheMount' }
      case 'stage':
        return { ...s, multiStage: a.multi, last: 'stage' }
    }
  },
  steps: (s) => [{ state: s, caption: captionFor(s) } satisfies Step<LayerState>],
}

function captionFor(s: LayerState): Caption {
  if (s.lastBuild.length === 0)
    return [{ text: 'Соберите образ, поменяйте файл и соберите ещё раз: ' }, term('кэш сборки', 'build-cache'), { text: ' переиспользует слой, только если не изменились он сам и все слои до него.' }]
  const rebuilt = s.lastBuild.filter((l) => l.status === 'built')
  const total = s.lastBuild.reduce((sum, l) => sum + l.seconds, 0)
  if (rebuilt.length === 0) return [{ text: `Все слои взяты из ` }, term('кэша сборки', 'build-cache'), { text: `. Сборка заняла ~0 с, образ ${s.imageMB} МБ.` }]
  const first = rebuilt[0] as Layer
  return [
    { text: `Первый пересобранный слой — «${first.instruction}»; после него ` },
    term('кэш сборки', 'build-cache'),
    { text: ' не помогает и пересобирается всё. ' },
    { text: `Сборка ~${Math.round(total)} с, ` },
    term('образ', 'container-image'),
    { text: ` ${s.imageMB} МБ.` },
  ]
}
