package usecase

import (
	"context"
	"errors"
	"fmt"

	"messenger/internal/chat/domain"
)

type ListMessagesQuery struct {
	ChatID    string
	ReaderID  string
	BeforeSeq int64 // 0 — с самого нового
	Limit     int   // 0 — domain.ListLimitDefault
}

// MessagePage — страница истории от новых к старым. NextBeforeSeq == 0 — дальше ничего нет;
// иначе это ключ следующей страницы (адаптер завернёт его в курсор).
type MessagePage struct {
	Messages      []domain.Message
	NextBeforeSeq int64
}

type ListMessages struct {
	chats    ChatFinder
	messages MessageLister
}

func NewListMessages(chats ChatFinder, messages MessageLister) (*ListMessages, error) {
	if chats == nil || messages == nil {
		return nil, errors.New("list messages: nil-зависимость")
	}
	return &ListMessages{chats: chats, messages: messages}, nil
}

// #region list-messages
func (uc *ListMessages) Execute(ctx context.Context, q ListMessagesQuery) (MessagePage, error) {
	chatID, err := domain.ParseChatID(q.ChatID)
	if err != nil {
		return MessagePage{}, err
	}
	reader, err := domain.ParseUserID(q.ReaderID)
	if err != nil {
		return MessagePage{}, err
	}
	if q.BeforeSeq < 0 {
		// Отрицательный ключ мог прийти только из испорченного курсора.
		return MessagePage{}, &domain.ValidationError{Field: "cursor", Reason: "некорректная позиция"}
	}
	limit, err := pageLimit(q.Limit)
	if err != nil {
		return MessagePage{}, err
	}
	chat, err := uc.chats.FindChat(ctx, chatID)
	if err != nil {
		return MessagePage{}, fmt.Errorf("list messages: %w", err)
	}
	if !chat.IsMember(reader) {
		return MessagePage{}, domain.ErrNotMember
	}
	// На одно больше, чем просили: так узнаём, есть ли следующая страница, без отдельного
	// запроса COUNT и без лишнего пустого запроса клиента в конце ленты.
	msgs, err := uc.messages.ListMessagesBefore(ctx, chatID, q.BeforeSeq, limit+1)
	if err != nil {
		return MessagePage{}, fmt.Errorf("list messages: %w", err)
	}
	if len(msgs) <= limit {
		return MessagePage{Messages: msgs}, nil
	}
	msgs = msgs[:limit]
	return MessagePage{Messages: msgs, NextBeforeSeq: msgs[limit-1].Seq()}, nil
}

// #endregion list-messages
