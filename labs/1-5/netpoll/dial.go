package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"time"
)

const (
	// ephemeralPortsPerAddr — сколько исходящих соединений помещается на один исходный адрес. Dialer с
	// LocalAddr делает bind(адрес:0) до connect без IP_BIND_ADDRESS_NO_PORT, и ядро занимает порт уже
	// при bind, не зная адреса назначения: предел — порты на исходный адрес, к каким бы серверам ни шли
	// соединения. По умолчанию net.ipv4.ip_local_port_range = 32768–60999, это 28 232 порта.
	ephemeralPortsPerAddr = 28000
	// maxSrcAddrs — исходные адреса 127.0.0.2…127.0.0.254: весь 127.0.0.0/8 — loopback, привязывать
	// к ним сокет можно без настройки интерфейса.
	maxSrcAddrs = 253
	// dialTimeout — локальное соединение устанавливается за микросекунды; секунда означает, что
	// очередь приёма сервера (somaxconn) переполнена или сервер не запущен.
	dialTimeout = time.Second
)

// #region dial
func dial(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("dial", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:9000", "адрес сервера")
	n := fs.Int("n", 1000, "сколько соединений открыть")
	src := fs.Int("src", 8, "сколько исходных адресов 127.0.0.2… использовать")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *n <= 0 || *src <= 0 || *src > maxSrcAddrs {
		return fmt.Errorf("нужно -n > 0 и 1 ≤ -src ≤ %d, получено -n %d -src %d", maxSrcAddrs, *n, *src)
	}
	if err := checkFDLimit(*n); err != nil {
		return err
	}
	// Каждое соединение занимает эфемерный порт на своём исходном адресе: с одного адреса больше ~28
	// тысяч не открыть, поэтому соединения раскладываются по нескольким адресам.
	if perAddr := (*n + *src - 1) / *src; perAddr > ephemeralPortsPerAddr {
		return &limitError{fmt.Sprintf("нужно %d портов на адрес, доступно около %d: увеличьте -src", perAddr, ephemeralPortsPerAddr)}
	}

	dialers := make([]net.Dialer, *src)
	for i := range dialers {
		dialers[i] = net.Dialer{
			LocalAddr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, byte(2+i))},
			Timeout:   dialTimeout,
		}
	}
	conns := make([]net.Conn, 0, *n)
	defer func() {
		for _, c := range conns {
			_ = c.Close() // закрытие будит горутину в Read; ошибка повторного закрытия не важна
		}
	}()
	start := time.Now()
	for i := range *n {
		c, err := dialers[i%*src].DialContext(ctx, "tcp", *addr)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("соединение %d: %w", i+1, err)
		}
		conns = append(conns, c)
	}
	fmt.Printf("открыто соединений: %d с %d адресов за %d мс\n", len(conns), *src, time.Since(start).Milliseconds())
	fmt.Println("держу соединения до SIGTERM")
	<-ctx.Done()
	return nil
}

// #endregion dial
