package main

import (
	"fmt"
	"io"
)

// version подставляет сборка: go build -ldflags "-X main.version=1.2.3". По нему в
// журнале видно, какая сборка ответила, — без этого «в проде старая версия?» не проверить.
var version = "dev"

// subcommand выполняет служебную команду бинаря. handled == false — аргументов нет,
// и main запускает сервер. В образе distroless нет shell и утилит, поэтому всё, что
// нужно рядом с сервером (версия, healthcheck), умеет сам бинарь.
func subcommand(args []string, stdout, stderr io.Writer) (code int, handled bool) {
	if len(args) == 0 {
		return 0, false
	}
	switch args[0] {
	case "version", "--version":
		_, _ = fmt.Fprintln(stdout, version)
		return 0, true
	default:
		_, _ = fmt.Fprintf(stderr, "неизвестная команда %q; есть: version\n", args[0])
		return 2, true
	}
}
