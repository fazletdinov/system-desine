package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
	"syscall"
	"time"
)

const (
	// readBufSize — буфер чтения на соединение: столько же по умолчанию даёт bufio.NewReader.
	readBufSize = 4096
	// settle — пауза после приёма последнего соединения: горутины успевают дойти до чтения и
	// запарковаться, их стеки — дорасти до рабочего размера.
	settle = 2 * time.Second
)

// #region serve-bufio
// serveBufio — классический обработчик: у каждого соединения свой bufio.Reader с буфером 4 КиБ,
// который живёт, пока живо соединение, даже если клиент молчит часами.
func serveBufio(c net.Conn) {
	defer func() { _ = c.Close() }() // ошибка закрытия простаивающего соединения ничего не меняет
	r := bufio.NewReaderSize(c, readBufSize)
	for {
		line, err := r.ReadString('\n') // нет данных → горутина паркуется в netpoller'е
		if err != nil {
			return
		}
		if _, err := io.WriteString(c, line); err != nil {
			return
		}
	}
}

// #endregion serve-bufio

// #region serve-lazy
var bufPool = sync.Pool{New: func() any {
	b := make([]byte, readBufSize)
	return &b
}}

// serveLazy ждёт готовности сокета без собственного буфера и берёт буфер из пула только на время
// чтения. RawConn.Read вызывает f; если f вернула false, горутина паркуется в netpoller'е до
// следующего события готовности и f вызывается снова. Это контраст для урока, а не замена gobwas/ws:
// горутина и её стек на соединение остаются.
func serveLazy(c net.Conn) {
	defer func() { _ = c.Close() }()
	rc, err := c.(syscall.Conn).SyscallConn()
	if err != nil {
		return
	}
	for {
		var buf *[]byte
		var n int
		var readErr error
		err := rc.Read(func(fd uintptr) bool {
			b := bufPool.Get().(*[]byte)
			for {
				n, readErr = syscall.Read(int(fd), *b)
				if readErr != syscall.EINTR {
					break
				}
			}
			if readErr == syscall.EAGAIN { // данных нет: вернуть буфер и ждать готовности
				bufPool.Put(b)
				return false
			}
			buf = b
			return true
		})
		if err != nil || readErr != nil || n == 0 { // n == 0 — клиент закрыл соединение
			if buf != nil {
				bufPool.Put(buf)
			}
			return
		}
		_, err = c.Write((*buf)[:n])
		bufPool.Put(buf)
		if err != nil {
			return
		}
	}
}

// #endregion serve-lazy

func serve(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:9000", "адрес для приёма соединений")
	mode := fs.String("mode", "bufio", "bufio — свой bufio.Reader на соединение; lazy — буфер из sync.Pool на время чтения")
	n := fs.Int("n", 1000, "сколько соединений принять перед замером")
	if err := fs.Parse(args); err != nil {
		return err
	}
	handlers := map[string]func(net.Conn){"bufio": serveBufio, "lazy": serveLazy}
	handle, ok := handlers[*mode]
	if !ok {
		return fmt.Errorf("неизвестный режим %q: нужен bufio или lazy", *mode)
	}
	if *n <= 0 {
		return fmt.Errorf("-n должно быть больше нуля, получено %d", *n)
	}
	if err := checkFDLimit(*n); err != nil {
		return err
	}

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", *addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer func() { _ = ln.Close() }()
	stopListen := context.AfterFunc(ctx, func() { _ = ln.Close() }) // SIGTERM прерывает Accept
	defer stopListen()

	runtime.GC()
	before, err := takeSnapshot()
	if err != nil {
		return err
	}
	fmt.Printf("режим: %s, ждём соединений: %d\n", *mode, *n)
	fmt.Printf("слушаю %s\n", ln.Addr())

	// Горутины-обработчики принадлежат serve: при выходе соединения закрываются, Wait дожидается всех.
	var wg sync.WaitGroup
	conns := make([]net.Conn, 0, *n)
	defer func() {
		for _, c := range conns {
			_ = c.Close() // закрытие будит горутину в Read; ошибка повторного закрытия не важна
		}
		wg.Wait()
	}()

	start := time.Now()
	for len(conns) < *n {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("accept после %d соединений: %w", len(conns), err)
		}
		conns = append(conns, c)
		wg.Go(func() { handle(c) })
	}
	fmt.Printf("принято соединений: %d за %d мс\n", len(conns), time.Since(start).Milliseconds())

	select {
	case <-ctx.Done():
		return nil
	case <-time.After(settle):
	}
	runtime.GC() // в замер попадает живая память, а не мусор, накопленный при приёме
	after, err := takeSnapshot()
	if err != nil {
		return err
	}
	report(before, after, len(conns))
	fmt.Println("держу соединения до SIGTERM")
	<-ctx.Done()
	return nil
}

func kib(b uint64) float64 { return float64(b) / 1024 }

func report(before, after snapshot, n int) {
	perConn := func(b, a uint64) float64 { return (kib(a) - kib(b)) / float64(n) }
	fmt.Printf("горутин:          до %10d, после %10d\n", before.goroutines, after.goroutines)
	fmt.Printf("RSS:              до %10d КиБ, после %10d КиБ\n", before.rssKiB, after.rssKiB)
	fmt.Printf("стеки горутин:    до %10.0f КиБ, после %10.0f КиБ\n", kib(before.stacks), kib(after.stacks))
	fmt.Printf("куча (объекты):   до %10.0f КиБ, после %10.0f КиБ\n", kib(before.objects), kib(after.objects))
	fmt.Printf("стеки для GC:     до %10.0f КиБ, после %10.0f КиБ (/gc/scan/stack:bytes)\n", kib(before.scanStack), kib(after.scanStack))
	fmt.Printf("sockstat до:      %s\n", before.tcp.line)
	fmt.Printf("sockstat после:   %s\n", after.tcp.line)
	pageKiB := float64(syscall.Getpagesize()) / 1024
	fmt.Printf("ядро: TCP-сокетов +%d (обе стороны, если клиент на этой машине), буферы +%d страниц (%.0f КиБ)\n",
		after.tcp.inUse-before.tcp.inUse, after.tcp.mem-before.tcp.mem, float64(after.tcp.mem-before.tcp.mem)*pageKiB)
	fmt.Printf("на соединение: RSS %.1f КиБ, стеки %.1f КиБ, куча %.1f КиБ\n",
		float64(after.rssKiB-before.rssKiB)/float64(n),
		perConn(before.stacks, after.stacks), perConn(before.objects, after.objects))
}
