package usecase_test

import (
	"context"
	"errors"
	"testing"

	"messenger/internal/chat/adapters/memory"
	"messenger/internal/chat/domain"
	"messenger/internal/chat/usecase"
)

// #region ctx-usecase-test
func TestSendMessageCanceledContextWritesNothing(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	c := directChat(t, e)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // клиент ушёл раньше, чем запрос дошёл до use case'а
	_, err := e.send.Execute(ctx, usecase.SendMessageCommand{ChatID: c.ID().String(), AuthorID: alice, ClientMessageID: clientID(1), Text: "привет"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ждали context.Canceled в цепочке, получили %v", err)
	}
	got, err := e.list.Execute(context.Background(), usecase.ListMessagesQuery{ChatID: c.ID().String(), ReaderID: bob, Limit: domain.ListLimitMax})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 0 {
		t.Fatalf("отменённая отправка записала %d сообщений", len(got.Messages))
	}
}

// Отмена между попытками: первая проиграла гонку, а клиент тем временем ушёл.
// Хранилище здесь контекст не проверяет (deafFinder) — останавливать повторы
// обязан сам use case.
func TestSendMessageStopsRetryWhenCanceled(t *testing.T) {
	t.Parallel()
	store := memory.NewStore()
	chat, err := domain.NewDirectChat(mustChat(t, "00000000-0000-7000-9000-000000000001"), mustUser(t, alice), mustUser(t, bob), t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateChat(context.Background(), chat); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	appender := &failingAppender{err: domain.ErrConcurrentUpdate, onCall: cancel}
	send, err := usecase.NewSendMessage(deafFinder{store}, appender, store, memory.Tx{}, fixedClock{}, &seqIDs{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = send.Execute(ctx, usecase.SendMessageCommand{ChatID: chat.ID().String(), AuthorID: alice, ClientMessageID: clientID(1), Text: "привет"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ждали context.Canceled в цепочке, получили %v", err)
	}
	if len(appender.ids) != 1 {
		t.Fatalf("попыток записи %d, ждали 1: после отмены повторять незачем", len(appender.ids))
	}
}

// #endregion ctx-usecase-test
