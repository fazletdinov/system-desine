package ident_test

import (
	"testing"

	"messenger/internal/kernel/ident"
)

func TestIsUUID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "канонический UUIDv7", in: "0192f0c1-7e3a-7b4c-8d5e-6f7a8b9c0d1e", want: true},
		{name: "UUIDv4: версию не проверяем", in: "c66c8f09-dcc4-4bb7-ad21-7297aca0b278", want: true},
		{name: "верхний регистр", in: "0192F0C1-7E3A-7B4C-8D5E-6F7A8B9C0D1E"},
		{name: "без дефисов", in: "0192f0c17e3a7b4c8d5e6f7a8b9c0d1e"},
		{name: "35 символов", in: "0192f0c1-7e3a-7b4c-8d5e-6f7a8b9c0d1"},
		{name: "37 символов", in: "0192f0c1-7e3a-7b4c-8d5e-6f7a8b9c0d1ee"},
		{name: "буква g", in: "0192f0c1-7e3a-7b4c-8d5e-6f7a8b9c0d1g"},
		{name: "дефис не на месте", in: "0192f0c17-e3a-7b4c-8d5e-6f7a8b9c0d1e"},
		// «а» — кириллица: два байта UTF-8, поэтому и длина в байтах другая.
		{name: "кириллица", in: "0192f0c1-7e3a-7b4c-8d5e-6f7a8b9c0dа"},
		{name: "пустая строка", in: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ident.IsUUID(tt.in); got != tt.want {
				t.Fatalf("IsUUID(%q) = %v, ждали %v", tt.in, got, tt.want)
			}
		})
	}
}
