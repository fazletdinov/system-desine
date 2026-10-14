package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"messenger/internal/chat/domain"
	"messenger/internal/chat/usecase"
	"messenger/internal/platform/cursor"
	"messenger/internal/platform/problem"
)

// #region cursors
// Курсор — деталь транспорта: use case'ы работают с ключами (seq, позиция в списке),
// а во что их завернуть для клиента, решает адаптер.

const (
	kindMessages = "messages"
	kindChats    = "chats"
)

// messagesCursor привязан к чату: курсор истории чата A, предъявленный для чата B,
// указал бы на чужую позицию — такой курсор отвергаем.
type messagesCursor struct {
	Chat string `json:"chat"`
	Seq  int64  `json:"seq"`
}

type chatsCursor struct {
	At time.Time `json:"at"`
	ID string    `json:"id"`
}

// badCursorReason — ответ клиенту на любой негодный курсор: что именно не так, ему
// знать незачем (курсор непрозрачен), а подробность уходит в журнал.
const badCursorReason = "курсор не годится: начните с первой страницы"

// badCursor отвечает 400 с invalid_params[0].name = "cursor". Курсор — понятие
// транспорта, поэтому проблему строит адаптер, а не домен (как и paramError).
func (h *Handler) badCursor(w http.ResponseWriter, r *http.Request, err error) {
	h.log.Info("негодный курсор", "path", r.URL.Path, "err", err)
	writeProblem(w, r, h.log, problem.Invalid("cursor", badCursorReason))
}

// encodeMessagesCursor: 0 — следующей страницы нет, и next_cursor будет null.
func encodeMessagesCursor(chatID domain.ChatID, beforeSeq int64) (*string, error) {
	if beforeSeq == 0 {
		return nil, nil
	}
	s, err := cursor.Encode(kindMessages, messagesCursor{Chat: chatID.String(), Seq: beforeSeq})
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// decodeMessagesCursor: нет курсора — первая страница (0).
func decodeMessagesCursor(raw *string, chatID domain.ChatID) (int64, error) {
	if raw == nil {
		return 0, nil
	}
	var c messagesCursor
	if err := cursor.Decode(*raw, kindMessages, &c); err != nil {
		return 0, err
	}
	if c.Chat != chatID.String() {
		return 0, fmt.Errorf("%w: курсор чата %s", cursor.ErrInvalid, c.Chat)
	}
	if c.Seq < 1 {
		return 0, fmt.Errorf("%w: seq %d", cursor.ErrInvalid, c.Seq)
	}
	return c.Seq, nil
}

func encodeChatsCursor(next *usecase.ChatPosition) (*string, error) {
	if next == nil {
		return nil, nil
	}
	s, err := cursor.Encode(kindChats, chatsCursor{At: next.LastActivityAt.UTC(), ID: next.ID.String()})
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func decodeChatsCursor(raw *string) (*usecase.ChatPosition, error) {
	if raw == nil {
		return nil, nil
	}
	var c chatsCursor
	if err := cursor.Decode(*raw, kindChats, &c); err != nil {
		return nil, err
	}
	id, err := domain.ParseChatID(c.ID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", cursor.ErrInvalid, err)
	}
	return &usecase.ChatPosition{LastActivityAt: c.At, ID: id}, nil
}

// #endregion cursors
