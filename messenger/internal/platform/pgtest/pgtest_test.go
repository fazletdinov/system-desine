package pgtest_test

import (
	"os"
	"strings"
	"testing"

	"messenger/internal/platform/pgtest"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Start(m)) }

// Образ тестов и образ compose — одна строка в двух файлах; разойдутся — тесты будут
// проверять не ту версию Postgres, на которой работает мессенджер.
func TestImageMatchesCompose(t *testing.T) {
	compose, err := os.ReadFile("../../../compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(compose), "image: "+pgtest.Image+"\n") {
		t.Fatalf("в compose.yaml нет image: %s", pgtest.Image)
	}
}

// Каждый тест начинает с базы из снимка: строка, записанная первым, второму не видна,
// а схема (таблицы миграций) на месте.
func TestPoolStartsFromSnapshot(t *testing.T) {
	const insert = `INSERT INTO users.users (id, username, display_name, created_at)
		VALUES ('019a0000-0000-7000-8000-000000000001', 'alice', 'Алиса', now())`
	for _, name := range []string{"первый", "второй"} {
		t.Run(name, func(t *testing.T) {
			pool := pgtest.Pool(t)
			var n int
			if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM users.users`).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 0 {
				t.Fatalf("в базе из снимка %d пользователей, ожидалось 0", n)
			}
			if _, err := pool.Exec(t.Context(), insert); err != nil {
				t.Fatal(err)
			}
		})
	}
}
