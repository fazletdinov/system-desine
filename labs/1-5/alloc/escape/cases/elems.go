package cases

// #region elems

// CaseElemsValues хранит в срезе сами значения: один массив на все элементы.
//
// escape:elems/values
func CaseElemsValues(n int) int {
	s := make([]point, 0, 8)
	for i := range n {
		s = append(s, point{x: i, y: i})
	}
	return len(s)
}

// CaseElemsPointers хранит указатели: каждый элемент создаётся в цикле и должен пережить свою
// итерацию, поэтому каждый — отдельный объект в куче (и отдельная работа для сборщика мусора).
//
// escape:elems/pointers
func CaseElemsPointers(n int) int {
	s := make([]*point, 0, 8)
	for i := range n {
		s = append(s, &point{x: i, y: i})
	}
	return len(s)
}

// #endregion elems
