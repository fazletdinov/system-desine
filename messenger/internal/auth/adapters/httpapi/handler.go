package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"messenger/internal/auth/adapters/httpapi/gen"
	"messenger/internal/auth/domain"
	"messenger/internal/auth/usecase"
	"messenger/internal/platform/httpauth"
	"messenger/internal/platform/httpjson"
	"messenger/internal/platform/middleware"
	"messenger/internal/platform/problem"
)

// signUpper и loginer — узкие порты на use case'ы, объявлены у потребителя.
type signUpper interface {
	Execute(ctx context.Context, cmd usecase.SignUpCommand) (usecase.Profile, error)
}

type loginer interface {
	Execute(ctx context.Context, cmd usecase.LoginCommand) (usecase.Token, error)
}

// Clock нужен, чтобы перевести момент истечения токена в expires_in — секунды «от сейчас».
type Clock interface {
	Now() time.Time
}

// Handler — ведущий адаптер контекста auth.
type Handler struct {
	signUp signUpper
	login  loginer
	clock  Clock
	log    *slog.Logger
}

func New(signUp signUpper, login loginer, clock Clock, log *slog.Logger) (*Handler, error) {
	if signUp == nil || login == nil || clock == nil || log == nil {
		return nil, errors.New("auth httpapi: nil-зависимость")
	}
	return &Handler{signUp: signUp, login: login, clock: clock, log: log}, nil
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

// #region create-token
// CreateToken — вход. Форма ответа взята из RFC 6749 §5.1 (access_token, token_type,
// expires_in, Cache-Control: no-store), но это не OAuth 2.0: нет клиентов, грантов и scope.
func (h *Handler) CreateToken(w http.ResponseWriter, r *http.Request) {
	var req gen.CreateTokenRequest
	if err := httpjson.Decode(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	tok, err := h.login.Execute(r.Context(), usecase.LoginCommand{Username: req.Username, Password: req.Password})
	if errors.Is(err, domain.ErrInvalidCredentials) {
		h.rejectLogin(w, r, err)
		return
	}
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	// Токен — секрет: ни браузер, ни прокси не должны положить ответ в кэш.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	h.respond(w, r, http.StatusOK, gen.TokenResponse{
		AccessToken: tok.Value,
		TokenType:   gen.Bearer,
		// Отбрасываем доли секунды: клиент не должен думать, что токен живёт дольше, чем
		// на самом деле. Поэтому сразу после выпуска expires_in обычно 899, а не 900.
		ExpiresIn: max(0, int(tok.ExpiresAt.Sub(h.clock.Now())/time.Second)),
	})
}

// rejectLogin отвечает одинаково на любую причину отказа: по ответу нельзя узнать,
// есть ли такой пользователь. Причина нужна оператору — она уходит в журнал под
// request_id. Имени и пароля в записи нет: журнал не должен копить чужие секреты.
func (h *Handler) rejectLogin(w http.ResponseWriter, r *http.Request, err error) {
	h.log.InfoContext(r.Context(), "вход отклонён", "reason", rejectReason(err), "request_id", middleware.RequestIDFrom(r.Context()))
	// RFC 9110 §15.5.2: у любого 401 есть WWW-Authenticate — как здесь аутентифицироваться.
	w.Header().Set("WWW-Authenticate", httpauth.Challenge(httpauth.ErrNoCredentials))
	writeProblem(w, r, h.log, problem.Problem{Code: problem.CodeInvalidCredentials, Detail: domain.ErrInvalidCredentials.Error()})
}

// #endregion create-token

func (h *Handler) respond(w http.ResponseWriter, r *http.Request, status int, v any) {
	if err := httpjson.Write(w, status, v); err != nil {
		h.log.Error("запись ответа", "path", r.URL.Path, "err", err)
	}
}
