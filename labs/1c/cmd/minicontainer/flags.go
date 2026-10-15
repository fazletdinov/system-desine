//go:build linux

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Лимиты — строки ровно в том виде, в каком их ждут файлы cgroup v2.
type limits struct {
	MemoryMax string // memory.max: байты
	PidsMax   string // pids.max: число задач (процессов и потоков)
	CPUMax    string // cpu.max: "<квота мкс> <период мкс>"
}

type runConfig struct {
	Hostname string
	Rootfs   string
	Argv     []string
	Limits   limits
}

const (
	// cpuPeriodUS — период планировщика CFS по умолчанию в ядре (100 мс): квота
	// «50%» — это 50 мс работы в каждые 100 мс.
	cpuPeriodUS = 100_000
	// maxCPUPercent — 1000 ядер; больше — почти наверняка опечатка.
	maxCPUPercent = 100_000
	// minMemory — одна страница: ядро округляет memory.max вниз до целых страниц, и
	// меньшее значение стало бы нулём. Заодно ловит забытый суффикс («--mem 64»).
	// Рабочий минимум выше (в лабе замерено: busybox sh — 1 МиБ, рантайм Go — 4 МиБ),
	// но маленький лимит — законный эксперимент: процесс получит OOM-kill.
	minMemory = 4 << 10
	// maxHostname — HOST_NAME_MAX в Linux.
	maxHostname = 64
)

var errUsage = errors.New("нужно: run [--mem 64M] [--pids 32] [--cpu 50%] [--hostname box] <rootfs> <команда> [аргументы…]")

func parseRun(args []string) (runConfig, error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	mem := fs.String("mem", "64M", "лимит памяти: 512K, 64M, 1G")
	pids := fs.Int("pids", 32, "лимит числа процессов и потоков")
	cpu := fs.String("cpu", "50%", "доля одного ядра: 50%, 150%")
	host := fs.String("hostname", "box", "имя хоста в контейнере")
	if err := fs.Parse(args); err != nil {
		return runConfig{}, fmt.Errorf("флаги: %w", err)
	}
	rest := fs.Args()
	if len(rest) < 2 {
		return runConfig{}, errUsage
	}
	memBytes, err := parseBytes(*mem)
	if err != nil {
		return runConfig{}, fmt.Errorf("--mem: %w", err)
	}
	cpuMax, err := parseCPU(*cpu)
	if err != nil {
		return runConfig{}, fmt.Errorf("--cpu: %w", err)
	}
	if *pids < 1 {
		return runConfig{}, fmt.Errorf("--pids: нужно не меньше 1, получено %d", *pids)
	}
	if *host == "" || len(*host) > maxHostname {
		return runConfig{}, fmt.Errorf("--hostname: длина от 1 до %d байт", maxHostname)
	}
	return runConfig{
		Hostname: *host,
		Rootfs:   rest[0],
		Argv:     rest[1:],
		Limits:   limits{MemoryMax: strconv.FormatInt(memBytes, 10), PidsMax: strconv.Itoa(*pids), CPUMax: cpuMax},
	}, nil
}

// parseBytes понимает двоичные суффиксы K, M, G (как docker run --memory).
func parseBytes(s string) (int64, error) {
	mult := int64(1)
	num := strings.ToUpper(s)
	switch {
	case strings.HasSuffix(num, "K"):
		mult, num = 1<<10, strings.TrimSuffix(num, "K")
	case strings.HasSuffix(num, "M"):
		mult, num = 1<<20, strings.TrimSuffix(num, "M")
	case strings.HasSuffix(num, "G"):
		mult, num = 1<<30, strings.TrimSuffix(num, "G")
	}
	n, err := strconv.ParseInt(num, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q — не размер: %w", s, err)
	}
	if n <= 0 || n > (1<<62)/mult {
		return 0, fmt.Errorf("%q — вне диапазона", s)
	}
	if n*mult < minMemory {
		return 0, fmt.Errorf("%q — меньше одной страницы (%d байт)", s, minMemory)
	}
	return n * mult, nil
}

func parseCPU(s string) (string, error) {
	pct, ok := strings.CutSuffix(s, "%")
	if !ok {
		return "", fmt.Errorf("%q — нужен процент, например 50%%", s)
	}
	n, err := strconv.Atoi(pct)
	if err != nil {
		return "", fmt.Errorf("%q — не число: %w", s, err)
	}
	if n < 1 || n > maxCPUPercent {
		return "", fmt.Errorf("%q — нужно от 1%% до %d%%", s, maxCPUPercent)
	}
	return fmt.Sprintf("%d %d", n*cpuPeriodUS/100, cpuPeriodUS), nil
}
