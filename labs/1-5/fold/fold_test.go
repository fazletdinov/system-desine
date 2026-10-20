package main

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"
)

// traces — вывод go tool pprof -traces в том виде, как его печатает pprof: шапка, затем
// образцы через разделитель, лист стека — первой строкой, с меткой значения перед ним.
// Первый и третий образцы — один стек (у третьего есть метка), второй делит с ними
// общий префикс main.main → main.handle.
const traces = `File: messenger
Type: cpu
Time: 2026-10-20 11:49:29 MSK
Duration: 20s, Total samples = 60ms ( 0.30%)
-----------+-------------------------------------------------------
      20ms   main.hash
             main.handle (inline)
             main.main
-----------+-------------------------------------------------------
      30ms   main.encode
             main.handle
             main.main
-----------+-------------------------------------------------------
      kind:  login
      10ms   main.hash
             main.handle
             main.main
-----------+-------------------------------------------------------
`

func TestParse(t *testing.T) {
	got, err := Parse(strings.NewReader(traces))
	if err != nil {
		t.Fatalf("Parse(): %v", err)
	}
	want := Profile{
		SampleType: "cpu",
		Unit:       "nanoseconds",
		Total:      60_000_000,
		Stacks: []Stack{
			{Frames: []string{"main.main", "main.handle", "main.encode"}, Value: 30_000_000},
			{Frames: []string{"main.main", "main.handle", "main.hash"}, Value: 30_000_000},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse() =\n%+v\nждали\n%+v", got, want)
	}
}

// block — вывод -traces с одним образцом заданного значения.
func block(value string) string {
	return "Type: alloc_space\n" +
		"-----------+-------------------------------------------------------\n" +
		"     bytes:  2kB\n" +
		"  " + value + "   main.alloc\n" +
		"             main.main\n" +
		"-----------+-------------------------------------------------------\n"
}

func TestParseUnits(t *testing.T) {
	tests := []struct {
		value    string
		unit     string
		want     int64
		noStacks bool
	}{
		{value: "1.50ms", unit: "nanoseconds", want: 1_500_000},
		{value: "2s", unit: "nanoseconds", want: 2_000_000_000},
		{value: "250us", unit: "nanoseconds", want: 250_000},
		{value: "250μs", unit: "nanoseconds", want: 250_000},
		{value: "700ns", unit: "nanoseconds", want: 700},
		{value: "512B", unit: "bytes", want: 512},
		{value: "516.01kB", unit: "bytes", want: 528394},
		{value: "1.16MB", unit: "bytes", want: 1216348},
		{value: "2GB", unit: "bytes", want: 2 << 30},
		{value: "42", unit: "count", want: 42},
		// pprof печатает нулевой образец как «0» без единицы: в свёртку он ничего не добавляет.
		{value: "0", unit: "count", noStacks: true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := Parse(strings.NewReader(block(tt.value)))
			if err != nil {
				t.Fatalf("Parse(): %v", err)
			}
			if got.Unit != tt.unit {
				t.Fatalf("Unit = %q, ждали %q", got.Unit, tt.unit)
			}
			if tt.noStacks {
				if len(got.Stacks) != 0 || got.Total != 0 {
					t.Fatalf("ждали пустую свёртку, получили %+v", got)
				}
				return
			}
			if len(got.Stacks) != 1 || got.Stacks[0].Value != tt.want || got.Total != tt.want {
				t.Fatalf("Parse() = %+v, ждали один стек и total %d", got, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	sep := "-----------+-------------------------------------------------------\n"
	tests := []struct {
		name  string
		input string
	}{
		{name: "нет шапки Type", input: sep + "      10ms   main.main\n" + sep},
		{name: "незнакомая единица", input: block("10parsecs")},
		{name: "не число", input: block("abcms")},
		{name: "время и байты вперемешку", input: "Type: cpu\n" + sep + "      10ms   main.a\n" + sep + "      10MB   main.b\n" + sep},
		{name: "образец без кадров", input: "Type: cpu\n" + sep + "      kind:  login\n" + sep},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := Parse(strings.NewReader(tt.input)); err == nil {
				t.Fatalf("Parse() = %+v, ждали ошибку", got)
			}
		})
	}
}

func TestMergeBelow(t *testing.T) {
	// Сумма 1000. main.main → main.handle (989) — толстая ветка; мелкие стеки под ней
	// сливаются в «прочее» под main.handle, а не в корень.
	p := Profile{
		SampleType: "cpu",
		Unit:       "nanoseconds",
		Total:      1000,
		Stacks: []Stack{
			{Frames: []string{"main.main", "main.cold", "main.a"}, Value: 8},
			{Frames: []string{"main.main", "main.handle"}, Value: 5},
			{Frames: []string{"main.main", "main.handle", "main.big"}, Value: 974},
			{Frames: []string{"main.main", "main.handle", "main.small"}, Value: 6},
			{Frames: []string{"main.main", "main.handle", "main.tiny"}, Value: 4},
			{Frames: []string{"runtime.gcBgMarkWorker", "runtime.scanobject"}, Value: 3},
		},
	}
	tests := []struct {
		name  string
		share float64
		want  []Stack
	}{
		{name: "порог 0 — без изменений", share: 0, want: p.Stacks},
		{
			name:  "порог 0,5 %: мелкий стек — в «прочее» под самым длинным толстым префиксом",
			share: 0.005,
			want: []Stack{
				{Frames: []string{"main.main", "main.cold", "main.a"}, Value: 8},
				{Frames: []string{"main.main", "main.handle"}, Value: 5},
				{Frames: []string{"main.main", "main.handle", "main.big"}, Value: 974},
				{Frames: []string{"main.main", "main.handle", "main.small"}, Value: 6},
				{Frames: []string{"main.main", "main.handle", OtherFrame}, Value: 4},
				// У стека нет ни одного префикса толще порога — только тогда «прочее» в корне.
				{Frames: []string{OtherFrame}, Value: 3},
			},
		},
		{
			name:  "порог 1 %: соседи сливаются в одно «прочее», собственное время толстого узла остаётся",
			share: 0.01,
			want: []Stack{
				// 5 < 10, но сам узел main.handle толще порога: стек не меняется.
				{Frames: []string{"main.main", "main.handle"}, Value: 5},
				{Frames: []string{"main.main", "main.handle", "main.big"}, Value: 974},
				{Frames: []string{"main.main", "main.handle", OtherFrame}, Value: 10},
				{Frames: []string{"main.main", OtherFrame}, Value: 8},
				{Frames: []string{OtherFrame}, Value: 3},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := p.MergeBelow(tt.share)
			if err != nil {
				t.Fatalf("MergeBelow(): %v", err)
			}
			if !reflect.DeepEqual(got.Stacks, tt.want) {
				t.Fatalf("Stacks =\n%+v\nждали\n%+v", got.Stacks, tt.want)
			}
			var sum int64
			for _, s := range got.Stacks {
				sum += s.Value
			}
			if got.Total != sum || got.Total != p.Total {
				t.Fatalf("Total = %d, сумма стеков %d, было %d — должны совпадать", got.Total, sum, p.Total)
			}
		})
	}
}

func TestEncode(t *testing.T) {
	cpu := Profile{SampleType: "cpu", Unit: "nanoseconds", Total: 30, Stacks: []Stack{
		{Frames: []string{"main.main", "main.a"}, Value: 10},
		{Frames: []string{"main.main", "main.b"}, Value: 20},
	}}
	heap := Profile{SampleType: "inuse_space", Unit: "bytes", Total: 7, Stacks: []Stack{
		{Frames: []string{"main.main", "main.b"}, Value: 7},
	}}
	var buf bytes.Buffer
	if err := Encode(&buf, []Profile{cpu, heap}); err != nil {
		t.Fatalf("Encode(): %v", err)
	}
	var got struct {
		Frames []string `json:"frames"`
		CPU    struct {
			SampleType string `json:"sampleType"`
			Unit       string `json:"unit"`
			Total      int64  `json:"total"`
			Stacks     []struct {
				F []int `json:"f"`
				V int64 `json:"v"`
			} `json:"stacks"`
		} `json:"cpu"`
		Heap json.RawMessage `json:"inuse_space"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("разбор JSON: %v\n%s", err, buf.String())
	}
	// Таблица кадров общая: main.main и main.b встречаются в двух профилях, но записаны один раз.
	if want := []string{"main.main", "main.a", "main.b"}; !reflect.DeepEqual(got.Frames, want) {
		t.Fatalf("frames = %q, ждали %q", got.Frames, want)
	}
	if got.CPU.SampleType != "cpu" || got.CPU.Unit != "nanoseconds" || got.CPU.Total != 30 {
		t.Fatalf("шапка cpu = %+v", got.CPU)
	}
	var stacks [][]string
	for _, s := range got.CPU.Stacks {
		var names []string
		for _, i := range s.F {
			names = append(names, got.Frames[i])
		}
		stacks = append(stacks, names)
	}
	if want := [][]string{{"main.main", "main.a"}, {"main.main", "main.b"}}; !reflect.DeepEqual(stacks, want) {
		t.Fatalf("стеки cpu = %q, ждали %q", stacks, want)
	}
	if len(got.Heap) == 0 {
		t.Fatal("нет профиля inuse_space")
	}
}

func TestEncodeRejectsDuplicateSampleType(t *testing.T) {
	p := Profile{SampleType: "cpu", Unit: "nanoseconds"}
	for _, ps := range [][]Profile{{p, p}, {{SampleType: "frames"}}} {
		if err := Encode(io.Discard, ps); err == nil {
			t.Errorf("Encode(%+v): ждали ошибку", ps)
		}
	}
}

func TestMergeBelowRejectsBadThreshold(t *testing.T) {
	for _, share := range []float64{-0.1, 1, 2} {
		if _, err := (Profile{}).MergeBelow(share); err == nil {
			t.Errorf("MergeBelow(%v): ждали ошибку", share)
		}
	}
}
