package usecase

import (
	"context"
	"errors"
	"fmt"

	"messenger/internal/chat/domain"
)

type GetChatQuery struct {
	ChatID   string
	ReaderID string
}

// GetChat — чат по id для его участника. Постороннему — domain.ErrNotMember, как и
// в истории сообщений: правило «чат видят только участники» одно на весь API.
type GetChat struct {
	chats ChatFinder
}

func NewGetChat(chats ChatFinder) (*GetChat, error) {
	if chats == nil {
		return nil, errors.New("get chat: nil-зависимость")
	}
	return &GetChat{chats: chats}, nil
}

func (uc *GetChat) Execute(ctx context.Context, q GetChatQuery) (*domain.Chat, error) {
	chatID, err := domain.ParseChatID(q.ChatID)
	if err != nil {
		return nil, err
	}
	reader, err := domain.ParseUserID(q.ReaderID)
	if err != nil {
		return nil, err
	}
	chat, err := uc.chats.FindChat(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("get chat: %w", err)
	}
	if !chat.IsMember(reader) {
		return nil, domain.ErrNotMember
	}
	return chat, nil
}
