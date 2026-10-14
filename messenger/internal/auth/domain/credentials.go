package domain

import "messenger/internal/kernel/ident"

// UserID — пользователь глазами контекста auth. Свой тип, как и в chat: контексты
// не делят доменные типы, только формат id (kernel/ident).
type UserID struct{ value string }

func ParseUserID(s string) (UserID, error) {
	if !ident.IsUUID(s) {
		return UserID{}, invalid("user_id", "должен быть UUID в нижнем регистре")
	}
	return UserID{value: s}, nil
}

func (id UserID) String() string { return id.value }
func (id UserID) IsZero() bool   { return id.value == "" }

// PasswordHash — результат функции хеширования. Домен не знает алгоритма: строка
// непрозрачна, её понимает только PasswordHasher.
type PasswordHash struct{ value string }

func NewPasswordHash(s string) (PasswordHash, error) {
	if s == "" {
		return PasswordHash{}, invalid("password_hash", "не задано")
	}
	return PasswordHash{value: s}, nil
}

func (h PasswordHash) String() string { return h.value }
func (h PasswordHash) IsZero() bool   { return h.value == "" }

// Credentials — учётные данные: чей пароль и его хеш. Самого пароля здесь нет и не будет.
type Credentials struct {
	userID UserID
	hash   PasswordHash
}

func NewCredentials(userID UserID, hash PasswordHash) (Credentials, error) {
	if userID.IsZero() {
		return Credentials{}, invalid("user_id", "не задано")
	}
	if hash.IsZero() {
		return Credentials{}, invalid("password_hash", "не задано")
	}
	return Credentials{userID: userID, hash: hash}, nil
}

func (c Credentials) UserID() UserID             { return c.userID }
func (c Credentials) PasswordHash() PasswordHash { return c.hash }
