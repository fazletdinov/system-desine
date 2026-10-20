package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSubcommand(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		handled bool
		code    int
		stdout  string
		stderr  string
	}{
		{"без аргументов — запуск сервера", nil, false, 0, "", ""},
		{"version", []string{"version"}, true, 0, "dev\n", ""},
		{"--version", []string{"--version"}, true, 0, "dev\n", ""},
		{"неизвестная команда", []string{"serve"}, true, 2, "", "неизвестная команда \"serve\"; есть: version, healthcheck"},
		{"healthcheck без сервера", []string{"healthcheck"}, true, 1, "", "проба"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Порт 1 никто не слушает: healthcheck обязан провалиться, а не найти чужой сервер.
			t.Setenv(envAddr, "127.0.0.1:1")
			var out, errOut bytes.Buffer
			code, handled := subcommand(c.args, &out, &errOut)
			if handled != c.handled || code != c.code || out.String() != c.stdout || !strings.Contains(errOut.String(), c.stderr) {
				t.Fatalf("code=%d handled=%v stdout=%q stderr=%q", code, handled, out.String(), errOut.String())
			}
		})
	}
}
