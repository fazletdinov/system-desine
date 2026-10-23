// Программа mem измеряет задержку памяти для урока 0.4: случайное обращение
// (обход цепочки указателей) в L1, L2 и RAM и последовательное чтение 1 МБ из RAM.
//
//	go run ./0-4/mem -mode latency   # l1_random_ns, l2_random_ns, l2_512k_random_ns, ram_random_ns
//	go run ./0-4/mem -mode seq       # ram_seq_1mb_us
//
// Последние строки вывода — RESULT <ключ> <значение> <единица>: их разбирают scenario.sh и тесты курса.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
)

// Размеры подобраны под i5-12400 (L1d 48 КиБ, L2 1,25 МиБ на ядро, L3 18 МиБ) с запасом:
// 16 КиБ точно в L1; 256 КиБ — в L2, но уже не в L1; 256 МиБ — в 14 раз больше L3.
// L2 меряется на 256 КиБ, а не «почти на весь L2»: 1 МиБ на страницах по 4 КиБ — 256 страниц,
// это больше первого уровня TLB, а случайные адреса ещё и сталкиваются в наборах кэша и частично
// уходят в L3 — замер показал бы смесь L2, L3 и промахов TLB. 512 КиБ — промежуточная точка,
// на которой эта смесь уже начинает быть видна. На другой машине уровни те же, если L1 ≥ 32 КиБ,
// L2 ≥ 512 КиБ и L3 ≤ 64 МиБ.
const (
	l1Bytes    = 16 << 10
	l2Bytes    = 256 << 10
	l2BigBytes = 512 << 10
	ramBytes   = 256 << 20
)

// plan — сколько работы на замер. Полный прогон — секунды на каждую строку,
// проверочный (-check) — доли секунды: там нужен только порядок величины.
type plan struct {
	cacheSteps, ramSteps, seqChunks, rounds int
}

var (
	fullPlan  = plan{cacheSteps: 200_000_000, ramSteps: 20_000_000, seqChunks: 4096, rounds: 7}
	checkPlan = plan{cacheSteps: 5_000_000, ramSteps: 1_000_000, seqChunks: 256, rounds: 3}
)

func main() {
	mode := flag.String("mode", "latency", "latency — случайные обращения в L1, L2, RAM; seq — 1 МБ подряд из RAM")
	check := flag.Bool("check", false, "короткий прогон для check:labs: только порядок величины")
	flag.Parse()

	p := fullPlan
	if *check {
		p = checkPlan
	}
	// Горутина замера не переезжает между потоками ОС посреди обхода. Ядро процессора это не
	// закрепляет — его закрепляет scenario.sh через taskset -c 2: иначе планировщик ОС мог бы
	// перенести поток на другое ядро (у гибридных CPU — ещё и на E-ядро) с холодным L1 и L2.
	runtime.LockOSThread()

	switch *mode {
	case "latency":
		runLatency(p)
	case "seq":
		runSeq(p)
	default:
		fmt.Fprintf(os.Stderr, "неизвестный -mode %q: нужен latency или seq\n", *mode)
		os.Exit(2)
	}
}

func runLatency(p plan) {
	levels := []struct {
		key, name string
		bytes     int
		steps     int
	}{
		{"l1_random_ns", "L1 (16 КиБ)", l1Bytes, p.cacheSteps},
		{"l2_random_ns", "L2 (256 КиБ)", l2Bytes, p.cacheSteps},
		{"l2_512k_random_ns", "L2 (512 КиБ)", l2BigBytes, p.cacheSteps},
		{"ram_random_ns", "RAM (256 МиБ)", ramBytes, p.ramSteps},
	}
	results := make([]float64, len(levels))
	for i, l := range levels {
		nodes := newChain(l.bytes, uint64(i+1))
		results[i] = chaseLatency(nodes, l.steps, p.rounds)
		fmt.Printf("случайное обращение, %-14s %8.2f нс (медиана %d замеров по %d переходов)\n",
			l.name, results[i], p.rounds, l.steps)
	}
	for i, l := range levels {
		fmt.Printf("RESULT %s %.2f ns\n", l.key, results[i])
	}
}

func runSeq(p plan) {
	buf := newBuffer(ramBytes)
	us := seqLatency(buf, p.seqChunks, p.rounds)
	fmt.Printf("последовательное чтение 1 МБ из буфера 256 МиБ: %.1f мкс (%.1f ГБ/с; медиана %d замеров по %d МБ)\n",
		us, mib/us/1e3, p.rounds, p.seqChunks)
	fmt.Printf("RESULT ram_seq_1mb_us %.1f us\n", us)
}
