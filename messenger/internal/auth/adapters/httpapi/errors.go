package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"messenger/internal/auth/adapters/httpapi/gen"
	"messenger/internal/auth/domain"
	"messenger/internal/platform/problem"
)

func writeError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	problem.Respond(w, r, log, err, problemFor)
}

func writeProblem(w http.ResponseWriter, r *http.Request, log *slog.Logger, p problem.Problem) {
	if err := problem.Write(w, r, p); err != nil {
		log.Error("запись ответа", "path", r.URL.Path, "err", err)
	}
}

// problemFor — строки таблицы, которые знает только контекст auth.
func problemFor(err error) (problem.Problem, bool) {
	var invalid *domain.ValidationError
	switch {
	case errors.As(err, &invalid):
		return problem.Invalid(invalid.Field, invalid.Reason), true
	case errors.Is(err, domain.ErrUsernameTaken):
		return problem.Problem{Code: problem.CodeUsernameTaken, Detail: domain.ErrUsernameTaken.Error()}, true
	default:
		return problem.Problem{}, false
	}
}

// rejectReason — короткое имя причины отказа во входе для журнала: по нему удобно
// считать и искать (сколько входов отклонено из-за несуществующих имён — признак перебора).
func rejectReason(err error) string {
	switch {
	case errors.Is(err, domain.ErrNoSuchUser):
		return "no_such_user"
	case errors.Is(err, domain.ErrNoCredentials):
		return "no_credentials"
	case errors.Is(err, domain.ErrWrongPassword):
		return "wrong_password"
	case errors.Is(err, domain.ErrPasswordTooLong):
		return "password_too_long"
	default:
		return "unknown"
	}
}

// paramError — ErrorHandlerFunc сгенерированной обёртки. У регистрации нет параметров
// пути и строки запроса, но обёртка требует обработчик, и его поведение — как у соседей.
// Функция повторена в каждом адаптере намеренно: у каждого gen-пакета свои типы ошибок
// (gen.InvalidParamFormatError и др. — разные типы в chat, user и auth), и общая версия
// потребовала бы либо дженериков над чужим сгенерированным кодом, либо разбора текста.
func (h *Handler) paramError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		format   *gen.InvalidParamFormatError
		required *gen.RequiredParamError
		tooMany  *gen.TooManyValuesForParamError
	)
	p := problem.Problem{Code: problem.CodeInvalidInput, Detail: "некорректные параметры запроса"}
	switch {
	case errors.As(err, &format):
		p = problem.Invalid(format.ParamName, "неверный формат")
	case errors.As(err, &required):
		p = problem.Invalid(required.ParamName, "обязательный параметр")
	case errors.As(err, &tooMany):
		p = problem.Invalid(tooMany.ParamName, "неверный формат")
	}
	writeProblem(w, r, h.log, p)
}
