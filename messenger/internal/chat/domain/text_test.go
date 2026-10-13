package domain_test

import (
	"strings"
	"testing"

	"messenger/internal/chat/domain"
)

func TestNewText(t *testing.T) {
	t.Parallel()
	// #region text-table
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "обычный текст", in: "привет", want: "привет"},
		{name: "пробелы по краям обрезаются", in: "  привет \n", want: "привет"},
		{name: "пустая строка", in: "", wantErr: true},
		{name: "только пробелы", in: " \t\n ", wantErr: true},
		{name: "ровно лимит кириллицей", in: strings.Repeat("я", domain.TextMaxLen), want: strings.Repeat("я", domain.TextMaxLen)},
		{name: "лимит + 1 кириллицей", in: strings.Repeat("я", domain.TextMaxLen+1), wantErr: true},
		{name: "ровно лимит эмодзи", in: strings.Repeat("🙂", domain.TextMaxLen), want: strings.Repeat("🙂", domain.TextMaxLen)},
		{name: "лимит + 1 эмодзи", in: strings.Repeat("🙂", domain.TextMaxLen+1), wantErr: true},
		{name: "лимит считается после обрезки", in: "  " + strings.Repeat("я", domain.TextMaxLen) + "\n", want: strings.Repeat("я", domain.TextMaxLen)},
		{name: "только пробелы Unicode", in: "\u00a0\u3000", wantErr: true},
		{name: "невалидный UTF-8", in: "при\xffвет", wantErr: true},
		{name: "переводы строк и табуляция внутри", in: "строка 1\r\n\tстрока 2", want: "строка 1\r\n\tстрока 2"},
		{name: "управляющий символ NUL", in: "при\x00вет", wantErr: true},
	}
	// #endregion text-table
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.NewText(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewText(%q): ждали ошибку, получили %q", tt.in, got.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("NewText(%q): неожиданная ошибка %v", tt.in, err)
			}
			if got.String() != tt.want {
				t.Fatalf("NewText(%q) = %q, ждали %q", tt.in, got.String(), tt.want)
			}
		})
	}
}

func TestNewTitle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "один символ", in: "Я", want: "Я"},
		{name: "пробелы по краям обрезаются", in: "  Семья ", want: "Семья"},
		{name: "ровно лимит", in: strings.Repeat("ж", domain.TitleMaxLen), want: strings.Repeat("ж", domain.TitleMaxLen)},
		{name: "лимит + 1", in: strings.Repeat("ж", domain.TitleMaxLen+1), wantErr: true},
		{name: "пустое", in: "   ", wantErr: true},
		{name: "перевод строки внутри", in: "Се\nмья", wantErr: true},
		{name: "невалидный UTF-8", in: "Се\xffмья", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.NewTitle(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewTitle(%q): ждали ошибку, получили %q", tt.in, got.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("NewTitle(%q): неожиданная ошибка %v", tt.in, err)
			}
			if got.String() != tt.want || got.IsZero() {
				t.Fatalf("NewTitle(%q) = %q, ждали %q", tt.in, got.String(), tt.want)
			}
		})
	}
}

func TestZeroValuesAreZero(t *testing.T) {
	t.Parallel()
	if !(domain.Text{}).IsZero() || !(domain.Title{}).IsZero() || !(domain.ChatID{}).IsZero() ||
		!(domain.MessageID{}).IsZero() || !(domain.UserID{}).IsZero() {
		t.Fatal("нулевые значения должны сообщать IsZero() == true")
	}
}
