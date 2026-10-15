package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbeURL(t *testing.T) {
	cases := []struct {
		addr    string
		want    string
		wantErr bool
	}{
		{":8080", "http://127.0.0.1:8080/livez", false},
		{"0.0.0.0:8080", "http://127.0.0.1:8080/livez", false},
		{"[::]:8080", "http://[::1]:8080/livez", false},
		{"127.0.0.1:9090", "http://127.0.0.1:9090/livez", false},
		{"localhost:8080", "http://localhost:8080/livez", false},
		{"[::1]:8080", "http://[::1]:8080/livez", false},
		{"8080", "", true},
		{"", "", true},
		{"host:", "", true},
	}
	for _, c := range cases {
		got, err := probeURL(c.addr)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("probeURL(%q) = %q, %v; ждали %q, ошибка=%v", c.addr, got, err, c.want, c.wantErr)
		}
	}
}

func TestHealthcheck(t *testing.T) {
	status := func(code int) *httptest.Server {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) }))
		t.Cleanup(srv.Close)
		return srv
	}
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	t.Cleanup(slow.Close)
	gone := httptest.NewServer(http.NotFoundHandler())
	gone.Close()

	cases := []struct {
		name    string
		url     string
		timeout time.Duration
		ok      bool
	}{
		{"200 — живой", status(http.StatusOK).URL, time.Second, true},
		{"503 — нет", status(http.StatusServiceUnavailable).URL, time.Second, false},
		{"не слушает — нет", gone.URL, time.Second, false},
		{"не успел — нет", slow.URL, 50 * time.Millisecond, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := healthcheck(t.Context(), &http.Client{Timeout: c.timeout}, c.url)
			if (err == nil) != c.ok {
				t.Fatalf("healthcheck: %v", err)
			}
		})
	}
}
