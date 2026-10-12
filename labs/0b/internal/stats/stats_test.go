package stats

import (
	"testing"
	"time"
)

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

func TestPercentile(t *testing.T) {
	sorted := []time.Duration{ms(1), ms(2), ms(3), ms(4), ms(5), ms(6), ms(7), ms(8), ms(9), ms(100)}
	tests := []struct {
		name string
		p    float64
		want time.Duration
	}{
		{"медиана — 5-е значение по nearest-rank", 50, ms(5)},
		{"p90 — 9-е значение", 90, ms(9)},
		{"p99 — последнее", 99, ms(100)},
		{"p0 — первое", 0, ms(1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Percentile(sorted, tt.p); got != tt.want {
				t.Fatalf("Percentile(%v) = %v, want %v", tt.p, got, tt.want)
			}
		})
	}
}

func TestPercentileEmpty(t *testing.T) {
	if got := Percentile(nil, 50); got != 0 {
		t.Fatalf("got %v, want 0", got)
	}
}

func TestSummaryDoesNotMutateInput(t *testing.T) {
	in := []time.Duration{ms(3), ms(1), ms(2)}
	s := Summary(in)
	if s.N != 3 || s.Min != ms(1) || s.Max != ms(3) || s.P50 != ms(2) {
		t.Fatalf("unexpected summary %+v", s)
	}
	if in[0] != ms(3) {
		t.Fatal("Summary must not sort the caller's slice")
	}
}
