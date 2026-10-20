package cases

import "testing"

var sinkInt int

// #region runtime-allocs

// TestDoesNotEscapeIsNotAlwaysStack проверяет при выполнении то, о чём молчит -gcflags=-m: у среза
// переменной длины «does not escape» значит лишь «на стеке есть буфер на 32 байта» (Go 1.25+), а у
// make([]point, 0, 8) — «на стеке массив на 8 элементов». Длиннее — аллокация в куче.
func TestDoesNotEscapeIsNotAlwaysStack(t *testing.T) {
	tests := []struct {
		name string
		f    func(int) int
		n    int
		want float64
	}{
		{"slice/var: 4 int = 32 байта помещаются в буфер", CaseSliceVar, 4, 0},
		{"slice/var: 5 int = 40 байт уже в куче", CaseSliceVar, 5, 1},
		{"elems/values: 8 точек помещаются в массив на стеке", CaseElemsValues, 8, 0},
		{"elems/values: девятая точка переносит массив в кучу", CaseElemsValues, 9, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := testing.AllocsPerRun(100, func() { sinkInt = tt.f(tt.n) })
			if got != tt.want {
				t.Errorf("аллокаций на вызов: %v, ждали %v", got, tt.want)
			}
		})
	}
}

// #endregion runtime-allocs
