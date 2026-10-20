import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Step } from '../core/types'

export type TricolorVariant = 'no-barrier' | 'barrier'
export type Color = 'white' | 'grey' | 'black'
export type ObjId = 'A' | 'B' | 'C' | 'D' | 'E' | 'F' | 'G' | 'X'
// ROOTS — корни: глобальные переменные и стеки горутин.
export type NodeId = ObjId | 'ROOTS'

export interface TriObject {
  id: ObjId
  color: Color
  freed: boolean
}

export interface TriEdge {
  from: NodeId
  to: ObjId
}

export interface TriWrite {
  from: ObjId // объект, в чьё поле пишет мутатор
  field: string
  to: ObjId | null // новое значение поля; null — указатель стёрт
  old: ObjId | null // прежнее значение поля
  shaded: readonly ObjId[] // что затенил барьер записи
}

export interface TricolorFrame {
  phase: 'off' | 'mark' | 'sweep'
  barrier: boolean
  objects: readonly TriObject[]
  edges: readonly TriEdge[]
  scanning: ObjId | null
  write: TriWrite | null
}

const IDS: readonly ObjId[] = ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'X']
// X ни от чего не достижим — это мусор. F держит только серый B, пока мутатор его не переложит.
const INITIAL_EDGES: readonly TriEdge[] = [
  { from: 'ROOTS', to: 'A' },
  { from: 'ROOTS', to: 'B' },
  { from: 'A', to: 'C' },
  { from: 'B', to: 'D' },
  { from: 'B', to: 'F' },
  { from: 'D', to: 'E' },
  { from: 'F', to: 'G' },
]

export function reachable(f: TricolorFrame): Set<ObjId> {
  const seen = new Set<ObjId>()
  const queue: NodeId[] = ['ROOTS']
  while (queue.length > 0) {
    const cur = queue.shift() as NodeId
    for (const e of f.edges) {
      if (e.from !== cur || seen.has(e.to)) continue
      seen.add(e.to)
      queue.push(e.to)
    }
  }
  return seen
}

// Рёбра, нарушающие сильный трёхцветный инвариант: чёрный объект указывает на белый.
export function blackToWhite(f: TricolorFrame): TriEdge[] {
  const color = new Map(f.objects.map((o) => [o.id, o.color]))
  return f.edges.filter((e) => e.from !== 'ROOTS' && color.get(e.from) === 'black' && color.get(e.to) === 'white')
}

// Сценарий пишется как последовательность мутаций рабочей копии; каждый кадр — полный снимок.
class Builder {
  private colors = new Map<ObjId, Color>(IDS.map((id) => [id, 'white']))
  private freed = new Set<ObjId>()
  private edges: TriEdge[] = [...INITIAL_EDGES]
  readonly steps: Step<TricolorFrame>[] = []

  constructor(private readonly barrier: boolean) {}

  color(id: ObjId, c: Color): this {
    this.colors.set(id, c)
    return this
  }

  // Затенить: белый становится серым, остальные не меняются.
  shade(id: ObjId): this {
    if (this.colors.get(id) === 'white') this.colors.set(id, 'grey')
    return this
  }

  // Просканировать серый объект: его белые дети — серые, сам он — чёрный.
  scan(id: ObjId): this {
    for (const e of this.edges) if (e.from === id) this.shade(e.to)
    return this.color(id, 'black')
  }

  link(from: ObjId, to: ObjId): this {
    this.edges.push({ from, to })
    return this
  }

  unlink(from: ObjId, to: ObjId): this {
    this.edges = this.edges.filter((e) => !(e.from === from && e.to === to))
    return this
  }

  sweep(): this {
    for (const [id, c] of this.colors) if (c === 'white') this.freed.add(id)
    return this
  }

  snap(phase: TricolorFrame['phase'], caption: Caption, extra: { scanning?: ObjId; write?: TriWrite } = {}): this {
    this.steps.push({
      state: {
        phase,
        barrier: this.barrier,
        objects: IDS.map((id) => ({ id, color: this.colors.get(id) as Color, freed: this.freed.has(id) })),
        edges: [...this.edges],
        scanning: extra.scanning ?? null,
        write: extra.write ?? null,
      },
      caption,
    })
    return this
  }
}

const TRICOLOR = term('трёхцветная разметка (tricolor marking)', 'tricolor-marking')
const BARRIER = term('барьер записи (write barrier)', 'write-barrier')
const STW = term('остановка мира (stop-the-world)', 'stop-the-world')
const GC = term('Сборщик мусора', 'garbage-collector')

export function tricolorScenario(variant: TricolorVariant): readonly Step<TricolorFrame>[] {
  const barrier = variant === 'barrier'
  const b = new Builder(barrier)

  b.snap('off', [
    { text: 'Куча программы: восемь объектов и указатели между ними. Корни — глобальные переменные и стеки горутин. ' },
    GC,
    { text: ' Go начинает цикл с короткой ' },
    STW,
    { text: barrier ? ': на доли миллисекунды все горутины останавливаются, чтобы включить ' : ': на доли миллисекунды все горутины останавливаются. В этом варианте представим, что Go не включает ' },
    BARRIER,
    { text: barrier ? '. Дальше разметка идёт параллельно с программой.' : ' — посмотрим, к чему это приведёт.' },
  ])
  b.shade('A').shade('B').snap('mark', [
    TRICOLOR,
    { text: ': белые объекты ещё не найдены, серые найдены, но их поля не просмотрены, чёрные просмотрены целиком. Сборщик затеняет всё, на что указывают корни: A и B серые. Стек горутины-мутатора, которая сейчас работает, сборщик просканирует позже.' },
  ])
  b.scan('A').snap('mark', 'Сборщик берёт серый A и просматривает его поля: C становится серым, A — чёрным. Чёрный объект сборщик больше не посетит в этом цикле: всё, на что он указывает, считается уже найденным.', { scanning: 'A' })
  b.scan('C').snap('mark', 'C указателей не содержит и сразу становится чёрным. Разметка идёт параллельно с программой: пока сборщик работает, горутины-мутаторы продолжают менять указатели.', { scanning: 'C' })

  if (barrier) {
    b.link('A', 'F').shade('F').snap('mark', [
      { text: 'Мутатор прочитал из B указатель на F и записал его в поле A: A.next = F. Запись указателя в кучу проходит через ' },
      BARRIER,
      { text: ': гибридный барьер Go (с версии 1.8) затеняет новое значение, пока стек горутины не просканирован. F становится серым, инвариант «чёрный не указывает на белый» сохранён.' },
    ], { write: { from: 'A', field: 'next', to: 'F', old: null, shaded: ['F'] } })
    b.unlink('B', 'F').snap('mark', 'Мутатор стирает поле B: B.f = nil. Вторая половина гибридного барьера затеняет старое значение поля (удаляемый указатель): F уже серый, так что ничего не меняется. Сборщик всё равно узнает о F.', { write: { from: 'B', field: 'f', to: null, old: 'F', shaded: ['F'] } })
  } else {
    b.link('A', 'F').snap('mark', 'Мутатор прочитал из B указатель на F и записал его в поле A: A.next = F. Барьера нет, F остаётся белым. Чёрный A указывает на белый F — инвариант нарушен, но пока это не беда: F ещё достижим из серого B.', { write: { from: 'A', field: 'next', to: 'F', old: null, shaded: [] } })
    b.unlink('B', 'F').snap('mark', 'Мутатор стирает поле B: B.f = nil. Теперь F держит только чёрный A, а его сборщик больше не просматривает. Из серых объектов путь к F исчез: сборщик его не найдёт.', { write: { from: 'B', field: 'f', to: null, old: 'F', shaded: [] } })
  }

  b.scan('B').snap('mark', `Сборщик просматривает B: D становится серым, B — чёрным. Поля f у B уже нет${barrier ? ', но F серый и стоит в очереди сборщика' : ', поэтому F не затеняется'}.`, { scanning: 'B' })
  b.scan('D').snap('mark', 'Сборщик просматривает D: E становится серым, D — чёрным.', { scanning: 'D' })
  if (barrier) {
    b.scan('E').snap('mark', 'E указателей не содержит и становится чёрным. Серым остался F — тот самый, которого затенил барьер.', { scanning: 'E' })
    b.scan('F').snap('mark', 'Сборщик просматривает F: G становится серым, F — чёрным. Без барьера сюда он бы не попал.', { scanning: 'F' })
    b.scan('G').snap('mark', 'G просмотрен и стал чёрным. Серых объектов не осталось: всё достижимое найдено.', { scanning: 'G' })
  } else {
    b.scan('E').snap('mark', 'E указателей не содержит и становится чёрным. Серых объектов не осталось — сборщик считает, что нашёл всё достижимое. Но F и G достижимы через A и при этом белые.', { scanning: 'E' })
  }

  b.snap('sweep', [
    { text: 'Завершение разметки — снова короткая ' },
    STW,
    { text: barrier ? ': барьер выключается. Белым остался только X, на который никто не указывает: это мусор.' : '. Белыми остались X (настоящий мусор), а также F и G, которые программа продолжает использовать через A.' },
  ])
  b.sweep()
  if (barrier)
    b.snap('sweep', [
      { text: 'Уборка (sweep) освобождает белые объекты: только X. Все живые данные сохранены. Цена — ' },
      BARRIER,
      { text: ' на каждой записи указателя во время разметки: несколько лишних инструкций, но без него параллельная разметка невозможна.' },
    ])
  else
    b.snap('sweep', [
      { text: 'Уборка освобождает все белые объекты: X, F и G. F и G живые — освобождён живой объект. Следующее обращение A.next прочитает память, которую уже отдали под другие данные. Именно поэтому ' },
      BARRIER,
      { text: ' обязателен для параллельной разметки.' },
    ])
  return b.steps
}
