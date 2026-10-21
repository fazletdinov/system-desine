package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"messenger/internal/auth/adapters/memory"
	"messenger/internal/auth/domain"
	"messenger/internal/auth/usecase"
)

const (
	alicePassword = "правильный-пароль-длинный"
	wrongPassword = "неправильный-пароль-длинный"
)

type env struct {
	hasher   *countingHasher
	profiles *profiles
	signUp   *usecase.SignUp
	login    *usecase.Login
	alice    usecase.Profile
}

// newEnv регистрирует alice с паролем alicePassword и orphan — профиль без учётных
// данных, след неудачной регистрации (ADR-0004).
func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{hasher: &countingHasher{}, profiles: newProfiles()}
	store := memory.NewStore()
	var err error
	if e.signUp, err = usecase.NewSignUp(e.hasher, e.profiles, stubValidator{}, store, memory.Tx{}); err != nil {
		t.Fatal(err)
	}
	if e.login, err = usecase.NewLogin(e.profiles, store, e.hasher, fixedIssuer{}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if e.alice, err = e.signUp.Execute(ctx, usecase.SignUpCommand{Username: "alice", DisplayName: "Алиса", Password: alicePassword}); err != nil {
		t.Fatal(err)
	}
	if _, err = e.profiles.RegisterProfile(ctx, "orphan", "Сирота"); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestLoginSuccess(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	tok, err := e.login.Execute(context.Background(), usecase.LoginCommand{Username: "alice", Password: alicePassword})
	if err != nil {
		t.Fatal(err)
	}
	if tok.Value != "tok:"+e.alice.ID.String() {
		t.Fatalf("токен %q выпущен не alice (%s)", tok.Value, e.alice.ID)
	}
}

// #region enumeration-test
func TestLoginDoesNotRevealUsers(t *testing.T) {
	t.Parallel()
	e := newEnv(t) // регистрирует alice с паролем "правильный-пароль-длинный"
	tests := []struct {
		name, username, password string
		reason                   error // внутренняя причина: для журнала, не для клиента
	}{
		{"нет такого пользователя", "nobody", "правильный-пароль-длинный", domain.ErrNoSuchUser},
		{"неверный пароль", "alice", "неправильный-пароль-длинный", domain.ErrWrongPassword},
		{"профиль без пароля (ADR-0004)", "orphan", "правильный-пароль-длинный", domain.ErrNoCredentials},
		{"некорректное имя", "Не Имя!", "правильный-пароль-длинный", domain.ErrNoSuchUser},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := e.hasher.verifies.Load()
			_, err := e.login.Execute(context.Background(), usecase.LoginCommand{Username: tt.username, Password: tt.password})
			if !errors.Is(err, domain.ErrInvalidCredentials) {
				t.Fatalf("ждали ErrInvalidCredentials, получили %v", err)
			}
			// Клиент увидит только ErrInvalidCredentials, а причина остаётся в ошибке —
			// обработчик входа запишет её в журнал.
			if !errors.Is(err, tt.reason) {
				t.Fatalf("в ошибке нет причины %v: %v", tt.reason, err)
			}
			// Каждый отказ — ровно одна проверка хеша: настоящего или фиктивного.
			if got := e.hasher.verifies.Load() - before; got != 1 {
				t.Fatalf("проверок хеша %d, ждали 1 — время ответа выдаст, есть ли пользователь", got)
			}
		})
	}
}

// #endregion enumeration-test

func TestLoginHugePassword(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	before := e.hasher.verifies.Load()
	huge := strings.Repeat("a", domain.PasswordMaxBytes+1)
	_, err := e.login.Execute(context.Background(), usecase.LoginCommand{Username: "alice", Password: huge})
	if !errors.Is(err, domain.ErrInvalidCredentials) || !errors.Is(err, domain.ErrPasswordTooLong) {
		t.Fatalf("ждали ErrInvalidCredentials с причиной ErrPasswordTooLong, получили %v", err)
	}
	if got := e.hasher.verifies.Load() - before; got != 0 {
		t.Fatalf("огромный пароль дошёл до хешера: %d проверок", got)
	}
}

// Сломанный хеш — сбой сервера, а не «неверный пароль»: иначе порча базы выглядела бы
// как волна неудачных входов.
func TestLoginVerifyErrorIsNotInvalidCredentials(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.hasher.verifyErr = errors.New("испорченный хеш")
	_, err := e.login.Execute(context.Background(), usecase.LoginCommand{Username: "alice", Password: alicePassword})
	if err == nil || errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("ждали ошибку сервера, получили %v", err)
	}
}

// Перегрузка хешера — не «неверный пароль»: клиент должен получить 503 и повторить,
// а не 401 и решить, что пароль не тот. Касается и настоящей проверки, и фиктивной
// (отказ по несуществующему имени тоже зовёт Verify).
func TestOverloadedIsNotInvalidCredentials(t *testing.T) {
	t.Parallel()
	for _, username := range []string{"alice", "nobody"} {
		t.Run("login "+username, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			e.hasher.verifyErr = fmt.Errorf("hashlimit: %w", usecase.ErrOverloaded)
			_, err := e.login.Execute(context.Background(), usecase.LoginCommand{Username: username, Password: alicePassword})
			if !errors.Is(err, usecase.ErrOverloaded) || errors.Is(err, domain.ErrInvalidCredentials) {
				t.Fatalf("ждали ErrOverloaded без ErrInvalidCredentials, получили %v", err)
			}
		})
	}
	t.Run("sign up", func(t *testing.T) {
		t.Parallel()
		hasher, prof := &countingHasher{hashErr: usecase.ErrOverloaded}, newProfiles()
		uc, err := usecase.NewSignUp(hasher, prof, stubValidator{}, memory.NewStore(), memory.Tx{})
		if err != nil {
			t.Fatal(err)
		}
		_, err = uc.Execute(context.Background(), usecase.SignUpCommand{Username: "bob", DisplayName: "Боб", Password: alicePassword})
		if !errors.Is(err, usecase.ErrOverloaded) || errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("ждали ErrOverloaded, получили %v", err)
		}
		if prof.calls != 0 {
			t.Fatalf("профиль создан %d раз, хотя хешер перегружен", prof.calls)
		}
	})
}

func TestSignUpOrder(t *testing.T) {
	t.Parallel()
	cmd := usecase.SignUpCommand{Username: "bob", DisplayName: "Боб", Password: alicePassword}
	errBoom := errors.New("бум")

	t.Run("хешер сломан — профиль не создан", func(t *testing.T) {
		t.Parallel()
		hasher, prof := &countingHasher{hashErr: errBoom}, newProfiles()
		uc, err := usecase.NewSignUp(hasher, prof, stubValidator{}, memory.NewStore(), memory.Tx{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := uc.Execute(context.Background(), cmd); !errors.Is(err, errBoom) {
			t.Fatalf("ждали ошибку хешера, получили %v", err)
		}
		if prof.calls != 0 {
			t.Fatalf("профиль создан %d раз, хотя хешер сломан", prof.calls)
		}
	})

	t.Run("регистратор упал — учётные данные не сохранены", func(t *testing.T) {
		t.Parallel()
		prof := newProfiles()
		prof.failErr = errBoom
		saver := &countingSaver{CredentialsSaver: memory.NewStore()}
		uc, err := usecase.NewSignUp(&countingHasher{}, prof, stubValidator{}, saver, memory.Tx{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := uc.Execute(context.Background(), cmd); !errors.Is(err, errBoom) {
			t.Fatalf("ждали ошибку регистратора, получили %v", err)
		}
		if n := saver.calls.Load(); n != 0 {
			t.Fatalf("учётные данные сохранялись %d раз без профиля", n)
		}
	})

	t.Run("сохранение упало — ошибка называет профиль, чья регистрация откатывается", func(t *testing.T) {
		t.Parallel()
		prof := newProfiles()
		saver := &countingSaver{CredentialsSaver: memory.NewStore(), failErr: errBoom}
		uc, err := usecase.NewSignUp(&countingHasher{}, prof, stubValidator{}, saver, memory.Tx{})
		if err != nil {
			t.Fatal(err)
		}
		_, err = uc.Execute(context.Background(), cmd)
		if !errors.Is(err, errBoom) {
			t.Fatalf("errors.Is не видит исходную ошибку: %v", err)
		}
		id, lookupErr := prof.UserIDByUsername(context.Background(), "bob")
		if lookupErr != nil {
			t.Fatal(lookupErr)
		}
		// По журналу оператор видит, какой профиль откатился вместе с регистрацией.
		// memory.Tx не откатывает, поэтому профиль здесь остаётся и его id можно найти.
		if !strings.Contains(err.Error(), id.String()) {
			t.Fatalf("в ошибке нет id профиля %s: %v", id, err)
		}
	})
}

// Оба шага регистрации — профиль и учётные данные — внутри одного WithinTx, а ошибка
// второго выходит из fn наружу: настоящий Transactor по ней откатит и профиль.
// Сам откат проверяет интеграционный тест на Postgres (адаптеры — задача урока 2.9).
func TestSignUpWritesProfileAndCredentialsInOneTx(t *testing.T) {
	t.Parallel()
	cmd := usecase.SignUpCommand{Username: "bob", DisplayName: "Боб", Password: alicePassword}
	errBoom := errors.New("бум")
	tests := []struct {
		name    string
		saveErr error
	}{
		{"успех", nil},
		{"сохранение учётных данных упало", errBoom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			prof, tx := newProfiles(), &recordingTx{}
			saver := &countingSaver{CredentialsSaver: memory.NewStore(), failErr: tt.saveErr}
			uc, err := usecase.NewSignUp(&countingHasher{}, prof, stubValidator{}, saver, tx)
			if err != nil {
				t.Fatal(err)
			}
			_, err = uc.Execute(context.Background(), cmd)
			if !errors.Is(err, tt.saveErr) || (tt.saveErr == nil) != (err == nil) {
				t.Fatalf("ждали %v, получили %v", tt.saveErr, err)
			}
			if n := tx.calls.Load(); n != 1 {
				t.Fatalf("WithinTx вызван %d раз, ждали 1", n)
			}
			if prof.callsOutsideTx != 0 || saver.callsOutsideTx.Load() != 0 {
				t.Fatalf("шаги вне транзакции: профиль %d, учётные данные %d", prof.callsOutsideTx, saver.callsOutsideTx.Load())
			}
			if prof.calls != 1 || saver.calls.Load() != 1 {
				t.Fatalf("шагов в транзакции: профиль %d, учётные данные %d — ждали по одному", prof.calls, saver.calls.Load())
			}
			if fnErr := tx.lastErr(); !errors.Is(fnErr, tt.saveErr) || (tt.saveErr == nil) != (fnErr == nil) {
				t.Fatalf("fn вернул Transactor'у %v, ждали %v — откатывать было бы нечего", fnErr, tt.saveErr)
			}
		})
	}
}

func TestSignUpPasswordPolicy(t *testing.T) {
	t.Parallel()
	hasher, prof := &countingHasher{}, newProfiles()
	uc, err := usecase.NewSignUp(hasher, prof, stubValidator{}, memory.NewStore(), memory.Tx{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = uc.Execute(context.Background(), usecase.SignUpCommand{Username: "bob", DisplayName: "Боб", Password: strings.Repeat("a", 14)})
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Field != "password" {
		t.Fatalf("ждали ValidationError по password, получили %v", err)
	}
	if hasher.hashes.Load() != 0 || prof.calls != 0 {
		t.Fatalf("слабый пароль дошёл дальше политики: хешей %d, профилей %d", hasher.hashes.Load(), prof.calls)
	}
}

// #region invalid-username-test
// Опечатка в имени не должна стоить полного хеша: Argon2id — самый дорогой шаг
// регистрации, и он идёт только после всех дешёвых проверок.
func TestSignUpInvalidUsernameDoesNotHash(t *testing.T) {
	t.Parallel()
	hasher, prof := &countingHasher{}, newProfiles()
	invalid := &domain.ValidationError{Field: "username", Reason: "только a–z, 0–9 и _"}
	uc, err := usecase.NewSignUp(hasher, prof, stubValidator{err: invalid}, memory.NewStore(), memory.Tx{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = uc.Execute(context.Background(), usecase.SignUpCommand{Username: "Не Имя!", DisplayName: "Боб", Password: alicePassword})
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Field != "username" {
		t.Fatalf("ждали ValidationError по username, получили %v", err)
	}
	if n := hasher.hashes.Load(); n != 0 {
		t.Fatalf("хешер вызван %d раз для некорректного имени", n)
	}
	if prof.calls != 0 {
		t.Fatalf("профиль создавался %d раз для некорректного имени", prof.calls)
	}
}

// #endregion invalid-username-test

func TestSignUpUsernameTaken(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	_, err := e.signUp.Execute(context.Background(), usecase.SignUpCommand{Username: "alice", DisplayName: "Другая", Password: wrongPassword})
	if !errors.Is(err, domain.ErrUsernameTaken) {
		t.Fatalf("ждали ErrUsernameTaken, получили %v", err)
	}
}

func TestConstructorsRejectNil(t *testing.T) {
	t.Parallel()
	h, p, s, v, x := &countingHasher{}, newProfiles(), memory.NewStore(), stubValidator{}, memory.Tx{}
	tests := []struct {
		name string
		new  func() error
	}{
		{"sign up: нет хешера", func() error { _, err := usecase.NewSignUp(nil, p, v, s, x); return err }},
		{"sign up: нет профилей", func() error { _, err := usecase.NewSignUp(h, nil, v, s, x); return err }},
		{"sign up: нет проверки имени", func() error { _, err := usecase.NewSignUp(h, p, nil, s, x); return err }},
		{"sign up: нет хранилища", func() error { _, err := usecase.NewSignUp(h, p, v, nil, x); return err }},
		{"sign up: нет транзакций", func() error { _, err := usecase.NewSignUp(h, p, v, s, nil); return err }},
		{"login: нет поиска", func() error { _, err := usecase.NewLogin(nil, s, h, fixedIssuer{}); return err }},
		{"login: нет хранилища", func() error { _, err := usecase.NewLogin(p, nil, h, fixedIssuer{}); return err }},
		{"login: нет хешера", func() error { _, err := usecase.NewLogin(p, s, nil, fixedIssuer{}); return err }},
		{"login: нет выпуска токенов", func() error { _, err := usecase.NewLogin(p, s, h, nil); return err }},
		{"login: хешер не считает фиктивный хеш", func() error {
			_, err := usecase.NewLogin(p, s, &countingHasher{hashErr: errors.New("бум")}, fixedIssuer{})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.new() == nil {
				t.Fatal("ждали ошибку конструктора")
			}
		})
	}
}
