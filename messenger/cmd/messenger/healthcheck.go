package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"messenger/internal/platform/health"
)

// healthcheckTimeout меньше --timeout=3s у HEALTHCHECK в Dockerfile: проверка должна
// закончиться сама и сказать почему, а не быть убитой Docker'ом молча.
const healthcheckTimeout = 2 * time.Second

// probeLimit — сколько байт тела дочитать: ответ пробы — пара десятков байт, а
// дочитанное тело позволяет переиспользовать соединение (здесь не нужно, но привычка).
const probeLimit = 1 << 10

// #region healthcheck
// probeURL — адрес /livez того же процесса. Сервер может слушать «все интерфейсы»
// (:8080, 0.0.0.0, [::]) — проба идёт на loopback того же семейства.
func probeURL(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("адрес %q: %w", addr, err)
	}
	if port == "" {
		return "", fmt.Errorf("адрес %q: нет порта", addr)
	}
	switch host {
	case "", "0.0.0.0":
		host = "127.0.0.1"
	case "::":
		host = "::1"
	}
	return "http://" + net.JoinHostPort(host, port) + health.LivePath, nil
}

func healthcheck(ctx context.Context, c *http.Client, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("проба: %w", err)
	}
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("проба: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, probeLimit))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("проба: статус %d", resp.StatusCode)
	}
	return nil
}

// #endregion healthcheck

// runHealthcheck — подкоманда `messenger healthcheck` для HEALTHCHECK в образе без shell.
func runHealthcheck(stderr io.Writer) int {
	url, err := probeURL(addrFromEnv())
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), healthcheckTimeout)
		defer cancel()
		err = healthcheck(ctx, &http.Client{}, url)
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
