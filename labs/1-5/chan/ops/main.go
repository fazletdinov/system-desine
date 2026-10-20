// Программа ops строит таблицу: что делают send, recv, close, len и cap над nil,
// открытым и закрытым каналом.
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"labs/1-5/labenv"
)

// blockedAfter — сколько ждать, прежде чем признать операцию заблокированной навсегда.
const blockedAfter = 50 * time.Millisecond

// #region ops
type channelState struct {
	name    string
	newChan func() chan int
}

type operation struct {
	name string
	run  func(ch chan int) string // возвращает результат; паника уходит в recover снаружи
}

var states = []channelState{
	{"nil", func() chan int { return nil }},
	{"открытый, cap 0", func() chan int { return make(chan int) }},
	{"открытый, пустой, cap 1", func() chan int { return make(chan int, 1) }},
	{"открытый, полный, cap 1", func() chan int { ch := make(chan int, 1); ch <- 1; return ch }},
	{"закрытый, cap 1", func() chan int { ch := make(chan int, 1); close(ch); return ch }},
}

var operations = []operation{
	{"`ch <- 1`", func(ch chan int) string { ch <- 1; return "ок" }},
	{"`<-ch`", func(ch chan int) string { v, ok := <-ch; return fmt.Sprintf("ок (v=%d, ok=%t)", v, ok) }},
	{"`close(ch)`", func(ch chan int) string { close(ch); return "ок" }},
	{"`len(ch)`", func(ch chan int) string { return fmt.Sprintf("ок (%d)", len(ch)) }},
	{"`cap(ch)`", func(ch chan int) string { return fmt.Sprintf("ок (%d)", cap(ch)) }},
}

// observe выполняет операцию в отдельной горутине: паника перехватывается,
// а операция, не вернувшаяся за blockedAfter, считается блокирующей.
// Заблокированная горутина остаётся жить до конца программы — для таблицы это неважно.
func observe(op operation, ch chan int) string {
	result := make(chan string, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				result <- fmt.Sprintf("panic: %v", r)
			}
		}()
		result <- op.run(ch)
	}()
	select {
	case r := <-result:
		return r
	case <-time.After(blockedAfter):
		return "блокирует навсегда"
	}
}

func printTable() {
	header := []string{"операция"}
	divider := []string{"---"}
	for _, s := range states {
		header = append(header, s.name)
		divider = append(divider, "---")
	}
	fmt.Println("| " + strings.Join(header, " | ") + " |")
	fmt.Println("| " + strings.Join(divider, " | ") + " |")
	for _, op := range operations {
		row := []string{op.name}
		for _, s := range states {
			row = append(row, observe(op, s.newChan()))
		}
		fmt.Println("| " + strings.Join(row, " | ") + " |")
	}
}

// #endregion ops

func main() {
	labenv.Print(os.Stdout)
	fmt.Println()
	printTable()
}
