// Пакет alloc — бенчмарки цены аллокаций к уроку 1.5.8: пары «как обычно пишут» и «без лишних
// аллокаций». Запуск: go test -run='^$' -bench=. -count=10 ./1-5/alloc/ (сравнение — benchstat).
package alloc

import (
	"bytes"
	"io"
	"strconv"
	"sync"
	"testing"
)

// elems — размер «пачки» в бенчмарках: типичный ответ API или страница выборки из БД.
const elems = 1000

var (
	sinkInts []int
	sinkSum  int64
)

// #region append

// BenchmarkAppendGrow — срез растёт сам: append перевыделяет массив, когда кончается ёмкость
// (вдвое, а с 256 элементов — плавнее), и копирует старое содержимое.
func BenchmarkAppendGrow(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var s []int
		for i := range elems {
			s = append(s, i)
		}
		sinkInts = s
	}
}

// BenchmarkAppendPrealloc — ёмкость известна заранее: одна аллокация на весь срез.
func BenchmarkAppendPrealloc(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		s := make([]int, 0, elems)
		for i := range elems {
			s = append(s, i)
		}
		sinkInts = s
	}
}

// #endregion append

// #region pool

// render пишет в буфер ответ примерно на 1 КиБ — как сериализация небольшого JSON.
func render(buf *bytes.Buffer, id int) {
	buf.WriteString(`{"id":`)
	buf.WriteString(strconv.Itoa(id))
	buf.WriteString(`,"items":[`)
	for i := range 64 {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(`"элемент"`)
	}
	buf.WriteString("]}")
}

// BenchmarkBufferNew — новый буфер на каждый запрос: он растёт с нуля и уходит в кучу, потому что
// его содержимое отдаётся в io.Writer.
func BenchmarkBufferNew(b *testing.B) {
	b.ReportAllocs()
	id := 0
	for b.Loop() {
		var buf bytes.Buffer
		render(&buf, id)
		_, _ = io.Discard.Write(buf.Bytes())
		id++
	}
}

var bufPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

// BenchmarkBufferPool — буфер из sync.Pool: после разогрева его массив уже нужного размера.
func BenchmarkBufferPool(b *testing.B) {
	b.ReportAllocs()
	id := 0
	for b.Loop() {
		buf := bufPool.Get().(*bytes.Buffer)
		buf.Reset()
		render(buf, id)
		_, _ = io.Discard.Write(buf.Bytes())
		bufPool.Put(buf)
		id++
	}
}

// #endregion pool

// #region slice-of-pointers

type item struct {
	id    int64
	score int64
	_     [16]byte // место под имя: элемент размером с настоящую запись
}

// BenchmarkSlicePointers — []*item: каждый элемент — отдельный объект в куче, обход прыгает по
// памяти, а сборщику мусора есть что размечать.
func BenchmarkSlicePointers(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		s := make([]*item, 0, elems)
		for i := range elems {
			s = append(s, &item{id: int64(i), score: int64(i % 7)})
		}
		var total int64
		for _, it := range s {
			total += it.score
		}
		sinkSum = total
	}
}

// BenchmarkSliceValues — []item: один непрерывный массив без указателей. Здесь он к тому же не
// убегает и умещается в порог стека (32 КиБ < 64 КиБ), поэтому аллокаций нет вовсе.
func BenchmarkSliceValues(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		s := make([]item, 0, elems)
		for i := range elems {
			s = append(s, item{id: int64(i), score: int64(i % 7)})
		}
		var total int64
		for i := range s {
			total += s[i].score
		}
		sinkSum = total
	}
}

// #endregion slice-of-pointers
