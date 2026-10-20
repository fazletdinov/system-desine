package cases

// #region closure

// repeat вызывает f k раз. Встраивание запрещено, чтобы замыкание осталось замыканием: иначе
// компилятор встроит и repeat, и само замыкание, и строки о нём в выводе -m не будет (а n всё
// равно останется на стеке).
//
//go:noinline
func repeat(k int, f func()) {
	for range k {
		f()
	}
}

// CaseClosureLocal — замыкание передаётся в функцию, которая его не сохраняет: замыкание и
// захваченная переменная n остаются в кадре стека.
//
// escape:closure/local
func CaseClosureLocal(k int) int {
	n := 0
	repeat(k, func() { n++ })
	return n
}

// CaseClosureEscaping возвращает замыкание: оно и захваченная им переменная переживают вызов.
//
// escape:closure/escaping
func CaseClosureEscaping() func() int {
	n := 0
	return func() int {
		n++
		return n
	}
}

// #endregion closure
