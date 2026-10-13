package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	UsernameMinLen    = 3
	UsernameMaxLen    = 32
	DisplayNameMaxLen = 64
)

// Username — уникальное имя для поиска и упоминаний: латиница в нижнем регистре, цифры, «_».
type Username struct{ value string }

// #region new-username
func NewUsername(s string) (Username, error) {
	if len(s) < UsernameMinLen || len(s) > UsernameMaxLen {
		return Username{}, invalid("username", fmt.Sprintf("длина от %d до %d символов", UsernameMinLen, UsernameMaxLen))
	}
	for i := range len(s) {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
			return Username{}, invalid("username", "только a–z, 0–9 и _")
		}
	}
	return Username{value: s}, nil
}

// #endregion new-username

func (u Username) String() string { return u.value }

// DisplayName — имя, которое видят собеседники. Любой язык, но без управляющих символов.
type DisplayName struct{ value string }

func NewDisplayName(s string) (DisplayName, error) {
	if !utf8.ValidString(s) {
		return DisplayName{}, invalid("display_name", "некорректный UTF-8")
	}
	s = strings.TrimSpace(s)
	if n := utf8.RuneCountInString(s); n == 0 || n > DisplayNameMaxLen {
		return DisplayName{}, invalid("display_name", fmt.Sprintf("от 1 до %d символов", DisplayNameMaxLen))
	}
	if strings.IndexFunc(s, unicode.IsControl) >= 0 {
		return DisplayName{}, invalid("display_name", "управляющие символы запрещены")
	}
	return DisplayName{value: s}, nil
}

func (d DisplayName) String() string { return d.value }

// User — сущность: её отличает id, а не набор полей.
type User struct {
	id          UserID
	username    Username
	displayName DisplayName
	createdAt   time.Time
}

// NewUser не возвращает ошибку: все аргументы уже прошли свои конструкторы.
func NewUser(id UserID, username Username, displayName DisplayName, createdAt time.Time) User {
	return User{id: id, username: username, displayName: displayName, createdAt: createdAt.UTC()}
}

func (u User) ID() UserID               { return u.id }
func (u User) Username() Username       { return u.username }
func (u User) DisplayName() DisplayName { return u.displayName }
func (u User) CreatedAt() time.Time     { return u.createdAt }
