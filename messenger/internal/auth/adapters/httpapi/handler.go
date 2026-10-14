package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"messenger/internal/auth/adapters/httpapi/gen"
	"messenger/internal/auth/usecase"
	"messenger/internal/platform/httpjson"
)

// signUpper — узкий порт на use case регистрации, объявлен у потребителя.
type signUpper interface {
	Execute(ctx context.Context, cmd usecase.SignUpCommand) (usecase.Profile, error)
}

// Handler — ведущий адаптер контекста auth.
type Handler struct {
	signUp signUpper
	log    *slog.Logger
}

func New(signUp signUpper, log *slog.Logger) (*Handler, error) {
	if signUp == nil || log == nil {
		return nil, errors.New("auth httpapi: nil-зависимость")
	}
	return &Handler{signUp: signUp, log: log}, nil
}

// usersPath — адрес коллекции пользователей: регистрация создаёт ресурс в ней,
// хотя читает его уже контекст user (GET /v1/users/{id}).
const usersPath = "/v1/users"

var _ gen.ServerInterface = (*Handler)(nil)

// Register вешает маршруты контракта (тег auth) на переданный mux.
func (h *Handler) Register(mux *http.ServeMux, mws ...gen.MiddlewareFunc) {
	gen.HandlerWithOptions(h, gen.StdHTTPServerOptions{
		BaseRouter:       mux,
		Middlewares:      mws,
		ErrorHandlerFunc: h.paramError,
	})
}

// #region signup-handler
// SignUp не логирует тело запроса и команду: в них пароль. В журнал попадают только
// ошибки, а их тексты пароля не содержат — ни домен, ни декодер его не цитируют.
func (h *Handler) SignUp(w http.ResponseWriter, r *http.Request) {
	var req gen.SignUpRequest
	if err := httpjson.Decode(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	profile, err := h.signUp.Execute(r.Context(), usecase.SignUpCommand{
		Username:    req.Username,
		DisplayName: req.DisplayName,
		Password:    req.Password,
	})
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	w.Header().Set("Location", usersPath+"/"+profile.ID.String())
	h.respond(w, r, http.StatusCreated, toUser(profile))
}

// #endregion signup-handler

func (h *Handler) respond(w http.ResponseWriter, r *http.Request, status int, v any) {
	if err := httpjson.Write(w, status, v); err != nil {
		h.log.Error("запись ответа", "path", r.URL.Path, "err", err)
	}
}
