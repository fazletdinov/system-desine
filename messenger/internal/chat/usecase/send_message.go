package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	sent     MessageByClientIDFinder
	tx       Transactor
	clock    Clock
	ids      IDGenerator
}

func NewSendMessage(chats ChatFinder, messages MessageAppender, sent MessageByClientIDFinder, tx Transactor, clock Clock, ids IDGenerator) (*SendMessage, error) {
	if chats == nil || messages == nil || sent == nil || tx == nil || clock == nil || ids == nil {
		return nil, errors.New("send message: nil-зависимость")
	}
	return &SendMessage{chats: chats, messages: messages, sent: sent, tx: tx, clock: clock, ids: ids}, nil
}

// outgoing — сообщение, которое отправляем: одно на все попытки.
type outgoing struct {
	chatID   domain.ChatID
	id       domain.MessageID
	clientID domain.ClientMessageID
	author   domain.UserID
	text     domain.Text
	sentAt   time.Time
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
	out := outgoing{chatID: chatID, id: msgID, clientID: clientID, author: author, text: text, sentAt: uc.clock.Now()}

	for attempt := 1; ; attempt++ {
		// Перед повтором: если клиент ушёл, пока проигрывали гонку, новая попытка
		// — работа впустую. Первую попытку останавливает хранилище.
		if attempt > 1 {
			if err := ctx.Err(); err != nil {
				return SendMessageResult{}, fmt.Errorf("send message: перед попыткой %d: %w", attempt, err)
			}
		}
		msg, err := uc.post(ctx, out)
		if err == nil {
			return SendMessageResult{Message: msg, Created: true}, nil
		}
		if errors.Is(err, domain.ErrDuplicateMessage) {
			return uc.saved(ctx, out)
		}
		if !errors.Is(err, domain.ErrConcurrentUpdate) || attempt == MaxSendAttempts {
			return SendMessageResult{}, fmt.Errorf("send message (попытка %d): %w", attempt, err)
		}
	}
}

// #region send-attempt
// post — одна попытка в своей транзакции: прочитать чат, применить правила агрегата,
// записать сообщение и новое состояние чата. Каждая попытка заново читает чат и заново
// зовёт Post: правила проверяются на свежем состоянии, а не на том, что проиграло гонку.
func (uc *SendMessage) post(ctx context.Context, out outgoing) (domain.Message, error) {
	var msg domain.Message
	err := uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		chat, err := uc.chats.FindChat(ctx, out.chatID)
		if err != nil {
			return err
		}
		expected := chat.Version()
		m, err := chat.Post(out.id, out.clientID, out.author, out.text, out.sentAt)
		if err != nil {
			return err
		}
		if err := uc.messages.AppendMessage(ctx, chat, m, expected); err != nil {
			return err
		}
		msg = m
		return nil
	})
	return msg, err
}

// #endregion send-attempt

// #region send-saved
// saved отвечает на повтор: сообщение с этим ключом уже сохранено. Читаем его после
// отката попытки, вне её транзакции: в Postgres нарушение уникальности обрывает
// транзакцию, и до ROLLBACK в ней не выполнится ни один запрос. Тот же ключ и тот же
// текст — это повтор; тот же ключ с другим текстом — ошибка клиента: ключ одного
// сообщения переиспользован для другого.
func (uc *SendMessage) saved(ctx context.Context, out outgoing) (SendMessageResult, error) {
	existing, err := uc.sent.FindMessageByClientID(ctx, out.chatID, out.author, out.clientID)
	if err != nil {
		return SendMessageResult{}, fmt.Errorf("send message: повтор %s: %w", out.clientID, err)
	}
	if existing.Text() != out.text {
		return SendMessageResult{}, domain.ErrClientMessageIDReused
	}
	return SendMessageResult{Message: existing}, nil
}

// #endregion send-saved

// #endregion send-execute
