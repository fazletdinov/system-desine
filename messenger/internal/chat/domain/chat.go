package domain

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

const (
	GroupMinMembers  = 2
	GroupMaxMembers  = 200
	ListLimitDefault = 50
	ListLimitMax     = 100
)

type Kind string

const (
	KindDirect Kind = "direct"
	KindGroup  Kind = "group"
)

func ParseKind(s string) (Kind, error) {
	switch k := Kind(s); k {
	case KindDirect, KindGroup:
		return k, nil
	default:
		return "", invalid("kind", `"direct" или "group"`)
	}
}

// #region chat
// Chat — агрегат: граница, внутри которой соблюдаются инварианты
// «отправитель — участник» и «seq идут подряд без пропусков».
// Менять чат можно только через его методы.
type Chat struct {
	id        ChatID
	kind      Kind
	title     Title
	members   []UserID // отсортированы, без повторов — для бинарного поиска
	lastSeq   int64
	version   int64 // растёт при каждом изменении — для оптимистической блокировки
	createdAt time.Time
}

// #endregion chat

func NewDirectChat(id ChatID, a, b UserID, createdAt time.Time) (*Chat, error) {
	if a == b {
		return nil, invalid("member_ids", "в личном чате два разных участника")
	}
	return &Chat{id: id, kind: KindDirect, members: normalizeMembers([]UserID{a, b}), createdAt: createdAt.UTC()}, nil
}

// NewGroupChat сам добавляет создателя в участники: группа, в которой нет её создателя,
// бессмысленна, а требовать это от клиента — лишняя ошибка на ровном месте.
func NewGroupChat(id ChatID, title Title, creator UserID, others []UserID, createdAt time.Time) (*Chat, error) {
	members := normalizeMembers(append([]UserID{creator}, others...))
	if len(members) < GroupMinMembers || len(members) > GroupMaxMembers {
		return nil, invalid("member_ids", fmt.Sprintf("в группе от %d до %d участников", GroupMinMembers, GroupMaxMembers))
	}
	return &Chat{id: id, kind: KindGroup, title: title, members: members, createdAt: createdAt.UTC()}, nil
}

func normalizeMembers(ids []UserID) []UserID {
	out := slices.Clone(ids)
	slices.SortFunc(out, compareUserID)
	return slices.Compact(out)
}

func compareUserID(a, b UserID) int { return strings.Compare(a.value, b.value) }

// DirectKeyFor — ключ пары для личного чата, не зависящий от порядка участников.
func DirectKeyFor(a, b UserID) string {
	if compareUserID(a, b) > 0 {
		a, b = b, a
	}
	return a.value + "|" + b.value
}

func (c *Chat) ID() ChatID           { return c.id }
func (c *Chat) Kind() Kind           { return c.kind }
func (c *Chat) Title() Title         { return c.title }
func (c *Chat) LastSeq() int64       { return c.lastSeq }
func (c *Chat) Version() int64       { return c.version }
func (c *Chat) CreatedAt() time.Time { return c.createdAt }

// Members отдаёт копию: срез, отданный наружу как есть, позволил бы менять агрегат в обход его правил.
func (c *Chat) Members() []UserID { return slices.Clone(c.members) }

func (c *Chat) IsMember(u UserID) bool {
	_, ok := slices.BinarySearchFunc(c.members, u, compareUserID)
	return ok
}

// DirectKey — ключ пары для личного чата; для группы ok == false.
func (c *Chat) DirectKey() (key string, ok bool) {
	if c.kind != KindDirect {
		return "", false
	}
	return DirectKeyFor(c.members[0], c.members[1]), true
}

// #region post
// Post — единственный способ добавить сообщение. Номер выдаёт агрегат,
// а не хранилище: правило «seq подряд» — бизнес-правило, ему место в домене.
func (c *Chat) Post(id MessageID, author UserID, text Text, sentAt time.Time) (Message, error) {
	if !c.IsMember(author) {
		return Message{}, ErrNotMember
	}
	c.lastSeq++
	c.version++
	return Message{id: id, chatID: c.id, authorID: author, text: text, seq: c.lastSeq, sentAt: sentAt.UTC()}, nil
}

// #endregion post
