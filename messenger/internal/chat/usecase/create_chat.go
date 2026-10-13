package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"messenger/internal/chat/domain"
)

type CreateChatCommand struct {
	CreatorID string
	Kind      string
	Title     string
	MemberIDs []string // без создателя; для личного чата — ровно один собеседник
}

type CreateChatResult struct {
	Chat    *domain.Chat
	Created bool // false — личный чат для этой пары уже был
}

type CreateChat struct {
	chats  ChatCreator
	direct DirectChatFinder
	users  UserChecker
	clock  Clock
	ids    IDGenerator
}

func NewCreateChat(chats ChatCreator, direct DirectChatFinder, users UserChecker, clock Clock, ids IDGenerator) (*CreateChat, error) {
	if chats == nil || direct == nil || users == nil || clock == nil || ids == nil {
		return nil, errors.New("create chat: nil-зависимость")
	}
	return &CreateChat{chats: chats, direct: direct, users: users, clock: clock, ids: ids}, nil
}

func (uc *CreateChat) Execute(ctx context.Context, cmd CreateChatCommand) (CreateChatResult, error) {
	kind, err := domain.ParseKind(cmd.Kind)
	if err != nil {
		return CreateChatResult{}, err
	}
	creator, err := domain.ParseUserID(cmd.CreatorID)
	if err != nil {
		return CreateChatResult{}, err
	}
	others := make([]domain.UserID, 0, len(cmd.MemberIDs))
	for _, s := range cmd.MemberIDs {
		id, err := domain.ParseUserID(s)
		if err != nil {
			return CreateChatResult{}, err
		}
		others = append(others, id)
	}
	if err := uc.ensureUsersExist(ctx, creator, others); err != nil {
		return CreateChatResult{}, err
	}
	if kind == domain.KindDirect {
		return uc.createDirect(ctx, creator, others)
	}
	return uc.createGroup(ctx, creator, cmd.Title, others)
}

// ensureUsersExist проверяет создателя и участников одним обращением к порту, но
// отвечает раздельно: несуществующий создатель — это не ошибка в списке участников.
func (uc *CreateChat) ensureUsersExist(ctx context.Context, creator domain.UserID, others []domain.UserID) error {
	missing, err := uc.users.MissingUsers(ctx, append([]domain.UserID{creator}, others...))
	if err != nil {
		return fmt.Errorf("create chat: проверка участников: %w", err)
	}
	if slices.Contains(missing, creator) {
		return &domain.ValidationError{Field: "creator_id", Reason: "нет такого пользователя"}
	}
	if len(missing) > 0 {
		names := make([]string, len(missing))
		for i, id := range missing {
			names[i] = id.String()
		}
		return &domain.ValidationError{Field: "member_ids", Reason: "нет таких пользователей: " + strings.Join(names, ", ")}
	}
	return nil
}

// #region create-direct
func (uc *CreateChat) createDirect(ctx context.Context, creator domain.UserID, others []domain.UserID) (CreateChatResult, error) {
	if len(others) != 1 {
		return CreateChatResult{}, &domain.ValidationError{Field: "member_ids", Reason: "для личного чата нужен ровно один собеседник"}
	}
	peer := others[0]
	if existing, err := uc.direct.FindDirectChat(ctx, creator, peer); err == nil {
		return CreateChatResult{Chat: existing}, nil
	} else if !errors.Is(err, domain.ErrChatNotFound) {
		return CreateChatResult{}, fmt.Errorf("create chat: %w", err)
	}
	id, err := uc.newChatID()
	if err != nil {
		return CreateChatResult{}, err
	}
	chat, err := domain.NewDirectChat(id, creator, peer, uc.clock.Now())
	if err != nil {
		return CreateChatResult{}, err
	}
	err = uc.chats.CreateChat(ctx, chat)
	if errors.Is(err, domain.ErrDirectChatExists) {
		// Гонка: между FindDirectChat и CreateChat чат для пары создал кто-то другой.
		existing, ferr := uc.direct.FindDirectChat(ctx, creator, peer)
		if ferr != nil {
			return CreateChatResult{}, fmt.Errorf("create chat: после гонки: %w", ferr)
		}
		return CreateChatResult{Chat: existing}, nil
	}
	if err != nil {
		return CreateChatResult{}, fmt.Errorf("create chat: %w", err)
	}
	return CreateChatResult{Chat: chat, Created: true}, nil
}

// #endregion create-direct

func (uc *CreateChat) createGroup(ctx context.Context, creator domain.UserID, rawTitle string, others []domain.UserID) (CreateChatResult, error) {
	title, err := domain.NewTitle(rawTitle)
	if err != nil {
		return CreateChatResult{}, err
	}
	id, err := uc.newChatID()
	if err != nil {
		return CreateChatResult{}, err
	}
	chat, err := domain.NewGroupChat(id, title, creator, others, uc.clock.Now())
	if err != nil {
		return CreateChatResult{}, err
	}
	if err := uc.chats.CreateChat(ctx, chat); err != nil {
		return CreateChatResult{}, fmt.Errorf("create chat: %w", err)
	}
	return CreateChatResult{Chat: chat, Created: true}, nil
}

func (uc *CreateChat) newChatID() (domain.ChatID, error) {
	raw := uc.ids.NewID()
	id, err := domain.ParseChatID(raw)
	if err != nil {
		// Без %w: ошибка разбора — *ValidationError, и адаптер ответил бы 400,
		// хотя виноват не клиент, а наш генератор. Это сбой сервера.
		return domain.ChatID{}, fmt.Errorf("create chat: генератор выдал не UUID: %q", raw)
	}
	return id, nil
}
