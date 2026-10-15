//go:build linux

package main

import (
	"slices"
	"testing"
)

func TestParseBytes(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"64M", 64 << 20, false},
		{"512K", 512 << 10, false},
		{"1G", 1 << 30, false},
		{"1048576", 1 << 20, false},
		{"64m", 64 << 20, false},
		{"", 0, true},
		{"0", 0, true},
		{"-5M", 0, true},
		{"12X", 0, true},
		{"M", 0, true},
		{"100", 0, true}, // меньше минимума: ядро округлит до страницы, а Go-рантайм не стартует
	}
	for _, c := range cases {
		got, err := parseBytes(c.in)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("parseBytes(%q) = %d, %v; ждали %d, ошибка=%v", c.in, got, err, c.want, c.wantErr)
		}
	}
}

func TestParseCPU(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"50%", "50000 100000", false},
		{"150%", "150000 100000", false},
		{"1%", "1000 100000", false},
		{"0%", "", true},
		{"50", "", true},
		{"abc%", "", true},
		{"100001%", "", true},
	}
	for _, c := range cases {
		got, err := parseCPU(c.in)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("parseCPU(%q) = %q, %v; ждали %q, ошибка=%v", c.in, got, err, c.want, c.wantErr)
		}
	}
}

func TestParseRun(t *testing.T) {
	cfg, err := parseRun([]string{"--mem", "32M", "--pids", "16", "--cpu", "25%", "--hostname", "lab-box", "/rootfs", "sh", "-c", "echo hi"})
	if err != nil {
		t.Fatal(err)
	}
	want := runConfig{
		Hostname: "lab-box",
		Rootfs:   "/rootfs",
		Argv:     []string{"sh", "-c", "echo hi"},
		Limits:   limits{MemoryMax: "33554432", PidsMax: "16", CPUMax: "25000 100000"},
	}
	if cfg.Hostname != want.Hostname || cfg.Rootfs != want.Rootfs || !slices.Equal(cfg.Argv, want.Argv) || cfg.Limits != want.Limits {
		t.Fatalf("cfg = %+v, ждали %+v", cfg, want)
	}

	defaults, err := parseRun([]string{"/rootfs", "sh"})
	if err != nil {
		t.Fatal(err)
	}
	if defaults.Limits != (limits{MemoryMax: "67108864", PidsMax: "32", CPUMax: "50000 100000"}) || defaults.Hostname != "box" {
		t.Fatalf("значения по умолчанию: %+v", defaults)
	}

	for _, bad := range [][]string{
		{},
		{"/rootfs"},
		{"--pids", "0", "/rootfs", "sh"},
		{"--hostname", "", "/rootfs", "sh"},
		{"--mem", "много", "/rootfs", "sh"},
		{"--нет-такого", "/rootfs", "sh"},
	} {
		if _, err := parseRun(bad); err == nil {
			t.Errorf("parseRun(%q): ждали ошибку", bad)
		}
	}
}
