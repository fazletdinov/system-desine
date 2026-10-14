package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"messenger/internal/chat/adapters/httpapi/gen"
	"messenger/internal/chat/domain"
	"messenger/internal/platform/httperror"
)

// writeError отдаёт ошибку общей политике (разбор тела, контекст, 500), а свои
// строки таблицы — валидацию и сигнальные ошибки контекста chat — передаёт ей в problemFor.
func writeError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	httperror.Write(w, r, log, err, problemFor)
}

// #region error-map
// problemFor — строки таблицы, которые знает только контекст chat.
func problemFor(err error) (httperror.Problem, bool) {
	var invalid *domain.ValidationError
	switch {
	case errors.As(err, &invalid):
		return httperror.Problem{Status: http.StatusBadRequest, Code: "invalid_input", Message: invalid.Reason, Field: invalid.Field}, true
	case errors.Is(err, domain.ErrChatNotFound):
		return httperror.Problem{Status: http.StatusNotFound, Code: "not_found", Message: domain.ErrChatNotFound.Error()}, true
	case errors.Is(err, domain.ErrNotMember):
		return httperror.Problem{Status: http.StatusForbidden, Code: "not_member", Message: domain.ErrNotMember.Error()}, true
	case errors.Is(err, domain.ErrConcurrentUpdate):
		return httperror.Problem{Status: http.StatusConflict, Code: "conflict", Message: domain.ErrConcurrentUpdate.Error()}, true
	default:
		return httperror.Problem{}, false
	}
}

// #endregion error-map

// #region param-error
// paramError — ErrorHandlerFunc сгенерированной обёртки: параметр пути или строки
// запроса не разобрался. Ошибки генератора переводим в доменную ValidationError,
// чтобы ответ был в общем формате, а поле называлось так же, как в контракте.
func (h *Handler) paramError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		format   *gen.InvalidParamFormatError
		required *gen.RequiredParamError
		tooMany  *gen.TooManyValuesForParamError
	)
	invalid := &domain.ValidationError{Reason: "некорректные параметры запроса"}
	switch {
	case errors.As(err, &format):
		invalid.Field, invalid.Reason = format.ParamName, "неверный формат"
	case errors.As(err, &required):
		invalid.Field, invalid.Reason = required.ParamName, "обязательный параметр"
	case errors.As(err, &tooMany):
		invalid.Field, invalid.Reason = tooMany.ParamName, "неверный формат"
	}
	writeError(w, r, h.log, invalid)
}

// #endregion param-error
