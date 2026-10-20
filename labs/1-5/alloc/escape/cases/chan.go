package cases

// #region chan

// CaseChanValue отправляет в канал копию значения: сам объект остаётся в кадре.
//
// escape:chan/value
func CaseChanValue(ch chan<- point) {
	p := &point{x: 1, y: 2}
	ch <- *p
}

// CaseChanPointer отправляет указатель: получатель — другая горутина со своим стеком, и компилятор
// не знает, когда она его прочитает, поэтому всё отправленное по указателю — в куче.
//
// escape:chan/pointer
func CaseChanPointer(ch chan<- *point) {
	p := &point{x: 1, y: 2}
	ch <- p
}

// #endregion chan
