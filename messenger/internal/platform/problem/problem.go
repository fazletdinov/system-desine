package problem

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"time"

	"messenger/internal/platform/middleware"
)

const (
	ContentType = "application/problem+json"
	// TypeBase — начало URI типа проблемы. Домен .example зарезервирован под примеры
	// (RFC 2606); в продакшене здесь адрес страницы документации об этой ошибке.
	TypeBase = "https://messenger.example/problems/"
)

// Code — стабильный машинный код проблемы: клиент ветвится по нему, а не по тексту.
// Новый код — это изменение контракта: константа, строка в registry и значение в
// ProblemCode в api/openapi.yaml (тест сверяет).
type Code string

const (
	CodeInvalidInput          Code = "invalid_input"
	CodeBadJSON               Code = "bad_json"
	CodeUnsupportedMediaType  Code = "unsupported_media_type"
	CodeBodyTooLarge          Code = "body_too_large"
	CodeUnauthenticated       Code = "unauthenticated"
	CodeInvalidCredentials    Code = "invalid_credentials"
	CodeNotMember             Code = "not_member"
	CodeNotFound              Code = "not_found"
	CodeMethodNotAllowed      Code = "method_not_allowed"
	CodeUsernameTaken         Code = "username_taken"
	CodeConflict              Code = "conflict"
	CodeClientMessageIDReused Code = "client_message_id_reused"
	CodeIdempotencyInProgress Code = "idempotency_in_progress"
	CodeIdempotencyKeyReused  Code = "idempotency_key_reused"
	CodeTimeout               Code = "timeout"
	CodeOverloaded            Code = "overloaded"
	CodeInternal              Code = "internal"
)

// title по RFC 9457 не меняется от случая к случаю — поэтому он в реестре, а подробности
// конкретного случая идут в detail.
type spec struct {
	status int
	title  string
}

// #region registry
var registry = map[Code]spec{
	CodeInvalidInput:          {http.StatusBadRequest, "Некорректные данные"},
	CodeBadJSON:               {http.StatusBadRequest, "Некорректный JSON"},
	CodeUnsupportedMediaType:  {http.StatusUnsupportedMediaType, "Неподдерживаемый формат тела"},
	CodeBodyTooLarge:          {http.StatusRequestEntityTooLarge, "Слишком большое тело запроса"},
	CodeUnauthenticated:       {http.StatusUnauthorized, "Не удалось определить пользователя"},
	CodeInvalidCredentials:    {http.StatusUnauthorized, "Неверное имя пользователя или пароль"},
	CodeNotMember:             {http.StatusForbidden, "Нет доступа к чату"},
	CodeNotFound:              {http.StatusNotFound, "Не найдено"},
	CodeMethodNotAllowed:      {http.StatusMethodNotAllowed, "Метод не поддерживается"},
	CodeUsernameTaken:         {http.StatusConflict, "Имя пользователя занято"},
	CodeConflict:              {http.StatusConflict, "Конфликт изменений"},
	CodeClientMessageIDReused: {http.StatusConflict, "client_message_id уже использован"},
	CodeIdempotencyInProgress: {http.StatusConflict, "Запрос с этим ключом ещё выполняется"},
	CodeIdempotencyKeyReused:  {http.StatusUnprocessableEntity, "Ключ идемпотентности использован для другого запроса"},
	CodeTimeout:               {http.StatusServiceUnavailable, "Сервер не успел ответить"},
	CodeOverloaded:            {http.StatusServiceUnavailable, "Сервер перегружен"},
	CodeInternal:              {http.StatusInternalServerError, "Внутренняя ошибка сервера"},
}

// #endregion registry

func Codes() []Code { return slices.Sorted(maps.Keys(registry)) }

type InvalidParam struct {
	Name   string
	Reason string
}

type Problem struct {
	Code          Code
	Detail        string
	InvalidParams []InvalidParam
	// RetryAfter > 0 — через сколько клиенту стоит повторить (заголовок Retry-After).
	// Это не поле тела: прокси и клиентские библиотеки читают именно заголовок.
	RetryAfter time.Duration
}

// Invalid — проблема с одним неверным полем или параметром; name — имя из контракта.
func Invalid(name, reason string) Problem {
	return Problem{Code: CodeInvalidInput, Detail: reason, InvalidParams: []InvalidParam{{Name: name, Reason: reason}}}
}

// #region wire
// wire — форма на проводе: поля RFC 9457 и наши расширения (code, request_id,
// invalid_params). Расширения RFC разрешает; клиенты обязаны игнорировать незнакомые.
type wire struct {
	Type          string      `json:"type"`
	Title         string      `json:"title"`
	Status        int         `json:"status"`
	Detail        string      `json:"detail,omitempty"`
	Instance      string      `json:"instance"`
	Code          Code        `json:"code"`
	RequestID     string      `json:"request_id,omitempty"`
	InvalidParams []wireParam `json:"invalid_params,omitempty"`
}

type wireParam struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// #endregion wire

// #region write
// Write пишет проблему. Незарегистрированный код — ошибка программиста: клиент получит
// честный 500, а вызывающий — ошибку для журнала.
func Write(w http.ResponseWriter, r *http.Request, p Problem) error {
	s, ok := registry[p.Code]
	var unknown error
	if !ok {
		unknown = fmt.Errorf("problem: незарегистрированный код %q", p.Code)
		p, s = Problem{Code: CodeInternal}, registry[CodeInternal]
	}
	body := wire{
		Type: TypeBase + string(p.Code), Title: s.title, Status: s.status, Detail: p.Detail,
		Instance: r.URL.Path, Code: p.Code, RequestID: middleware.RequestIDFrom(r.Context()),
	}
	for _, ip := range p.InvalidParams {
		body.InvalidParams = append(body.InvalidParams, wireParam(ip))
	}
	w.Header().Set("Content-Type", ContentType)
	if p.RetryAfter > 0 {
		w.Header().Set("Retry-After", retryAfterSeconds(p.RetryAfter))
	}
	w.WriteHeader(s.status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		return fmt.Errorf("problem: запись ответа: %w", err)
	}
	return unknown
}

// #endregion write

// retryAfterSeconds — целые секунды (RFC 9110 §10.2.3 не допускает долей) с округлением
// вверх: клиент, повторивший раньше обещанного, скорее всего снова получил бы отказ.
func retryAfterSeconds(d time.Duration) string {
	return strconv.FormatInt(int64((d+time.Second-1)/time.Second), 10)
}
