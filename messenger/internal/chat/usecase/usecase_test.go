package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"messenger/internal/chat/adapters/memory"
	"messenger/internal/chat/domain"
	"messenger/internal/chat/usecase"
)

const (
	alice = "00000000-0000-7000-8000-000000000001"
	bob   = "00000000-0000-7000-8000-000000000002"
	eve   = "00000000-0000-7000-8000-000000000003"
)

type env struct {
	create *usecase.CreateChat
	send   *usecase.SendMessage
	list   *usecase.ListMessages
	chats  *usecase.ListChats
	get    *usecase.GetChat
}

func newEnv(t *testing.T) env {
	t.Helper()
	return newEnvWithClock(t, fixedClock{})
}

// newEnvWithClock — то же окружение с заданными часами: тестам порядка списка чатов
// нужно, чтобы время шло.
func newEnvWithClock(t *testing.T, clk usecase.Clock) env {
	t.Helper()
	store := memory.NewStore()
	ids := &seqIDs{}
	users := newKnownUsers(mustUser(t, alice), mustUser(t, bob), mustUser(t, eve))
	create, err := usecase.NewCreateChat(store, store, users, clk, ids)
	if err != nil {
		t.Fatal(err)
	}
	send, err := usecase.NewSendMessage(store, store, clk, ids)
	if err != nil {
		t.Fatal(err)
	}
	list, err := usecase.NewListMessages(store, store)
	if err != nil {
		t.Fatal(err)
	}
	chats, err := usecase.NewListChats(store)
	if err != nil {
		t.Fatal(err)
	}
	get, err := usecase.NewGetChat(store)
	if err != nil {
		t.Fatal(err)
	}
	return env{create: create, send: send, list: list, chats: chats, get: get}
}

func mustUser(t *testing.T, s string) domain.UserID {
	t.Helper()
	id, err := domain.ParseUserID(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func directChat(t *testing.T, e env) *domain.Chat {
	t.Helper()
	res, err := e.create.Execute(context.Background(), usecase.CreateChatCommand{CreatorID: alice, Kind: "direct", MemberIDs: []string{bob}})
	if err != nil {
		t.Fatal(err)
	}
	return res.Chat
}

func TestCreateDirectChatTwiceReturnsExisting(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	first := directChat(t, e)
	res, err := e.create.Execute(context.Background(), usecase.CreateChatCommand{CreatorID: bob, Kind: "direct", MemberIDs: []string{alice}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Created || res.Chat.ID() != first.ID() {
		t.Fatalf("ждали существующий чат %s, получили %s (created=%v)", first.ID(), res.Chat.ID(), res.Created)
	}
}

func TestCreateDirectChatConcurrent(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	const n = 20
	ids := make([]domain.ChatID, n)
	var created atomic.Int64
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			res, err := e.create.Execute(context.Background(), usecase.CreateChatCommand{CreatorID: alice, Kind: "direct", MemberIDs: []string{bob}})
			if err != nil {
				t.Error(err)
				return
			}
			if res.Created {
				created.Add(1)
			}
			ids[i] = res.Chat.ID()
		})
	}
	wg.Wait()
	for _, id := range ids[1:] {
		if id != ids[0] {
			t.Fatalf("для одной пары создано несколько личных чатов: %s и %s", ids[0], id)
		}
	}
	if created.Load() != 1 {
		t.Fatalf("Created = true у %d вызовов, ждали ровно у одного", created.Load())
	}
}

// Ветку гонки воспроизводим детерминированно: первый FindDirectChat «не видит» чат,
// который уже лежит в хранилище, поэтому CreateChat вернёт ErrDirectChatExists.
func TestCreateDirectChatLostRaceReturnsExisting(t *testing.T) {
	t.Parallel()
	store := memory.NewStore()
	ids := &seqIDs{}
	users := newKnownUsers(mustUser(t, alice), mustUser(t, bob))
	first, err := usecase.NewCreateChat(store, store, users, fixedClock{}, ids)
	if err != nil {
		t.Fatal(err)
	}
	existing, err := first.Execute(context.Background(), usecase.CreateChatCommand{CreatorID: alice, Kind: "direct", MemberIDs: []string{bob}})
	if err != nil {
		t.Fatal(err)
	}
	late, err := usecase.NewCreateChat(store, &missOnce{DirectChatFinder: store}, users, fixedClock{}, ids)
	if err != nil {
		t.Fatal(err)
	}
	res, err := late.Execute(context.Background(), usecase.CreateChatCommand{CreatorID: bob, Kind: "direct", MemberIDs: []string{alice}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Created || res.Chat.ID() != existing.Chat.ID() {
		t.Fatalf("после проигранной гонки ждали существующий чат %s, получили %s (created=%v)", existing.Chat.ID(), res.Chat.ID(), res.Created)
	}
}

func TestCreateChatUnknownCreator(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	_, err := e.create.Execute(context.Background(), usecase.CreateChatCommand{
		CreatorID: "00000000-0000-7000-8000-000000000099", Kind: "direct", MemberIDs: []string{bob},
	})
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Field != "creator_id" {
		t.Fatalf("ждали ValidationError по creator_id, получили %v", err)
	}
}

func TestCreateChatUnknownMember(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	_, err := e.create.Execute(context.Background(), usecase.CreateChatCommand{
		CreatorID: alice, Kind: "direct", MemberIDs: []string{"00000000-0000-7000-8000-000000000099"},
	})
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Field != "member_ids" {
		t.Fatalf("ждали ValidationError по member_ids, получили %v", err)
	}
}

func TestSendAndList(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	c := directChat(t, e)
	ctx := context.Background()
	for i := 1; i <= 3; i++ {
		m, err := e.send.Execute(ctx, usecase.SendMessageCommand{ChatID: c.ID().String(), AuthorID: alice, Text: fmt.Sprintf("сообщение %d", i)})
		if err != nil {
			t.Fatal(err)
		}
		if m.Seq() != int64(i) {
			t.Fatalf("seq = %d, ждали %d", m.Seq(), i)
		}
	}
	got, err := e.list.Execute(ctx, usecase.ListMessagesQuery{ChatID: c.ID().String(), ReaderID: bob, BeforeSeq: 3, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 2 || got.Messages[0].Seq() != 2 || got.Messages[1].Seq() != 1 || got.NextBeforeSeq != 0 {
		t.Fatalf("ждали seq 2 и 1 без следующей страницы, получили %d сообщений, next %d", len(got.Messages), got.NextBeforeSeq)
	}
}

func TestNonMemberCannotSendOrRead(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	c := directChat(t, e)
	ctx := context.Background()
	_, err := e.send.Execute(ctx, usecase.SendMessageCommand{ChatID: c.ID().String(), AuthorID: eve, Text: "привет"})
	if !errors.Is(err, domain.ErrNotMember) {
		t.Fatalf("send: ждали ErrNotMember, получили %v", err)
	}
	_, err = e.list.Execute(ctx, usecase.ListMessagesQuery{ChatID: c.ID().String(), ReaderID: eve})
	if !errors.Is(err, domain.ErrNotMember) {
		t.Fatalf("list: ждали ErrNotMember, получили %v", err)
	}
}

func TestListLimits(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	c := directChat(t, e)
	tests := []struct {
		name   string
		before int64
		limit  int
		field  string
	}{
		{name: "отрицательная позиция", before: -1, field: "cursor"},
		{name: "limit больше максимума", limit: domain.ListLimitMax + 1, field: "limit"},
		{name: "отрицательный limit", limit: -1, field: "limit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := e.list.Execute(context.Background(), usecase.ListMessagesQuery{ChatID: c.ID().String(), ReaderID: alice, BeforeSeq: tt.before, Limit: tt.limit})
			var ve *domain.ValidationError
			if !errors.As(err, &ve) || ve.Field != tt.field {
				t.Fatalf("ждали ValidationError по %s, получили %v", tt.field, err)
			}
		})
	}
}

// #region concurrent-test
func TestSendMessageConcurrent(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	c := directChat(t, e)
	const workers, perWorker = 8, 50
	var (
		mu        sync.Mutex
		ok, retry int
	)
	var wg sync.WaitGroup
	for w := range workers {
		wg.Go(func() {
			for i := range perWorker {
				author := alice
				if (w+i)%2 == 0 {
					author = bob
				}
				_, err := e.send.Execute(context.Background(), usecase.SendMessageCommand{ChatID: c.ID().String(), AuthorID: author, Text: "x"})
				mu.Lock()
				switch {
				case err == nil:
					ok++
				case errors.Is(err, domain.ErrConcurrentUpdate):
					retry++ // честный исход оптимистической блокировки под нагрузкой
				default:
					t.Error(err)
				}
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	if ok == 0 || ok+retry != workers*perWorker {
		t.Fatalf("успешно %d, конфликтов %d, всего ждали %d", ok, retry, workers*perWorker)
	}
	// История идёт от новых к старым; собираем её целиком и разворачиваем.
	var seqs []int64
	for before := int64(0); ; {
		page, err := e.list.Execute(context.Background(), usecase.ListMessagesQuery{ChatID: c.ID().String(), ReaderID: alice, BeforeSeq: before, Limit: domain.ListLimitMax})
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range page.Messages {
			seqs = append(seqs, m.Seq())
		}
		if page.NextBeforeSeq == 0 {
			break
		}
		before = page.NextBeforeSeq
	}
	slices.Reverse(seqs)
	if len(seqs) != ok {
		t.Fatalf("в истории %d сообщений, успешных отправок %d", len(seqs), ok)
	}
	for i, s := range seqs {
		if s != int64(i+1) {
			t.Fatalf("seq идут с пропуском или повтором: позиция %d, seq %d", i, s)
		}
	}
	t.Logf("успешно %d, исчерпали %d попытки: %d", ok, usecase.MaxSendAttempts, retry)
}

// #endregion concurrent-test

func TestCreateGroupChat(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	res, err := e.create.Execute(context.Background(), usecase.CreateChatCommand{
		CreatorID: alice, Kind: "group", Title: "Семья", MemberIDs: []string{bob, eve},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := res.Chat
	if !res.Created || c.Kind() != domain.KindGroup || c.Title().String() != "Семья" || len(c.Members()) != 3 || !c.IsMember(mustUser(t, alice)) {
		t.Fatalf("группа создана неверно: created=%v kind=%s участников %d", res.Created, c.Kind(), len(c.Members()))
	}
	if !c.CreatedAt().Equal(t0) {
		t.Fatalf("CreatedAt = %v, ждали время от часов %v", c.CreatedAt(), t0)
	}
}

func TestCreateDirectChatNeedsExactlyOnePeer(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	_, err := e.create.Execute(context.Background(), usecase.CreateChatCommand{CreatorID: alice, Kind: "direct", MemberIDs: []string{bob, eve}})
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Field != "member_ids" {
		t.Fatalf("ждали ValidationError по member_ids, получили %v", err)
	}
}

func TestSendToUnknownChat(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	_, err := e.send.Execute(context.Background(), usecase.SendMessageCommand{ChatID: "00000000-0000-7000-9000-000000000099", AuthorID: alice, Text: "привет"})
	if !errors.Is(err, domain.ErrChatNotFound) {
		t.Fatalf("ждали ErrChatNotFound, получили %v", err)
	}
}

// Генератор id — наша зависимость, а не ввод клиента: его сбой не должен стать
// ValidationError, иначе HTTP-адаптер ответит 400 за ошибку сервера.
func TestGarbageIDIsNotValidationError(t *testing.T) {
	t.Parallel()
	store := memory.NewStore()
	create, err := usecase.NewCreateChat(store, store, newKnownUsers(mustUser(t, alice), mustUser(t, bob)), fixedClock{}, garbageIDs{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = create.Execute(context.Background(), usecase.CreateChatCommand{CreatorID: alice, Kind: "direct", MemberIDs: []string{bob}})
	if err == nil || errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("ждали ошибку сервера, не валидации, получили %v", err)
	}
}

func TestChatConstructorsRejectNil(t *testing.T) {
	t.Parallel()
	store := memory.NewStore()
	users := newKnownUsers()
	ids := &seqIDs{}
	tests := []struct {
		name string
		new  func() error
	}{
		{name: "create: нет хранилища", new: func() error {
			_, err := usecase.NewCreateChat(nil, store, users, fixedClock{}, ids)
			return err
		}},
		{name: "create: нет UserChecker", new: func() error {
			_, err := usecase.NewCreateChat(store, store, nil, fixedClock{}, ids)
			return err
		}},
		{name: "send: нет часов", new: func() error {
			_, err := usecase.NewSendMessage(store, store, nil, ids)
			return err
		}},
		{name: "send: нет генератора", new: func() error {
			_, err := usecase.NewSendMessage(store, store, fixedClock{}, nil)
			return err
		}},
		{name: "list: нет MessageLister", new: func() error {
			_, err := usecase.NewListMessages(store, nil)
			return err
		}},
		{name: "list chats: нет ChatLister", new: func() error {
			_, err := usecase.NewListChats(nil)
			return err
		}},
		{name: "get chat: нет ChatFinder", new: func() error {
			_, err := usecase.NewGetChat(nil)
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.new() == nil {
				t.Fatal("ждали ошибку конструктора")
			}
		})
	}
}

func TestSendMessageRetryPolicy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		err       error
		wantCalls int
	}{
		{name: "конфликт версий — повтор до предела", err: domain.ErrConcurrentUpdate, wantCalls: usecase.MaxSendAttempts},
		{name: "другая ошибка — без повтора", err: errors.New("диск недоступен"), wantCalls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := memory.NewStore()
			chat, err := domain.NewDirectChat(mustChat(t, "00000000-0000-7000-9000-000000000001"), mustUser(t, alice), mustUser(t, bob), t0)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.CreateChat(context.Background(), chat); err != nil {
				t.Fatal(err)
			}
			appender := &failingAppender{err: tt.err}
			send, err := usecase.NewSendMessage(store, appender, fixedClock{}, &seqIDs{})
			if err != nil {
				t.Fatal(err)
			}
			_, err = send.Execute(context.Background(), usecase.SendMessageCommand{ChatID: chat.ID().String(), AuthorID: alice, Text: "привет"})
			if !errors.Is(err, tt.err) {
				t.Fatalf("ждали %v в цепочке, получили %v", tt.err, err)
			}
			if len(appender.ids) != tt.wantCalls {
				t.Fatalf("попыток записи %d, ждали %d", len(appender.ids), tt.wantCalls)
			}
			for _, id := range appender.ids[1:] {
				if id != appender.ids[0] {
					t.Fatalf("повтор отправил другое сообщение: %s вместо %s", id, appender.ids[0])
				}
			}
		})
	}
}

func mustChat(t *testing.T, s string) domain.ChatID {
	t.Helper()
	id, err := domain.ParseChatID(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
