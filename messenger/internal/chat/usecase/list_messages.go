package usecase

import (
	"context"
	"errors"
	"fmt"

	"messenger/internal/chat/domain"
)

type ListMessagesQuery struct {
	ChatID   string
	ReaderID string
	AfterSeq int64 // 0 — с начала
	Limit    int   // 0 — domain.ListLimitDefault
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

func (uc *ListMessages) Execute(ctx context.Context, q ListMessagesQuery) ([]domain.Message, error) {
	chatID, err := domain.ParseChatID(q.ChatID)
	if err != nil {
		return nil, err
	}
	reader, err := domain.ParseUserID(q.ReaderID)
	if err != nil {
		return nil, err
	}
	if q.AfterSeq < 0 {
		return nil, &domain.ValidationError{Field: "after", Reason: "не может быть отрицательным"}
	}
	limit := q.Limit
	if limit == 0 {
		limit = domain.ListLimitDefault
	}
	if limit < 1 || limit > domain.ListLimitMax {
		return nil, &domain.ValidationError{Field: "limit", Reason: fmt.Sprintf("от 1 до %d", domain.ListLimitMax)}
	}
	chat, err := uc.chats.FindChat(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	if !chat.IsMember(reader) {
		return nil, domain.ErrNotMember
	}
	msgs, err := uc.messages.ListMessages(ctx, chatID, q.AfterSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	return msgs, nil
}
