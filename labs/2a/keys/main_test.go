package main

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseFlags(t *testing.T) {
	noEnv := func(string) string { return "" }
	envDSN := func(k string) string {
		if k == "LAB2A_DSN" {
			return "postgres://env"
		}
		return ""
	}
	tests := []struct {
		name     string
		args     []string
		getenv   func(string) string
		wantRows int
		wantDSN  string
		wantErr  bool
	}{
		{name: "умолчания — миллион строк и база compose", getenv: noEnv, wantRows: 1_000_000, wantDSN: defaultDSN},
		{name: "-n задаёт число строк", args: []string{"-n", "100000"}, getenv: noEnv, wantRows: 100_000, wantDSN: defaultDSN},
		{name: "LAB2A_DSN меняет базу по умолчанию", getenv: envDSN, wantRows: 1_000_000, wantDSN: "postgres://env"},
		{name: "-dsn важнее LAB2A_DSN", args: []string{"-dsn", "postgres://flag"}, getenv: envDSN, wantRows: 1_000_000, wantDSN: "postgres://flag"},
		{name: "ноль строк — ошибка", args: []string{"-n", "0"}, getenv: noEnv, wantErr: true},
		{name: "неизвестный флаг — ошибка", args: []string{"-x"}, getenv: noEnv, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parseFlags(tt.args, tt.getenv)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseFlags(%q) = %+v, ждали ошибку", tt.args, cfg)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseFlags(%q): %v", tt.args, err)
			}
			if cfg.rows != tt.wantRows || cfg.dsn != tt.wantDSN {
				t.Errorf("parseFlags(%q) = {rows: %d, dsn: %q}, ждали {rows: %d, dsn: %q}",
					tt.args, cfg.rows, cfg.dsn, tt.wantRows, tt.wantDSN)
			}
		})
	}
}

func TestTopUpRowsIsTenthAndAtLeastOne(t *testing.T) {
	for rows, want := range map[int]int{1_000_000: 100_000, 100_000: 10_000, 5: 1} {
		if got := topUpRows(rows); got != want {
			t.Errorf("topUpRows(%d) = %d, ждали %d", rows, got, want)
		}
	}
}

// Проверки scenario.sh читают из вывода строки-соотношения; тест держит этот контракт.
func TestReportRatioLinesParseLikeScenario(t *testing.T) {
	results := []result{
		sample("bigint", 2208*kib, 9*mib, 1*mib),
		sample("uuidv4", 4416*kib, 18*mib, 5*mib),
		sample("uuidv7", 3312*kib, 15*mib, 2*mib),
	}
	var b strings.Builder
	if err := report(&b, "18.6", results); err != nil {
		t.Fatalf("report: %v", err)
	}
	out := b.String()

	tests := []struct {
		pattern string
		want    float64
	}{
		{`uuidv4 к uuidv7: индекс ×([0-9.]+)`, 4416.0 / 3312},
		{`uuidv4 к uuidv7: .*WAL дозагрузки ×([0-9.]+)`, 5.0 / 2},
		{`uuidv7 к bigint: индекс ×([0-9.]+)`, 3312.0 / 2208},
		{`uuidv4 к bigint: .*WAL загрузки ×([0-9.]+)`, 2},
	}
	for _, tt := range tests {
		m := regexp.MustCompile(tt.pattern).FindStringSubmatch(out)
		if m == nil {
			t.Errorf("в выводе нет %q:\n%s", tt.pattern, out)
			continue
		}
		got, err := strconv.ParseFloat(m[1], 64)
		if err != nil || got < tt.want-0.01 || got > tt.want+0.01 {
			t.Errorf("%q: %s, ждали %.2f", tt.pattern, m[1], tt.want)
		}
	}
}

func TestReportTableRowStartsWithKindAndIndexKiB(t *testing.T) {
	var b strings.Builder
	if err := report(&b, "18.6", []result{sample("uuidv7", 3104*kib, mib, mib)}); err != nil {
		t.Fatalf("report: %v", err)
	}
	for line := range strings.Lines(b.String()) {
		f := strings.Fields(line)
		if len(f) > 1 && f[0] == "uuidv7" {
			if f[1] != "3104" {
				t.Errorf("второй столбец строки %q = %s, ждали размер индекса 3104 КиБ", line, f[1])
			}
			return
		}
	}
	t.Fatalf("нет строки таблицы uuidv7:\n%s", b.String())
}

func TestReportWithoutResultsFails(t *testing.T) {
	if err := report(&strings.Builder{}, "18.6", nil); err == nil {
		t.Fatal("report без замеров не вернул ошибку")
	}
}

func sample(kind string, index, loadWAL, topUpWAL int64) result {
	return result{
		kind:       kind,
		load:       phase{rows: 100_000, elapsed: 200 * time.Millisecond, walBytes: loadWAL},
		topUp:      phase{rows: 10_000, elapsed: 20 * time.Millisecond, walBytes: topUpWAL, walFPI: 7},
		tableBytes: 5120 * kib,
		indexBytes: index,
		leafPages:  index / 8192,
		leafFill:   89.8,
	}
}
