package usecase

import (
	"context"
	"errors"
	"fmt"

	"messenger/internal/chat/domain"
)

// MaxSendAttempts — сколько раз повторять отправку при конфликте версий.
// Больше трёх — признак сильной конкуренции за один чат: лучше честно вернуть
// ошибку (клиент повторит позже), чем крутиться в цикле (модуль 5 — ретраи с джиттером).
const MaxSendAttempts = 3

type SendMessageCommand struct {
	ChatID   string
	AuthorID string
	Text     string
}

type SendMessage struct {
	chats    ChatFinder
	messages MessageAppender
	clock    Clock
	ids      IDGenerator
}

func NewSendMessage(chats ChatFinder, messages MessageAppender, clock Clock, ids IDGenerator) (*SendMessage, error) {
	if chats == nil || messages == nil || clock == nil || ids == nil {
		return nil, errors.New("send message: nil-зависимость")
	}
	return &SendMessage{chats: chats, messages: messages, clock: clock, ids: ids}, nil
}

// #region send-execute
func (uc *SendMessage) Execute(ctx context.Context, cmd SendMessageCommand) (domain.Message, error) {
	chatID, err := domain.ParseChatID(cmd.ChatID)
	if err != nil {
		return domain.Message{}, err
	}
	author, err := domain.ParseUserID(cmd.AuthorID)
	if err != nil {
		return domain.Message{}, err
	}
	text, err := domain.NewText(cmd.Text)
	if err != nil {
		return domain.Message{}, err
	}
	// Id и время берём один раз, до цикла: повтор отправляет то же сообщение, а не новое.
	rawID := uc.ids.NewID()
	msgID, err := domain.ParseMessageID(rawID)
	if err != nil {
		// Без %w: сбой генератора — ошибка сервера, а не ValidationError клиента.
		return domain.Message{}, fmt.Errorf("send message: генератор выдал не UUID: %q", rawID)
	}
	sentAt := uc.clock.Now()

	// Каждая попытка заново читает чат и заново зовёт Post: правила агрегата
	// проверяются на свежем состоянии, а не на том, что проиграло гонку.
	for attempt := 1; ; attempt++ {
		// Перед повтором: если клиент ушёл, пока проигрывали гонку, новая попытка
		// — работа впустую. Первую попытку останавливает хранилище.
		if attempt > 1 {
			if err := ctx.Err(); err != nil {
				return domain.Message{}, fmt.Errorf("send message: перед попыткой %d: %w", attempt, err)
			}
		}
		chat, err := uc.chats.FindChat(ctx, chatID)
		if err != nil {
			return domain.Message{}, fmt.Errorf("send message: %w", err)
		}
		expected := chat.Version()
		msg, err := chat.Post(msgID, author, text, sentAt)
		if err != nil {
			return domain.Message{}, err
		}
		err = uc.messages.AppendMessage(ctx, chat, msg, expected)
		if err == nil {
			return msg, nil
		}
		if !errors.Is(err, domain.ErrConcurrentUpdate) || attempt == MaxSendAttempts {
			return domain.Message{}, fmt.Errorf("send message (попытка %d): %w", attempt, err)
		}
	}
}

// #endregion send-execute
