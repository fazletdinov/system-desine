package httperror_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"messenger/internal/platform/httperror"
	"messenger/internal/platform/httpjson"
)

var errKnown = errors.New("известная доменная ошибка")

// known — пример таблицы контекста: узнаёт одну ошибку.
func known(err error) (httperror.Problem, bool) {
	if errors.Is(err, errKnown) {
		return httperror.Problem{Status: http.StatusConflict, Code: "known", Message: "известная", Field: "x"}, true
	}
	return httperror.Problem{}, false
}

type body struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Field   string `json:"field"`
	} `json:"error"`
}

func TestWrite(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int // 0 — ответ не пишется
		wantCode   string
		wantField  string
		wantLevel  string // уровень записи в логе; пусто — лог пуст
	}{
		{name: "ошибка разбора тела", err: &httpjson.DecodeError{Status: 413, Code: "body_too_large", Msg: "много"}, wantStatus: 413, wantCode: "body_too_large"},
		{name: "ошибка разбора с полем", err: &httpjson.DecodeError{Status: 400, Code: "bad_json", Msg: "неизвестное поле", Field: "a"}, wantStatus: 400, wantCode: "bad_json", wantField: "a"},
		{name: "таблица контекста", err: fmt.Errorf("обёртка: %w", errKnown), wantStatus: 409, wantCode: "known", wantField: "x"},
		{name: "таблица контекста важнее контекста", err: fmt.Errorf("%w: %w", errKnown, context.Canceled), wantStatus: 409, wantCode: "known", wantField: "x"},
		{name: "клиент ушёл", err: fmt.Errorf("x: %w", context.Canceled), wantLevel: "INFO"},
		{name: "дедлайн", err: fmt.Errorf("x: %w", context.DeadlineExceeded), wantStatus: 503, wantCode: "timeout"},
		{name: "прочее", err: errors.New("секрет БД"), wantStatus: 500, wantCode: "internal", wantLevel: "ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&logs, nil))
			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/chats/1/messages", nil)
			w := httptest.NewRecorder()

			httperror.Write(w, r, log, tt.err, known)

			if tt.wantStatus == 0 {
				if w.Body.Len() != 0 {
					t.Errorf("ответ записан: %s", w.Body)
				}
			} else {
				if w.Code != tt.wantStatus {
					t.Fatalf("status %d, want %d", w.Code, tt.wantStatus)
				}
				var b body
				if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
					t.Fatal(err)
				}
				if b.Error.Code != tt.wantCode || b.Error.Field != tt.wantField {
					t.Errorf("code %q field %q, want %q %q", b.Error.Code, b.Error.Field, tt.wantCode, tt.wantField)
				}
				if strings.Contains(w.Body.String(), "секрет") {
					t.Errorf("ответ раскрывает внутреннюю ошибку: %s", w.Body)
				}
			}
			if tt.wantLevel == "" {
				if logs.Len() != 0 {
					t.Errorf("лог должен быть пуст: %s", logs.String())
				}
				return
			}
			if !strings.Contains(logs.String(), `"level":"`+tt.wantLevel+`"`) || !strings.Contains(logs.String(), `"path":"/chats/1/messages"`) {
				t.Errorf("в логе нет %s с путём: %s", tt.wantLevel, logs.String())
			}
		})
	}
}
