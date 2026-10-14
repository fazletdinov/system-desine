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
	ChatID          string
	AuthorID        string
	ClientMessageID string
	Text            string
}

// SendMessageResult.Created == false — это повтор уже сохранённого сообщения:
// адаптер ответит 200 вместо 201, а клиент получит то же сообщение, что и в первый раз.
type SendMessageResult struct {
	Message domain.Message
	Created bool
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
func (uc *SendMessage) Execute(ctx context.Context, cmd SendMessageCommand) (SendMessageResult, error) {
	chatID, err := domain.ParseChatID(cmd.ChatID)
	if err != nil {
		return SendMessageResult{}, err
	}
	author, err := domain.ParseUserID(cmd.AuthorID)
	if err != nil {
		return SendMessageResult{}, err
	}
	clientID, err := domain.ParseClientMessageID(cmd.ClientMessageID)
	if err != nil {
		return SendMessageResult{}, err
	}
	text, err := domain.NewText(cmd.Text)
	if err != nil {
		return SendMessageResult{}, err
	}
	// Id и время берём один раз, до цикла: повтор отправляет то же сообщение, а не новое.
	rawID := uc.ids.NewID()
	msgID, err := domain.ParseMessageID(rawID)
	if err != nil {
		// Без %w: сбой генератора — ошибка сервера, а не ValidationError клиента.
		return SendMessageResult{}, fmt.Errorf("send message: генератор выдал не UUID: %q", rawID)
	}
	sentAt := uc.clock.Now()

	// Каждая попытка заново читает чат и заново зовёт Post: правила агрегата
	// проверяются на свежем состоянии, а не на том, что проиграло гонку.
	for attempt := 1; ; attempt++ {
		// Перед повтором: если клиент ушёл, пока проигрывали гонку, новая попытка
		// — работа впустую. Первую попытку останавливает хранилище.
		if attempt > 1 {
			if err := ctx.Err(); err != nil {
				return SendMessageResult{}, fmt.Errorf("send message: перед попыткой %d: %w", attempt, err)
			}
		}
		chat, err := uc.chats.FindChat(ctx, chatID)
		if err != nil {
			return SendMessageResult{}, fmt.Errorf("send message: %w", err)
		}
		expected := chat.Version()
		msg, err := chat.Post(msgID, clientID, author, text, sentAt)
		if err != nil {
			return SendMessageResult{}, err
		}
		err = uc.messages.AppendMessage(ctx, chat, msg, expected)
		if err == nil {
			return SendMessageResult{Message: msg, Created: true}, nil
		}
		var dup *domain.DuplicateMessageError
		if errors.As(err, &dup) {
			// Тот же ключ и тот же текст — это повтор; тот же ключ с другим текстом —
			// ошибка клиента: ключ одного сообщения переиспользован для другого.
			if dup.Existing.Text() != text {
				return SendMessageResult{}, domain.ErrClientMessageIDReused
			}
			return SendMessageResult{Message: dup.Existing}, nil
		}
		if !errors.Is(err, domain.ErrConcurrentUpdate) || attempt == MaxSendAttempts {
			return SendMessageResult{}, fmt.Errorf("send message (попытка %d): %w", attempt, err)
		}
	}
}

// #endregion send-execute
