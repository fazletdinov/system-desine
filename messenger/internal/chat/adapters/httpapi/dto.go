package httpapi

import (
	"messenger/internal/chat/adapters/httpapi/gen"
	"messenger/internal/chat/domain"
)

// #region dto
// DTO — форма данных на проводе. С урока 1.9 их не пишут руками: типы с тегами json
// сгенерированы из api/openapi.yaml в пакет gen. Домен о них по-прежнему не знает,
// а мапперы — явный перевод домена в DTO — остаются здесь, в адаптере.

// Время — в UTC: клиенту не нужно гадать о часовом поясе сервера. Поле time.Time
// encoding/json пишет в RFC 3339 — формате date-time из контракта.

func toChat(c *domain.Chat) gen.Chat {
	members := c.Members()
	ids := make([]string, len(members))
	for i, m := range members {
		ids[i] = m.String()
	}
	out := gen.Chat{
		Id:        c.ID().String(),
		Kind:      gen.ChatKind(c.Kind()),
		MemberIds: ids,
		CreatedAt: c.CreatedAt().UTC(),
	}
	// У личного чата названия нет: nil-указатель с omitempty — поля нет в ответе.
	if title := c.Title().String(); title != "" {
		out.Title = &title
	}
	return out
}

func toMessage(m domain.Message) gen.Message {
	return gen.Message{
		Id:       m.ID().String(),
		ChatId:   m.ChatID().String(),
		AuthorId: m.AuthorID().String(),
		Text:     m.Text().String(),
		Seq:      m.Seq(),
		SentAt:   m.SentAt().UTC(),
	}
}

// toMessageList создаёт срез через make, а не var: nil-срез encoding/json
// пишет как null, и клиенту пришлось бы отдельно обрабатывать «null» и «[]».
func toMessageList(msgs []domain.Message) gen.MessageList {
	out := make([]gen.Message, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, toMessage(m))
	}
	return gen.MessageList{Messages: out}
}

// #endregion dto
