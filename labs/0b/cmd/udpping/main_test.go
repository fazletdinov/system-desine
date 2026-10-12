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
	sent := time.Unix(1700000000, 123456789)
	pkt := encode(42, sent)
	seq, got, err := decode(pkt[:])
	if err != nil || seq != 42 || !got.Equal(sent) {
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
	printReport(&buf, udpstat.Analyze(4, []udpstat.Reply{{Seq: 0, RTT: time.Millisecond}, {Seq: 2, RTT: 3 * time.Millisecond}}))
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
