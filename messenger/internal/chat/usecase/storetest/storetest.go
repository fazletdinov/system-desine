// Package storetest — контрактные тесты хранилища чатов: одна батарея проверок
// для любой реализации портов. Сейчас её проходит memory.Store, в модуле 2a —
// Postgres-адаптер. Если обе реализации проходят контракт, use case'ам всё равно,
// с какой из них работать.
package storetest

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
	usecase.ChatLister
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
		{"история до seq", testListMessagesBefore},
		{"страница истории — копия", testListReturnsCopy},
		{"чаты участника по активности", testListChatsByMember},
		{"чаты с одним временем — порядок по id", testListChatsTieBreak},
		{"limit < 1 у списка чатов", testListChatsLimit},
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

// seqs читает историю целиком и возвращает номера сообщений по возрастанию.
func seqs(t *testing.T, s Store, chatID domain.ChatID) []int64 {
	t.Helper()
	msgs, err := s.ListMessagesBefore(context.Background(), chatID, 0, domain.ListLimitMax)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]int64, 0, len(msgs))
	for _, m := range msgs {
		got = append(got, m.Seq())
	}
	slices.Reverse(got) // хранилище отдаёт от новых к старым
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
	winner := post(t, first, 1)
	if err := s.AppendMessage(ctx, first, winner, 0); err != nil {
		t.Fatal(err)
	}
	// second прочитан до записи first — его версия 0 устарела.
	if err := s.AppendMessage(ctx, second, post(t, second, 2), 0); !errors.Is(err, domain.ErrConcurrentUpdate) {
		t.Fatalf("ждали ErrConcurrentUpdate, получили %v", err)
	}
	// В истории — сообщение победителя: опоздавший не должен его ни дополнить, ни заменить.
	msgs, err := s.ListMessagesBefore(ctx, c.ID(), 0, domain.ListLimitMax)
	if err != nil || len(msgs) != 1 || msgs[0].ID() != winner.ID() || msgs[0].Text() != winner.Text() {
		t.Fatalf("после конфликта в истории должно остаться только сообщение победителя: %d сообщений, %v", len(msgs), err)
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

func testListMessagesBefore(t *testing.T, s Store) {
	c := newDirect(t, 1, 1, 2)
	create(t, s, c)
	appendN(t, s, c.ID(), 5)
	tests := []struct {
		before   int64
		limit    int
		wantSeqs []int64
	}{
		{0, 2, []int64{5, 4}}, // 0 — с самого нового
		{4, 2, []int64{3, 2}},
		{2, 10, []int64{1}},
		{1, 10, nil},
		{100, 2, []int64{5, 4}}, // ключ больше любого — тоже с самого нового
		{0, 0, nil},             // порт обещает пустую страницу на limit < 1
		{0, -1, nil},            // и не падает на отрицательном
	}
	for _, tt := range tests {
		msgs, err := s.ListMessagesBefore(context.Background(), c.ID(), tt.before, tt.limit)
		if err != nil {
			t.Fatal(err)
		}
		var got []int64
		for _, m := range msgs {
			got = append(got, m.Seq())
		}
		if fmt.Sprint(got) != fmt.Sprint(tt.wantSeqs) {
			t.Fatalf("before=%d limit=%d: seq %v, ждали %v", tt.before, tt.limit, got, tt.wantSeqs)
		}
	}
}

func testListReturnsCopy(t *testing.T, s Store) {
	ctx := context.Background()
	c := newDirect(t, 1, 1, 2)
	create(t, s, c)
	appendN(t, s, c.ID(), 2)
	page, err := s.ListMessagesBefore(ctx, c.ID(), 0, 1)
	if err != nil || len(page) != 1 {
		t.Fatalf("первая страница: %d сообщений, %v", len(page), err)
	}
	rest, err := s.ListMessagesBefore(ctx, c.ID(), 2, 1)
	if err != nil || len(rest) != 1 {
		t.Fatalf("вторая страница: %d сообщений, %v", len(rest), err)
	}
	// Запись в элемент страницы не должна дойти до хранилища.
	page[0] = rest[0]
	if got := seqs(t, s, c.ID()); fmt.Sprint(got) != "[1 2]" {
		t.Fatalf("история изменилась через возвращённую страницу: seq %v", got)
	}
}

// postAt сохраняет сообщение от первого участника со временем at — так тест двигает
// активность чата.
func postAt(t *testing.T, s Store, chatID domain.ChatID, n int, at time.Time) {
	t.Helper()
	err := tryAppend(context.Background(), s, chatID, func(cur *domain.Chat) (domain.Message, error) {
		text, err := domain.NewText(fmt.Sprintf("сообщение %d", n))
		if err != nil {
			return domain.Message{}, err
		}
		return cur.Post(id(t, domain.ParseMessageID, "a000", n), cur.Members()[0], text, at)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// listChats — id чатов страницы по порядку.
func listChats(t *testing.T, s Store, member domain.UserID, after *usecase.ChatPosition, limit int) []domain.ChatID {
	t.Helper()
	chats, err := s.ListChatsByMember(context.Background(), member, after, limit)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]domain.ChatID, 0, len(chats))
	for _, c := range chats {
		ids = append(ids, c.ID())
	}
	return ids
}

func testListChatsByMember(t *testing.T, s Store) {
	a, b, c := newDirect(t, 1, 1, 2), newDirect(t, 2, 1, 3), newDirect(t, 3, 1, 4)
	foreign := newDirect(t, 4, 2, 3) // без пользователя 1
	for _, ch := range []*domain.Chat{a, b, c, foreign} {
		create(t, s, ch)
	}
	// Активность: b — самый свежий, затем a, затем c (без сообщений, время создания).
	postAt(t, s, a.ID(), 1, t0.Add(time.Minute))
	postAt(t, s, b.ID(), 2, t0.Add(2*time.Minute))
	postAt(t, s, foreign.ID(), 3, t0.Add(time.Hour))

	all := listChats(t, s, user(t, 1), nil, 10)
	if want := []domain.ChatID{b.ID(), a.ID(), c.ID()}; fmt.Sprint(all) != fmt.Sprint(want) {
		t.Fatalf("чаты %v, ждали %v", all, want)
	}
	// after строгий: чат на самой позиции не повторяется.
	after := &usecase.ChatPosition{LastActivityAt: t0.Add(time.Minute), ID: a.ID()}
	if got, want := listChats(t, s, user(t, 1), after, 10), []domain.ChatID{c.ID()}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("после a: %v, ждали %v", got, want)
	}
	if got := listChats(t, s, user(t, 1), nil, 1); len(got) != 1 || got[0] != b.ID() {
		t.Fatalf("limit 1: %v, ждали только %s", got, b.ID())
	}
	got, err := s.ListChatsByMember(context.Background(), user(t, 1), nil, 1)
	if err != nil || !got[0].LastActivityAt().Equal(t0.Add(2*time.Minute)) {
		t.Fatalf("хранилище потеряло активность: %v, %v", got[0].LastActivityAt(), err)
	}
}

// Три чата без сообщений, созданные в один момент: порядок держится на id, и страница
// с курсором на границе не теряет и не повторяет чат.
func testListChatsTieBreak(t *testing.T, s Store) {
	chats := []*domain.Chat{newDirect(t, 1, 1, 2), newDirect(t, 2, 1, 3), newDirect(t, 3, 1, 4)}
	for _, c := range chats {
		create(t, s, c)
	}
	first := listChats(t, s, user(t, 1), nil, 2)
	if want := []domain.ChatID{chats[2].ID(), chats[1].ID()}; fmt.Sprint(first) != fmt.Sprint(want) {
		t.Fatalf("первая страница %v, ждали %v (по убыванию id)", first, want)
	}
	after := &usecase.ChatPosition{LastActivityAt: t0, ID: first[1]}
	if got, want := listChats(t, s, user(t, 1), after, 2), []domain.ChatID{chats[0].ID()}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("вторая страница %v, ждали %v", got, want)
	}
}

func testListChatsLimit(t *testing.T, s Store) {
	create(t, s, newDirect(t, 1, 1, 2))
	for _, limit := range []int{0, -1} {
		if got := listChats(t, s, user(t, 1), nil, limit); len(got) != 0 {
			t.Fatalf("limit %d: %v, ждали пустую страницу", limit, got)
		}
	}
}

// Окно гонки «прочитал версию — записал» узкое, и один забег его может не задеть.
// Поэтому забегов несколько, каждый — в своём чате.
func testConcurrentAppend(t *testing.T, s Store) {
	const rounds = 20
	for r := range rounds {
		c := newDirect(t, r+1, 2*r+1, 2*r+2)
		create(t, s, c)
		raceAppend(t, s, c, r)
	}
}

// raceAppend запускает одновременные записи в чат c и проверяет историю после них.
func raceAppend(t *testing.T, s Store, c *domain.Chat, round int) {
	t.Helper()
	ctx := context.Background()
	const writers = 16
	// Всё, что может вызвать t.Fatal, готовим до горутин: FailNow из чужой горутины
	// запрещён (go vet, анализатор testinggoroutine).
	author := c.Members()[0]
	text, err := domain.NewText("одновременно")
	if err != nil {
		t.Fatal(err)
	}
	msgIDs := make([]domain.MessageID, writers)
	for w := range writers {
		msgIDs[w] = id(t, domain.ParseMessageID, "a000", round*writers+w+1)
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
			err := tryAppend(ctx, s, c.ID(), func(cur *domain.Chat) (domain.Message, error) {
				return cur.Post(msgIDs[w], author, text, t0)
			})
			switch {
			case err == nil:
				mu.Lock()
				ok++
				mu.Unlock()
			case !errors.Is(err, domain.ErrConcurrentUpdate):
				t.Error(err)
			}
			// Чтение истории наперегонки с чужими записями — работа для детектора гонок.
			if _, err := s.ListMessagesBefore(ctx, c.ID(), 0, domain.ListLimitMax); err != nil {
				t.Error(err)
			}
		})
	}
	close(start)
	wg.Wait()
	// Хотя бы одна запись обязана пройти: хранилище, которое отказывает всем, контракт не выполняет.
	if ok < 1 {
		t.Fatal("ни одна из одновременных записей не прошла")
	}
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

// tryAppend — одна попытка записи, как в use case: прочитать чат, добавить сообщение
// через агрегат и сохранить с версией, которую прочитали.
func tryAppend(ctx context.Context, s Store, chatID domain.ChatID, post func(*domain.Chat) (domain.Message, error)) error {
	cur, err := s.FindChat(ctx, chatID)
	if err != nil {
		return err
	}
	v := cur.Version()
	msg, err := post(cur)
	if err != nil {
		return err
	}
	return s.AppendMessage(ctx, cur, msg, v)
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
		{"ListMessagesBefore", func(ctx context.Context) error { _, err := s.ListMessagesBefore(ctx, c.ID(), 0, 0); return err }},
		{"ListChatsByMember", func(ctx context.Context) error { _, err := s.ListChatsByMember(ctx, a, nil, 0); return err }},
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
