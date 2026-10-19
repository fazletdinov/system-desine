//go:build labdemo

// Программа leak воспроизводит утечку горутин из release notes Go 1.26 и ловит её
// профилем goroutineleak (в Go 1.27 он общедоступен без GOEXPERIMENT).
package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"labs/1-5/labenv"
)

const (
	taskCount = 10
	failingAt = 2 // задача с этим номером возвращает ошибку
	// gcRounds — сколько раз прогнать GC: профиль утечек строится по достижимости,
	// и одной фазы не всегда хватает, чтобы пометить все заблокированные горутины.
	gcRounds = 3
	// workTime — работа успешных задач: ошибочная задача отвечает первой,
	// поэтому функция возвращается раньше, чем остальные отправят результат.
	workTime = 20 * time.Millisecond
)

var errTask = errors.New("задача не удалась")

type outcome struct {
	value int
	err   error
}

func process(n int) (int, error) {
	if n == failingAt {
		return 0, errTask
	}
	time.Sleep(workTime)
	return n * n, nil
}

// #region leak
// processWorkItems — функция из release notes Go 1.26. Канал небуферизованный, а при первой
// ошибке функция возвращается: остальные горутины навсегда застревают на отправке в ch,
// на который больше никто не ссылается.
func processWorkItems(items []int) ([]int, error) {
	ch := make(chan outcome)
	for _, n := range items {
		go func() {
			v, err := process(n)
			ch <- outcome{v, err}
		}()
	}

	var results []int
	for range items {
		r := <-ch
		if r.err != nil {
			return nil, r.err // ранний возврат: получателя у ch больше нет
		}
		results = append(results, r.value)
	}
	return results, nil
}

// #endregion leak

// #region leak-fix
// processWorkItemsFixed отличается одной строкой: буфер канала равен числу задач,
// поэтому каждая горутина отправляет результат и выходит, даже если читать уже некому.
// Для настоящих сервисов добавьте ctx и errgroup — их показывает мессенджер (урок 1.5.6).
func processWorkItemsFixed(items []int) ([]int, error) {
	ch := make(chan outcome, len(items))
	for _, n := range items {
		go func() {
			v, err := process(n)
			ch <- outcome{v, err}
		}()
	}

	var results []int
	for range items {
		r := <-ch
		if r.err != nil {
			return nil, r.err
		}
		results = append(results, r.value)
	}
	return results, nil
}

// #endregion leak-fix

func items() []int {
	out := make([]int, taskCount)
	for i := range out {
		out[i] = i
	}
	return out
}

// leakedProfile прогоняет GC, пишет профиль goroutineleak (debug=1) и возвращает
// его текст и число утёкших горутин из строки "goroutineleak profile: total N".
func leakedProfile() (text string, count int, err error) {
	time.Sleep(2 * workTime) // дать успешным задачам дойти до отправки в ch
	for range gcRounds {
		runtime.GC()
	}
	var buf bytes.Buffer
	if err := pprof.Lookup("goroutineleak").WriteTo(&buf, 1); err != nil {
		return "", 0, fmt.Errorf("запись профиля: %w", err)
	}
	header, _, _ := strings.Cut(buf.String(), "\n")
	total, ok := strings.CutPrefix(header, "goroutineleak profile: total ")
	if !ok {
		return "", 0, fmt.Errorf("неожиданный заголовок профиля: %q", header)
	}
	count, err = strconv.Atoi(total)
	if err != nil {
		return "", 0, fmt.Errorf("число в заголовке профиля: %w", err)
	}
	return buf.String(), count, nil
}

// report вызывает функцию и печатает, сколько горутин осталось заблокированными.
// Профиль накопительный, поэтому исправленную версию запускаем первой.
func report(name string, process func([]int) ([]int, error), withProfile bool) error {
	_, err := process(items())
	fmt.Printf("%s: ошибка %q\n", name, err)
	text, count, perr := leakedProfile()
	if perr != nil {
		return perr
	}
	fmt.Printf("утекло горутин: %d\n", count)
	if withProfile {
		fmt.Print(text)
	}
	return nil
}

func main() {
	labenv.Print(os.Stdout)

	if err := report("processWorkItemsFixed", processWorkItemsFixed, false); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := report("processWorkItems", processWorkItems, true); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
