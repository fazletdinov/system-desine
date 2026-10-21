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
	hasher    PasswordHasher
	profiles  ProfileRegistrar
	validator ProfileValidator
	creds     CredentialsSaver
	tx        Transactor
}

func NewSignUp(hasher PasswordHasher, profiles ProfileRegistrar, validator ProfileValidator, creds CredentialsSaver, tx Transactor) (*SignUp, error) {
	if hasher == nil || profiles == nil || validator == nil || creds == nil || tx == nil {
		return nil, errors.New("sign up: nil-зависимость")
	}
	return &SignUp{hasher: hasher, profiles: profiles, validator: validator, creds: creds, tx: tx}, nil
}

// #region signup
func (uc *SignUp) Execute(ctx context.Context, cmd SignUpCommand) (Profile, error) {
	// #region signup-order
	// Порядок — от дешёвого к дорогому: политика пароля и правила имён стоят микросекунды,
	// а хеш пароля намеренно медленный и жадный до памяти. Дорогой шаг — после всех
	// дешёвых проверок, иначе опечатка в имени тратит полный хеш и место в очереди хешера.
	pw, err := domain.NewPassword(cmd.Password)
	if err != nil {
		return Profile{}, err
	}
	if err := uc.validator.ValidateProfile(ctx, cmd.Username, cmd.DisplayName); err != nil {
		return Profile{}, fmt.Errorf("sign up: %w", err)
	}
	// Хешируем до создания профиля: если хешер сломан или перегружен, узнаем это,
	// ничего не создав.
	hash, err := uc.hasher.Hash(ctx, pw)
	if err != nil {
		return Profile{}, fmt.Errorf("sign up: хеширование: %w", err)
	}
	// #endregion signup-order
	// Профиль и учётные данные — в одной транзакции: упадёт второй шаг — откатится и
	// первый, и имя не останется занятым профилем без пароля (бывшее окно ADR-0004).
	// Хеш посчитан выше, вне транзакции: она держит соединение с базой, и держать его
	// сотни миллисекунд Argon2id — значит отнимать его у других запросов.
	var profile Profile
	err = uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		// Имя проверено, но занятость проверяет только сам RegisterProfile: между проверкой
		// и созданием его мог занять другой запрос.
		p, err := uc.profiles.RegisterProfile(ctx, cmd.Username, cmd.DisplayName)
		if err != nil {
			return err
		}
		creds, err := domain.NewCredentials(p.ID, hash)
		if err != nil {
			// Без %w: пустой id от соседнего контекста — сбой сервера, а не ошибка данных клиента.
			return fmt.Errorf("некорректные учётные данные для профиля %q", p.ID)
		}
		if err := uc.creds.SaveCredentials(ctx, creds); err != nil {
			return fmt.Errorf("учётные данные профиля %s: %w", p.ID, err)
		}
		profile = p
		return nil
	})
	if err != nil {
		return Profile{}, fmt.Errorf("sign up: %w", err)
	}
	return profile, nil
}

// #endregion signup
