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
		{name: "невалидный UTF-8", in: "при\xffвет", wantErr: true},
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
