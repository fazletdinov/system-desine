package memory_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"messenger/internal/chat/adapters/memory"
	"messenger/internal/chat/domain"
)

var t0 = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

func uid(t *testing.T, n int) domain.UserID {
	t.Helper()
	id, err := domain.ParseUserID(fmt.Sprintf("00000000-0000-7000-8000-%012d", n))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func cid(t *testing.T, n int) domain.ChatID {
	t.Helper()
	id, err := domain.ParseChatID(fmt.Sprintf("00000000-0000-7000-9000-%012d", n))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func direct(t *testing.T, n, a, b int) *domain.Chat {
	t.Helper()
	c, err := domain.NewDirectChat(cid(t, n), uid(t, a), uid(t, b), t0)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// post добавляет в чат сообщение от участника a и сохраняет его.
func post(t *testing.T, s *memory.Store, c *domain.Chat, n int) domain.Message {
	t.Helper()
	id, err := domain.ParseMessageID(fmt.Sprintf("00000000-0000-7000-a000-%012d", n))
	if err != nil {
		t.Fatal(err)
	}
	text, err := domain.NewText("привет")
	if err != nil {
		t.Fatal(err)
	}
	expected := c.Version()
	m, err := c.Post(id, c.Members()[0], text, t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AppendMessage(context.Background(), c, m, expected); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestCreateAndFindChat(t *testing.T) {
	t.Parallel()
	s := memory.NewStore()
	ctx := context.Background()
	c := direct(t, 1, 1, 2)
	if err := s.CreateChat(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err := s.FindChat(ctx, c.ID())
	if err != nil || got.ID() != c.ID() || got == c {
		t.Fatalf("FindChat: %v, %v — ждали копию сохранённого чата", got, err)
	}
	byPair, err := s.FindDirectChat(ctx, uid(t, 2), uid(t, 1))
	if err != nil || byPair.ID() != c.ID() {
		t.Fatalf("FindDirectChat в обратном порядке участников: %v, %v", byPair, err)
	}
	if _, err := s.FindChat(ctx, cid(t, 9)); !errors.Is(err, domain.ErrChatNotFound) {
		t.Fatalf("неизвестный чат: ждали ErrChatNotFound, получили %v", err)
	}
	if _, err := s.FindDirectChat(ctx, uid(t, 1), uid(t, 3)); !errors.Is(err, domain.ErrChatNotFound) {
		t.Fatalf("неизвестная пара: ждали ErrChatNotFound, получили %v", err)
	}
}

func TestCreateDirectChatForTakenPair(t *testing.T) {
	t.Parallel()
	s := memory.NewStore()
	ctx := context.Background()
	if err := s.CreateChat(ctx, direct(t, 1, 1, 2)); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateChat(ctx, direct(t, 2, 2, 1)); !errors.Is(err, domain.ErrDirectChatExists) {
		t.Fatalf("ждали ErrDirectChatExists, получили %v", err)
	}
	if _, err := s.FindChat(ctx, cid(t, 2)); !errors.Is(err, domain.ErrChatNotFound) {
		t.Fatalf("отклонённый чат не должен сохраниться: %v", err)
	}
}

func TestAppendMessageStaleVersion(t *testing.T) {
	t.Parallel()
	s := memory.NewStore()
	ctx := context.Background()
	c := direct(t, 1, 1, 2)
	if err := s.CreateChat(ctx, c); err != nil {
		t.Fatal(err)
	}
	// Две копии чата прочитаны с одной версией 0. Первая сохраняется, вторая опоздала.
	stale, err := s.FindChat(ctx, c.ID())
	if err != nil {
		t.Fatal(err)
	}
	post(t, s, c, 1)
	id, err := domain.ParseMessageID("00000000-0000-7000-a000-000000000002")
	if err != nil {
		t.Fatal(err)
	}
	text, err := domain.NewText("опоздавшее")
	if err != nil {
		t.Fatal(err)
	}
	m, err := stale.Post(id, uid(t, 2), text, t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AppendMessage(ctx, stale, m, 0); !errors.Is(err, domain.ErrConcurrentUpdate) {
		t.Fatalf("ждали ErrConcurrentUpdate, получили %v", err)
	}
	msgs, err := s.ListMessages(ctx, c.ID(), 0, domain.ListLimitMax)
	if err != nil || len(msgs) != 1 || msgs[0].Text().String() != "привет" {
		t.Fatalf("история изменилась после конфликта: %d сообщений, %v", len(msgs), err)
	}
	if got, err := s.FindChat(ctx, c.ID()); err != nil || got.Version() != 1 || got.LastSeq() != 1 {
		t.Fatalf("состояние чата изменилось после конфликта: %v", err)
	}
}

func TestAppendMessageUnknownChat(t *testing.T) {
	t.Parallel()
	s := memory.NewStore()
	c := direct(t, 1, 1, 2)
	text, err := domain.NewText("привет")
	if err != nil {
		t.Fatal(err)
	}
	id, err := domain.ParseMessageID("00000000-0000-7000-a000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	m, err := c.Post(id, uid(t, 1), text, t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AppendMessage(context.Background(), c, m, 0); !errors.Is(err, domain.ErrChatNotFound) {
		t.Fatalf("ждали ErrChatNotFound, получили %v", err)
	}
}

func TestListMessagesPages(t *testing.T) {
	t.Parallel()
	s := memory.NewStore()
	ctx := context.Background()
	c := direct(t, 1, 1, 2)
	if err := s.CreateChat(ctx, c); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 5; i++ {
		post(t, s, c, i)
	}
	tests := []struct {
		name     string
		after    int64
		limit    int
		wantSeqs []int64
	}{
		{name: "с начала", after: 0, limit: 2, wantSeqs: []int64{1, 2}},
		{name: "из середины", after: 2, limit: 2, wantSeqs: []int64{3, 4}},
		{name: "хвост короче лимита", after: 4, limit: 10, wantSeqs: []int64{5}},
		{name: "after за концом", after: 99, limit: 10, wantSeqs: []int64{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			msgs, err := s.ListMessages(ctx, c.ID(), tt.after, tt.limit)
			if err != nil {
				t.Fatal(err)
			}
			got := make([]int64, len(msgs))
			for i, m := range msgs {
				got[i] = m.Seq()
			}
			if fmt.Sprint(got) != fmt.Sprint(tt.wantSeqs) {
				t.Fatalf("seq %v, ждали %v", got, tt.wantSeqs)
			}
		})
	}
}

func TestListMessagesReturnsCopy(t *testing.T) {
	t.Parallel()
	s := memory.NewStore()
	ctx := context.Background()
	c := direct(t, 1, 1, 2)
	if err := s.CreateChat(ctx, c); err != nil {
		t.Fatal(err)
	}
	first := post(t, s, c, 1)
	second := post(t, s, c, 2)
	page, err := s.ListMessages(ctx, c.ID(), 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	// Запись в элемент и append в свободную ёмкость не должны дойти до хранилища.
	page[0] = second
	_ = append(page, first)
	got, err := s.ListMessages(ctx, c.ID(), 0, 2)
	if err != nil || len(got) != 2 || got[0].Seq() != 1 || got[1].Seq() != 2 {
		t.Fatalf("история изменилась через возвращённую страницу: %v, %v", got, err)
	}
}

func TestListMessagesNonPositiveLimit(t *testing.T) {
	t.Parallel()
	s := memory.NewStore()
	ctx := context.Background()
	c := direct(t, 1, 1, 2)
	if err := s.CreateChat(ctx, c); err != nil {
		t.Fatal(err)
	}
	post(t, s, c, 1)
	for _, limit := range []int{0, -1} {
		got, err := s.ListMessages(ctx, c.ID(), 0, limit)
		if err != nil || len(got) != 0 {
			t.Fatalf("limit %d: ждали пустую страницу, получили %d сообщений, %v", limit, len(got), err)
		}
	}
}
