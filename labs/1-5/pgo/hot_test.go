package main

import "testing"

func TestRoute(t *testing.T) {
	r := NewRouter()
	tests := []struct {
		line string
		want int
	}{
		{"text|8|привет", len("привет") + 8%7},
		{"join|5|", 5},
		{"leave|5|", -5},
		{"text|8|", -1},
		{"text|0|привет", -1},
		{"text|x|привет", -1},
		{"unknown|1|x", -1},
		{"без разделителей", -1},
	}
	for _, tt := range tests {
		if got := r.Route(tt.line); got != tt.want {
			t.Errorf("Route(%q) = %d, ждали %d", tt.line, got, tt.want)
		}
	}
}

// #region bench

// BenchmarkRoute — горячий путь для PGO: профиль этого бенчмарка становится default.pgo.
func BenchmarkRoute(b *testing.B) {
	r := NewRouter()
	lines := Workload(workloadSize)
	sum := 0
	for b.Loop() {
		for _, l := range lines {
			sum += r.Route(l)
		}
	}
	if sum == 0 { // результат используется: компилятор не выбросит работу цикла
		b.Fatal("пустой поток сообщений")
	}
}

// #endregion bench
