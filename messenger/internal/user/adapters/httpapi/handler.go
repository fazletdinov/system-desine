package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"messenger/internal/platform/httpjson"
	"messenger/internal/user/domain"
	"messenger/internal/user/usecase"
)

// Узкие порты на use case'ы объявлены здесь, у потребителя: обработчику нужен
// один метод Execute, а в тестах его подменяет фейк.

type userRegistrar interface {
	Execute(ctx context.Context, cmd usecase.RegisterUserCommand) (domain.User, error)
}

type userGetter interface {
	Execute(ctx context.Context, q usecase.GetUserQuery) (domain.User, error)
}

// Handler — ведущий адаптер контекста user: переводит HTTP в вызовы use case'ов и обратно.
type Handler struct {
	register userRegistrar
	get      userGetter
	log      *slog.Logger
}

func New(register userRegistrar, get userGetter, log *slog.Logger) (*Handler, error) {
	if register == nil || get == nil || log == nil {
		return nil, errors.New("user httpapi: nil-зависимость")
	}
	return &Handler{register: register, get: get, log: log}, nil
}

// usersPath — адрес коллекции пользователей; адрес пользователя — usersPath + "/" + id.
const usersPath = "/v1/users"

// Register вешает маршруты на переданный mux, а не на http.DefaultServeMux:
// какой mux и с какими middleware — решает composition root.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/users", h.registerUser)
	mux.HandleFunc("GET /v1/users/{id}", h.getUser)
}

func (h *Handler) registerUser(w http.ResponseWriter, r *http.Request) {
	var req registerUserRequest
	if err := httpjson.Decode(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	user, err := h.register.Execute(r.Context(), usecase.RegisterUserCommand{
		Username:    req.Username,
		DisplayName: req.DisplayName,
	})
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	// Location — адрес созданного ресурса (RFC 9110, §9.3.3 и §15.3.2): без него клиент
	// по стандарту счёл бы созданным сам /v1/users. GET по этому адресу вернёт то же тело.
	w.Header().Set("Location", usersPath+"/"+user.ID().String())
	h.respond(w, r, http.StatusCreated, toUserResponse(user))
}

func (h *Handler) getUser(w http.ResponseWriter, r *http.Request) {
	user, err := h.get.Execute(r.Context(), usecase.GetUserQuery{ID: r.PathValue("id")})
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	h.respond(w, r, http.StatusOK, toUserResponse(user))
}

// respond пишет успешный ответ. Ошибка записи — только в лог: заголовки уже ушли.
func (h *Handler) respond(w http.ResponseWriter, r *http.Request, status int, v any) {
	if err := httpjson.Write(w, status, v); err != nil {
		h.log.Error("запись ответа", "path", r.URL.Path, "err", err)
	}
}
