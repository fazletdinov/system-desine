package domain_test

import (
	"errors"
	"fmt"
	"slices"
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

func cmid(t *testing.T, n int) domain.ClientMessageID {
	t.Helper()
	id, err := domain.ParseClientMessageID(fmt.Sprintf("00000000-0000-7000-e000-%012d", n))
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
	if _, err := c.Post(mid(t, 1), cmid(t, 1), uid(t, 3), text(t, "я не участник"), t0); !errors.Is(err, domain.ErrNotMember) {
		t.Fatalf("ждали ErrNotMember, получили %v", err)
	}
	if c.LastSeq() != 0 || c.Version() != 0 {
		t.Fatal("отклонённое сообщение не должно менять чат")
	}
	msk := time.FixedZone("MSK", 3*60*60)
	for i := 1; i <= 3; i++ {
		sentAt := t0.In(msk)
		m, err := c.Post(mid(t, i), cmid(t, i), uid(t, 1), text(t, "привет"), sentAt)
		if err != nil {
			t.Fatal(err)
		}
		if m.Seq() != int64(i) || c.LastSeq() != int64(i) || c.Version() != int64(i) {
			t.Fatalf("сообщение %d: seq=%d lastSeq=%d version=%d", i, m.Seq(), c.LastSeq(), c.Version())
		}
		if m.ID() != mid(t, i) || m.ClientID() != cmid(t, i) || m.ChatID() != cid(t, 1) || m.AuthorID() != uid(t, 1) || m.Text().String() != "привет" {
			t.Fatalf("сообщение %d: поля не совпадают", i)
		}
		if m.SentAt().Location() != time.UTC || !m.SentAt().Equal(sentAt) {
			t.Fatalf("SentAt = %v, ждали тот же момент в UTC", m.SentAt())
		}
	}
}

func TestChatPostRejectsZeroArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		id        domain.MessageID
		clientID  domain.ClientMessageID
		author    domain.UserID
		text      domain.Text
		wantField string
	}{
		{name: "нулевой id сообщения", id: domain.MessageID{}, clientID: cmid(t, 1), author: uid(t, 1), text: text(t, "привет"), wantField: "message_id"},
		{name: "нулевой client_message_id", id: mid(t, 1), clientID: domain.ClientMessageID{}, author: uid(t, 1), text: text(t, "привет"), wantField: "client_message_id"},
		{name: "нулевой автор", id: mid(t, 1), clientID: cmid(t, 1), author: domain.UserID{}, text: text(t, "привет"), wantField: "author_id"},
		{name: "нулевой текст", id: mid(t, 1), clientID: cmid(t, 1), author: uid(t, 1), text: domain.Text{}, wantField: "text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, err := domain.NewDirectChat(cid(t, 1), uid(t, 1), uid(t, 2), t0)
			if err != nil {
				t.Fatal(err)
			}
			_, err = c.Post(tt.id, tt.clientID, tt.author, tt.text, t0)
			var ve *domain.ValidationError
			if !errors.As(err, &ve) || ve.Field != tt.wantField || ve.Reason != "не задано" {
				t.Fatalf("ждали «%s: не задано», получили %v", tt.wantField, err)
			}
			if c.LastSeq() != 0 || c.Version() != 0 {
				t.Fatal("отклонённое сообщение не должно менять чат")
			}
		})
	}
}

func TestNewChatRejectsZeroArgs(t *testing.T) {
	t.Parallel()
	title, err := domain.NewTitle("Семья")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		new  func(t *testing.T) (*domain.Chat, error)
	}{
		{name: "личный: нулевой id чата", new: func(t *testing.T) (*domain.Chat, error) {
			return domain.NewDirectChat(domain.ChatID{}, uid(t, 1), uid(t, 2), t0)
		}},
		{name: "личный: нулевой участник", new: func(t *testing.T) (*domain.Chat, error) {
			return domain.NewDirectChat(cid(t, 1), uid(t, 1), domain.UserID{}, t0)
		}},
		{name: "группа: нулевой id чата", new: func(t *testing.T) (*domain.Chat, error) {
			return domain.NewGroupChat(domain.ChatID{}, title, uid(t, 1), []domain.UserID{uid(t, 2)}, t0)
		}},
		{name: "группа: нулевое название", new: func(t *testing.T) (*domain.Chat, error) {
			return domain.NewGroupChat(cid(t, 1), domain.Title{}, uid(t, 1), []domain.UserID{uid(t, 2)}, t0)
		}},
		{name: "группа: нулевой создатель", new: func(t *testing.T) (*domain.Chat, error) {
			return domain.NewGroupChat(cid(t, 1), title, domain.UserID{}, []domain.UserID{uid(t, 2), uid(t, 3)}, t0)
		}},
		{name: "группа: нулевой участник", new: func(t *testing.T) (*domain.Chat, error) {
			return domain.NewGroupChat(cid(t, 1), title, uid(t, 1), []domain.UserID{uid(t, 2), {}}, t0)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if c, err := tt.new(t); err == nil {
				t.Fatalf("ждали ошибку, получили чат %v", c.ID())
			}
		})
	}
}

func TestDirectKey(t *testing.T) {
	t.Parallel()
	direct, err := domain.NewDirectChat(cid(t, 1), uid(t, 2), uid(t, 1), t0)
	if err != nil {
		t.Fatal(err)
	}
	if key, ok := direct.DirectKey(); !ok || key != domain.DirectKeyFor(uid(t, 1), uid(t, 2)) {
		t.Fatalf("DirectKey() = %q, %v", key, ok)
	}
	title, err := domain.NewTitle("Семья")
	if err != nil {
		t.Fatal(err)
	}
	group, err := domain.NewGroupChat(cid(t, 2), title, uid(t, 1), []domain.UserID{uid(t, 2)}, t0)
	if err != nil {
		t.Fatal(err)
	}
	if key, ok := group.DirectKey(); ok || key != "" {
		t.Fatalf("у группы нет ключа пары, получили %q, %v", key, ok)
	}
}

func TestParseKind(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in      string
		want    domain.Kind
		wantErr bool
	}{
		{in: "direct", want: domain.KindDirect},
		{in: "group", want: domain.KindGroup},
		{in: "Group", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tt := range tests {
		got, err := domain.ParseKind(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Fatalf("ParseKind(%q) = %q, %v", tt.in, got, err)
		}
	}
}

func TestParseIDsRejectBadInput(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"", "0192F0C1-7E3A-7B4C-8D5E-6F7A8B9C0D1E", "0192f0c17e3a7b4c8d5e6f7a8b9c0d1e", "0192f0c1-7e3a-7b4c-8d5e-6f7a8b9c0d1"} {
		if _, err := domain.ParseChatID(in); err == nil {
			t.Errorf("ParseChatID(%q): ждали ошибку", in)
		}
		if _, err := domain.ParseMessageID(in); err == nil {
			t.Errorf("ParseMessageID(%q): ждали ошибку", in)
		}
		if _, err := domain.ParseUserID(in); err == nil {
			t.Errorf("ParseUserID(%q): ждали ошибку", in)
		}
		if _, err := domain.ParseClientMessageID(in); err == nil {
			t.Errorf("ParseClientMessageID(%q): ждали ошибку", in)
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
	// Проверяем и состав, и участника, которого «затёрли»: будь срез общим, uid(1) исчез бы из чата.
	if want := []domain.UserID{uid(t, 1), uid(t, 2)}; !slices.Equal(c.Members(), want) || !c.IsMember(uid(t, 1)) {
		t.Fatalf("Members() должен отдавать копию: снаружи нельзя менять агрегат, участники %v", c.Members())
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	t.Parallel()
	title, err := domain.NewTitle("Семья")
	if err != nil {
		t.Fatal(err)
	}
	c, err := domain.NewGroupChat(cid(t, 1), title, uid(t, 1), []domain.UserID{uid(t, 2), uid(t, 3)}, t0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Post(mid(t, 1), cmid(t, 1), uid(t, 2), text(t, "привет"), t0); err != nil {
		t.Fatal(err)
	}
	snap := c.Snapshot()
	r := domain.RestoreChat(snap)
	if r.ID() != c.ID() || r.Kind() != c.Kind() || r.Title() != c.Title() || !r.CreatedAt().Equal(c.CreatedAt()) ||
		!slices.Equal(r.Members(), c.Members()) || r.LastSeq() != 1 || r.Version() != 1 {
		t.Fatalf("восстановленный чат отличается: %+v", r.Snapshot())
	}
	// Восстановленный чат — рабочий агрегат: следующий seq продолжает счёт.
	if m, err := r.Post(mid(t, 2), cmid(t, 2), uid(t, 3), text(t, "и тебе"), t0); err != nil || m.Seq() != 2 {
		t.Fatalf("Post после RestoreChat: seq %d, ошибка %v", m.Seq(), err)
	}
	snap.Members[0] = uid(t, 9)
	if !c.IsMember(uid(t, 1)) || c.IsMember(uid(t, 9)) {
		t.Fatalf("снимок делит срез участников с чатом: %v", c.Members())
	}
	if !r.IsMember(uid(t, 1)) || r.IsMember(uid(t, 9)) {
		t.Fatalf("RestoreChat делит срез участников со снимком: %v", r.Members())
	}
}

func TestPostMovesLastActivityForwardOnly(t *testing.T) {
	t.Parallel()
	c, err := domain.NewDirectChat(cid(t, 1), uid(t, 1), uid(t, 2), t0)
	if err != nil {
		t.Fatal(err)
	}
	if !c.LastActivityAt().Equal(t0) {
		t.Fatalf("у нового чата активность %v, ждали время создания %v", c.LastActivityAt(), t0)
	}
	post := func(n int, at time.Time) {
		t.Helper()
		if _, err := c.Post(mid(t, n), cmid(t, n), c.Members()[0], text(t, "привет"), at); err != nil {
			t.Fatal(err)
		}
	}
	post(1, t0.Add(time.Minute))
	post(2, t0.Add(30*time.Second)) // часы другого сервера отстают — назад не идём
	if want := t0.Add(time.Minute); !c.LastActivityAt().Equal(want) {
		t.Fatalf("активность %v, ждали %v", c.LastActivityAt(), want)
	}
	if got := domain.RestoreChat(c.Snapshot()).LastActivityAt(); !got.Equal(c.LastActivityAt()) {
		t.Fatalf("снимок потерял активность: %v", got)
	}
}

func TestParseClientMessageID(t *testing.T) {
	t.Parallel()
	const valid = "0192f0c1-7e3a-7b4c-8d5e-6f7a8b9c0d1e"
	got, err := domain.ParseClientMessageID(valid)
	if err != nil || got.String() != valid || got.IsZero() {
		t.Fatalf("ParseClientMessageID(%q) = %q, %v", valid, got, err)
	}
	_, err = domain.ParseClientMessageID("не-uuid")
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Field != "client_message_id" {
		t.Fatalf("ждали ValidationError по client_message_id, получили %v", err)
	}
}
