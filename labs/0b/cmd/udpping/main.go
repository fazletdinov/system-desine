// Команда udpping — UDP-пинг: сервер (-serve) возвращает датаграммы, клиент считает потери и RTT.
package main

import (
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"labs/0b/internal/stats"
	"labs/0b/internal/udpstat"
)

// packetSize: 4 байта seq (big endian) + 8 байт метки времени отправки (наносекунды от старта клиента).
const packetSize = 12

type config struct {
	serve    bool
	addr     string
	n        int
	interval time.Duration
	wait     time.Duration
}

func parseFlags(args []string) (config, error) {
	var c config
	fs := flag.NewFlagSet("udpping", flag.ContinueOnError)
	fs.BoolVar(&c.serve, "serve", false, "запустить сервер")
	fs.StringVar(&c.addr, "addr", ":7001", "адрес: для сервера — где слушать, для клиента — куда слать")
	fs.IntVar(&c.n, "n", 200, "клиент: сколько датаграмм отправить")
	fs.DurationVar(&c.interval, "interval", 10*time.Millisecond, "клиент: пауза между датаграммами")
	fs.DurationVar(&c.wait, "wait", time.Second, "клиент: сколько ждать ответы после последней отправки")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if err := c.validate(); err != nil {
		return config{}, err
	}
	return c, nil
}

func (c config) validate() error {
	if c.serve {
		return nil
	}
	var errs []error
	if c.n <= 0 {
		errs = append(errs, fmt.Errorf("-n должен быть больше нуля, получено %d", c.n))
	}
	if c.interval <= 0 {
		errs = append(errs, fmt.Errorf("-interval должен быть больше нуля, получено %v", c.interval))
	}
	if c.wait <= 0 {
		errs = append(errs, fmt.Errorf("-wait должен быть больше нуля, получено %v", c.wait))
	}
	return errors.Join(errs...)
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Error("флаги", "err", err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg, log); err != nil {
		log.Error("udpping", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config, log *slog.Logger) error {
	if cfg.serve {
		var lc net.ListenConfig
		pc, err := lc.ListenPacket(ctx, "udp", cfg.addr)
		if err != nil {
			return fmt.Errorf("listen %s: %w", cfg.addr, err)
		}
		log.Info("слушаю", "addr", pc.LocalAddr().String())
		return serve(ctx, pc, log)
	}
	return runClient(ctx, cfg, os.Stdout)
}

// serve отвечает каждой датаграммой тем же содержимым тому же адресу, пока ctx не отменён.
func serve(ctx context.Context, pc net.PacketConn, log *slog.Logger) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	// Закрытие сокета — единственный способ прервать ReadFrom.
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		if err := pc.Close(); err != nil {
			log.Warn("закрытие сокета", "err", err)
		}
	}()
	defer wg.Wait()

	buf := make([]byte, 1500)
	var count int
	for {
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				log.Info("остановлен", "datagrams", count)
				return nil
			}
			return fmt.Errorf("read: %w", err)
		}
		if _, err := pc.WriteTo(buf[:n], addr); err != nil {
			log.Warn("не смог ответить", "client", addr.String(), "err", err)
			continue
		}
		count++
	}
}

// encode кладёт в пакет номер и метку: время от старта клиента по монотонным часам.
// Настенные часы (UnixNano) могут скакнуть из-за NTP и дать отрицательный или огромный RTT.
func encode(seq uint32, stamp time.Duration) [packetSize]byte {
	var b [packetSize]byte
	binary.BigEndian.PutUint32(b[0:4], seq)
	binary.BigEndian.PutUint64(b[4:12], uint64(stamp))
	return b
}

func decode(b []byte) (seq uint32, stamp time.Duration, err error) {
	if len(b) != packetSize {
		return 0, 0, fmt.Errorf("длина датаграммы %d, ожидалось %d", len(b), packetSize)
	}
	seq = binary.BigEndian.Uint32(b[0:4])
	stamp = time.Duration(binary.BigEndian.Uint64(b[4:12]))
	return seq, stamp, nil
}

func runClient(ctx context.Context, cfg config, out io.Writer) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp", cfg.addr)
	if err != nil {
		return fmt.Errorf("dial %s: %w", cfg.addr, err)
	}
	defer func() { _ = conn.Close() }() // ping уже вернул результат, ошибка закрытия ничего не меняет

	rep, err := ping(ctx, conn, cfg)
	if err != nil {
		return err
	}
	printReport(out, rep)
	return nil
}

// report — результат анализа плюс то, что не вписывается в потери и дубликаты.
type report struct {
	udpstat.Result
	Malformed int // датаграммы неверной длины
	Refused   int // ICMP "port unreachable" на подключённом UDP-сокете
}

// ping владеет двумя горутинами — отправителем и получателем — и дожидается обеих.
// Получатель останавливается дедлайном чтения, который отправитель выставляет через wait
// после последней отправки (или сразу при отмене ctx).
func ping(ctx context.Context, conn net.Conn, cfg config) (report, error) {
	var (
		wg      sync.WaitGroup
		replies []udpstat.Reply // пишет только получатель; читаем после wg.Wait
		rep     report          // Malformed/Refused пишет только получатель
		sent    int
		sendErr error
		recvErr error
	)
	base := time.Now() // time.Since использует монотонные часы

	// #region client

	wg.Add(1)
	go func() { // отправитель
		defer wg.Done()
		// Будим получателя при любом выходе; если не вышло — закрываем сокет, иначе wg.Wait зависнет.
		defer func() {
			if sendErr == nil {
				select {
				case <-ctx.Done():
				case <-time.After(cfg.wait):
				}
			}
			if err := conn.SetReadDeadline(time.Now()); err != nil {
				sendErr = errors.Join(sendErr, fmt.Errorf("set read deadline: %w", err))
				_ = conn.Close() // Read получателя вернёт ошибку, горутина выйдет
			}
		}()
		for i := range cfg.n {
			pkt := encode(uint32(i), time.Since(base))
			if _, err := conn.Write(pkt[:]); err != nil {
				sendErr = fmt.Errorf("send seq %d: %w", i, err)
				return
			}
			sent++
			if i == cfg.n-1 {
				return // после последнего пакета пауза не нужна: ждём ровно -wait
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(cfg.interval):
			}
		}
	}()

	wg.Add(1)
	go func() { // получатель
		defer wg.Done()
		buf := make([]byte, 64)
		for {
			n, err := conn.Read(buf)
			switch {
			case errors.Is(err, os.ErrDeadlineExceeded):
				return // штатное завершение
			case errors.Is(err, syscall.ECONNREFUSED):
				rep.Refused++ // сервер не слушает: ICMP вернулся, ждём дальше
				continue
			case err != nil:
				recvErr = fmt.Errorf("recv: %w", err)
				return
			}
			seq, stamp, err := decode(buf[:n])
			if err != nil {
				rep.Malformed++
				continue
			}
			replies = append(replies, udpstat.Reply{Seq: seq, RTT: time.Since(base) - stamp})
		}
	}()

	wg.Wait()
	rep.Result = udpstat.Analyze(sent, replies)

	// #endregion client

	if err := errors.Join(sendErr, recvErr); err != nil {
		return report{}, err
	}
	return rep, nil
}

func printReport(out io.Writer, r report) {
	var pct float64
	if r.Sent > 0 {
		pct = float64(r.Lost) / float64(r.Sent) * 100
	}
	fmt.Fprintf(out, "sent=%d received=%d lost=%d (%.1f%%) dup=%d reordered=%d\n",
		r.Sent, r.Received, r.Lost, pct, r.Duplicates, r.Reordered)
	if r.Malformed > 0 || r.Refused > 0 {
		fmt.Fprintf(out, "unexpected: malformed=%d refused=%d\n", r.Malformed, r.Refused)
	}
	if s := stats.Summary(r.RTTs); s.N > 0 {
		fmt.Fprintf(out, "rtt: n=%d min=%v p50=%v p99=%v max=%v\n", s.N, s.Min, s.P50, s.P99, s.Max)
	}
}
