package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"messenger/internal/platform/problem"
	"messenger/internal/user/adapters/httpapi/gen"
	"messenger/internal/user/domain"
)

// writeError отдаёт ошибку общей политике (разбор тела, контекст, 500), а свои
// строки таблицы — валидацию и сигнальные ошибки контекста user — передаёт ей в problemFor.
func writeError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	problem.Respond(w, r, log, err, problemFor)
}

// writeProblem пишет уже готовую проблему; ошибку записи — только в журнал: заголовки ушли.
func writeProblem(w http.ResponseWriter, r *http.Request, log *slog.Logger, p problem.Problem) {
	if err := problem.Write(w, r, p); err != nil {
		log.Error("запись ответа", "path", r.URL.Path, "err", err)
	}
}

// problemFor — строки таблицы, которые знает только контекст user. Detail — текст
// сигнальной ошибки: он написан для людей и не раскрывает устройства сервера.
func problemFor(err error) (problem.Problem, bool) {
	var invalid *domain.ValidationError
	switch {
	case errors.As(err, &invalid):
		return problem.Invalid(invalid.Field, invalid.Reason), true
	case errors.Is(err, domain.ErrUserNotFound):
		return problem.Problem{Code: problem.CodeNotFound, Detail: domain.ErrUserNotFound.Error()}, true
	case errors.Is(err, domain.ErrUsernameTaken):
		return problem.Problem{Code: problem.CodeUsernameTaken, Detail: domain.ErrUsernameTaken.Error()}, true
	default:
		return problem.Problem{}, false
	}
}

// paramError — ErrorHandlerFunc сгенерированной обёртки: параметр пути или строки
// запроса не разобрался. Ошибки генератора — свои типы в каждом gen-пакете; переводим
// их в invalid_input с именем параметра из контракта. Текст err.Error() наружу не идёт:
// в нём детали разбора на языке Go, клиенту они не помогут.
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
