package domain_test

import (
	"strings"
	"testing"
	"time"

	"messenger/internal/user/domain"
)

func TestNewUsername(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{name: "короче минимума", in: "ab", wantErr: true},
		{name: "ровно минимум", in: "abc"},
		{name: "ровно максимум", in: strings.Repeat("a", domain.UsernameMaxLen)},
		{name: "максимум + 1", in: strings.Repeat("a", domain.UsernameMaxLen+1), wantErr: true},
		{name: "верхний регистр", in: "Ivan", wantErr: true},
		{name: "кириллица", in: "иван", wantErr: true},
		{name: "цифры и подчёркивание", in: "ivan_99"},
		{name: "дефис", in: "ivan-99", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.NewUsername(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewUsername(%q): ждали ошибку, получили %q", tt.in, got.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("NewUsername(%q): неожиданная ошибка %v", tt.in, err)
			}
			if got.String() != tt.in {
				t.Fatalf("NewUsername(%q) = %q", tt.in, got.String())
			}
		})
	}
}

func TestNewDisplayName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "пробелы по краям обрезаются", in: "  Иван  ", want: "Иван"},
		{name: "пустое", in: "", wantErr: true},
		{name: "ровно максимум рун", in: strings.Repeat("ж", domain.DisplayNameMaxLen), want: strings.Repeat("ж", domain.DisplayNameMaxLen)},
		{name: "максимум + 1", in: strings.Repeat("ж", domain.DisplayNameMaxLen+1), wantErr: true},
		{name: "управляющий символ", in: "a\x00b", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.NewDisplayName(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewDisplayName(%q): ждали ошибку, получили %q", tt.in, got.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("NewDisplayName(%q): неожиданная ошибка %v", tt.in, err)
			}
			if got.String() != tt.want {
				t.Fatalf("NewDisplayName(%q) = %q, ждали %q", tt.in, got.String(), tt.want)
			}
		})
	}
}

func TestParseUserID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{name: "канонический UUID", in: "0192f0c1-7e3a-7b4c-8d5e-6f7a8b9c0d1e"},
		{name: "верхний регистр", in: "0192F0C1-7E3A-7B4C-8D5E-6F7A8B9C0D1E", wantErr: true},
		{name: "35 символов", in: "0192f0c1-7e3a-7b4c-8d5e-6f7a8b9c0d1", wantErr: true},
		{name: "без дефисов", in: "0192f0c17e3a7b4c8d5e6f7a8b9c0d1e", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.ParseUserID(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseUserID(%q): ждали ошибку, получили %q", tt.in, got.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseUserID(%q): неожиданная ошибка %v", tt.in, err)
			}
			if got.String() != tt.in || got.IsZero() {
				t.Fatalf("ParseUserID(%q) = %q", tt.in, got.String())
			}
		})
	}
}

func TestZeroUserIDIsZero(t *testing.T) {
	t.Parallel()
	var id domain.UserID
	if !id.IsZero() {
		t.Fatal("нулевое значение UserID должно сообщать IsZero() == true")
	}
}

func TestNewUserNormalizesTimeToUTC(t *testing.T) {
	t.Parallel()
	id, err := domain.ParseUserID("0192f0c1-7e3a-7b4c-8d5e-6f7a8b9c0d1e")
	if err != nil {
		t.Fatal(err)
	}
	name, err := domain.NewUsername("ivan")
	if err != nil {
		t.Fatal(err)
	}
	display, err := domain.NewDisplayName("Иван")
	if err != nil {
		t.Fatal(err)
	}
	msk := time.FixedZone("MSK", 3*60*60)
	created := time.Date(2026, 10, 13, 15, 0, 0, 0, msk)
	u := domain.NewUser(id, name, display, created)
	if u.ID() != id || u.Username() != name || u.DisplayName() != display {
		t.Fatal("NewUser потерял поля")
	}
	if u.CreatedAt().Location() != time.UTC || !u.CreatedAt().Equal(created) {
		t.Fatalf("CreatedAt = %v, ждали тот же момент в UTC", u.CreatedAt())
	}
}
