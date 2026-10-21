package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"messenger/internal/platform/pgtest"
	"messenger/migrations"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Start(m)) }

// setDatabaseURL кладёт строку подключения в файл и указывает на него переменной —
// так же, как compose передаёт секрет db_url.
func setDatabaseURL(t *testing.T, dsn string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "db_url")
	if err := os.WriteFile(file, []byte(dsn+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envDatabaseURLFile, file)
}

// scratchDatabase создаёт в контейнере pgtest пустую базу без миграций (снимок pgtest
// уже мигрирован, а здесь проверяется путь «с нуля») и возвращает строку подключения к ней.
func scratchDatabase(t *testing.T, name string) string {
	t.Helper()
	pool := pgtest.Pool(t)
	ident := pgx.Identifier{name}.Sanitize()
	if _, err := pool.Exec(t.Context(), "CREATE DATABASE "+ident); err != nil {
		t.Fatal(err)
	}
	// Удаляем сами: Restore пересоздаёт только базу pgtest, а с -count=2 повторный CREATE
	// упал бы на «уже есть». t.Context() к Cleanup уже отменён.
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DROP DATABASE "+ident+" WITH (FORCE)"); err != nil {
			t.Errorf("удаление базы %s: %v", name, err)
		}
	})
	c := pool.Config().ConnConfig
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.User, c.Password),
		Host:     net.JoinHostPort(c.Host, strconv.Itoa(int(c.Port))),
		Path:     "/" + name,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

func migrateCmd(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = runMigrate(t.Context(), args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func mustContain(t *testing.T, got string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Fatalf("нет %q в выводе:\n%s", w, got)
		}
	}
}

func TestMigrateUpFromScratch(t *testing.T) {
	dsn := scratchDatabase(t, "scratch_up")
	setDatabaseURL(t, dsn)

	code, out, errOut := migrateCmd(t, "status")
	if code != 0 {
		t.Fatalf("status до up: код %d, stderr %s", code, errOut)
	}
	mustContain(t, out, "ожидает     00001_init.sql", "версия схемы: 0, ожидают применения: 1")

	code, out, errOut = migrateCmd(t, "up")
	if code != 0 {
		t.Fatalf("up: код %d, stderr %s", code, errOut)
	}
	mustContain(t, out, "OK    up 00001_init.sql", fmt.Sprintf("версия схемы: %d", migrations.Required))

	code, out, errOut = migrateCmd(t, "status")
	if code != 0 {
		t.Fatalf("status после up: код %d, stderr %s", code, errOut)
	}
	mustContain(t, out, "применена   00001_init.sql", "ожидают применения: 0")
	if strings.Contains(out, "ожидает ") {
		t.Fatalf("после up остались неприменённые миграции:\n%s", out)
	}

	// Сервер читает версию своим пулом, а не через goose, — она должна совпасть.
	pool, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := checkSchema(t.Context(), pool); err != nil {
		t.Fatalf("checkSchema после up: %v", err)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	setDatabaseURL(t, scratchDatabase(t, "scratch_twice"))
	if code, _, errOut := migrateCmd(t, "up"); code != 0 {
		t.Fatalf("первый up: код %d, stderr %s", code, errOut)
	}
	code, out, errOut := migrateCmd(t, "up")
	if code != 0 {
		t.Fatalf("второй up: код %d, stderr %s", code, errOut)
	}
	mustContain(t, out, "применять нечего", fmt.Sprintf("версия схемы: %d", migrations.Required))
	if strings.Contains(out, "OK ") {
		t.Fatalf("второй up что-то применил:\n%s", out)
	}
}

func TestStartRefusesOutdatedSchema(t *testing.T) {
	cases := []struct {
		name    string
		prepare string // SQL поверх базы из снимка (все миграции применены)
		wantErr string // "" — сервер стартует
	}{
		{"схема актуальна", "", ""},
		{"миграция не применена", fmt.Sprintf("DELETE FROM goose_db_version WHERE version_id >= %d", migrations.Required),
			fmt.Sprintf("версия схемы 0, бинарю нужна %d — примените миграции", migrations.Required)},
		{"миграции не применялись", "DROP TABLE goose_db_version",
			fmt.Sprintf("версия схемы 0, бинарю нужна %d — примените миграции", migrations.Required)},
		{"база новее бинаря", fmt.Sprintf("INSERT INTO goose_db_version (version_id, is_applied) VALUES (%d, true)", migrations.Required+1), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pool := pgtest.Pool(t)
			if c.prepare != "" {
				if _, err := pool.Exec(t.Context(), c.prepare); err != nil {
					t.Fatal(err)
				}
			}
			err := checkSchema(t.Context(), pool)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("checkSchema: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("checkSchema: %v, ожидалось %q", err, c.wantErr)
			}
		})
	}
}

// Ошибки до похода в базу — без Docker. Пароль из строки подключения не должен попасть
// ни в одну из них: вывод migrate уходит в журнал деплоя.
func TestMigrateConfigErrors(t *testing.T) {
	const password = "s3cr3t-пароль"
	cases := []struct {
		name    string
		args    []string
		dsn     *string // nil — переменная не задана
		code    int
		wantErr string
	}{
		{"без подкоманды", nil, nil, 2, migrateUsage},
		{"неизвестная подкоманда", []string{"down"}, nil, 2, migrateUsage},
		{"нет переменной", []string{"up"}, nil, 1, "задайте " + envDatabaseURLFile},
		{"пустой файл", []string{"up"}, new(""), 1, "пуст"},
		{"строка не разбирается", []string{"status"}, new("postgres://app:" + password + "@db:порт/messenger"), 1, "строка подключения не разобрана"},
		// Порт 1 никто не слушает: соединение отвергнуто сразу, без ожидания срока.
		{"база недоступна", []string{"up"}, new("postgres://app:" + password + "@127.0.0.1:1/messenger?sslmode=disable"), 1, "миграции:"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(envDatabaseURLFile, "")
			if c.dsn != nil {
				setDatabaseURL(t, *c.dsn)
			}
			code, out, errOut := migrateCmd(t, c.args...)
			if code != c.code || !strings.Contains(errOut, c.wantErr) {
				t.Fatalf("код %d, stderr %q; ожидались %d и %q", code, errOut, c.code, c.wantErr)
			}
			if strings.Contains(out+errOut, password) {
				t.Fatalf("пароль в выводе: %s%s", out, errOut)
			}
		})
	}
}
