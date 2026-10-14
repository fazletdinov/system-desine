package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"messenger/internal/chat/adapters/httpapi/gen"
	"messenger/internal/chat/domain"
	"messenger/internal/chat/usecase"
	"messenger/internal/platform/httpjson"
)

// Узкие порты на use case'ы объявлены здесь, у потребителя: обработчику нужен
// один метод Execute, а в тестах его подменяет фейк.

type chatCreator interface {
	Execute(ctx context.Context, cmd usecase.CreateChatCommand) (usecase.CreateChatResult, error)
}

type messageSender interface {
	Execute(ctx context.Context, cmd usecase.SendMessageCommand) (domain.Message, error)
}

type messageLister interface {
	Execute(ctx context.Context, q usecase.ListMessagesQuery) ([]domain.Message, error)
}

// Handler — ведущий адаптер контекста chat: разобрать запрос → вызвать use case →
// смаппить ответ. Бизнес-правил здесь нет: все проверки данных — в домене.
type Handler struct {
	create chatCreator
	send   messageSender
	list   messageLister
	log    *slog.Logger
}

func New(create chatCreator, send messageSender, list messageLister, log *slog.Logger) (*Handler, error) {
	if create == nil || send == nil || list == nil || log == nil {
		return nil, errors.New("chat httpapi: nil-зависимость")
	}
	return &Handler{create: create, send: send, list: list, log: log}, nil
}

// chatsPath — адрес коллекции чатов; адрес чата — chatsPath + "/" + id.
const chatsPath = "/v1/chats"

// #region register
// Компилятор сверяет обработчик с контрактом: метода нет или сигнатура другая —
// сборка падает, а не клиент в проде.
var _ gen.ServerInterface = (*Handler)(nil)

// Register вешает маршруты контракта на переданный mux, а не на http.DefaultServeMux:
// какой mux и с какими middleware — решает composition root. Пути, методы и разбор
// параметров пути и строки запроса — в сгенерированном gen.HandlerWithOptions;
// mws оборачивают каждую операцию после разбора параметров.
func (h *Handler) Register(mux *http.ServeMux, mws ...gen.MiddlewareFunc) {
	gen.HandlerWithOptions(h, gen.StdHTTPServerOptions{
		BaseRouter:       mux,
		Middlewares:      mws,
		ErrorHandlerFunc: h.paramError,
	})
}

// #endregion register

func (h *Handler) CreateChat(w http.ResponseWriter, r *http.Request) {
	creator, ok := h.caller(w, r)
	if !ok {
		return
	}
	var req gen.CreateChatRequest
	if err := httpjson.Decode(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	cmd := usecase.CreateChatCommand{CreatorID: creator.String(), Kind: string(req.Kind), MemberIDs: req.MemberIds}
	if req.Title != nil {
		cmd.Title = *req.Title
	}
	res, err := h.create.Execute(r.Context(), cmd)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	// 200 вместо 201: личный чат для пары уже был — ничего нового не создано.
	status := http.StatusOK
	if res.Created {
		status = http.StatusCreated
	}
	// Location — и при 200. У 201 это адрес созданного (RFC 9110, §9.3.3 — SHOULD);
	// для 200 стандарт смысла Location не задаёт, но и не запрещает. Ставим, чтобы
	// клиент узнавал адрес чата одинаково — создан тот сейчас или был раньше.
	w.Header().Set("Location", chatsPath+"/"+res.Chat.ID().String())
	h.respond(w, r, status, toChat(res.Chat))
}

// #region send-handler
func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request, id gen.ID) {
	author, ok := h.caller(w, r)
	if !ok {
		return
	}
	var req gen.SendMessageRequest
	if err := httpjson.Decode(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	msg, err := h.send.Execute(r.Context(), usecase.SendMessageCommand{
		ChatID:   id.String(),
		AuthorID: author.String(),
		Text:     req.Text,
	})
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	h.respond(w, r, http.StatusCreated, toMessage(msg))
}

// #endregion send-handler

// ListMessages: необязательные параметры — указатели; nil — «не передан», и тогда
// 0: значение по умолчанию и проверку диапазона выбирает use case, а не адаптер.
func (h *Handler) ListMessages(w http.ResponseWriter, r *http.Request, id gen.ID, params gen.ListMessagesParams) {
	reader, ok := h.caller(w, r)
	if !ok {
		return
	}
	q := usecase.ListMessagesQuery{ChatID: id.String(), ReaderID: reader.String()}
	if params.After != nil {
		q.AfterSeq = *params.After
	}
	if params.Limit != nil {
		q.Limit = *params.Limit
	}
	msgs, err := h.list.Execute(r.Context(), q)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	h.respond(w, r, http.StatusOK, toMessageList(msgs))
}

// respond пишет успешный ответ. Ошибка записи — только в лог: заголовки уже ушли.
func (h *Handler) respond(w http.ResponseWriter, r *http.Request, status int, v any) {
	if err := httpjson.Write(w, status, v); err != nil {
		h.log.Error("запись ответа", "path", r.URL.Path, "err", err)
	}
}
