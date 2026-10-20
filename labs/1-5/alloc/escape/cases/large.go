package cases

// #region large

// Порог для неявных переменных (new, &T{}, make) — ir.MaxImplicitStackVarSize = 64 КиБ
// (cmd/compile/internal/ir/cfg.go). У явной «var a [N]byte» порог вдвое больше —
// ir.MaxStackVarSize = 128 КиБ.

// CaseLargeFits — ровно 64 КиБ: ещё помещается в кадр стека.
//
// escape:large/fits
func CaseLargeFits() byte {
	b := make([]byte, 64<<10)
	return b[len(b)-1]
}

// CaseLargeOver — на 8 байт больше порога: тот же код, но массив уходит в кучу.
//
// escape:large/over
func CaseLargeOver() byte {
	b := make([]byte, 64<<10+8)
	return b[len(b)-1]
}

// #endregion large
