package main

import "time"

// mib — объём одного последовательного чтения: «1 МБ подряд из RAM» строки шкалы.
const mib = 1 << 20

// sum складывает слайс в четыре независимых аккумулятора: с одним каждое сложение
// ждало бы предыдущее, и замер упёрся бы в цепочку сложений, а не в память.
func sum(xs []uint64) uint64 {
	var a, b, c, d uint64
	i := 0
	for ; i+4 <= len(xs); i += 4 {
		a += xs[i]
		b += xs[i+1]
		c += xs[i+2]
		d += xs[i+3]
	}
	for ; i < len(xs); i++ {
		a += xs[i]
	}
	return a + b + c + d
}

// newBuffer выделяет буфер и пишет в каждую страницу: иначе первое чтение платило бы
// за page fault и обнуление страницы ядром, а не за чтение из памяти.
func newBuffer(sizeBytes int) []uint64 {
	buf := make([]uint64, sizeBytes/8)
	for i := range buf {
		buf[i] = uint64(i)
	}
	return buf
}

// seqLatency читает подряд по мегабайту, каждый раз следующий мегабайт буфера по кругу,
// и возвращает медиану из rounds замеров, мкс на 1 МБ. К повторному чтению того же
// мегабайта прочитано ещё len(buf)−1 МБ: при буфере больше L3 он давно вытеснен из кэша,
// и данные идут из RAM. Префетчер при этом работает — так и задумано: это чтение подряд.
func seqLatency(buf []uint64, chunks, rounds int) float64 {
	const perChunk = mib / 8
	nChunks := len(buf) / perChunk
	k := 0
	read := func(count int) {
		for range count {
			sink += sum(buf[k*perChunk : (k+1)*perChunk])
			k = (k + 1) % nChunks
		}
	}
	read(nChunks) // прогрев: один проход по всему буферу
	per := make([]float64, rounds)
	for r := range per {
		t0 := time.Now()
		read(chunks)
		per[r] = float64(time.Since(t0).Nanoseconds()) / 1e3 / float64(chunks)
	}
	return median(per)
}
