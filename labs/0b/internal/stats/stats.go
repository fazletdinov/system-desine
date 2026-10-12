// Package stats — статистика задержек для лабораторных.
package stats

import (
	"math"
	"slices"
	"time"
)

// Percentile — метод nearest-rank: наименьшее значение, ниже или равно которому доля p% измерений.
func Percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(math.Ceil(p / 100 * float64(len(sorted))))
	rank = max(1, min(rank, len(sorted)))
	return sorted[rank-1]
}

type Result struct {
	N                  int
	Min, P50, P99, Max time.Duration
}

// Summary не сортирует срез вызывающего: копируем, чтобы не было скрытых побочных эффектов.
func Summary(ds []time.Duration) Result {
	if len(ds) == 0 {
		return Result{}
	}
	sorted := slices.Clone(ds)
	slices.Sort(sorted)
	return Result{N: len(sorted), Min: sorted[0], P50: Percentile(sorted, 50), P99: Percentile(sorted, 99), Max: sorted[len(sorted)-1]}
}
