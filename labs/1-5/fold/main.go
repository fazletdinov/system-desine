// Программа fold — свёртка профиля для песочницы flamegraph (урок 1.5.10): читает из stdin
// вывод go tool pprof -traces, складывает одинаковые стеки и пишет в stdout JSON
//
//	{"sampleType": "cpu", "unit": "nanoseconds", "total": …, "stacks": [{"frames": [корень … лист], "value": …}]}
//
// С -min стеки, чья доля меньше порога, сливаются в один стек «прочее» — так файл данных
// остаётся маленьким, а сумма не меняется. Профиль с /debug/pprof уже символизирован,
// бинарь pprof не нужен; -unit печатает значения целыми в базовой единице, без округления
// до сотых МБ/мс (fold понимает и масштабированные — ms, MB, kB):
//
//	go tool pprof -traces -unit=ns cpu.pprof | go run ./1-5/fold -min 0.001 > cpu.json
//	go tool pprof -traces -unit=B -sample_index=alloc_space heap.pprof | go run ./1-5/fold -min 0.001
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	minShare := flag.Float64("min", 0, "слить в «прочее» стеки с долей меньше порога (0.001 = 0,1 %)")
	flag.Parse()
	if err := run(*minShare); err != nil {
		fmt.Fprintf(os.Stderr, "fold: %v\n", err)
		os.Exit(1)
	}
}

func run(minShare float64) error {
	p, err := Parse(os.Stdin)
	if err != nil {
		return fmt.Errorf("разбор -traces: %w", err)
	}
	if len(p.Stacks) == 0 {
		return fmt.Errorf("в профиле %s нет ни одного ненулевого стека", p.SampleType)
	}
	p, err = p.MergeBelow(minShare)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(os.Stdout).Encode(p); err != nil {
		return fmt.Errorf("запись JSON: %w", err)
	}
	return nil
}
