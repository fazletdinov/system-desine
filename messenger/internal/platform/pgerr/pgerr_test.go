package pgerr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"messenger/internal/platform/pgerr"
)

func TestIsUniqueViolation(t *testing.T) {
	unique := &pgconn.PgError{Code: "23505", ConstraintName: "users_username_key"}
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"то ограничение", unique, true},
		{"обёрнутая ошибка", fmt.Errorf("сохранение: %w", unique), true},
		{"другое ограничение", &pgconn.PgError{Code: "23505", ConstraintName: "users_pkey"}, false},
		{"другой код с тем же именем", &pgconn.PgError{Code: "23514", ConstraintName: "users_username_key"}, false},
		{"не ошибка PostgreSQL", errors.New("users_username_key"), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pgerr.IsUniqueViolation(tt.err, "users_username_key"); got != tt.want {
				t.Fatalf("IsUniqueViolation(%v) = %v, ждали %v", tt.err, got, tt.want)
			}
		})
	}
}
