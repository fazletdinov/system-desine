package cases

import "fmt"

// #region iface

// Sink — глобальная переменная-интерфейс: всё, что в неё положено, живёт сколько угодно долго.
var Sink any

func describe(p *point) string {
	if p.x > p.y {
		return "правее диагонали"
	}
	return "левее диагонали"
}

// CaseIfaceConcrete передаёт указатель в свою функцию: компилятор видит её тело и знает, что
// указатель не сохраняется.
//
// escape:iface/concrete
func CaseIfaceConcrete(x, y int) string {
	p := &point{x: x, y: y}
	return describe(p)
}

// CaseIfacePrintln передаёт тот же указатель в fmt.Println: параметр там — ...any, и fmt через
// рефлексию может сохранить значение, поэтому компилятор считает, что оно убегает.
//
// escape:iface/println
func CaseIfacePrintln(x, y int) {
	p := &point{x: x, y: y}
	fmt.Println(p)
}

// CaseIfaceAny сохраняет указатель в глобальную переменную-интерфейс.
//
// escape:iface/any
func CaseIfaceAny(x, y int) {
	p := &point{x: x, y: y}
	Sink = p
}

// #endregion iface
