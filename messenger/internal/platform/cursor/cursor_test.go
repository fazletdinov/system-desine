package cursor_test

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"messenger/internal/platform/cursor"
)

type position struct {
	Seq int64 `json:"seq"`
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	s, err := cursor.Encode("messages", position{Seq: 42})
	if err != nil {
		t.Fatal(err)
	}
	// Курсор идёт в строку запроса как есть: только символы base64url, без '='.
	if strings.ContainsAny(s, "+/=") {
		t.Fatalf("курсор %q требует экранирования в URL", s)
	}
	var got position
	if err := cursor.Decode(s, "messages", &got); err != nil {
		t.Fatal(err)
	}
	if got.Seq != 42 {
		t.Fatalf("seq %d, ждали 42", got.Seq)
	}
}

func TestDecodeRejects(t *testing.T) {
	t.Parallel()
	raw := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	valid, err := cursor.Encode("messages", position{Seq: 1})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		in   string
	}{
		{name: "курсор другого списка", in: mustEncode(t, "chats", position{Seq: 1})},
		{name: "другая версия формата", in: raw(`{"v":2,"k":"messages","p":{"seq":1}}`)},
		{name: "не base64", in: "!!!" + valid},
		{name: "base64, но не JSON", in: raw("seq=1")},
		{name: "лишнее поле в конверте", in: raw(`{"v":1,"k":"messages","p":{"seq":1},"x":1}`)},
		{name: "лишнее поле в данных", in: raw(`{"v":1,"k":"messages","p":{"seq":1,"admin":true}}`)},
		{name: "хвост после JSON", in: raw(`{"v":1,"k":"messages","p":{"seq":1}} {}`)},
		{name: "данные не того типа", in: raw(`{"v":1,"k":"messages","p":{"seq":"1"}}`)},
		{name: "пустая строка", in: ""},
		{name: "длиннее MaxLen", in: strings.Repeat("A", cursor.MaxLen+1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var p position
			if err := cursor.Decode(tt.in, "messages", &p); !errors.Is(err, cursor.ErrInvalid) {
				t.Fatalf("ждали ErrInvalid, получили %v", err)
			}
		})
	}
}

func TestEncodeUnsupportedPayload(t *testing.T) {
	t.Parallel()
	if _, err := cursor.Encode("messages", func() {}); err == nil {
		t.Fatal("функцию не закодировать в JSON — ждали ошибку")
	}
}

func mustEncode(t *testing.T, kind string, payload any) string {
	t.Helper()
	s, err := cursor.Encode(kind, payload)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
