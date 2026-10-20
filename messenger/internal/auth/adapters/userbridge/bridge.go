package userbridge

import (
	"context"
	"errors"
	"fmt"

	authdomain "messenger/internal/auth/domain"
	authusecase "messenger/internal/auth/usecase"
	userdomain "messenger/internal/user/domain"
	userusecase "messenger/internal/user/usecase"
)

// Узкие порты на use case'ы контекста user — объявлены здесь, у потребителя.
type Registrar interface {
	Execute(ctx context.Context, cmd userusecase.RegisterUserCommand) (userdomain.User, error)
}

type Finder interface {
	Execute(ctx context.Context, q userusecase.FindByUsernameQuery) (userdomain.User, error)
}

// Bridge реализует authusecase.ProfileRegistrar, authusecase.ProfileValidator
// и authusecase.UserLookup.
type Bridge struct {
	register Registrar
	find     Finder
}

func New(register Registrar, find Finder) (*Bridge, error) {
	if register == nil || find == nil {
		return nil, errors.New("userbridge: nil-зависимость")
	}
	return &Bridge{register: register, find: find}, nil
}

// #region translate
func (b *Bridge) RegisterProfile(ctx context.Context, username, displayName string) (authusecase.Profile, error) {
	u, err := b.register.Execute(ctx, userusecase.RegisterUserCommand{Username: username, DisplayName: displayName})
	switch {
	case err == nil:
	case errors.Is(err, userdomain.ErrInvalid):
		// Ошибку данных переводим в модель auth с тем же полем: клиенту всё равно,
		// какой контекст проверял имя.
		return authusecase.Profile{}, translateInvalid(err)
	case errors.Is(err, userdomain.ErrUsernameTaken):
		return authusecase.Profile{}, authdomain.ErrUsernameTaken
	default:
		return authusecase.Profile{}, fmt.Errorf("userbridge: %w", err)
	}
	id, err := toAuthID(u.ID())
	if err != nil {
		return authusecase.Profile{}, err
	}
	return authusecase.Profile{ID: id, Username: u.Username().String(), DisplayName: u.DisplayName().String(), CreatedAt: u.CreatedAt()}, nil
}

// #endregion translate

// ValidateProfile проверяет имена конструкторами домена user — теми же, что зовёт
// RegisterUser, поэтому проверка до хеша и при создании профиля не расходятся.
// Это чистые функции без хранилища, так что ctx не нужен: занятость имени здесь
// не проверяется — это делает только RegisterProfile.
func (b *Bridge) ValidateProfile(_ context.Context, username, displayName string) error {
	if _, err := userdomain.NewUsername(username); err != nil {
		return translateInvalid(err)
	}
	if _, err := userdomain.NewDisplayName(displayName); err != nil {
		return translateInvalid(err)
	}
	return nil
}

// translateInvalid переводит ошибку данных user в модель auth с тем же полем и причиной —
// одно правило перевода для RegisterProfile и ValidateProfile. Ошибка модели user
// наружу не выходит: auth не должен уметь её разбирать.
func translateInvalid(err error) error {
	var invalid *userdomain.ValidationError
	if errors.As(err, &invalid) {
		return &authdomain.ValidationError{Field: invalid.Field, Reason: invalid.Reason}
	}
	return fmt.Errorf("userbridge: %w", err)
}

func (b *Bridge) UserIDByUsername(ctx context.Context, username string) (authdomain.UserID, error) {
	u, err := b.find.Execute(ctx, userusecase.FindByUsernameQuery{Username: username})
	var invalid *userdomain.ValidationError
	switch {
	case err == nil:
	case errors.As(err, &invalid), errors.Is(err, userdomain.ErrUserNotFound):
		// Некорректное имя для входа — то же «нет такого»: иначе по ответу видно правила имён.
		return authdomain.UserID{}, authdomain.ErrNoSuchUser
	default:
		return authdomain.UserID{}, fmt.Errorf("userbridge: %w", err)
	}
	return toAuthID(u.ID())
}

func toAuthID(id userdomain.UserID) (authdomain.UserID, error) {
	out, err := authdomain.ParseUserID(id.String())
	if err != nil {
		// Без %w: соседний контекст вернул не UUID — сбой сервера, а не ошибка клиента.
		return authdomain.UserID{}, fmt.Errorf("userbridge: контекст user вернул не UUID: %q", id.String())
	}
	return out, nil
}
