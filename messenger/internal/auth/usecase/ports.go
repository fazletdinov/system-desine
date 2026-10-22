package usecase

import (
	"context"
	"errors"
	"time"

	"messenger/internal/auth/domain"
)

// #region ports
// ErrOverloaded — часть контракта PasswordHasher: хешер отказал сразу, потому что
// очередь на вычисление полна. Объявлена здесь, у порта, чтобы адаптер HTTP переводил
// её в 503, не зная, какая реализация ограничивает хеширование.
var ErrOverloaded = errors.New("auth: слишком много одновременных вычислений хеша")

type PasswordHasher interface {
	// Hash и Verify могут вернуть ErrOverloaded (очередь полна) или ошибку ctx (истёк срок ожидания).
	Hash(ctx context.Context, p domain.Password) (domain.PasswordHash, error)
	// Verify сравнивает пароль с хешем. Время сравнения не зависит от того, в каком
	// байте они расходятся. Кроме ошибок выше — ошибка, если хеш испорчен, а не «не совпал».
	Verify(ctx context.Context, plaintext string, hash domain.PasswordHash) (bool, error)
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

// ProfileValidator проверяет имя и отображаемое имя по правилам контекста user, ничего
// не создавая. Регистрация зовёт его до хеширования: отказ из-за опечатки в имени
// не должен стоить полного хеша пароля — намеренно медленного и дорогого по памяти.
type ProfileValidator interface {
	// ValidateProfile — *domain.ValidationError с полем username или display_name, как RegisterProfile.
	ValidateProfile(ctx context.Context, username, displayName string) error
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

// #region transactor
// Transactor задаёт границу транзакции: регистрация пишет профиль (контекст user) и
// учётные данные (auth) — либо оба, либо ничего (ADR-0015). Use case не знает, чем
// транзакция обеспечена: в Postgres — platform/pgtx, в тестах — фейк.
type Transactor interface {
	// WithinTx выполняет fn в одной транзакции: ошибка fn — откат. fn не запускает
	// горутины с переданным ctx: транзакция pgx не для одновременного использования.
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// #endregion transactor
