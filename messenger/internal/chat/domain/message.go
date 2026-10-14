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
