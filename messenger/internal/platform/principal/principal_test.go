package principal_test

import (
	"context"
	"testing"

	"messenger/internal/platform/principal"
)

func TestUserIDFrom(t *testing.T) {
	tests := []struct {
		name   string
		ctx    context.Context
		wantID string
		wantOK bool
	}{
		{"пустой контекст — личности нет", context.Background(), "", false},
		{"id положен — читается", principal.WithUserID(context.Background(), "u"), "u", true},
		{"пустая строка — личности нет", principal.WithUserID(context.Background(), ""), "", false},
		{"строка-ключ с тем же именем не подменяет", context.WithValue(context.Background(), "ctxKey", "чужой"), "", false}, //nolint:staticcheck // проверяем именно ключ-строку
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := principal.UserIDFrom(tt.ctx)
			if id != tt.wantID || ok != tt.wantOK {
				t.Fatalf("UserIDFrom = %q, %v; ждали %q, %v", id, ok, tt.wantID, tt.wantOK)
			}
		})
	}
}
