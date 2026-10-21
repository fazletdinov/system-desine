package postgres_test

import (
	"fmt"
	"testing"

	"messenger/internal/chat/adapters/postgres"
	"messenger/internal/chat/domain"
	"messenger/internal/platform/pgtest"
)

func TestMissingUsers(t *testing.T) {
	const ghost1, ghost2 = "00000000-0000-7000-8000-000000000098", "00000000-0000-7000-8000-000000000099"
	pool := pgtest.Pool(t)
	seedUsers(t, pool, user1, user2)
	checker, err := postgres.NewUserChecker(pool)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		ids  []string
		want []string
	}{
		{"все есть", []string{user1, user2}, nil},
		{"пустой список", nil, nil},
		{"нет двоих — по порядку id", []string{ghost2, user1, ghost1}, []string{ghost1, ghost2}},
		{"повтор отсутствующего — один раз", []string{ghost1, ghost1, user2}, []string{ghost1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids := make([]domain.UserID, len(tt.ids))
			for i, s := range tt.ids {
				ids[i] = parse(t, domain.ParseUserID, s)
			}
			missing, err := checker.MissingUsers(t.Context(), ids)
			if err != nil {
				t.Fatal(err)
			}
			got := make([]string, len(missing))
			for i, id := range missing {
				got[i] = id.String()
			}
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Fatalf("MissingUsers = %v, ждали %v", got, tt.want)
			}
		})
	}
}
