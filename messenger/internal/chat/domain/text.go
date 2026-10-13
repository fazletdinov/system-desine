package domain

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	TextMaxLen  = 4096 // как у Telegram: длиннее — уже не сообщение, а документ
	TitleMaxLen = 128
)

// Text — текст сообщения. Длина считается в символах Unicode (рунах), а не в байтах:
// «я» в UTF-8 — два байта, эмодзи — четыре, а пользователь видит один символ.
type Text struct{ value string }

// #region new-text
func NewText(s string) (Text, error) {
	if !utf8.ValidString(s) {
		return Text{}, invalid("text", "некорректный UTF-8")
	}
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	if n == 0 {
		return Text{}, invalid("text", "пустое сообщение")
	}
	if n > TextMaxLen {
		return Text{}, invalid("text", fmt.Sprintf("не длиннее %d символов", TextMaxLen))
	}
	if strings.IndexFunc(s, isForbiddenInText) >= 0 {
		return Text{}, invalid("text", "управляющие символы запрещены")
	}
	return Text{value: s}, nil
}

// #endregion new-text

// isForbiddenInText пропускает переводы строк и табуляцию — без них не написать
// многострочное сообщение, — а остальные управляющие символы запрещает: в чате им
// не место, а NUL вдобавок не примет столбец text в Postgres (модуль 2a).
func isForbiddenInText(r rune) bool {
	return unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t'
}

func (t Text) String() string { return t.value }

// IsZero сообщает, что значение не было создано через NewText (нулевое значение структуры).
func (t Text) IsZero() bool { return t.value == "" }

// Title — название группы.
type Title struct{ value string }

func NewTitle(s string) (Title, error) {
	if !utf8.ValidString(s) {
		return Title{}, invalid("title", "некорректный UTF-8")
	}
	s = strings.TrimSpace(s)
	if n := utf8.RuneCountInString(s); n == 0 || n > TitleMaxLen {
		return Title{}, invalid("title", fmt.Sprintf("от 1 до %d символов", TitleMaxLen))
	}
	if strings.IndexFunc(s, unicode.IsControl) >= 0 {
		return Title{}, invalid("title", "управляющие символы запрещены")
	}
	return Title{value: s}, nil
}

func (t Title) String() string { return t.value }

// IsZero сообщает, что значение не было создано через NewTitle (нулевое значение структуры).
func (t Title) IsZero() bool { return t.value == "" }
