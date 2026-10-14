package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"messenger/internal/platform/httpjson"
	"messenger/internal/user/adapters/httpapi/gen"
	"messenger/internal/user/domain"
	"messenger/internal/user/usecase"
)

// Узкий порт на use case объявлен здесь, у потребителя: обработчику нужен
// один метод Execute, а в тестах его подменяет фейк. Регистрация — в контексте auth.
type userGetter interface {
	Execute(ctx context.Context, q usecase.GetUserQuery) (domain.User, error)
}

// Handler — ведущий адаптер контекста user: переводит HTTP в вызовы use case'ов и обратно.
type Handler struct {
	get userGetter
	log *slog.Logger
}

func New(get userGetter, log *slog.Logger) (*Handler, error) {
	if get == nil || log == nil {
		return nil, errors.New("user httpapi: nil-зависимость")
	}
	return &Handler{get: get, log: log}, nil
}

// Компилятор сверяет обработчик с контрактом: метода нет или сигнатура другая —
// сборка падает, а не клиент в проде.
var _ gen.ServerInterface = (*Handler)(nil)

// Register вешает маршруты контракта на переданный mux, а не на http.DefaultServeMux:
// какой mux и с какими middleware — решает composition root. Пути, методы и разбор
// параметра пути — в сгенерированном gen.HandlerWithOptions.
func (h *Handler) Register(mux *http.ServeMux, mws ...gen.MiddlewareFunc) {
	gen.HandlerWithOptions(h, gen.StdHTTPServerOptions{
		BaseRouter:       mux,
		Middlewares:      mws,
		ErrorHandlerFunc: h.paramError,
	})
}

// #region get-me
// GetMe — профиль вызывающего. Это тот же GetUser, только id берётся не из пути,
// а из личности, которую установила аутентификация: клиенту не нужно знать свой id.
// Отличие одно: «пользователь не найден» здесь — не 404. Токен верен, а его субъекта
// уже нет (в памяти — после перезапуска), значит, токен больше никого не называет: 401.
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	id, ok := h.caller(w, r)
	if !ok {
		return
	}
	user, err := h.get.Execute(r.Context(), usecase.GetUserQuery{ID: id})
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		h.subjectGone(w, r)
	case err != nil:
		writeError(w, r, h.log, err)
	default:
		h.respond(w, r, http.StatusOK, toUser(user))
	}
}

// #endregion get-me

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request, id gen.ID) {
	user, err := h.get.Execute(r.Context(), usecase.GetUserQuery{ID: id.String()})
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	h.respond(w, r, http.StatusOK, toUser(user))
}

// respond пишет успешный ответ. Ошибка записи — только в лог: заголовки уже ушли.
func (h *Handler) respond(w http.ResponseWriter, r *http.Request, status int, v any) {
	if err := httpjson.Write(w, status, v); err != nil {
		h.log.Error("запись ответа", "path", r.URL.Path, "err", err)
	}
}
