package cases

// #region inline

// newPoint сама по себе возвращает указатель — её объект убегает. Но функция маленькая, и
// компилятор встраивает её в вызывающего: там видно, что указатель дальше не уходит.
func newPoint(x, y int) *point { return &point{x: x, y: y} }

// newPointNoinline — тот же код, встраивание запрещено директивой. Ключ варианта стоит и здесь:
// решение «в кучу» компилятор принимает при компиляции этой функции, и строка -m относится к ней.
//
// escape:inline/noinline
//
//go:noinline
func newPointNoinline(x, y int) *point { return &point{x: x, y: y} }

// CaseInlineInlined: после встраивания &point{} оказывается в этой функции и остаётся на стеке.
//
// escape:inline/inlined
func CaseInlineInlined(x, y int) int {
	p := newPoint(x, y)
	return p.x + p.y
}

// CaseInlineNoinline: newPointNoinline вызывается по-настоящему и возвращает указатель на объект
// в куче — решение принято при компиляции её самой, а не вызывающего.
//
// escape:inline/noinline
func CaseInlineNoinline(x, y int) int {
	p := newPointNoinline(x, y)
	return p.x + p.y
}

// #endregion inline
