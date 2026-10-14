package usecase

import (
	"context"
	"errors"
	"fmt"

	"messenger/internal/auth/domain"
)

type SignUpCommand struct {
	Username    string
	DisplayName string
	Password    string
}

// SignUp — регистрация: профиль в контексте user плюс учётные данные здесь (ADR-0004).
type SignUp struct {
	hasher   PasswordHasher
	profiles ProfileRegistrar
	creds    CredentialsSaver
}

func NewSignUp(hasher PasswordHasher, profiles ProfileRegistrar, creds CredentialsSaver) (*SignUp, error) {
	if hasher == nil || profiles == nil || creds == nil {
		return nil, errors.New("sign up: nil-зависимость")
	}
	return &SignUp{hasher: hasher, profiles: profiles, creds: creds}, nil
}

// #region signup
func (uc *SignUp) Execute(ctx context.Context, cmd SignUpCommand) (Profile, error) {
	pw, err := domain.NewPassword(cmd.Password)
	if err != nil {
		return Profile{}, err
	}
	// Хешируем до создания профиля: если хешер сломан, узнаем это, ничего не создав.
	hash, err := uc.hasher.Hash(pw)
	if err != nil {
		return Profile{}, fmt.Errorf("sign up: хеширование: %w", err)
	}
	profile, err := uc.profiles.RegisterProfile(ctx, cmd.Username, cmd.DisplayName)
	if err != nil {
		return Profile{}, fmt.Errorf("sign up: %w", err)
	}
	creds, err := domain.NewCredentials(profile.ID, hash)
	if err != nil {
		// Без %w: пустой id от соседнего контекста — сбой сервера, а не ошибка данных клиента.
		return Profile{}, fmt.Errorf("sign up: некорректные учётные данные для профиля %q", profile.ID)
	}
	if err := uc.creds.SaveCredentials(ctx, creds); err != nil {
		// Профиль уже создан, а пароля у него нет — войти он не сможет. Это известная
		// неатомарность двух контекстов (ADR-0004); закроет её транзакция в модуле 2a.
		return Profile{}, fmt.Errorf("sign up: профиль %s создан без учётных данных: %w", profile.ID, err)
	}
	return profile, nil
}

// #endregion signup
