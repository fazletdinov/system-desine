package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"messenger/internal/user/adapters/memory"
	"messenger/internal/user/domain"
	"messenger/internal/user/usecase"
)

var t0 = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return t0 }

// seqIDs выдаёт предсказуемые UUID. Тесты пакета не параллельны внутри одного генератора,
// поэтому простого счётчика хватает.
type seqIDs struct{ n int }

func (g *seqIDs) NewID() string {
	g.n++
	return fmt.Sprintf("00000000-0000-7000-8000-%012d", g.n)
}

type garbageIDs struct{}

func (garbageIDs) NewID() string { return "не-uuid" }

// countingCreator считает обращения к хранилищу: невалидный ввод не должен до него дойти.
type countingCreator struct {
	usecase.UserCreator
	calls int
}

func (c *countingCreator) CreateUser(ctx context.Context, u domain.User) error {
	c.calls++
	return c.UserCreator.CreateUser(ctx, u)
}

func newRegister(t *testing.T, users usecase.UserCreator, ids usecase.IDGenerator) *usecase.RegisterUser {
	t.Helper()
	uc, err := usecase.NewRegisterUser(users, fixedClock{}, ids)
	if err != nil {
		t.Fatal(err)
	}
	return uc
}

func TestRegisterUser(t *testing.T) {
	t.Parallel()
	store := memory.NewStore()
	register := newRegister(t, store, &seqIDs{})
	u, err := register.Execute(context.Background(), usecase.RegisterUserCommand{Username: "alice", DisplayName: " Алиса "})
	if err != nil {
		t.Fatal(err)
	}
	if u.ID().String() != "00000000-0000-7000-8000-000000000001" || !u.CreatedAt().Equal(t0) {
		t.Fatalf("id %s и время %v должны прийти от генератора и часов", u.ID(), u.CreatedAt())
	}
	if u.Username().String() != "alice" || u.DisplayName().String() != "Алиса" {
		t.Fatalf("имя %q, отображаемое имя %q", u.Username(), u.DisplayName())
	}
	get, err := usecase.NewGetUser(store)
	if err != nil {
		t.Fatal(err)
	}
	got, err := get.Execute(context.Background(), usecase.GetUserQuery{ID: u.ID().String()})
	if err != nil || got != u {
		t.Fatalf("GetUser вернул %+v, %v; ждали только что созданного", got, err)
	}
}

func TestRegisterUserTakenUsername(t *testing.T) {
	t.Parallel()
	register := newRegister(t, memory.NewStore(), &seqIDs{})
	ctx := context.Background()
	if _, err := register.Execute(ctx, usecase.RegisterUserCommand{Username: "alice", DisplayName: "Алиса"}); err != nil {
		t.Fatal(err)
	}
	_, err := register.Execute(ctx, usecase.RegisterUserCommand{Username: "alice", DisplayName: "Другая Алиса"})
	if !errors.Is(err, domain.ErrUsernameTaken) {
		t.Fatalf("ждали ErrUsernameTaken, получили %v", err)
	}
}

func TestRegisterUserInvalidInputSkipsStore(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		cmd   usecase.RegisterUserCommand
		field string
	}{
		{name: "заглавная буква в имени", cmd: usecase.RegisterUserCommand{Username: "Alice", DisplayName: "Алиса"}, field: "username"},
		{name: "пустое отображаемое имя", cmd: usecase.RegisterUserCommand{Username: "alice", DisplayName: "   "}, field: "display_name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := &countingCreator{UserCreator: memory.NewStore()}
			_, err := newRegister(t, store, &seqIDs{}).Execute(context.Background(), tt.cmd)
			var ve *domain.ValidationError
			if !errors.As(err, &ve) || ve.Field != tt.field {
				t.Fatalf("ждали ValidationError по %s, получили %v", tt.field, err)
			}
			if store.calls != 0 {
				t.Fatalf("невалидный ввод дошёл до хранилища: %d вызовов", store.calls)
			}
		})
	}
}

// Сбой генератора — ошибка сервера, а не клиента: ValidationError здесь дала бы 400.
func TestRegisterUserGarbageIDIsNotValidationError(t *testing.T) {
	t.Parallel()
	store := &countingCreator{UserCreator: memory.NewStore()}
	_, err := newRegister(t, store, garbageIDs{}).Execute(context.Background(), usecase.RegisterUserCommand{Username: "alice", DisplayName: "Алиса"})
	if err == nil || errors.Is(err, domain.ErrInvalid) || store.calls != 0 {
		t.Fatalf("ждали ошибку сервера без записи, получили %v (вызовов хранилища %d)", err, store.calls)
	}
}

func TestGetUserErrors(t *testing.T) {
	t.Parallel()
	get, err := usecase.NewGetUser(memory.NewStore())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_, err = get.Execute(ctx, usecase.GetUserQuery{ID: "00000000-0000-7000-8000-000000000099"})
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("неизвестный id: ждали ErrUserNotFound, получили %v", err)
	}
	_, err = get.Execute(ctx, usecase.GetUserQuery{ID: "42"})
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Field != "user_id" {
		t.Fatalf("кривой id: ждали ValidationError по user_id, получили %v", err)
	}
}

func TestUserConstructorsRejectNil(t *testing.T) {
	t.Parallel()
	store := memory.NewStore()
	tests := []struct {
		name string
		new  func() error
	}{
		{name: "register: нет хранилища", new: func() error {
			_, err := usecase.NewRegisterUser(nil, fixedClock{}, &seqIDs{})
			return err
		}},
		{name: "register: нет часов", new: func() error {
			_, err := usecase.NewRegisterUser(store, nil, &seqIDs{})
			return err
		}},
		{name: "register: нет генератора", new: func() error {
			_, err := usecase.NewRegisterUser(store, fixedClock{}, nil)
			return err
		}},
		{name: "get: нет хранилища", new: func() error {
			_, err := usecase.NewGetUser(nil)
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
