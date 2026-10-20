package cases

// #region return

// CaseReturnValue создаёт point через new, но наружу отдаёт копию: сам объект вызов не переживает.
//
// escape:return/value
func CaseReturnValue(x, y int) point {
	p := new(point)
	p.x, p.y = x, y
	return *p
}

// CaseReturnPointer — та же функция, но возвращает указатель: объект нужен после выхода из
// функции, а её кадр стека к тому времени уже занят другими вызовами.
//
// escape:return/pointer
func CaseReturnPointer(x, y int) *point {
	p := new(point)
	p.x, p.y = x, y
	return p
}

// #endregion return
