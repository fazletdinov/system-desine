package domain

import "time"

// Message — неизменяемое значение: после Post его поля не меняются.
type Message struct {
	id       MessageID
	clientID ClientMessageID
	chatID   ChatID
	authorID UserID
	text     Text
	seq      int64
	sentAt   time.Time
}

func (m Message) ID() MessageID  { return m.id }
func (m Message) ChatID() ChatID { return m.chatID }

// ClientID — естественный ключ сообщения: id, присвоенный клиентом до первой попытки.
func (m Message) ClientID() ClientMessageID { return m.clientID }
func (m Message) AuthorID() UserID          { return m.authorID }
func (m Message) Text() Text                { return m.text }
func (m Message) Seq() int64                { return m.seq }
func (m Message) SentAt() time.Time         { return m.sentAt }

// MessageSnapshot — плоская копия сообщения для хранилищ, как ChatSnapshot для чата.
// Без тегов json/db: как раскладывать поля по колонкам, решает адаптер.
type MessageSnapshot struct {
	ID       MessageID
	ClientID ClientMessageID
	ChatID   ChatID
	AuthorID UserID
	Text     Text
	Seq      int64
	SentAt   time.Time
}

// RestoreMessage — сообщение, прочитанное из хранилища. Как и RestoreChat, доверяет
// снимку: сообщение создал Post и сохранило наше хранилище, а поля по отдельности
// адаптер уже разобрал конструкторами Parse…/New….
func RestoreMessage(s MessageSnapshot) Message {
	return Message{id: s.ID, clientID: s.ClientID, chatID: s.ChatID, authorID: s.AuthorID, text: s.Text, seq: s.Seq, sentAt: s.SentAt.UTC()}
}
