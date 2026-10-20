// Программа netpoll измеряет цену простаивающего TCP-соединения в модели «горутина на соединение»
// (урок 1.5.7). Подкоманда serve принимает N соединений и печатает, сколько памяти процесса и ядра
// пришлось на каждое; dial открывает эти N соединений с нескольких адресов 127.0.0.x и держит их.
//
//	netpoll serve -addr 127.0.0.1:9000 -mode bufio -n 10000
//	netpoll dial  -addr 127.0.0.1:9000 -n 10000 -src 8
//
// Обе подкоманды работают до SIGTERM или SIGINT.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"labs/1-5/labenv"
)

const usage = "использование: netpoll serve|dial [флаги]; netpoll serve -h — флаги подкоманды"

func main() {
	labenv.Print(os.Stdout)
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	var run func(context.Context, []string) error
	switch os.Args[1] {
	case "serve":
		run = serve
	case "dial":
		run = dial
	default:
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	err := run(ctx, os.Args[2:])
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "netpoll %s: %v\n", os.Args[1], err)
	if limit := (*limitError)(nil); errors.As(err, &limit) {
		os.Exit(2)
	}
	os.Exit(1)
}
