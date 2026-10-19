//go:build labdemo

// Программа iface показывает гонку за переменную-интерфейс: значение интерфейса — это пара
// из двух слов (тип и данные), и запись пары не атомарна. Читатель может увидеть тип
// одного значения и данные другого.
package main

import (
	"fmt"
	"os"
	"sync"

	"labs/1-5/labenv"
)

const writes = 20_000_000

type speaker interface{ Speak() int }

// small хранится в интерфейсе как указатель на число в памяти; метод берёт число по указателю.
type small int

func (s small) Speak() int { return int(s) }

// callback хранится в интерфейсе самим указателем на замыкание; метод по этому указателю
// читает адрес кода и прыгает на него.
type callback func() int

func (c callback) Speak() int { return c() }

// #region iface
var shared speaker = small(1)

func two() int { return 2 }

// set вынесена из цикла, чтобы компилятор не выбросил первую из двух записей подряд как мёртвую.
//
//go:noinline
func set(v speaker) { shared = v }

func main() {
	labenv.Print(os.Stdout)

	var wg sync.WaitGroup
	wg.Add(3)
	// Каждый писатель сам чередует оба типа: так пара слов меняется, пока читатель
	// её читает, даже когда на машине всего два ядра.
	writer := func() {
		defer wg.Done()
		for range writes {
			set(small(1))      // (itab small, указатель на число 1)
			set(callback(two)) // (itab callback, указатель на замыкание)
		}
	}
	go writer()
	go writer()
	go func() { // читатель: itab из одного слова, данные из другого — callback с указателем на число 1
		defer wg.Done()
		sum := 0
		for range writes {
			sum += shared.Speak()
		}
		fmt.Println("читатель дожил до конца, сумма", sum)
	}()
	wg.Wait()
	fmt.Println("гонка не сломала программу в этот раз")
}

// #endregion iface
