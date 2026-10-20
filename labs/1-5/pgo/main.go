// Программа pgo — горячий путь для урока о PGO (1.5.10): разбор и маршрутизация сообщений через
// интерфейс. Сравнение сборок без профиля и с профилем — бенчмарком из hot_test.go:
//
//	go test -run='^$' -bench=Route -cpuprofile=cpu.pprof -o pgo.test ./1-5/pgo/
//	cp cpu.pprof 1-5/pgo/default.pgo
//	go test -run='^$' -bench=Route -count=10 -pgo=off ./1-5/pgo/ > off.txt
//	go test -run='^$' -bench=Route -count=10 -pgo=auto ./1-5/pgo/ > pgo.txt
//	benchstat off.txt pgo.txt
//
// Сама программа гоняет тот же поток строк заданное время и печатает пропускную способность. С
// -cpuprofile она пишет свой профиль — так PGO применяют к сборке программы, а не тестов (символы в
// тестовом бинаре — labs/1-5/pgo.…, в программе — main.…):
//
//	go run ./1-5/pgo -cpuprofile 1-5/pgo/default.pgo
//	go build -pgo=auto -o pgo ./1-5/pgo
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	"labs/1-5/labenv"
)

// workloadSize — строк в потоке: достаточно, чтобы не уместиться в кэш предсказателя переходов.
const workloadSize = 10_000

func main() {
	dur := flag.Duration("dur", 2*time.Second, "сколько гонять поток")
	profile := flag.String("cpuprofile", "", "записать профиль CPU в файл (например, 1-5/pgo/default.pgo)")
	flag.Parse()
	labenv.Print(os.Stdout)
	if *profile != "" {
		stop, err := startProfile(*profile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "pgo: %v\n", err)
			os.Exit(1)
		}
		defer stop()
	}

	r := NewRouter()
	lines := Workload(workloadSize)
	start := time.Now()
	n, sum := 0, 0
	for time.Since(start) < *dur {
		for _, l := range lines {
			sum += r.Route(l)
		}
		n += len(lines)
	}
	el := time.Since(start)
	fmt.Printf("сообщений: %d за %s, %.1f млн/с, %.1f нс на сообщение (контрольная сумма %d)\n",
		n, el.Round(time.Millisecond), float64(n)/el.Seconds()/1e6, float64(el.Nanoseconds())/float64(n), sum)
}

// startProfile включает CPU-профилировщик; возвращённая функция останавливает его и закрывает файл.
func startProfile(path string) (func(), error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("создание профиля: %w", err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("запуск профилировщика: %w", err)
	}
	return func() {
		pprof.StopCPUProfile()
		if err := f.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "pgo: закрытие профиля: %v\n", err)
		}
	}, nil
}
