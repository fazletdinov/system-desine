package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

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

// Register вешает маршруты на переданный mux, а не на http.DefaultServeMux:
// какой mux и с какими middleware — решает composition root.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/chats", h.createChat)
	mux.HandleFunc("POST /v1/chats/{id}/messages", h.sendMessage)
	mux.HandleFunc("GET /v1/chats/{id}/messages", h.listMessages)
}

func (h *Handler) createChat(w http.ResponseWriter, r *http.Request) {
	creator, ok := h.caller(w, r)
	if !ok {
		return
	}
	var req createChatRequest
	if err := httpjson.Decode(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	res, err := h.create.Execute(r.Context(), usecase.CreateChatCommand{
		CreatorID: creator.String(), Kind: req.Kind, Title: req.Title, MemberIDs: req.MemberIDs,
	})
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
	h.respond(w, r, status, toChatResponse(res.Chat))
}

// #region send-handler
func (h *Handler) sendMessage(w http.ResponseWriter, r *http.Request) {
	author, ok := h.caller(w, r)
	if !ok {
		return
	}
	var req sendMessageRequest
	if err := httpjson.Decode(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	msg, err := h.send.Execute(r.Context(), usecase.SendMessageCommand{
		ChatID:   r.PathValue("id"),
		AuthorID: author.String(),
		Text:     req.Text,
	})
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	h.respond(w, r, http.StatusCreated, toMessageResponse(msg))
}

// #endregion send-handler

func (h *Handler) listMessages(w http.ResponseWriter, r *http.Request) {
	reader, ok := h.caller(w, r)
	if !ok {
		return
	}
	q := usecase.ListMessagesQuery{ChatID: r.PathValue("id"), ReaderID: reader.String()}
	var err error
	if q.AfterSeq, err = queryInt64(r, "after"); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	if q.Limit, err = queryInt(r, "limit"); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	msgs, err := h.list.Execute(r.Context(), q)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	h.respond(w, r, http.StatusOK, toListMessagesResponse(msgs))
}

// queryInt64 разбирает параметр строки запроса; отсутствующий — 0 (значение по умолчанию
// выбирает use case). Диапазон тоже проверяет use case: здесь — только «это число».
func queryInt64(r *http.Request, name string) (int64, error) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, &domain.ValidationError{Field: name, Reason: "должно быть целым числом"}
	}
	return n, nil
}

func queryInt(r *http.Request, name string) (int, error) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, &domain.ValidationError{Field: name, Reason: "должно быть целым числом"}
	}
	return n, nil
}

// respond пишет успешный ответ. Ошибка записи — только в лог: заголовки уже ушли.
func (h *Handler) respond(w http.ResponseWriter, r *http.Request, status int, v any) {
	if err := httpjson.Write(w, status, v); err != nil {
		h.log.Error("запись ответа", "path", r.URL.Path, "err", err)
	}
}
