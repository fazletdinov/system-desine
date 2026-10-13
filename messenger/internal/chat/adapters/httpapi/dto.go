package httpapi

import (
	"time"

	"messenger/internal/chat/domain"
)

// #region dto
// DTO — форма данных на проводе: отдельные типы с тегами json. Домен о них не знает,
// поэтому поле в домене можно переименовать, не сломав ни одного клиента.

type createChatRequest struct {
	Kind      string   `json:"kind"`
	Title     string   `json:"title"`
	MemberIDs []string `json:"member_ids"`
}

type chatResponse struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`
	Title     string   `json:"title,omitempty"` // у личного чата названия нет
	MemberIDs []string `json:"member_ids"`
	CreatedAt string   `json:"created_at"`
}

type sendMessageRequest struct {
	Text string `json:"text"`
}

type messageResponse struct {
	ID       string `json:"id"`
	ChatID   string `json:"chat_id"`
	AuthorID string `json:"author_id"`
	Text     string `json:"text"`
	Seq      int64  `json:"seq"`
	SentAt   string `json:"sent_at"`
}

type listMessagesResponse struct {
	Messages []messageResponse `json:"messages"`
}

// Мапперы — явный перевод домена в DTO. Время — RFC 3339 в UTC: клиенту не нужно
// гадать о часовом поясе сервера.

func toChatResponse(c *domain.Chat) chatResponse {
	members := c.Members()
	ids := make([]string, len(members))
	for i, m := range members {
		ids[i] = m.String()
	}
	return chatResponse{
		ID:        c.ID().String(),
		Kind:      string(c.Kind()),
		Title:     c.Title().String(),
		MemberIDs: ids,
		CreatedAt: c.CreatedAt().UTC().Format(time.RFC3339Nano),
	}
}

func toMessageResponse(m domain.Message) messageResponse {
	return messageResponse{
		ID:       m.ID().String(),
		ChatID:   m.ChatID().String(),
		AuthorID: m.AuthorID().String(),
		Text:     m.Text().String(),
		Seq:      m.Seq(),
		SentAt:   m.SentAt().UTC().Format(time.RFC3339Nano),
	}
}

// toListMessagesResponse создаёт срез через make, а не var: nil-срез encoding/json
// пишет как null, и клиенту пришлось бы отдельно обрабатывать «null» и «[]».
func toListMessagesResponse(msgs []domain.Message) listMessagesResponse {
	out := make([]messageResponse, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, toMessageResponse(m))
	}
	return listMessagesResponse{Messages: out}
}

// #endregion dto
