// Программа fold — свёртка профилей для песочницы flamegraph (урок 1.5.10): читает выводы
// go tool pprof -traces (файлы из аргументов или один из stdin), складывает одинаковые стеки
// и пишет в stdout один JSON с общей таблицей кадров:
//
//	{"frames": ["net/http.(*conn).serve", …], "cpu": {"sampleType": "cpu", "unit": "nanoseconds", "total": …, "stacks": [{"f": [0, 4, 9], "v": …}]}, …}
//
// Кадры стека — индексы в frames от корня к листу. С -min стек, чья доля меньше порога,
// сливается в блок «прочее» под самым длинным своим префиксом, который толще порога, —
// файл остаётся маленьким, сумма не меняется. Профиль с /debug/pprof уже символизирован,
// бинарь pprof не нужен; -unit печатает значения целыми в базовой единице, без округления
// до сотых МБ/мс (fold понимает и масштабированные — ms, MB, kB):
//
//	go tool pprof -traces -unit=ns cpu.pprof > cpu.txt
//	go tool pprof -traces -unit=B -sample_index=alloc_space heap.pprof > alloc.txt
//	go tool pprof -traces -unit=B -sample_index=inuse_space heap.pprof > inuse.txt
//	go run ./1-5/fold -min 0.001 cpu.txt alloc.txt inuse.txt > flame-messenger.json
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	minShare := flag.Float64("min", 0, "слить в «прочее» стеки с долей меньше порога (0.001 = 0,1 %)")
	flag.Parse()
	if err := run(*minShare, flag.Args()); err != nil {
		fmt.Fprintf(os.Stderr, "fold: %v\n", err)
		os.Exit(1)
	}
}

func run(minShare float64, files []string) error {
	var profiles []Profile
	if len(files) == 0 {
		p, err := foldOne(os.Stdin, "stdin", minShare)
		if err != nil {
			return err
		}
		profiles = append(profiles, p)
	}
	for _, name := range files {
		p, err := foldFile(name, minShare)
		if err != nil {
			return err
		}
		profiles = append(profiles, p)
	}
	return Encode(os.Stdout, profiles)
}

func foldFile(name string, minShare float64) (Profile, error) {
	f, err := os.Open(name)
	if err != nil {
		return Profile{}, fmt.Errorf("открытие: %w", err)
	}
	// Файл только читаем: ошибка Close ничего не теряет.
	defer func() { _ = f.Close() }()
	return foldOne(f, name, minShare)
}

func foldOne(r io.Reader, name string, minShare float64) (Profile, error) {
	p, err := Parse(r)
	if err != nil {
		return Profile{}, fmt.Errorf("%s: разбор -traces: %w", name, err)
	}
	if len(p.Stacks) == 0 {
		return Profile{}, fmt.Errorf("%s: в профиле %s нет ни одного ненулевого стека", name, p.SampleType)
	}
	p, err = p.MergeBelow(minShare)
	if err != nil {
		return Profile{}, fmt.Errorf("%s: %w", name, err)
	}
	return p, nil
}
