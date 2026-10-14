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
	// lastActivityAt — время последнего сообщения, до первого — время создания;
	// ключ сортировки списка чатов.
	lastActivityAt time.Time
}

// #endregion chat

// errNotSet — общий ответ на нулевое значение аргумента: конструкторы и методы агрегата
// не доверяют тому, что вызывающий не забыл создать значение через Parse…/New….
const errNotSet = "не задано"

func NewDirectChat(id ChatID, a, b UserID, createdAt time.Time) (*Chat, error) {
	if id.IsZero() {
		return nil, invalid("chat_id", errNotSet)
	}
	if a.IsZero() || b.IsZero() {
		return nil, invalid("member_ids", errNotSet)
	}
	if a == b {
		return nil, invalid("member_ids", "в личном чате два разных участника")
	}
	return &Chat{id: id, kind: KindDirect, members: normalizeMembers([]UserID{a, b}), createdAt: createdAt.UTC(), lastActivityAt: createdAt.UTC()}, nil
}

// NewGroupChat сам добавляет создателя в участники: группа, в которой нет её создателя,
// бессмысленна, а требовать это от клиента — лишняя ошибка на ровном месте.
func NewGroupChat(id ChatID, title Title, creator UserID, others []UserID, createdAt time.Time) (*Chat, error) {
	if id.IsZero() {
		return nil, invalid("chat_id", errNotSet)
	}
	if title.IsZero() {
		return nil, invalid("title", errNotSet)
	}
	if creator.IsZero() || slices.ContainsFunc(others, UserID.IsZero) {
		return nil, invalid("member_ids", errNotSet)
	}
	members := normalizeMembers(append([]UserID{creator}, others...))
	if len(members) < GroupMinMembers || len(members) > GroupMaxMembers {
		return nil, invalid("member_ids", fmt.Sprintf("в группе от %d до %d участников", GroupMinMembers, GroupMaxMembers))
	}
	return &Chat{
		id: id, kind: KindGroup, title: title, members: members,
		createdAt: createdAt.UTC(), lastActivityAt: createdAt.UTC(),
	}, nil
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

// LastActivityAt — время последнего сообщения, а у чата без сообщений — время создания.
func (c *Chat) LastActivityAt() time.Time { return c.lastActivityAt }

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
	switch {
	case id.IsZero():
		return Message{}, invalid("message_id", errNotSet)
	case author.IsZero():
		return Message{}, invalid("author_id", errNotSet)
	case text.IsZero():
		return Message{}, invalid("text", errNotSet)
	}
	if !c.IsMember(author) {
		return Message{}, ErrNotMember
	}
	sentAt = sentAt.UTC()
	c.lastSeq++
	c.version++
	// Активность не идёт назад: часы разных экземпляров сервиса расходятся, и сообщение
	// с чуть меньшим временем не должно опускать чат в списке.
	if sentAt.After(c.lastActivityAt) {
		c.lastActivityAt = sentAt
	}
	return Message{id: id, chatID: c.id, authorID: author, text: text, seq: c.lastSeq, sentAt: sentAt}, nil
}

// #endregion post

// #region snapshot
// ChatSnapshot — плоская копия состояния агрегата для хранилищ.
// Без тегов json/db: как раскладывать поля по колонкам, решает адаптер.
type ChatSnapshot struct {
	ID        ChatID
	Kind      Kind
	Title     Title
	Members   []UserID
	LastSeq   int64
	Version   int64
	CreatedAt time.Time
	// LastActivityAt хранится, а не вычисляется по сообщениям: по нему сортируется
	// список чатов, и хранилищу нужен готовый ключ для индекса.
	LastActivityAt time.Time
}

// Snapshot копирует и срез участников: снимок, который хранилище держит у себя,
// не должен меняться вместе с агрегатом, из которого он сделан.
func (c *Chat) Snapshot() ChatSnapshot {
	return ChatSnapshot{
		ID: c.id, Kind: c.kind, Title: c.title, Members: slices.Clone(c.members),
		LastSeq: c.lastSeq, Version: c.version, CreatedAt: c.createdAt, LastActivityAt: c.lastActivityAt,
	}
}

// RestoreChat доверяет снимку: он получен из Snapshot и сохранён нашим же хранилищем,
// поэтому инварианты повторно не проверяются.
func RestoreChat(s ChatSnapshot) *Chat {
	return &Chat{
		id: s.ID, kind: s.Kind, title: s.Title, members: slices.Clone(s.Members),
		lastSeq: s.LastSeq, version: s.Version, createdAt: s.CreatedAt, lastActivityAt: s.LastActivityAt,
	}
}

// #endregion snapshot
