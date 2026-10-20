import type { Caption, CaptionSegment } from '../core/caption'
import { term } from '../core/caption'
import type { Simulation, Step } from '../core/types'
import rawRecords from '../data/escape-lab.json'

// Решение компилятора по варианту — запись JSON, которую печатает labs/1-5/alloc/escape/dump
// (go build -gcflags=-m=1). Каталог ниже знает только тексты для показа; решения берутся из JSON.
export interface CompilerRecord {
  case: string
  variant: string
  file: string
  line: number
  where: 'stack' | 'heap'
  compiler: string
}

// Для читателя три состояния: «does not escape» не всегда означает «всегда на стеке».
export type Verdict = 'stack' | 'heap' | 'stack-limited'

export interface Caveat {
  badge: string // текст плашки третьего состояния
  note: string
}

export interface VariantDef {
  id: string
  toggleText: string
  // Фрагменты исходника дословно из labs/1-5/alloc/escape/cases/*.go; последним идёт сама функция Case….
  source: readonly string[]
  rule: Caption
  caveat?: Caveat
}

export interface CaseDef {
  id: string
  title: string
  toggleLabel: string // одна деталь, которую переключаем
  variants: readonly VariantDef[]
}

const t = (text: string): CaptionSegment => ({ text })
const ESCAPE = term('Escape-анализ', 'escape-analysis')

const CAVEAT_SLICE_VAR: Caveat = {
  badge: 'стек до 32 Б, дальше куча',
  note: 'Компилятор пишет «does not escape», но с Go 1.25 это значит лишь: на стеке заведён буфер на 32 байта. Если срез в него помещается (здесь — до четырёх int), он на стеке; длиннее — make выделяет память в куче уже при выполнении.',
}

const CAVEAT_ELEMS_VALUES: Caveat = {
  badge: 'стек, пока n ≤ 8',
  note: 'Массив на 8 элементов лежит в кадре стека, пока в срез попадает не больше 8 точек (n ≤ 8). На девятом append срез растёт, и новый массив выделяется в куче.',
}

export const CASES: readonly CaseDef[] = [
  {
    id: 'chan',
    title: 'Отправка в канал',
    toggleLabel: 'Что отправляем в канал',
    variants: [
      {
        id: 'value',
        toggleText: 'значение (*p)',
        source: [`func CaseChanValue(ch chan<- point) {
	p := &point{x: 1, y: 2}
	ch <- *p
}`],
        rule: [ESCAPE, t(' видит, что в канал уходит копия: сам объект остаётся в кадре и на '), term('стеке', 'goroutine-stack'), t('.')],
      },
      {
        id: 'pointer',
        toggleText: 'указатель (p)',
        source: [`func CaseChanPointer(ch chan<- *point) {
	p := &point{x: 1, y: 2}
	ch <- p
}`],
        rule: [t('Получатель — другая горутина со своим '), term('стеком', 'goroutine-stack'), t(', и компилятор не знает, когда она прочитает объект, поэтому всё, что отправлено по указателю, лежит в '), term('куче', 'heap-allocation'), t('.')],
      },
    ],
  },
  {
    id: 'closure',
    title: 'Замыкание',
    toggleLabel: 'Что делаем с замыканием',
    variants: [
      {
        id: 'local',
        toggleText: 'передаём в функцию, которая его не сохраняет',
        source: [
          `//go:noinline
func repeat(k int, f func()) {
	for range k {
		f()
	}
}`,
          `func CaseClosureLocal(k int) int {
	n := 0
	repeat(k, func() { n++ })
	return n
}`,
        ],
        rule: [t('repeat не сохраняет f, поэтому замыкание и захваченная переменная n остаются в кадре '), term('стека', 'goroutine-stack'), t('.')],
      },
      {
        id: 'escaping',
        toggleText: 'возвращаем наружу',
        source: [`func CaseClosureEscaping() func() int {
	n := 0
	return func() int {
		n++
		return n
	}
}`],
        rule: [t('Замыкание и захваченная им n переживают вызов, а кадр стека к тому времени уже занят другими вызовами, поэтому n «moved to heap» — переезжает в '), term('кучу', 'heap-allocation'), t('.')],
      },
    ],
  },
  {
    id: 'elems',
    title: 'Элементы среза',
    toggleLabel: 'Что хранит срез',
    variants: [
      {
        id: 'values',
        toggleText: 'значения (point)',
        source: [`func CaseElemsValues(n int) int {
	s := make([]point, 0, 8)
	for i := range n {
		s = append(s, point{x: i, y: i})
	}
	return len(s)
}`],
        rule: [t('Срез хранит сами значения: один массив на все элементы, и он не покидает функцию.')],
        caveat: CAVEAT_ELEMS_VALUES,
      },
      {
        id: 'pointers',
        toggleText: 'указатели (*point)',
        source: [`func CaseElemsPointers(n int) int {
	s := make([]*point, 0, 8)
	for i := range n {
		s = append(s, &point{x: i, y: i})
	}
	return len(s)
}`],
        rule: [t('Сам массив указателей остаётся на стеке (пока элементов не больше восьми), но каждый '), term('&point{}', 'heap-allocation'), t(' передаётся в append, а escape-анализ считает добавляемые значения уходящими в кучу: при росте append скопирует их в новый массив в куче (-m=2: append(s, &point{...}) (call parameter) → {heap}). Каждый элемент — отдельный объект в куче и отдельная работа для сборщика мусора.')],
      },
    ],
  },
  {
    id: 'iface',
    title: 'Указатель и интерфейс',
    toggleLabel: 'Куда передаём указатель',
    variants: [
      {
        id: 'concrete',
        toggleText: 'в свою функцию',
        source: [
          `func describe(p *point) string {
	if p.x > p.y {
		return "правее диагонали"
	}
	return "левее диагонали"
}`,
          `func CaseIfaceConcrete(x, y int) string {
	p := &point{x: x, y: y}
	return describe(p)
}`,
        ],
        rule: [ESCAPE, t(' видит тело describe и знает, что указатель не сохраняется; к тому же вызов '), term('встраивается', 'inlining'), t('.')],
      },
      {
        id: 'println',
        toggleText: 'в fmt.Println',
        source: [`func CaseIfacePrintln(x, y int) {
	p := &point{x: x, y: y}
	fmt.Println(p)
}`],
        rule: [t('Параметр Println — ...any, и fmt через рефлексию может сохранить значение, поэтому компилятор считает, что оно убегает в '), term('кучу', 'heap-allocation'), t('.')],
      },
      {
        id: 'any',
        toggleText: 'в глобальную any',
        source: [`func CaseIfaceAny(x, y int) {
	p := &point{x: x, y: y}
	Sink = p
}`],
        rule: [t('Всё, что положено в глобальную переменную-интерфейс, может жить сколько угодно, поэтому объект обязан быть в '), term('куче', 'heap-allocation'), t('.')],
      },
    ],
  },
  {
    id: 'inline',
    title: 'Встраивание функции',
    toggleLabel: 'Встраивание newPoint',
    variants: [
      {
        id: 'inlined',
        toggleText: 'разрешено',
        source: [
          'func newPoint(x, y int) *point { return &point{x: x, y: y} }',
          `func CaseInlineInlined(x, y int) int {
	p := newPoint(x, y)
	return p.x + p.y
}`,
        ],
        rule: [t('Компилятор '), term('встраивает', 'inlining'), t(' newPoint в вызывающего: после этого видно, что указатель дальше не уходит, и объект остаётся на стеке.')],
      },
      {
        id: 'noinline',
        toggleText: 'запрещено (//go:noinline)',
        source: [
          `//go:noinline
func newPointNoinline(x, y int) *point { return &point{x: x, y: y} }`,
          `func CaseInlineNoinline(x, y int) int {
	p := newPointNoinline(x, y)
	return p.x + p.y
}`,
        ],
        rule: [t('Функция вызывается по-настоящему и возвращает указатель, а решение принимается при компиляции её самой: объект нужен после выхода, значит, он в '), term('куче', 'heap-allocation'), t('.')],
      },
    ],
  },
  {
    id: 'large',
    title: 'Размер объекта',
    toggleLabel: 'Размер массива в make',
    variants: [
      {
        id: 'fits',
        toggleText: '64 КиБ',
        source: [`func CaseLargeFits() byte {
	b := make([]byte, 64<<10)
	return b[len(b)-1]
}`],
        rule: [t('Для неявных переменных — make, new, &T{} — порог ir.MaxImplicitStackVarSize = 64 КиБ. Ровно 64 КиБ ещё помещаются в кадр стека. У явной «var a [N]byte» порог вдвое больше: ir.MaxStackVarSize = 128 КиБ.')],
      },
      {
        id: 'over',
        toggleText: '64 КиБ + 8 Б',
        source: [`func CaseLargeOver() byte {
	b := make([]byte, 64<<10+8)
	return b[len(b)-1]
}`],
        rule: [t('На 8 байт больше ir.MaxImplicitStackVarSize (64 КиБ для make, new, &T{}): тот же код, но массив уходит в '), term('кучу', 'heap-allocation'), t('. Явная «var a [N]byte» осталась бы на стеке до ir.MaxStackVarSize = 128 КиБ.')],
      },
    ],
  },
  {
    id: 'return',
    title: 'Возврат из функции',
    toggleLabel: 'Что возвращаем',
    variants: [
      {
        id: 'value',
        toggleText: 'копию (*p)',
        source: [`func CaseReturnValue(x, y int) point {
	p := new(point)
	p.x, p.y = x, y
	return *p
}`],
        rule: [t('Наружу отдаётся копия, сам объект вызов не переживает, поэтому new(point) остаётся на '), term('стеке', 'goroutine-stack'), t('.')],
      },
      {
        id: 'pointer',
        toggleText: 'указатель (p)',
        source: [`func CaseReturnPointer(x, y int) *point {
	p := new(point)
	p.x, p.y = x, y
	return p
}`],
        rule: [t('Объект нужен после выхода из функции, а её кадр стека к тому времени уже занят другими вызовами, поэтому он в '), term('куче', 'heap-allocation'), t('.')],
      },
    ],
  },
  {
    id: 'slice',
    title: 'Срез через make',
    toggleLabel: 'Размер и судьба среза',
    variants: [
      {
        id: 'const',
        toggleText: 'длина — константа',
        source: [
          `func sum(s []int) int {
	t := 0
	for i := range s {
		s[i] = i
		t += s[i]
	}
	return t
}`,
          `func CaseSliceConst() int {
	s := make([]int, 16)
	return sum(s)
}`,
        ],
        rule: [t('Длина известна при компиляции: 128 байт помещаются в кадр '), term('стека', 'goroutine-stack'), t(' (порог для make — MaxImplicitStackVarSize, 64 КиБ).')],
      },
      {
        id: 'var',
        toggleText: 'длина n известна при выполнении',
        source: [
          `func sum(s []int) int {
	t := 0
	for i := range s {
		s[i] = i
		t += s[i]
	}
	return t
}`,
          `func CaseSliceVar(n int) int {
	s := make([]int, n)
	return sum(s)
}`,
        ],
        rule: [t('Длина известна только при выполнении, и компилятор не может заранее сказать, поместится ли срез в кадр. Срез не покидает функцию, поэтому вердикт «does not escape»; что он значит на деле — в оговорке ниже.')],
        caveat: CAVEAT_SLICE_VAR,
      },
      {
        id: 'returned',
        toggleText: 'срез возвращается',
        source: [`func CaseSliceReturned(n int) []int {
	return make([]int, n)
}`],
        rule: [t('Срез возвращается из функции: его массив переживает вызов и всегда лежит в '), term('куче', 'heap-allocation'), t('.')],
      },
    ],
  },
]

const records: readonly CompilerRecord[] = rawRecords as CompilerRecord[]

export function recordFor(caseId: string, variantId: string): CompilerRecord {
  const r = records.find((x) => x.case === caseId && x.variant === variantId)
  if (r === undefined) throw new Error(`escape-lab: в JSON нет записи ${caseId}/${variantId}`)
  return r
}

const findVariant = (caseId: string, variantId: string): VariantDef | undefined =>
  CASES.find((c) => c.id === caseId)?.variants.find((v) => v.id === variantId)

export function verdictOf(caseId: string, variantId: string): Verdict {
  if (recordFor(caseId, variantId).where === 'heap') return 'heap'
  return findVariant(caseId, variantId)?.caveat === undefined ? 'stack' : 'stack-limited'
}

const HEAP_RE = /escapes to heap|moved to heap/
const STACK_RE = /does not escape/

// Строки вывода -m, на которых компилятор принял решение для выражения со строки line.
export function decisiveLines(r: CompilerRecord): string[] {
  const re = r.where === 'heap' ? HEAP_RE : STACK_RE
  return r.compiler.split('\n').filter((l) => l.startsWith(`${r.file}:${r.line}:`) && re.test(l))
}

export interface EscapeFrame {
  caseTitle: string
  toggleLabel: string
  source: string
  record: CompilerRecord
  verdict: Verdict
  compiler: string
  decisive: readonly string[]
  caveat: Caveat | null
}

export interface EscapeLabState {
  caseId: string
  variantId: string
}

export type EscapeLabAction =
  | { type: 'select-case'; caseId: string }
  | { type: 'select-variant'; variantId: string }
  | { type: 'reset' }

const first = CASES[0]!
const initialState: EscapeLabState = { caseId: first.id, variantId: first.variants[0]!.id }

const VERDICT_WORD: Record<Verdict, string> = {
  stack: 'объект на стеке',
  heap: 'объект в куче',
  'stack-limited': 'на стеке только пока всё помещается',
}

export const escapeLabSimulation: Simulation<EscapeLabState, EscapeLabAction, EscapeFrame> = {
  initial: () => initialState,
  apply(state, action) {
    switch (action.type) {
      case 'select-case': {
        const c = CASES.find((x) => x.id === action.caseId)
        return c === undefined ? state : { caseId: c.id, variantId: c.variants[0]!.id }
      }
      case 'select-variant':
        return findVariant(state.caseId, action.variantId) === undefined ? state : { ...state, variantId: action.variantId }
      case 'reset':
        return initialState
    }
  },
  steps(state): readonly Step<EscapeFrame>[] {
    const c = CASES.find((x) => x.id === state.caseId) ?? first
    const v = findVariant(c.id, state.variantId) ?? c.variants[0]!
    const record = recordFor(c.id, v.id)
    const verdict = verdictOf(c.id, v.id)
    const frame: EscapeFrame = {
      caseTitle: c.title,
      toggleLabel: c.toggleLabel,
      source: v.source.join('\n\n'),
      record,
      verdict,
      compiler: record.compiler,
      decisive: decisiveLines(record),
      caveat: v.caveat ?? null,
    }
    const caption: Caption = [ESCAPE, t(`: ${VERDICT_WORD[verdict]}. `), ...(typeof v.rule === 'string' ? [t(v.rule)] : v.rule)]
    return [{ state: frame, caption }]
  },
}
