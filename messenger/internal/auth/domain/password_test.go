package domain_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"messenger/internal/auth/domain"
)

func TestNewPassword(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{name: "14 символов", in: strings.Repeat("a", 14), wantErr: true},
		{name: "15 символов", in: strings.Repeat("a", 15)},
		{name: "15 кириллических: 30 байт, но 15 символов", in: strings.Repeat("я", 15)},
		{name: "128 символов", in: strings.Repeat("a", 128)},
		{name: "129 символов", in: strings.Repeat("a", 129), wantErr: true},
		// 🔑 — один символ (руна) и 4 байта: 14 букв + эмодзи = 15 символов.
		{name: "эмодзи — один символ", in: strings.Repeat("a", 14) + "🔑"},
		{name: "эмодзи не добирает до 15", in: strings.Repeat("a", 13) + "🔑", wantErr: true},
		{name: "нулевой байт", in: strings.Repeat("a", 15) + "\x00", wantErr: true},
		{name: "перевод строки", in: strings.Repeat("a", 15) + "\n", wantErr: true},
		{name: "некорректный UTF-8", in: strings.Repeat("a", 15) + "\xff", wantErr: true},
		{name: "пробелы допустимы", in: "correct horse battery staple"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := domain.NewPassword(tt.in)
			if tt.wantErr {
				var ve *domain.ValidationError
				if !errors.As(err, &ve) || ve.Field != "password" {
					t.Fatalf("ждали ValidationError по password, получили %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("неожиданная ошибка: %v", err)
			}
			if p.Plaintext() != tt.in {
				t.Fatalf("Plaintext() вернул не тот пароль")
			}
		})
	}
}

// Пароль не должен утечь через fmt: ни %v, ни %#v, ни %s, ни в составе структуры.
func TestPasswordHiddenFromFmt(t *testing.T) {
	t.Parallel()
	const secret = "очень-секретный-пароль"
	p, err := domain.NewPassword(secret)
	if err != nil {
		t.Fatal(err)
	}
	out := fmt.Sprintf("%v %#v %s %+v", p, p, p, struct{ P domain.Password }{p})
	if strings.Contains(out, secret) {
		t.Fatalf("пароль виден в выводе fmt: %s", out)
	}
}
