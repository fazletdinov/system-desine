package main

import (
	"bufio"
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

// Profile — свёрнутые стеки одного типа выборки: формат данных песочницы flamegraph.
type Profile struct {
	SampleType string  `json:"sampleType"` // строка Type: из шапки pprof: cpu, alloc_space, inuse_space…
	Unit       string  `json:"unit"`       // nanoseconds, bytes или count — как sample_type в самом профиле
	Total      int64   `json:"total"`      // сумма значений всех стеков
	Stacks     []Stack `json:"stacks"`
}

// Stack — один уникальный стек и сумма значений его образцов.
type Stack struct {
	Frames []string `json:"frames"` // от корня (main, goexit) к листу — функции, где взят образец
	Value  int64    `json:"value"`
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
	p := parser{sums: map[string]int64{}, frames: map[string][]string{}}
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

	sums   map[string]int64
	frames map[string][]string
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
	key := strings.Join(frames, "\x00")
	p.sums[key] += v
	p.frames[key] = frames
	return nil
}

func (p *parser) profile() (Profile, error) {
	if p.sampleType == "" {
		return Profile{}, errors.New("нет строки «Type:» в шапке — это не вывод go tool pprof -traces")
	}
	out := Profile{SampleType: p.sampleType, Unit: p.unit, Stacks: []Stack{}}
	if out.Unit == "" {
		out.Unit = units[""].base
	}
	keys := make([]string, 0, len(p.sums))
	for k := range p.sums {
		keys = append(keys, k)
	}
	// Порядок — по кадрам: так файл данных воспроизводим и читается в diff.
	slices.Sort(keys)
	for _, k := range keys {
		out.Stacks = append(out.Stacks, Stack{Frames: p.frames[k], Value: p.sums[k]})
		out.Total += p.sums[k]
	}
	return out, nil
}

// frameName — имя функции без отметки pprof о встраивании: для свёртки это та же функция.
func frameName(s string) string {
	return strings.TrimSuffix(strings.TrimSpace(s), " (inline)")
}

// MergeBelow сливает стеки, доля которых в Total меньше minShare, в один стек
// [OtherFrame] в конце списка. Total не меняется: вес мелких стеков не теряется.
func (p Profile) MergeBelow(minShare float64) (Profile, error) {
	if minShare < 0 || minShare >= 1 || math.IsNaN(minShare) {
		return Profile{}, fmt.Errorf("порог %v вне [0, 1)", minShare)
	}
	out := p
	out.Stacks = make([]Stack, 0, len(p.Stacks)+1)
	var other int64
	for _, s := range p.Stacks {
		if float64(s.Value) < minShare*float64(p.Total) {
			other += s.Value
			continue
		}
		out.Stacks = append(out.Stacks, s)
	}
	if other > 0 {
		out.Stacks = append(out.Stacks, Stack{Frames: []string{OtherFrame}, Value: other})
	}
	return out, nil
}
