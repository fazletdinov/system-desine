package usecase

import (
	"context"
	"errors"
	"fmt"

	"messenger/internal/chat/domain"
)

type ListChatsQuery struct {
	MemberID string
	After    *ChatPosition // nil — с начала списка
	Limit    int           // 0 — domain.ListLimitDefault
}

// ChatPage — страница списка чатов, самые активные первыми.
type ChatPage struct {
	Chats []*domain.Chat
	Next  *ChatPosition // nil — дальше ничего нет
}

type ListChats struct {
	chats ChatLister
}

func NewListChats(chats ChatLister) (*ListChats, error) {
	if chats == nil {
		return nil, errors.New("list chats: nil-зависимость")
	}
	return &ListChats{chats: chats}, nil
}

// #region list-chats
func (uc *ListChats) Execute(ctx context.Context, q ListChatsQuery) (ChatPage, error) {
	member, err := domain.ParseUserID(q.MemberID)
	if err != nil {
		return ChatPage{}, err
	}
	limit, err := pageLimit(q.Limit)
	if err != nil {
		return ChatPage{}, err
	}
	chats, err := uc.chats.ListChatsByMember(ctx, member, q.After, limit+1)
	if err != nil {
		return ChatPage{}, fmt.Errorf("list chats: %w", err)
	}
	if len(chats) <= limit {
		return ChatPage{Chats: chats}, nil
	}
	chats = chats[:limit]
	last := chats[limit-1]
	return ChatPage{Chats: chats, Next: &ChatPosition{LastActivityAt: last.LastActivityAt(), ID: last.ID()}}, nil
}

// #endregion list-chats
