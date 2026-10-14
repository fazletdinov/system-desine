package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"messenger/internal/chat/domain"
	"messenger/internal/chat/usecase"
)

// Листаем историю, пока в чат пишут: курсор не даёт ни дублей, ни пропусков среди тех
// сообщений, что были до начала листания.
func TestListMessagesStableUnderInserts(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	c := directChat(t, e)
	ctx := context.Background()
	send := func(n int) {
		t.Helper()
		if _, err := e.send.Execute(ctx, usecase.SendMessageCommand{ChatID: c.ID().String(), AuthorID: alice, ClientMessageID: clientID(n), Text: fmt.Sprintf("сообщение %d", n)}); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= 12; i++ {
		send(i)
	}
	var seen []int64
	before := int64(0)
	for page := 0; ; page++ {
		res, err := e.list.Execute(ctx, usecase.ListMessagesQuery{ChatID: c.ID().String(), ReaderID: bob, BeforeSeq: before, Limit: 5})
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range res.Messages {
			seen = append(seen, m.Seq())
		}
		if page == 0 {
			for i := 13; i <= 15; i++ { // новые сообщения между первой и второй страницей
				send(i)
			}
		}
		if res.NextBeforeSeq == 0 {
			break
		}
		before = res.NextBeforeSeq
	}
	want := []int64{12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	if fmt.Sprint(seen) != fmt.Sprint(want) {
		t.Fatalf("прочитали %v, ждали %v", seen, want)
	}
}

// Ровно limit сообщений — следующей страницы нет, и клиент не делает лишний пустой запрос.
func TestListMessagesLastPageHasNoCursor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		messages int
		wantNext int64
	}{
		{name: "ровно limit — курсора нет", messages: 3, wantNext: 0},
		{name: "limit+1 — курсор на последнем показанном", messages: 4, wantNext: 2},
		{name: "пустой чат", messages: 0, wantNext: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			c := directChat(t, e)
			for i := range tt.messages {
				if _, err := e.send.Execute(context.Background(), usecase.SendMessageCommand{ChatID: c.ID().String(), AuthorID: alice, ClientMessageID: clientID(i + 1), Text: fmt.Sprint(i)}); err != nil {
					t.Fatal(err)
				}
			}
			res, err := e.list.Execute(context.Background(), usecase.ListMessagesQuery{ChatID: c.ID().String(), ReaderID: alice, Limit: 3})
			if err != nil {
				t.Fatal(err)
			}
			if res.NextBeforeSeq != tt.wantNext || len(res.Messages) != min(tt.messages, 3) {
				t.Fatalf("сообщений %d, next %d; ждали %d и %d", len(res.Messages), res.NextBeforeSeq, min(tt.messages, 3), tt.wantNext)
			}
		})
	}
}

func TestListChatsPaging(t *testing.T) {
	t.Parallel()
	clk := &steppingClock{}
	e := newEnvWithClock(t, clk)
	ctx := context.Background()
	create := func(cmd usecase.CreateChatCommand) *domain.Chat {
		t.Helper()
		clk.Advance()
		res, err := e.create.Execute(ctx, cmd)
		if err != nil {
			t.Fatal(err)
		}
		return res.Chat
	}
	withBob := create(usecase.CreateChatCommand{CreatorID: alice, Kind: "direct", MemberIDs: []string{bob}})
	withEve := create(usecase.CreateChatCommand{CreatorID: alice, Kind: "direct", MemberIDs: []string{eve}})
	group := create(usecase.CreateChatCommand{CreatorID: alice, Kind: "group", Title: "Семья", MemberIDs: []string{bob, eve}})
	foreign := create(usecase.CreateChatCommand{CreatorID: eve, Kind: "direct", MemberIDs: []string{bob}})
	// Сообщение поднимает самый старый чат наверх.
	clk.Advance()
	if _, err := e.send.Execute(ctx, usecase.SendMessageCommand{ChatID: withBob.ID().String(), AuthorID: bob, ClientMessageID: clientID(1), Text: "привет"}); err != nil {
		t.Fatal(err)
	}

	first, err := e.chats.Execute(ctx, usecase.ListChatsQuery{MemberID: alice, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got := chatIDs(first.Chats); fmt.Sprint(got) != fmt.Sprint([]domain.ChatID{withBob.ID(), group.ID()}) {
		t.Fatalf("первая страница %v, ждали [с Бобом, группа]", got)
	}
	if first.Next == nil || first.Next.ID != group.ID() || !first.Next.LastActivityAt.Equal(group.LastActivityAt()) {
		t.Fatalf("курсор первой страницы %+v, ждали позицию группы", first.Next)
	}
	second, err := e.chats.Execute(ctx, usecase.ListChatsQuery{MemberID: alice, After: first.Next, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got := chatIDs(second.Chats); fmt.Sprint(got) != fmt.Sprint([]domain.ChatID{withEve.ID()}) || second.Next != nil {
		t.Fatalf("вторая страница %v, next %+v; ждали [с Евой] без курсора", got, second.Next)
	}
	for _, c := range append(first.Chats, second.Chats...) {
		if c.ID() == foreign.ID() {
			t.Fatal("Алисе виден чат Евы и Боба")
		}
	}
}

func chatIDs(chats []*domain.Chat) []domain.ChatID {
	ids := make([]domain.ChatID, len(chats))
	for i, c := range chats {
		ids[i] = c.ID()
	}
	return ids
}

func TestListChatsInvalid(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	tests := []struct {
		name  string
		q     usecase.ListChatsQuery
		field string
	}{
		{name: "кривой участник", q: usecase.ListChatsQuery{MemberID: "alice"}, field: "user_id"},
		{name: "limit больше максимума", q: usecase.ListChatsQuery{MemberID: alice, Limit: domain.ListLimitMax + 1}, field: "limit"},
		{name: "отрицательный limit", q: usecase.ListChatsQuery{MemberID: alice, Limit: -1}, field: "limit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := e.chats.Execute(context.Background(), tt.q)
			var ve *domain.ValidationError
			if !errors.As(err, &ve) || ve.Field != tt.field {
				t.Fatalf("ждали ValidationError по %s, получили %v", tt.field, err)
			}
		})
	}
}

func TestGetChat(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	c := directChat(t, e)
	tests := []struct {
		name      string
		q         usecase.GetChatQuery
		wantErr   error
		wantField string
	}{
		{name: "участник видит чат", q: usecase.GetChatQuery{ChatID: c.ID().String(), ReaderID: bob}},
		{name: "посторонний — не участник", q: usecase.GetChatQuery{ChatID: c.ID().String(), ReaderID: eve}, wantErr: domain.ErrNotMember},
		{name: "неизвестный чат", q: usecase.GetChatQuery{ChatID: "00000000-0000-7000-9000-000000000099", ReaderID: bob}, wantErr: domain.ErrChatNotFound},
		{name: "кривой id чата", q: usecase.GetChatQuery{ChatID: "x", ReaderID: bob}, wantField: "chat_id"},
		{name: "кривой id читателя", q: usecase.GetChatQuery{ChatID: c.ID().String(), ReaderID: "x"}, wantField: "user_id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := e.get.Execute(context.Background(), tt.q)
			switch {
			case tt.wantField != "":
				var ve *domain.ValidationError
				if !errors.As(err, &ve) || ve.Field != tt.wantField {
					t.Fatalf("ждали ValidationError по %s, получили %v", tt.wantField, err)
				}
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ждали %v, получили %v", tt.wantErr, err)
				}
			default:
				if err != nil || got.ID() != c.ID() {
					t.Fatalf("ждали чат %s, получили %v, %v", c.ID(), got, err)
				}
			}
		})
	}
}
