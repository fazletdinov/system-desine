package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"labs/0b/internal/udpstat"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	sent := 123456789 * time.Nanosecond
	pkt := encode(42, sent)
	seq, got, err := decode(pkt[:])
	if err != nil || seq != 42 || got != sent {
		t.Fatalf("decode = %d, %v, %v; want 42, %v", seq, got, err, sent)
	}
}

func TestDecodeRejectsWrongLength(t *testing.T) {
	if _, _, err := decode(make([]byte, packetSize-1)); err == nil {
		t.Fatal("want error for short datagram")
	}
}

func TestPrintReport(t *testing.T) {
	var buf bytes.Buffer
	printReport(&buf, report{Result: udpstat.Analyze(4, []udpstat.Reply{{Seq: 0, RTT: time.Millisecond}, {Seq: 2, RTT: 3 * time.Millisecond}})})
	first, _, _ := strings.Cut(buf.String(), "\n")
	if want := "sent=4 received=2 lost=2 (50.0%) dup=0 reordered=0"; first != want {
		t.Fatalf("got %q, want %q", first, want)
	}
}

func TestClientAgainstServerOnLoopback(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, pc, slog.New(slog.NewTextHandler(io.Discard, nil))) }()

	var out bytes.Buffer
	cfg := config{addr: pc.LocalAddr().String(), n: 5, interval: time.Millisecond, wait: 100 * time.Millisecond}
	if err := runClient(context.Background(), cfg, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "sent=5 received=5 lost=0") {
		t.Fatalf("unexpected report %q", out.String())
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serve: %v", err)
	}
}

func TestValidate(t *testing.T) {
	ok := config{n: 1, interval: 1, wait: 1}
	tests := []struct {
		name    string
		mutate  func(*config)
		wantErr bool
	}{
		{"корректный клиент", func(*config) {}, false},
		{"n равно нулю", func(c *config) { c.n = 0 }, true},
		{"нулевой interval", func(c *config) { c.interval = 0 }, true},
		{"нулевой wait", func(c *config) { c.wait = 0 }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := ok
			tt.mutate(&c)
			if err := c.validate(); (err != nil) != tt.wantErr {
				t.Fatalf("validate() = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClientCountsRefusedWhenNobodyListens(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := pc.LocalAddr().String()
	pc.Close() // порт освобождён: на отправку придёт ICMP port unreachable

	var out bytes.Buffer
	cfg := config{addr: addr, n: 3, interval: 5 * time.Millisecond, wait: 100 * time.Millisecond}
	if err := runClient(context.Background(), cfg, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "received=0") || !strings.Contains(out.String(), "refused=") {
		t.Fatalf("unexpected report %q", out.String())
	}
}
