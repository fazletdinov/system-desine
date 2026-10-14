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
		// Отдаём клиенту ключ сортировки: по нему он сам поднимет чат вверх, когда
		// придёт новое сообщение, не перечитывая список.
		LastActivityAt: c.LastActivityAt().UTC(),
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

// Страницы создают items через make, а не var: nil-срез encoding/json пишет как null,
// и клиенту пришлось бы отдельно обрабатывать «null» и «[]». next == nil — next_cursor
// уходит как null: поле есть всегда, его отсутствие не нужно угадывать.

func toMessagePage(msgs []domain.Message, next *string) gen.MessagePage {
	out := make([]gen.Message, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, toMessage(m))
	}
	return gen.MessagePage{Items: out, NextCursor: next}
}

func toChatPage(chats []*domain.Chat, next *string) gen.ChatPage {
	out := make([]gen.Chat, 0, len(chats))
	for _, c := range chats {
		out = append(out, toChat(c))
	}
	return gen.ChatPage{Items: out, NextCursor: next}
}

// #endregion dto
