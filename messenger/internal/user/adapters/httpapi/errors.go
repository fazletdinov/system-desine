package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"messenger/internal/platform/httperror"
	"messenger/internal/user/domain"
)

// writeError отдаёт ошибку общей политике (разбор тела, контекст, 500), а свои
// строки таблицы — валидацию и сигнальные ошибки контекста user — передаёт ей в problemFor.
func writeError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	httperror.Write(w, r, log, err, problemFor)
}

// problemFor — строки таблицы, которые знает только контекст user.
func problemFor(err error) (httperror.Problem, bool) {
	var invalid *domain.ValidationError
	switch {
	case errors.As(err, &invalid):
		return httperror.Problem{Status: http.StatusBadRequest, Code: "invalid_input", Message: invalid.Reason, Field: invalid.Field}, true
	case errors.Is(err, domain.ErrUserNotFound):
		return httperror.Problem{Status: http.StatusNotFound, Code: "not_found", Message: domain.ErrUserNotFound.Error()}, true
	case errors.Is(err, domain.ErrUsernameTaken):
		return httperror.Problem{Status: http.StatusConflict, Code: "username_taken", Message: domain.ErrUsernameTaken.Error()}, true
	default:
		return httperror.Problem{}, false
	}
}
