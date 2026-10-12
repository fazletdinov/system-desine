// Команда tlsdemo — что происходит в TLS: get показывает версию, ALPN и цепочку сертификатов,
// inspect — состояние соединения и возобновление сессии.
package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// maxShow — сколько байт тела ответа печатать.
const maxShow = 4 << 10

const usage = `использование:
  tlsdemo get     -url https://host:port/ [-ca ca.pem] [-server-name имя] [-timeout 5s]
  tlsdemo inspect -addr host:port [-ca ca.pem] [-server-name имя] [-timeout 5s]`

type config struct {
	cmd        string // "get" или "inspect"
	url        string // get
	addr       string // inspect
	ca         string
	serverName string
	timeout    time.Duration
}

func parseArgs(args []string) (config, error) {
	if len(args) == 0 {
		return config{}, errors.New("нужна подкоманда\n" + usage)
	}
	c := config{cmd: args[0]}
	fs := flag.NewFlagSet("tlsdemo "+c.cmd, flag.ContinueOnError)
	switch c.cmd {
	case "get":
		fs.StringVar(&c.url, "url", "https://api.messenger.example:8443/", "адрес запроса")
	case "inspect":
		fs.StringVar(&c.addr, "addr", "172.28.0.10:8443", "host:port сервера")
	default:
		return config{}, fmt.Errorf("неизвестная подкоманда %q\n%s", c.cmd, usage)
	}
	fs.StringVar(&c.ca, "ca", "", "PEM с доверенным CA; пусто — системные корни")
	fs.StringVar(&c.serverName, "server-name", "", "имя для SNI и проверки сертификата; пусто — хост из адреса")
	fs.DurationVar(&c.timeout, "timeout", 5*time.Second, "таймаут на подключение и ответ")
	if err := fs.Parse(args[1:]); err != nil {
		return config{}, err
	}
	if fs.NArg() > 0 {
		return config{}, fmt.Errorf("лишние аргументы: %q", fs.Args())
	}
	if err := c.validate(); err != nil {
		return config{}, err
	}
	return c, nil
}

func (c config) validate() error {
	var errs []error
	switch c.cmd {
	case "get":
		if u, err := url.Parse(c.url); err != nil || u.Scheme != "https" || u.Host == "" {
			errs = append(errs, fmt.Errorf("-url: нужен адрес вида https://host:port/, получено %q", c.url))
		}
	case "inspect":
		if _, _, err := net.SplitHostPort(c.addr); err != nil {
			errs = append(errs, fmt.Errorf("-addr: нужен host:port: %w", err))
		}
	}
	if c.timeout <= 0 {
		errs = append(errs, fmt.Errorf("-timeout должен быть больше нуля, получено %v", c.timeout))
	}
	return errors.Join(errs...)
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Error("аргументы", "err", err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	run := runGet
	if cfg.cmd == "inspect" {
		run = runInspect
	}
	if err := run(ctx, cfg, os.Stdout); err != nil {
		if why := explainTLS(err); why != "" {
			// Если не пишется и stderr, сообщить об ошибке некуда: остаётся код возврата.
			_, _ = fmt.Fprintf(os.Stderr, "проверка сертификата не прошла: %s\nисходная ошибка: %v\n", why, err)
		} else {
			log.Error("tlsdemo "+cfg.cmd, "err", err)
		}
		os.Exit(1)
	}
}

// explainTLS переводит ошибку проверки сертификата на человеческий язык; "" — ошибка другая.
func explainTLS(err error) string {
	var (
		hostname  x509.HostnameError
		unknownCA x509.UnknownAuthorityError
		invalid   x509.CertificateInvalidError
	)
	switch {
	case errors.As(err, &hostname):
		var names []string
		if hostname.Certificate != nil {
			names = hostname.Certificate.DNSNames
		}
		return fmt.Sprintf("сертификат выдан не для %q, а для %q — проверьте -server-name", hostname.Host, names)
	case errors.As(err, &unknownCA):
		return "сертификат подписан неизвестным CA: его нет среди доверенных корней — проверьте -ca"
	case errors.As(err, &invalid) && invalid.Reason == x509.Expired:
		return "срок действия сертификата истёк или ещё не начался — проверьте часы и -ttl сервера"
	case errors.As(err, &invalid):
		return fmt.Sprintf("сертификат недействителен: %v", invalid)
	default:
		return ""
	}
}

func loadRoots(path string) (*x509.CertPool, error) {
	if path == "" {
		return nil, nil // nil в tls.Config.RootCAs — системные корни
	}
	pemData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemData) {
		return nil, fmt.Errorf("read CA: в %s нет PEM-сертификатов", path)
	}
	return pool, nil
}

// #region client-config

// clientTLSConfig — проверка сервера по нашему CA. RootCAs — кому доверяем (nil — системным
// корням), ServerName — какое имя обязан содержать сертификат (оно же уходит в SNI).
// InsecureSkipVerify здесь нет и быть не должно: без проверки TLS шифрует разговор
// с кем угодно, в том числе с атакующим посередине.
func clientTLSConfig(roots *x509.CertPool, serverName string) *tls.Config {
	return &tls.Config{
		RootCAs:    roots,
		ServerName: serverName,
		MinVersion: tls.VersionTLS12,
	}
}

// #endregion client-config

func runGet(ctx context.Context, cfg config, out io.Writer) error {
	roots, err := loadRoots(cfg.ca)
	if err != nil {
		return err
	}
	tr := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: cfg.timeout}).DialContext,
		TLSClientConfig:       clientTLSConfig(roots, cfg.serverName),
		TLSHandshakeTimeout:   cfg.timeout,
		ResponseHeaderTimeout: cfg.timeout,
		ForceAttemptHTTP2:     true, // со своим TLSClientConfig HTTP/2 иначе не предлагается
	}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: cfg.timeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.url, nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err // *url.Error уже содержит метод и адрес: `Get "https://…": …`
	}
	defer func() { _ = resp.Body.Close() }() // показываем только начало тела; соединение больше не нужно
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxShow))
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	return printGet(out, resp, body)
}

// printGet пишет через bufio.Writer: он запоминает первую ошибку записи, следующие записи
// ничего не делают, а Flush её возвращает. Поэтому ошибка вывода проверяется один раз.
func printGet(out io.Writer, resp *http.Response, body []byte) error {
	w := bufio.NewWriter(out)
	printf := func(format string, args ...any) { _, _ = fmt.Fprintf(w, format, args...) } // ошибку вернёт Flush
	st := resp.TLS
	printf("TLS: версия=%s ALPN=%s шифр=%s\n",
		tls.VersionName(st.Version), alpn(st.NegotiatedProtocol), tls.CipherSuiteName(st.CipherSuite))
	printf("цепочка от сервера:\n")
	for i, c := range st.PeerCertificates {
		printf("  %d: subject=%s issuer=%s срок=%s..%s\n", i, c.Subject, c.Issuer,
			c.NotBefore.UTC().Format(time.RFC3339), c.NotAfter.UTC().Format(time.RFC3339))
	}
	printf("ответ: %s %s\n%s", resp.Proto, resp.Status, body)
	if err := w.Flush(); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

func alpn(p string) string {
	if p == "" {
		return "-" // сервер не выбрал протокол: значит, HTTP/1.1 без ALPN
	}
	return p
}

// runInspect подключается дважды с общим кэшем сессий: второе рукопожатие возобновляет
// первую сессию по билету (session ticket) и не проверяет цепочку заново.
func runInspect(ctx context.Context, cfg config, out io.Writer) error {
	roots, err := loadRoots(cfg.ca)
	if err != nil {
		return err
	}
	cache := tls.NewLRUClientSessionCache(8)
	for i := 1; i <= 2; i++ {
		line, err := inspectOnce(ctx, cfg, roots, cache)
		if err != nil {
			return fmt.Errorf("соединение %d: %w", i, err)
		}
		if _, err := fmt.Fprintf(out, "соединение %d: %s\n", i, line); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
	}
	return nil
}

// inspectOnce устанавливает TCP и TLS по отдельности, чтобы время каждого рукопожатия
// было видно само по себе (tls.Dialer делает оба шага разом).
func inspectOnce(ctx context.Context, cfg config, roots *x509.CertPool, cache tls.ClientSessionCache) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	tlsCfg := clientTLSConfig(roots, cfg.serverName)
	tlsCfg.ClientSessionCache = cache
	// Только HTTP/1.1: после рукопожатия мы шлём простой запрос, а для h2 понадобились бы кадры HTTP/2.
	tlsCfg.NextProtos = []string{"http/1.1"}
	if tlsCfg.ServerName == "" {
		// tls.Client, в отличие от tls.Dialer, сам имя из адреса не берёт, а без имени
		// проверить сертификат нельзя.
		host, _, err := net.SplitHostPort(cfg.addr)
		if err != nil {
			return "", fmt.Errorf("addr %q: %w", cfg.addr, err)
		}
		tlsCfg.ServerName = host
	}

	var d net.Dialer
	start := time.Now()
	raw, err := d.DialContext(ctx, "tcp", cfg.addr)
	if err != nil {
		return "", fmt.Errorf("dial %s: %w", cfg.addr, err)
	}
	tcpTime := time.Since(start)
	conn := tls.Client(raw, tlsCfg)
	defer func() { _ = conn.Close() }() // ответ уже прочитан; ошибка закрытия ничего не меняет

	deadline, _ := ctx.Deadline() // есть всегда: ctx создан с таймаутом
	if err := raw.SetDeadline(deadline); err != nil {
		return "", fmt.Errorf("set deadline: %w", err)
	}
	// Отмена по сигналу должна прервать чтение раньше дедлайна.
	stop := context.AfterFunc(ctx, func() { _ = raw.SetDeadline(time.Now()) })
	defer stop()

	start = time.Now()
	if err := conn.HandshakeContext(ctx); err != nil {
		return "", fmt.Errorf("tls handshake: %w", err)
	}
	tlsTime := time.Since(start)

	// В TLS 1.3 билет сессии приходит после рукопожатия, а клиент разбирает его, только
	// когда читает из соединения. Поэтому делаем один запрос и читаем ответ до конца:
	// без этого кэш останется пустым и второе соединение не возобновится.
	status, err := oneRequest(conn, cfg)
	if err != nil {
		return "", err
	}

	st := conn.ConnectionState()
	return fmt.Sprintf("версия=%s шифр=%s ALPN=%s DidResume=%t TCP=%v TLS=%v статус=%s",
		tls.VersionName(st.Version), tls.CipherSuiteName(st.CipherSuite), alpn(st.NegotiatedProtocol),
		st.DidResume, tcpTime.Round(10*time.Microsecond), tlsTime.Round(10*time.Microsecond), status), nil
}

func oneRequest(conn net.Conn, cfg config) (string, error) {
	host := cfg.serverName
	if host == "" {
		host = cfg.addr
	}
	req, err := http.NewRequest(http.MethodGet, "https://"+host+"/", nil)
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Close = true // Connection: close — сервер закроет соединение после ответа
	if err := req.Write(conn); err != nil {
		return "", fmt.Errorf("write request: %w", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	_, err = io.Copy(io.Discard, resp.Body)
	if err := errors.Join(err, resp.Body.Close()); err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	return resp.Status, nil
}
