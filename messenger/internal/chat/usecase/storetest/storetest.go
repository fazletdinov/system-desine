// Package storetest — контрактные тесты хранилища чатов: одна батарея проверок
// для любой реализации портов. Сейчас её проходит memory.Store, в модуле 2a —
// Postgres-адаптер. Если обе реализации проходят контракт, use case'ам всё равно,
// с какой из них работать.
package storetest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"messenger/internal/chat/domain"
	"messenger/internal/chat/usecase"
)

// Store — всё, что use case'ы чатов ждут от хранилища.
type Store interface {
	usecase.ChatCreator
	usecase.ChatFinder
	usecase.DirectChatFinder
	usecase.MessageAppender
	usecase.MessageLister
}

var t0 = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

// #region run
// Run прогоняет контракт против хранилища, которое создаёт newStore. Каждая проверка
// получает своё, пустое хранилище: параллельные проверки не делят состояние.
func Run(t *testing.T, newStore func(t *testing.T) Store) {
	t.Helper()
	tests := []struct {
		name string
		fn   func(t *testing.T, s Store)
	}{
		{"создание и поиск", testCreateAndFind},
		{"второй личный чат для пары", testDirectUnique},
		{"оптимистическая блокировка", testVersionConflict},
		{"запись в неизвестный чат", testAppendUnknownChat},
		{"история по seq", testListPaging},
		{"страница истории — копия", testListReturnsCopy},
		{"одновременная запись", testConcurrentAppend},
		{"отменённый контекст", testCanceledContext},
		{"найденный чат — копия", testFindReturnsCopy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.fn(t, newStore(t))
		})
	}
}

// #endregion run

func id[T any](t *testing.T, parse func(string) (T, error), prefix string, n int) T {
	t.Helper()
	v, err := parse(fmt.Sprintf("00000000-0000-7000-%s-%012d", prefix, n))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func user(t *testing.T, n int) domain.UserID {
	t.Helper()
	return id(t, domain.ParseUserID, "8000", n)
}

// newDirect — личный чат номер n между пользователями a и b.
func newDirect(t *testing.T, n, a, b int) *domain.Chat {
	t.Helper()
	c, err := domain.NewDirectChat(id(t, domain.ParseChatID, "9000", n), user(t, a), user(t, b), t0)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// post добавляет в агрегат сообщение номер n от первого участника, но не сохраняет его.
func post(t *testing.T, c *domain.Chat, n int) domain.Message {
	t.Helper()
	text, err := domain.NewText(fmt.Sprintf("сообщение %d", n))
	if err != nil {
		t.Fatal(err)
	}
	m, err := c.Post(id(t, domain.ParseMessageID, "a000", n), c.Members()[0], text, t0)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// create сохраняет чат и падает, если хранилище отказало.
func create(t *testing.T, s Store, c *domain.Chat) {
	t.Helper()
	if err := s.CreateChat(context.Background(), c); err != nil {
		t.Fatal(err)
	}
}

// find читает чат и падает, если его нет.
func find(t *testing.T, s Store, chatID domain.ChatID) *domain.Chat {
	t.Helper()
	c, err := s.FindChat(context.Background(), chatID)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// appendN сохраняет n сообщений подряд, каждый раз перечитывая чат, как это делает use case.
func appendN(t *testing.T, s Store, chatID domain.ChatID, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		cur := find(t, s, chatID)
		v := cur.Version()
		if err := s.AppendMessage(context.Background(), cur, post(t, cur, i), v); err != nil {
			t.Fatal(err)
		}
	}
}

// seqs читает историю целиком и возвращает номера сообщений по порядку.
func seqs(t *testing.T, s Store, chatID domain.ChatID) []int64 {
	t.Helper()
	msgs, err := s.ListMessages(context.Background(), chatID, 0, domain.ListLimitMax)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]int64, 0, len(msgs))
	for _, m := range msgs {
		got = append(got, m.Seq())
	}
	return got
}

func testCreateAndFind(t *testing.T, s Store) {
	ctx := context.Background()
	c := newDirect(t, 1, 1, 2)
	create(t, s, c)
	got := find(t, s, c.ID())
	if got == c {
		t.Fatal("FindChat вернул тот же объект, что передали в CreateChat, а не прочитанную копию")
	}
	if got.ID() != c.ID() || got.Kind() != c.Kind() || fmt.Sprint(got.Members()) != fmt.Sprint(c.Members()) || got.Version() != 0 {
		t.Fatalf("найден не тот чат: %+v", got.Snapshot())
	}
	if _, err := s.FindChat(ctx, id(t, domain.ParseChatID, "9000", 99)); !errors.Is(err, domain.ErrChatNotFound) {
		t.Fatalf("неизвестный чат: ждали ErrChatNotFound, получили %v", err)
	}
	if _, err := s.FindDirectChat(ctx, user(t, 1), user(t, 3)); !errors.Is(err, domain.ErrChatNotFound) {
		t.Fatalf("неизвестная пара: ждали ErrChatNotFound, получили %v", err)
	}
}

func testDirectUnique(t *testing.T, s Store) {
	ctx := context.Background()
	first := newDirect(t, 1, 1, 2)
	create(t, s, first)
	// Та же пара в обратном порядке — тот же личный чат.
	second := newDirect(t, 2, 2, 1)
	if err := s.CreateChat(ctx, second); !errors.Is(err, domain.ErrDirectChatExists) {
		t.Fatalf("ждали ErrDirectChatExists, получили %v", err)
	}
	if _, err := s.FindChat(ctx, second.ID()); !errors.Is(err, domain.ErrChatNotFound) {
		t.Fatalf("отклонённый чат не должен сохраниться: %v", err)
	}
	got, err := s.FindDirectChat(ctx, user(t, 2), user(t, 1)) // порядок участников не важен
	if err != nil || got.ID() != first.ID() {
		t.Fatalf("FindDirectChat: %v", err)
	}
}

// #region version-test
func testVersionConflict(t *testing.T, s Store) {
	ctx := context.Background()
	c := newDirect(t, 1, 1, 2)
	create(t, s, c)
	first, second := find(t, s, c.ID()), find(t, s, c.ID())
	if err := s.AppendMessage(ctx, first, post(t, first, 1), 0); err != nil {
		t.Fatal(err)
	}
	// second прочитан до записи first — его версия 0 устарела.
	if err := s.AppendMessage(ctx, second, post(t, second, 2), 0); !errors.Is(err, domain.ErrConcurrentUpdate) {
		t.Fatalf("ждали ErrConcurrentUpdate, получили %v", err)
	}
	if got := seqs(t, s, c.ID()); fmt.Sprint(got) != "[1]" {
		t.Fatalf("после конфликта в истории должно остаться одно сообщение с seq 1: %v", got)
	}
	if got := find(t, s, c.ID()); got.Version() != 1 || got.LastSeq() != 1 {
		t.Fatalf("конфликт изменил чат: версия %d, lastSeq %d", got.Version(), got.LastSeq())
	}
}

// #endregion version-test

func testAppendUnknownChat(t *testing.T, s Store) {
	c := newDirect(t, 1, 1, 2) // не сохранён
	if err := s.AppendMessage(context.Background(), c, post(t, c, 1), 0); !errors.Is(err, domain.ErrChatNotFound) {
		t.Fatalf("ждали ErrChatNotFound, получили %v", err)
	}
}

func testListPaging(t *testing.T, s Store) {
	c := newDirect(t, 1, 1, 2)
	create(t, s, c)
	appendN(t, s, c.ID(), 5)
	tests := []struct {
		after    int64
		limit    int
		wantSeqs []int64
	}{
		{0, 2, []int64{1, 2}},
		{2, 2, []int64{3, 4}},
		{4, 10, []int64{5}},
		{5, 10, nil},
		{100, 10, nil},
		{0, 0, nil},  // порт обещает пустую страницу на limit < 1
		{0, -1, nil}, // и не падает на отрицательном
	}
	for _, tt := range tests {
		msgs, err := s.ListMessages(context.Background(), c.ID(), tt.after, tt.limit)
		if err != nil {
			t.Fatal(err)
		}
		var got []int64
		for _, m := range msgs {
			got = append(got, m.Seq())
		}
		if fmt.Sprint(got) != fmt.Sprint(tt.wantSeqs) {
			t.Fatalf("after=%d limit=%d: seq %v, ждали %v", tt.after, tt.limit, got, tt.wantSeqs)
		}
	}
}

func testListReturnsCopy(t *testing.T, s Store) {
	ctx := context.Background()
	c := newDirect(t, 1, 1, 2)
	create(t, s, c)
	appendN(t, s, c.ID(), 2)
	page, err := s.ListMessages(ctx, c.ID(), 0, 1)
	if err != nil || len(page) != 1 {
		t.Fatalf("первая страница: %d сообщений, %v", len(page), err)
	}
	rest, err := s.ListMessages(ctx, c.ID(), 1, 1)
	if err != nil || len(rest) != 1 {
		t.Fatalf("вторая страница: %d сообщений, %v", len(rest), err)
	}
	// Запись в элемент и append в свободную ёмкость не должны дойти до хранилища.
	page[0] = rest[0]
	_ = append(page, page[0])
	if got := seqs(t, s, c.ID()); fmt.Sprint(got) != "[1 2]" {
		t.Fatalf("история изменилась через возвращённую страницу: seq %v", got)
	}
}

func testConcurrentAppend(t *testing.T, s Store) {
	ctx := context.Background()
	c := newDirect(t, 1, 1, 2)
	create(t, s, c)
	const writers = 16
	// Всё, что может вызвать t.Fatal, готовим до горутин: FailNow из чужой горутины
	// запрещён (go vet, анализатор testinggoroutine).
	author := user(t, 1)
	text, err := domain.NewText("одновременно")
	if err != nil {
		t.Fatal(err)
	}
	msgIDs := make([]domain.MessageID, writers)
	for w := range writers {
		msgIDs[w] = id(t, domain.ParseMessageID, "a000", w+1)
	}
	var (
		wg sync.WaitGroup
		mu sync.Mutex
		ok int
	)
	// Общий старт: без него первые горутины успевают закончить, пока создаются следующие.
	start := make(chan struct{})
	for w := range writers {
		wg.Go(func() {
			<-start
			cur, err := s.FindChat(ctx, c.ID())
			if err != nil {
				t.Error(err)
				return
			}
			v := cur.Version()
			msg, err := cur.Post(msgIDs[w], author, text, t0)
			if err != nil {
				t.Error(err)
				return
			}
			err = s.AppendMessage(ctx, cur, msg, v)
			if err != nil && !errors.Is(err, domain.ErrConcurrentUpdate) {
				t.Error(err)
			}
			if err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
			// Чтение истории наперегонки с чужими записями — работа для детектора гонок.
			if _, err := s.ListMessages(ctx, c.ID(), 0, domain.ListLimitMax); err != nil {
				t.Error(err)
			}
		})
	}
	close(start)
	wg.Wait()
	got := seqs(t, s, c.ID())
	if len(got) != ok {
		t.Fatalf("успешных записей %d, в истории %d", ok, len(got))
	}
	for i, seq := range got {
		if seq != int64(i+1) {
			t.Fatalf("seq с пропуском или повтором: позиция %d, seq %d", i, seq)
		}
	}
}

// Отменённый или истёкший контекст останавливает каждый метод, и ни один ничего не пишет.
func testCanceledContext(t *testing.T, s Store) {
	c := newDirect(t, 1, 1, 2)
	create(t, s, c)
	other := newDirect(t, 2, 1, 3)
	msg := post(t, c, 1) // c теперь версии 1, в хранилище — 0
	a, b := user(t, 1), user(t, 2)

	calls := []struct {
		name string
		call func(ctx context.Context) error
	}{
		{"CreateChat", func(ctx context.Context) error { return s.CreateChat(ctx, other) }},
		{"FindChat", func(ctx context.Context) error { _, err := s.FindChat(ctx, c.ID()); return err }},
		{"FindDirectChat", func(ctx context.Context) error { _, err := s.FindDirectChat(ctx, a, b); return err }},
		{"AppendMessage", func(ctx context.Context) error { return s.AppendMessage(ctx, c, msg, 0) }},
		// limit 0: проверка контекста должна идти раньше проверки лимита.
		{"ListMessages", func(ctx context.Context) error { _, err := s.ListMessages(ctx, c.ID(), 0, 0); return err }},
	}
	for _, done := range doneContexts(t) {
		for _, tt := range calls {
			if err := tt.call(done.ctx); !errors.Is(err, done.want) {
				t.Errorf("%s, %s: ждали %v, получили %v", tt.name, done.name, done.want, err)
			}
		}
	}

	if _, err := s.FindChat(context.Background(), other.ID()); !errors.Is(err, domain.ErrChatNotFound) {
		t.Fatalf("отменённый CreateChat сохранил чат: %v", err)
	}
	if got := seqs(t, s, c.ID()); len(got) != 0 {
		t.Fatalf("отменённый AppendMessage записал сообщения: seq %v", got)
	}
	if got := find(t, s, c.ID()); got.Version() != 0 {
		t.Fatalf("отменённый AppendMessage изменил версию чата: %d", got.Version())
	}
}

type doneContext struct {
	name string
	ctx  context.Context
	want error
}

// doneContexts — контекст, отменённый явно, и контекст с дедлайном в прошлом.
func doneContexts(t *testing.T) []doneContext {
	t.Helper()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	t.Cleanup(cancelExpired)
	return []doneContext{
		{"отменённый", canceled, context.Canceled},
		{"истёкший", expired, context.DeadlineExceeded},
	}
}

func testFindReturnsCopy(t *testing.T, s Store) {
	ctx := context.Background()
	c := newDirect(t, 1, 1, 2)
	create(t, s, c)
	got := find(t, s, c.ID())
	post(t, got, 1) // меняем найденный агрегат, но не сохраняем
	byPair, err := s.FindDirectChat(ctx, user(t, 1), user(t, 2))
	if err != nil {
		t.Fatal(err)
	}
	post(t, byPair, 2)
	again := find(t, s, c.ID())
	if again.Version() != 0 || again.LastSeq() != 0 {
		t.Fatal("изменение найденного чата без AppendMessage не должно попадать в хранилище")
	}
}
