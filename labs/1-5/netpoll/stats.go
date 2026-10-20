package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/metrics"
	"strconv"
	"strings"
	"syscall"
)

const (
	metricGoroutines = "/sched/goroutines:goroutines"
	metricStacks     = "/memory/classes/heap/stacks:bytes"
	metricObjects    = "/memory/classes/heap/objects:bytes"
	metricScanStack  = "/gc/scan/stack:bytes"

	// fdReserve — дескрипторы сверх соединений: stdin/stdout/stderr, слушающий сокет, epoll и
	// eventfd netpoller'а, файлы /proc при замере. 64 — с запасом: точное число зависит от версии Go.
	fdReserve = 64
)

// limitError — нехватка системного ресурса (дескрипторов, портов): main завершает программу с кодом 2,
// чтобы отличить «поднимите лимит» от ошибки в работе.
type limitError struct{ msg string }

func (e *limitError) Error() string { return e.msg }

// checkFDLimit сравнивает потребность в дескрипторах с мягким RLIMIT_NOFILE. Go при старте сам
// поднимает мягкий лимит до жёсткого (syscall/rlimit.go), поэтому здесь видно уже поднятое значение.
func checkFDLimit(conns int) error {
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
		return fmt.Errorf("чтение RLIMIT_NOFILE: %w", err)
	}
	need := uint64(conns) + fdReserve
	if need > lim.Cur {
		return &limitError{fmt.Sprintf("нужно %d дескрипторов, лимит %d: поднимите ulimit -n", need, lim.Cur)}
	}
	return nil
}

// parseVmRSS достаёт VmRSS (КиБ) из текста /proc/<pid>/status.
func parseVmRSS(status string) (int, error) {
	for line := range strings.SplitSeq(status, "\n") {
		rest, ok := strings.CutPrefix(line, "VmRSS:")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) != 2 || fields[1] != "kB" {
			return 0, fmt.Errorf("неожиданная строка VmRSS: %q", line)
		}
		kib, err := strconv.Atoi(fields[0])
		if err != nil {
			return 0, fmt.Errorf("разбор VmRSS: %w", err)
		}
		return kib, nil
	}
	return 0, errors.New("в status нет строки VmRSS")
}

// tcpStat — строка TCP из /proc/net/sockstat: сокеты в работе и память буферов ядра в страницах.
type tcpStat struct {
	line  string
	inUse int
	mem   int
}

// parseSockstatTCP разбирает строку вида «TCP: inuse 5 orphan 0 tw 0 alloc 7 mem 1».
func parseSockstatTCP(sockstat string) (tcpStat, error) {
	for line := range strings.SplitSeq(sockstat, "\n") {
		rest, ok := strings.CutPrefix(line, "TCP:")
		if !ok {
			continue
		}
		st := tcpStat{line: line, inUse: -1, mem: -1}
		fields := strings.Fields(rest)
		for i := 0; i+1 < len(fields); i += 2 {
			v, err := strconv.Atoi(fields[i+1])
			if err != nil {
				return tcpStat{}, fmt.Errorf("разбор %s в строке TCP: %w", fields[i], err)
			}
			switch fields[i] {
			case "inuse":
				st.inUse = v
			case "mem":
				st.mem = v
			}
		}
		if st.inUse < 0 || st.mem < 0 {
			return tcpStat{}, fmt.Errorf("в строке TCP нет inuse или mem: %q", line)
		}
		return st, nil
	}
	return tcpStat{}, errors.New("в sockstat нет строки TCP")
}

// snapshot — то, что сравнивается до и после приёма соединений.
type snapshot struct {
	rssKiB     int
	goroutines uint64
	stacks     uint64
	objects    uint64
	scanStack  uint64
	tcp        tcpStat
}

// #region snapshot
func takeSnapshot() (snapshot, error) {
	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return snapshot{}, fmt.Errorf("чтение status: %w", err)
	}
	rss, err := parseVmRSS(string(status))
	if err != nil {
		return snapshot{}, err
	}
	sockstat, err := os.ReadFile("/proc/net/sockstat")
	if err != nil {
		return snapshot{}, fmt.Errorf("чтение sockstat: %w", err)
	}
	tcp, err := parseSockstatTCP(string(sockstat))
	if err != nil {
		return snapshot{}, err
	}
	s := []metrics.Sample{{Name: metricGoroutines}, {Name: metricStacks}, {Name: metricObjects}, {Name: metricScanStack}}
	metrics.Read(s)
	return snapshot{
		rssKiB:     rss,
		goroutines: s[0].Value.Uint64(),
		stacks:     s[1].Value.Uint64(),
		objects:    s[2].Value.Uint64(),
		scanStack:  s[3].Value.Uint64(),
		tcp:        tcp,
	}, nil
}

// #endregion snapshot
