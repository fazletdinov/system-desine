package usecase

import (
	"context"
	"errors"
	"fmt"

	"messenger/internal/auth/domain"
)

// dummyPassword — пароль фиктивного хеша. Войти им нельзя: хеш не привязан ни к кому.
const dummyPassword = "фиктивный пароль для выравнивания времени входа"

type LoginCommand struct {
	Username string
	Password string
}

// Login — вход по имени и паролю; результат — токен доступа.
type Login struct {
	users  UserLookup
	creds  CredentialsFinder
	hasher PasswordHasher
	tokens TokenIssuer
	dummy  domain.PasswordHash
}

// NewLogin заранее считает фиктивный хеш тем же хешером: его параметры совпадают
// с настоящими, значит, и время проверки тоже.
func NewLogin(users UserLookup, creds CredentialsFinder, hasher PasswordHasher, tokens TokenIssuer) (*Login, error) {
	if users == nil || creds == nil || hasher == nil || tokens == nil {
		return nil, errors.New("login: nil-зависимость")
	}
	pw, err := domain.NewPassword(dummyPassword)
	if err != nil {
		return nil, fmt.Errorf("login: фиктивный пароль не проходит политику: %w", err)
	}
	dummy, err := hasher.Hash(pw)
	if err != nil {
		return nil, fmt.Errorf("login: фиктивный хеш: %w", err)
	}
	return &Login{users: users, creds: creds, hasher: hasher, tokens: tokens, dummy: dummy}, nil
}

// #region login
func (uc *Login) Execute(ctx context.Context, cmd LoginCommand) (Token, error) {
	if len(cmd.Password) > domain.PasswordMaxBytes {
		// Ничего не выдаёт: длину пароля атакующий знает и так.
		return Token{}, rejected(domain.ErrPasswordTooLong)
	}
	id, err := uc.users.UserIDByUsername(ctx, cmd.Username)
	if errors.Is(err, domain.ErrNoSuchUser) {
		return Token{}, uc.reject(cmd.Password, err)
	}
	if err != nil {
		return Token{}, fmt.Errorf("login: %w", err)
	}
	creds, err := uc.creds.FindCredentials(ctx, id)
	if errors.Is(err, domain.ErrNoCredentials) {
		// Профиль без пароля — след неудачной регистрации (ADR-0004). Для клиента — то же «неверно».
		return Token{}, uc.reject(cmd.Password, err)
	}
	if err != nil {
		return Token{}, fmt.Errorf("login: %w", err)
	}
	ok, err := uc.hasher.Verify(cmd.Password, creds.PasswordHash())
	if err != nil {
		return Token{}, fmt.Errorf("login: %w", err)
	}
	if !ok {
		return Token{}, rejected(domain.ErrWrongPassword)
	}
	tok, err := uc.tokens.Issue(id)
	if err != nil {
		return Token{}, fmt.Errorf("login: выпуск токена: %w", err)
	}
	return tok, nil
}

// reject тратит на отказ столько же, сколько настоящая проверка: иначе по времени
// ответа видно, есть ли пользователь (быстро — нет, медленно — есть, но пароль не тот).
func (uc *Login) reject(password string, reason error) error {
	if _, err := uc.hasher.Verify(password, uc.dummy); err != nil {
		return fmt.Errorf("login: фиктивная проверка: %w", err)
	}
	return rejected(reason)
}

// rejected — отказ во входе: снаружи всегда ErrInvalidCredentials, а настоящая причина
// остаётся внутри ошибки (errors.Is видит обе). Обработчик входа пишет её в журнал,
// клиенту отдаёт только invalid_credentials.
func rejected(reason error) error {
	return fmt.Errorf("%w: %w", domain.ErrInvalidCredentials, reason)
}

// #endregion login
