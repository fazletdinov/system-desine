package httpjson

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
)

// MaxBodyBytes — лимит тела запроса. Сообщение — до 4096 символов (≤ 16 КиБ в UTF-8),
// группа — до 200 id по 36 байт; 1 МиБ — с большим запасом, но не даёт забить память.
const MaxBodyBytes = 1 << 20

// DecodeError — ошибка разбора тела; Status — HTTP-статус, который стоит вернуть.
type DecodeError struct {
	Status int
	Code   string
	Msg    string
}

func (e *DecodeError) Error() string { return e.Msg }

// #region decode
// Decode читает из тела ровно один JSON-объект в dst. Запрос без Content-Type пропускаем
// сознательно: заголовок необязателен (RFC 9110, §8.3 разрешает получателю самому
// определить тип), а строгий декодер ниже всё равно отвергнет всё, что не JSON.
// Явно чужой тип — например, application/x-www-form-urlencoded, который curl -d ставит
// по умолчанию, — получает 415: клиент ошибся, и лучше сказать ему об этом прямо.
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	if ct := r.Header.Get("Content-Type"); ct != "" {
		mt, _, err := mime.ParseMediaType(ct)
		if err != nil || mt != "application/json" {
			return &DecodeError{Status: http.StatusUnsupportedMediaType, Code: "unsupported_media_type", Msg: "нужен Content-Type: application/json"}
		}
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return decodeError(err)
	}
	// Второй объект в теле — почти всегда ошибка клиента; молча игнорировать его нельзя.
	// Лимит может кончиться и здесь, на «хвосте» после объекта, — тогда это 413, а не 400.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return decodeError(err)
		}
		return &DecodeError{Status: http.StatusBadRequest, Code: "bad_json", Msg: "в теле должен быть ровно один JSON-объект"}
	}
	return nil
}

func decodeError(err error) *DecodeError {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return &DecodeError{Status: http.StatusRequestEntityTooLarge, Code: "body_too_large", Msg: fmt.Sprintf("тело больше %d байт", MaxBodyBytes)}
	}
	if errors.Is(err, io.EOF) {
		return &DecodeError{Status: http.StatusBadRequest, Code: "bad_json", Msg: "пустое тело"}
	}
	return &DecodeError{Status: http.StatusBadRequest, Code: "bad_json", Msg: "некорректный JSON: " + err.Error()}
}

// #endregion decode

// Write пишет v как JSON. Ошибку кодирования возвращает вызывающему: заголовки уже
// отправлены, исправить ответ нельзя — остаётся только залогировать.
func Write(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return fmt.Errorf("httpjson: запись ответа: %w", err)
	}
	return nil
}

// errorBody — единый формат ошибки для всего API: {"error":{"code","message","field"}}.
// Клиент ветвится по code (стабильный машинный ключ), message — для человека и может
// меняться, field — только у ошибок валидации.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// WriteError пишет ошибку в едином формате. Пустой field в ответ не попадает.
func WriteError(w http.ResponseWriter, status int, code, msg, field string) error {
	return Write(w, status, errorBody{Error: errorDetail{Code: code, Message: msg, Field: field}})
}
