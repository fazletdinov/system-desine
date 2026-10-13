package domain_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

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

func mid(t *testing.T, n int) domain.MessageID {
	t.Helper()
	id, err := domain.ParseMessageID(fmt.Sprintf("00000000-0000-7000-a000-%012d", n))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func text(t *testing.T, s string) domain.Text {
	t.Helper()
	x, err := domain.NewText(s)
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func TestNewDirectChat(t *testing.T) {
	t.Parallel()
	if _, err := domain.NewDirectChat(cid(t, 1), uid(t, 1), uid(t, 1), t0); err == nil {
		t.Fatal("личный чат с самим собой: ждали ошибку")
	}
	c, err := domain.NewDirectChat(cid(t, 1), uid(t, 2), uid(t, 1), t0)
	if err != nil {
		t.Fatal(err)
	}
	if c.Kind() != domain.KindDirect || len(c.Members()) != 2 || !c.IsMember(uid(t, 1)) || !c.IsMember(uid(t, 2)) {
		t.Fatalf("неверный чат: %+v", c.Members())
	}
	if domain.DirectKeyFor(uid(t, 1), uid(t, 2)) != domain.DirectKeyFor(uid(t, 2), uid(t, 1)) {
		t.Fatal("ключ личного чата должен не зависеть от порядка участников")
	}
}

func TestNewGroupChatMembers(t *testing.T) {
	t.Parallel()
	title, err := domain.NewTitle("Семья")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		others  int
		wantLen int
		wantErr bool
	}{
		{name: "создатель один — мало", others: 0, wantErr: true},
		{name: "создатель и один участник", others: 1, wantLen: 2},
		{name: "ровно максимум", others: domain.GroupMaxMembers - 1, wantLen: domain.GroupMaxMembers},
		{name: "максимум + 1", others: domain.GroupMaxMembers, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			others := make([]domain.UserID, 0, tt.others+1)
			for i := range tt.others {
				others = append(others, uid(t, i+2))
			}
			others = append(others, uid(t, 1)) // создатель в списке ещё раз — дубль схлопывается
			c, err := domain.NewGroupChat(cid(t, 1), title, uid(t, 1), others, t0)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ждали ошибку, получили %d участников", len(c.Members()))
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(c.Members()) != tt.wantLen || !c.IsMember(uid(t, 1)) {
				t.Fatalf("участников %d, ждали %d, создатель внутри: %v", len(c.Members()), tt.wantLen, c.IsMember(uid(t, 1)))
			}
		})
	}
}

func TestChatPost(t *testing.T) {
	t.Parallel()
	c, err := domain.NewDirectChat(cid(t, 1), uid(t, 1), uid(t, 2), t0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Post(mid(t, 1), uid(t, 3), text(t, "я не участник"), t0); !errors.Is(err, domain.ErrNotMember) {
		t.Fatalf("ждали ErrNotMember, получили %v", err)
	}
	if c.LastSeq() != 0 || c.Version() != 0 {
		t.Fatal("отклонённое сообщение не должно менять чат")
	}
	for i := 1; i <= 3; i++ {
		m, err := c.Post(mid(t, i), uid(t, 1), text(t, "привет"), t0)
		if err != nil {
			t.Fatal(err)
		}
		if m.Seq() != int64(i) || c.LastSeq() != int64(i) || c.Version() != int64(i) {
			t.Fatalf("сообщение %d: seq=%d lastSeq=%d version=%d", i, m.Seq(), c.LastSeq(), c.Version())
		}
	}
}

func TestMembersIsCopy(t *testing.T) {
	t.Parallel()
	c, err := domain.NewDirectChat(cid(t, 1), uid(t, 1), uid(t, 2), t0)
	if err != nil {
		t.Fatal(err)
	}
	m := c.Members()
	m[0] = uid(t, 9)
	if c.IsMember(uid(t, 9)) {
		t.Fatal("Members() должен отдавать копию: снаружи нельзя менять агрегат")
	}
}
