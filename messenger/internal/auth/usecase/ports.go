package usecase

import (
	"context"
	"time"

	"messenger/internal/auth/domain"
)

// #region ports
type PasswordHasher interface {
	Hash(p domain.Password) (domain.PasswordHash, error)
	// Verify сравнивает пароль с хешем. Время сравнения не зависит от того, в каком
	// байте они расходятся. Ошибка — только если хеш испорчен, а не «не совпал».
	Verify(plaintext string, hash domain.PasswordHash) (bool, error)
}

// Profile — профиль пользователя глазами auth: ровно то, что нужно ответу регистрации.
type Profile struct {
	ID          domain.UserID
	Username    string
	DisplayName string
	CreatedAt   time.Time
}

// ProfileRegistrar создаёт профиль в контексте user. Реализация — адаптер userbridge;
// в модуле 6 — вызов сервиса users.
type ProfileRegistrar interface {
	// Ошибки данных — *domain.ValidationError, занятое имя — domain.ErrUsernameTaken.
	RegisterProfile(ctx context.Context, username, displayName string) (Profile, error)
}

type UserLookup interface {
	// UserIDByUsername — domain.ErrNoSuchUser, если имени нет или оно некорректно.
	UserIDByUsername(ctx context.Context, username string) (domain.UserID, error)
}

type CredentialsSaver interface {
	// SaveCredentials — domain.ErrCredentialsExist, если у пользователя они уже есть.
	SaveCredentials(ctx context.Context, c domain.Credentials) error
}

type CredentialsFinder interface {
	// FindCredentials — domain.ErrNoCredentials, если их нет.
	FindCredentials(ctx context.Context, id domain.UserID) (domain.Credentials, error)
}

// Token — выпущенный токен доступа.
type Token struct {
	Value     string
	ExpiresAt time.Time
}

// TokenIssuer выпускает токен доступа. Как он устроен (JWT, подпись, срок) — дело адаптера.
type TokenIssuer interface {
	Issue(userID domain.UserID) (Token, error)
}

// #endregion ports
