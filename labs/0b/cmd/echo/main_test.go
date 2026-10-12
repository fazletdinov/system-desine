package main

import (
	"bufio"
	"errors"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMakeLine(t *testing.T) {
	tests := []struct {
		name      string
		seq, size int
		wantLen   int
	}{
		{"добивает до размера", 3, 32, 33},
		{"префикс длиннее размера не режется", 12345, 4, len("seq=12345 \n")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeLine(tt.seq, tt.size)
			if len(got) != tt.wantLen || !strings.HasSuffix(got, "\n") {
				t.Fatalf("makeLine(%d,%d) = %q (len %d), want len %d", tt.seq, tt.size, got, len(got), tt.wantLen)
			}
		})
	}
}

func TestHandleEchoesLinesUntilClientCloses(t *testing.T) {
	srv, cli := net.Pipe()
	type result struct {
		lines int
		err   error
	}
	done := make(chan result, 1)
	go func() {
		n, err := handle(srv, time.Second)
		done <- result{n, err}
	}()

	r := bufio.NewReader(cli)
	for _, msg := range []string{"a\n", "bb\n"} {
		go cli.Write([]byte(msg))
		got, err := r.ReadString('\n')
		if err != nil || got != msg {
			t.Fatalf("echo = %q, %v; want %q", got, err, msg)
		}
	}
	cli.Close()

	res := <-done
	// Закрытие конца net.Pipe даёт ErrClosedPipe, а не io.EOF как у TCP; важно, что цикл завершился.
	if res.lines != 2 || res.err == nil {
		t.Fatalf("lines=%d err=%v", res.lines, res.err)
	}
}

func TestHandleClosesIdleConnection(t *testing.T) {
	srv, cli := net.Pipe()
	defer cli.Close()
	_, err := handle(srv, 20*time.Millisecond)
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
}

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		reply   string
		wantErr bool
	}{
		{"совпавшее эхо", "hi\n", false},
		{"искажённое эхо", "ho\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, cli := net.Pipe()
			defer cli.Close()
			go func() {
				defer srv.Close()
				bufio.NewReader(srv).ReadString('\n')
				srv.Write([]byte(tt.reply))
			}()
			_, err := roundTrip(cli, bufio.NewReader(cli), "hi\n", time.Second)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
