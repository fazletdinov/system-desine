package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// OtherFrame — единственный кадр стека, в который MergeBelow сливает мелкие стеки.
const OtherFrame = "прочее"

// separator — строка между образцами в выводе -traces (cmd/vendor/.../pprof/internal/report,
// printTraces).
const separator = "-----------+-------------------------------------------------------"

// Profile — свёрнутые стеки одного типа выборки; в файл данных их пишет Encode.
type Profile struct {
	SampleType string // строка Type: из шапки pprof: cpu, alloc_space, inuse_space…
	Unit       string // nanoseconds, bytes или count — как sample_type в самом профиле
	Total      int64  // сумма значений всех стеков
	Stacks     []Stack
}

// Stack — один уникальный стек и сумма значений его образцов.
type Stack struct {
	Frames []string // от корня стека горутины к листу — функции, где взят образец
	Value  int64
}

// unit — единица значения в выводе pprof: во сколько базовых она и к какой базовой относится.
type unit struct {
	base   string
	factor float64
}

// units — единицы, которыми pprof подписывает значения (internal/measurement): время — в
// наносекунды, память — в байты, причём kB/MB у pprof двоичные (1 kB = 1024 B).
var units = map[string]unit{
	"ns":  {"nanoseconds", 1},
	"us":  {"nanoseconds", 1e3},
	"μs":  {"nanoseconds", 1e3},
	"ms":  {"nanoseconds", 1e6},
	"s":   {"nanoseconds", 1e9},
	"hrs": {"nanoseconds", 3600e9},
	"B":   {"bytes", 1},
	"kB":  {"bytes", 1 << 10},
	"MB":  {"bytes", 1 << 20},
	"GB":  {"bytes", 1 << 30},
	"TB":  {"bytes", 1 << 40},
	"PB":  {"bytes", 1 << 50},
	"":    {"count", 1},
}

var (
	// valueLine — первая строка стека: значение (до 10 знаков вправо) и три пробела перед листом.
	valueLine = regexp.MustCompile(`^\s*(-?[0-9]+(?:\.[0-9]+)?)(\S*)   (\S.*)$`)
	// labelLine — метка образца «ключ:  значение» перед стеком.
	labelLine = regexp.MustCompile(`^\s*\S+:  `)
)

// Parse читает вывод go tool pprof -traces и сворачивает его: одинаковые стеки
// суммируются, значения переводятся в базовую единицу, кадры идут от корня к листу.
func Parse(r io.Reader) (Profile, error) {
	p := parser{stacks: newStackSet()}
	sc := bufio.NewScanner(r)
	// Строка кадра — имя функции; с длинными дженериками она бывает больше 64 КиБ по умолчанию.
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for n := 1; sc.Scan(); n++ {
		if err := p.line(sc.Text()); err != nil {
			return Profile{}, fmt.Errorf("строка %d: %w", n, err)
		}
	}
	if err := sc.Err(); err != nil {
		return Profile{}, fmt.Errorf("чтение: %w", err)
	}
	if err := p.flush(); err != nil {
		return Profile{}, err
	}
	return p.profile()
}

// parser — состояние разбора: шапка, текущий образец и накопленные суммы стеков.
type parser struct {
	sampleType string
	inSamples  bool // прошли первый разделитель — дальше образцы, а не шапка
	unit       string

	value   float64
	hasVal  bool
	pending []string // кадры текущего образца от листа к корню, как их печатает pprof
	labels  bool     // у текущего образца были метки

	stacks *stackSet
}

func (p *parser) line(s string) error {
	if s == separator {
		if err := p.flush(); err != nil {
			return err
		}
		p.inSamples = true
		return nil
	}
	if !p.inSamples {
		if t, ok := strings.CutPrefix(s, "Type: "); ok {
			p.sampleType = strings.TrimSpace(t)
		}
		return nil
	}
	if strings.TrimSpace(s) == "" {
		return nil
	}
	if !p.hasVal {
		if labelLine.MatchString(s) {
			p.labels = true
			return nil
		}
		return p.first(s)
	}
	p.pending = append(p.pending, frameName(s))
	return nil
}

// first разбирает первую строку стека: значение образца и лист.
func (p *parser) first(s string) error {
	m := valueLine.FindStringSubmatch(s)
	if m == nil {
		return fmt.Errorf("ждали «значение   функция», получили %q", s)
	}
	num, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return fmt.Errorf("значение %q: %w", m[1], err)
	}
	u, ok := units[m[2]]
	if !ok {
		return fmt.Errorf("незнакомая единица %q в %q", m[2], s)
	}
	// Ноль pprof печатает без единицы — по нему тип значения не определить, и он ничего не весит.
	if num != 0 {
		if p.unit != "" && p.unit != u.base {
			return fmt.Errorf("единицы вперемешку: %s и %s", p.unit, u.base)
		}
		p.unit = u.base
	}
	p.value = num * u.factor
	p.hasVal = true
	p.pending = append(p.pending[:0], frameName(m[3]))
	return nil
}

// flush закрывает текущий образец: переворачивает стек и прибавляет значение к сумме.
func (p *parser) flush() error {
	defer func() { p.hasVal, p.labels, p.pending = false, false, p.pending[:0] }()
	if !p.hasVal {
		if p.labels {
			return errors.New("у образца есть метки, но нет стека")
		}
		return nil
	}
	v := int64(math.Round(p.value))
	if v == 0 {
		return nil
	}
	frames := slices.Clone(p.pending)
	slices.Reverse(frames)
	p.stacks.add(frames, v)
	return nil
}

func (p *parser) profile() (Profile, error) {
	if p.sampleType == "" {
		return Profile{}, errors.New("нет строки «Type:» в шапке — это не вывод go tool pprof -traces")
	}
	out := p.stacks.profile()
	out.SampleType, out.Unit = p.sampleType, p.unit
	if out.Unit == "" {
		out.Unit = units[""].base
	}
	return out, nil
}

// frameName — имя функции без отметки pprof о встраивании: для свёртки это та же функция.
func frameName(s string) string {
	return strings.TrimSuffix(strings.TrimSpace(s), " (inline)")
}

// MergeBelow сливает каждый стек, чья доля в Total меньше minShare, в узел OtherFrame под
// самым длинным его префиксом, который сам не тоньше порога: мелкие вызовы handle → …
// становятся блоком «прочее» над handle, а не в корне, и флейм-граф показывает, где они.
// Стек без единого такого префикса уходит в «прочее» корня. Если толще порога весь стек
// (его узел толстый за счёт детей), стек не меняется. Total не меняется.
func (p Profile) MergeBelow(minShare float64) (Profile, error) {
	if minShare < 0 || minShare >= 1 || math.IsNaN(minShare) {
		return Profile{}, fmt.Errorf("порог %v вне [0, 1)", minShare)
	}
	limit := minShare * float64(p.Total)
	// prefix — вес каждого префикса стека: значение узла флейм-графа с этими кадрами.
	prefix := map[string]int64{}
	for _, s := range p.Stacks {
		for d := 1; d <= len(s.Frames); d++ {
			prefix[strings.Join(s.Frames[:d], "\x00")] += s.Value
		}
	}
	merged := newStackSet()
	for _, s := range p.Stacks {
		if float64(s.Value) >= limit {
			merged.add(s.Frames, s.Value)
			continue
		}
		d := len(s.Frames)
		for d > 0 && float64(prefix[strings.Join(s.Frames[:d], "\x00")]) < limit {
			d--
		}
		if d == len(s.Frames) {
			merged.add(s.Frames, s.Value)
			continue
		}
		merged.add(append(slices.Clone(s.Frames[:d]), OtherFrame), s.Value)
	}
	out := merged.profile()
	out.SampleType, out.Unit = p.SampleType, p.Unit
	return out, nil
}

// stackSet складывает значения одинаковых стеков и выдаёт их в порядке кадров —
// так файл данных воспроизводим и читается в diff.
type stackSet struct {
	sums   map[string]int64
	frames map[string][]string
}

func newStackSet() *stackSet {
	return &stackSet{sums: map[string]int64{}, frames: map[string][]string{}}
}

func (s *stackSet) add(frames []string, v int64) {
	key := strings.Join(frames, "\x00")
	s.sums[key] += v
	s.frames[key] = frames
}

func (s *stackSet) profile() Profile {
	out := Profile{Stacks: []Stack{}}
	keys := make([]string, 0, len(s.sums))
	for k := range s.sums {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		out.Stacks = append(out.Stacks, Stack{Frames: s.frames[k], Value: s.sums[k]})
		out.Total += s.sums[k]
	}
	return out
}

// framesKey — ключ таблицы кадров в файле данных; профили лежат рядом под своим sampleType.
const framesKey = "frames"

type compactStack struct {
	F []int `json:"f"` // индексы в общей таблице кадров, от корня к листу
	V int64 `json:"v"`
}

type compactProfile struct {
	SampleType string         `json:"sampleType"`
	Unit       string         `json:"unit"`
	Total      int64          `json:"total"`
	Stacks     []compactStack `json:"stacks"`
}

// Encode пишет профили одним JSON-объектом с общей таблицей кадров:
//
//	{"frames": ["main.main", …], "cpu": {"sampleType": "cpu", "unit": …, "total": …, "stacks": [{"f": [0, 3, 7], "v": …}]}, …}
//
// Имя функции длиной в десятки знаков повторяется в сотнях стеков; индекс вместо имени
// уменьшает файл в несколько раз.
func Encode(w io.Writer, profiles []Profile) error {
	out := map[string]any{}
	var table []string
	index := map[string]int{}
	for _, p := range profiles {
		if p.SampleType == framesKey {
			return fmt.Errorf("тип выборки %q совпадает с ключом таблицы кадров", p.SampleType)
		}
		if _, dup := out[p.SampleType]; dup {
			return fmt.Errorf("профиль %s передан дважды", p.SampleType)
		}
		cp := compactProfile{SampleType: p.SampleType, Unit: p.Unit, Total: p.Total, Stacks: make([]compactStack, 0, len(p.Stacks))}
		for _, s := range p.Stacks {
			ids := make([]int, len(s.Frames))
			for i, f := range s.Frames {
				id, ok := index[f]
				if !ok {
					id = len(table)
					index[f] = id
					table = append(table, f)
				}
				ids[i] = id
			}
			cp.Stacks = append(cp.Stacks, compactStack{F: ids, V: s.Value})
		}
		out[p.SampleType] = cp
	}
	out[framesKey] = table
	if err := json.NewEncoder(w).Encode(out); err != nil {
		return fmt.Errorf("запись JSON: %w", err)
	}
	return nil
}
