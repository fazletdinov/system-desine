package usecase_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"messenger/internal/chat/domain"
	"messenger/internal/chat/usecase"
)

var t0 = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return t0 }

// steppingClock — часы, которые идут только по команде: тест сам решает, какое время
// получит следующее событие, и порядок по активности не зависит от скорости машины.
type steppingClock struct{ sec atomic.Int64 }

func (c *steppingClock) Now() time.Time { return t0.Add(time.Duration(c.sec.Load()) * time.Second) }

func (c *steppingClock) Advance() { c.sec.Add(1) }

// seqIDs выдаёт предсказуемые UUID: тесты сравнивают id, а не случайные строки.
type seqIDs struct{ n atomic.Int64 }

func (g *seqIDs) NewID() string {
	return fmt.Sprintf("00000000-0000-7000-b000-%012d", g.n.Add(1))
}

// clientID — client_message_id номер n: его, в отличие от id сообщения, придумывает клиент.
func clientID(n int) string {
	return fmt.Sprintf("00000000-0000-7000-e000-%012d", n)
}

// knownUsers — фейк UserChecker: «существуют» только перечисленные пользователи.
type knownUsers struct {
	mu  sync.Mutex
	ids map[domain.UserID]bool
}

func newKnownUsers(ids ...domain.UserID) *knownUsers {
	k := &knownUsers{ids: make(map[domain.UserID]bool)}
	for _, id := range ids {
		k.ids[id] = true
	}
	return k
}

func (k *knownUsers) MissingUsers(_ context.Context, ids []domain.UserID) ([]domain.UserID, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	var missing []domain.UserID
	for _, id := range ids {
		if !k.ids[id] {
			missing = append(missing, id)
		}
	}
	return missing, nil
}

// garbageIDs — сломанный генератор: проверяем, что его сбой не выдаётся за ошибку клиента.
type garbageIDs struct{}

func (garbageIDs) NewID() string { return "не-uuid" }

// missOnce — DirectChatFinder, который на первом вызове «не видит» чат: так тест
// попадает в ветку гонки CreateChat без расчёта на планировщик.
type missOnce struct {
	usecase.DirectChatFinder
	missed atomic.Bool
}

func (m *missOnce) FindDirectChat(ctx context.Context, a, b domain.UserID) (*domain.Chat, error) {
	if m.missed.CompareAndSwap(false, true) {
		return nil, domain.ErrChatNotFound
	}
	return m.DirectChatFinder.FindDirectChat(ctx, a, b)
}

// failingAppender всегда отвечает заданной ошибкой и запоминает id и client_message_id
// каждого сообщения.
// onCall, если задан, вызывается перед ответом: так тест вмешивается в середину
// попытки, например отменяет контекст запроса.
type failingAppender struct {
	err     error
	onCall  func()
	mu      sync.Mutex
	ids     []domain.MessageID
	clients []domain.ClientMessageID
	// callsOutsideTx — вызовы без транзакции recordingTx в ctx.
	callsOutsideTx atomic.Int32
}

func (f *failingAppender) AppendMessage(ctx context.Context, _ *domain.Chat, msg domain.Message, _ int64) error {
	if !inTx(ctx) {
		f.callsOutsideTx.Add(1)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ids = append(f.ids, msg.ID())
	f.clients = append(f.clients, msg.ClientID())
	if f.onCall != nil {
		f.onCall()
	}
	return f.err
}

// deafFinder — ChatFinder, который не смотрит на контекст вызывающего. С ним тест
// проверяет проверку отмены в самом use case'е, а не в хранилище.
type deafFinder struct{ usecase.ChatFinder }

func (d deafFinder) FindChat(_ context.Context, id domain.ChatID) (*domain.Chat, error) {
	return d.ChatFinder.FindChat(context.Background(), id)
}

// recordingTx — Transactor, который ничего не откатывает, а запоминает: сколько раз
// его звали и что вернул последний fn. Метка в ctx позволяет фейкам хранилищ понять,
// вызваны ли они внутри WithinTx, — так же, как Postgres-адаптер найдёт в ctx транзакцию.
type recordingTx struct {
	calls atomic.Int32
	mu    sync.Mutex
	err   error
}

type inTxKey struct{}

func inTx(ctx context.Context) bool { return ctx.Value(inTxKey{}) != nil }

func (r *recordingTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	r.calls.Add(1)
	err := fn(context.WithValue(ctx, inTxKey{}, true))
	r.mu.Lock()
	r.err = err
	r.mu.Unlock()
	return err
}

func (r *recordingTx) lastErr() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

// txAwareFinder считает чтения сохранённого сообщения и те из них, что пришлись
// на транзакцию recordingTx.
type txAwareFinder struct {
	usecase.MessageByClientIDFinder
	calls     atomic.Int32
	callsInTx atomic.Int32
}

func (f *txAwareFinder) FindMessageByClientID(ctx context.Context, chatID domain.ChatID, author domain.UserID, clientID domain.ClientMessageID) (domain.Message, error) {
	f.calls.Add(1)
	if inTx(ctx) {
		f.callsInTx.Add(1)
	}
	return f.MessageByClientIDFinder.FindMessageByClientID(ctx, chatID, author, clientID)
}
