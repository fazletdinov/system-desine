package cases

// #region slice

// CaseSliceConst — длина известна при компиляции: 128 байт помещаются в кадр стека.
//
// escape:slice/const
func CaseSliceConst() int {
	s := make([]int, 16)
	return sum(s)
}

// CaseSliceVar — длина известна только при выполнении. Компилятор пишет «does not escape», но это
// не значит «всегда на стеке»: с Go 1.25 он заводит на стеке буфер на 32 байта и берёт его, если
// срез туда помещается (здесь — до четырёх int); длинный срез выделяется в куче при выполнении.
//
// escape:slice/var
func CaseSliceVar(n int) int {
	s := make([]int, n)
	return sum(s)
}

// CaseSliceReturned — срез возвращается из функции: его массив переживает вызов и всегда в куче.
//
// escape:slice/returned
func CaseSliceReturned(n int) []int {
	return make([]int, n)
}

func sum(s []int) int {
	t := 0
	for i := range s {
		s[i] = i
		t += s[i]
	}
	return t
}

// #endregion slice
