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

// packetSize: 4 байта seq (big endian) + 8 байт времени отправки (UnixNano).
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
	return c, nil
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
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
		pc.Close()
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

func encode(seq uint32, sent time.Time) [packetSize]byte {
	var b [packetSize]byte
	binary.BigEndian.PutUint32(b[0:4], seq)
	binary.BigEndian.PutUint64(b[4:12], uint64(sent.UnixNano()))
	return b
}

func decode(b []byte) (seq uint32, sent time.Time, err error) {
	if len(b) != packetSize {
		return 0, time.Time{}, fmt.Errorf("длина датаграммы %d, ожидалось %d", len(b), packetSize)
	}
	seq = binary.BigEndian.Uint32(b[0:4])
	sent = time.Unix(0, int64(binary.BigEndian.Uint64(b[4:12])))
	return seq, sent, nil
}

func runClient(ctx context.Context, cfg config, out io.Writer) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp", cfg.addr)
	if err != nil {
		return fmt.Errorf("dial %s: %w", cfg.addr, err)
	}
	defer conn.Close()

	res, err := ping(ctx, conn, cfg)
	if err != nil {
		return err
	}
	printReport(out, res)
	return nil
}

// ping владеет двумя горутинами — отправителем и получателем — и дожидается обеих.
// Получатель останавливается дедлайном чтения, который выставляется через wait после
// последней отправки (или сразу при отмене ctx).
func ping(ctx context.Context, conn net.Conn, cfg config) (udpstat.Result, error) {
	var (
		wg      sync.WaitGroup
		replies []udpstat.Reply // пишет только получатель; читаем после wg.Wait
		sent    int
		sendErr error
	)

	// #region client

	wg.Add(1)
	go func() { // отправитель
		defer wg.Done()
		defer func() {
			select {
			case <-ctx.Done():
				conn.SetReadDeadline(time.Now())
			case <-time.After(cfg.wait):
				conn.SetReadDeadline(time.Now())
			}
		}()
		for i := range cfg.n {
			pkt := encode(uint32(i), time.Now())
			if _, err := conn.Write(pkt[:]); err != nil {
				sendErr = fmt.Errorf("send seq %d: %w", i, err)
				return
			}
			sent++
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
			if err != nil {
				return // дедлайн — штатное завершение, остальное увидим по потерям
			}
			seq, sentAt, err := decode(buf[:n])
			if err != nil {
				continue
			}
			replies = append(replies, udpstat.Reply{Seq: seq, RTT: time.Since(sentAt)})
		}
	}()

	wg.Wait()

	// #endregion client

	if sendErr != nil {
		return udpstat.Result{}, sendErr
	}
	return udpstat.Analyze(sent, replies), nil
}

func printReport(out io.Writer, r udpstat.Result) {
	var pct float64
	if r.Sent > 0 {
		pct = float64(r.Lost) / float64(r.Sent) * 100
	}
	fmt.Fprintf(out, "sent=%d received=%d lost=%d (%.1f%%) dup=%d reordered=%d\n",
		r.Sent, r.Received, r.Lost, pct, r.Duplicates, r.Reordered)
	if s := stats.Summary(r.RTTs); s.N > 0 {
		fmt.Fprintf(out, "rtt: n=%d min=%v p50=%v p99=%v max=%v\n", s.N, s.Min, s.P50, s.P99, s.Max)
	}
}
