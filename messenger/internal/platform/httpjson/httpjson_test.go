package httpjson_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"messenger/internal/platform/httpjson"
)

type payload struct {
	Text string `json:"text"`
}

func TestDecode(t *testing.T) {
	// Тело, которое больше лимита уже на первом объекте.
	huge := `{"text":"` + strings.Repeat("a", httpjson.MaxBodyBytes) + `"}`
	// Первый объект укладывается в лимит, лимит кончается на «хвосте» после него:
	// проверяет, что второй Decode тоже узнаёт MaxBytesError.
	hugeTail := `{"text":"a"}` + strings.Repeat(" ", httpjson.MaxBodyBytes)

	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int // 0 — ошибки нет
		wantCode    string
	}{
		{name: "корректное тело", contentType: "application/json", body: `{"text":"привет"}`},
		{name: "charset в Content-Type", contentType: "application/json; charset=utf-8", body: `{"text":"a"}`},
		{name: "без Content-Type — пропускаем сознательно", body: `{"text":"a"}`},
		{name: "пробелы после объекта", contentType: "application/json", body: "{\"text\":\"a\"}\n  \n"},
		{name: "тело больше лимита", contentType: "application/json", body: huge, wantStatus: http.StatusRequestEntityTooLarge, wantCode: "body_too_large"},
		{name: "лимит кончился после первого объекта", contentType: "application/json", body: hugeTail, wantStatus: http.StatusRequestEntityTooLarge, wantCode: "body_too_large"},
		{name: "неизвестное поле", contentType: "application/json", body: `{"text":"a","admin":true}`, wantStatus: http.StatusBadRequest, wantCode: "bad_json"},
		{name: "два объекта подряд", contentType: "application/json", body: `{"text":"a"}{"text":"b"}`, wantStatus: http.StatusBadRequest, wantCode: "bad_json"},
		{name: "мусор после объекта", contentType: "application/json", body: `{"text":"a"} x`, wantStatus: http.StatusBadRequest, wantCode: "bad_json"},
		{name: "пустое тело", contentType: "application/json", body: "", wantStatus: http.StatusBadRequest, wantCode: "bad_json"},
		{name: "не тот тип поля", contentType: "application/json", body: `{"text":42}`, wantStatus: http.StatusBadRequest, wantCode: "bad_json"},
		{name: "text/plain", contentType: "text/plain", body: `{"text":"a"}`, wantStatus: http.StatusUnsupportedMediaType, wantCode: "unsupported_media_type"},
		{name: "кривой Content-Type", contentType: "application/json; ;", body: `{"text":"a"}`, wantStatus: http.StatusUnsupportedMediaType, wantCode: "unsupported_media_type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(tt.body))
			if tt.contentType != "" {
				r.Header.Set("Content-Type", tt.contentType)
			}
			var dst payload
			err := httpjson.Decode(httptest.NewRecorder(), r, &dst)

			if tt.wantStatus == 0 {
				if err != nil {
					t.Fatalf("Decode: %v, want nil", err)
				}
				if dst.Text == "" {
					t.Fatal("поле text не заполнено")
				}
				return
			}
			var de *httpjson.DecodeError
			if !errors.As(err, &de) {
				t.Fatalf("Decode: %v (%T), want *DecodeError", err, err)
			}
			if de.Status != tt.wantStatus || de.Code != tt.wantCode {
				t.Errorf("Decode: status %d code %q, want %d %q", de.Status, de.Code, tt.wantStatus, tt.wantCode)
			}
		})
	}
}

func TestWrite(t *testing.T) {
	w := httptest.NewRecorder()
	if err := httpjson.Write(w, http.StatusCreated, payload{Text: "привет"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if w.Code != http.StatusCreated {
		t.Errorf("status %d, want %d", w.Code, http.StatusCreated)
	}
	if got, want := w.Header().Get("Content-Type"), "application/json; charset=utf-8"; got != want {
		t.Errorf("Content-Type %q, want %q", got, want)
	}
	if got, want := w.Body.String(), "{\"text\":\"привет\"}\n"; got != want {
		t.Errorf("body %q, want %q", got, want)
	}
}

func TestWriteUnencodable(t *testing.T) {
	w := httptest.NewRecorder()
	if err := httpjson.Write(w, http.StatusOK, map[string]any{"ch": make(chan int)}); err == nil {
		t.Fatal("Write: want error for unencodable value")
	}
}

// richPayload — DTO с полями разных типов: сообщения об ошибке типа должны говорить
// на языке JSON, а не Go.
type richPayload struct {
	Text      string   `json:"text"`
	Seq       int64    `json:"seq"`
	MemberIDs []string `json:"member_ids"`
}

// TestDecodeMessages проверяет, что клиент получает наш текст: без имён Go-типов,
// без имени DTO и без префикса «json:» из encoding/json.
func TestDecodeMessages(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantMsg   string
		wantField string
	}{
		{name: "пустое тело", body: "", wantMsg: "пустое тело"},
		{name: "оборванный JSON", body: `{"text":"a"`, wantMsg: "JSON оборван на середине"},
		{name: "синтаксис", body: `{"text":}`, wantMsg: "некорректный JSON: ошибка синтаксиса около байта 9"},
		{name: "число вместо строки", body: `{"text":42}`, wantMsg: "поле text: ожидается строка", wantField: "text"},
		{name: "строка вместо числа", body: `{"seq":"1"}`, wantMsg: "поле seq: ожидается число", wantField: "seq"},
		{name: "объект вместо массива", body: `{"member_ids":{}}`, wantMsg: "поле member_ids: ожидается массив", wantField: "member_ids"},
		{name: "число вместо строки в массиве", body: `{"member_ids":[1]}`, wantMsg: "поле member_ids: ожидается строка", wantField: "member_ids"},
		{name: "массив вместо объекта", body: `[]`, wantMsg: "тело должно быть JSON-объектом"},
		{name: "неизвестное поле", body: `{"text":"a","author_id":"x"}`, wantMsg: `неизвестное поле "author_id"`, wantField: "author_id"},
		{name: "второй объект", body: `{"text":"a"}{"text":"b"}`, wantMsg: "в теле должен быть ровно один JSON-объект"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(tt.body))
			r.Header.Set("Content-Type", "application/json")
			var dst richPayload
			err := httpjson.Decode(httptest.NewRecorder(), r, &dst)

			var de *httpjson.DecodeError
			if !errors.As(err, &de) {
				t.Fatalf("Decode: %v (%T), want *DecodeError", err, err)
			}
			if de.Msg != tt.wantMsg || de.Field != tt.wantField {
				t.Errorf("Msg %q Field %q, want %q %q", de.Msg, de.Field, tt.wantMsg, tt.wantField)
			}
			for _, leak := range []string{"Go", "struct", "richPayload", "json:", "int64", "[]string"} {
				if strings.Contains(de.Msg, leak) {
					t.Errorf("Msg %q раскрывает внутреннее: %q", de.Msg, leak)
				}
			}
		})
	}
}
