package userbridge_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"messenger/internal/auth/adapters/userbridge"
	authdomain "messenger/internal/auth/domain"
	userdomain "messenger/internal/user/domain"
	userusecase "messenger/internal/user/usecase"
)

const aliceID = "00000000-0000-7000-a000-000000000001"

type fakeRegister struct {
	fn func(context.Context, userusecase.RegisterUserCommand) (userdomain.User, error)
}

func (f fakeRegister) Execute(ctx context.Context, cmd userusecase.RegisterUserCommand) (userdomain.User, error) {
	return f.fn(ctx, cmd)
}

type fakeFind struct {
	fn func(context.Context, userusecase.FindByUsernameQuery) (userdomain.User, error)
}

func (f fakeFind) Execute(ctx context.Context, q userusecase.FindByUsernameQuery) (userdomain.User, error) {
	return f.fn(ctx, q)
}

func alice(t *testing.T) userdomain.User {
	t.Helper()
	id, err := userdomain.ParseUserID(aliceID)
	if err != nil {
		t.Fatal(err)
	}
	name, err := userdomain.NewUsername("alice")
	if err != nil {
		t.Fatal(err)
	}
	display, err := userdomain.NewDisplayName("Алиса")
	if err != nil {
		t.Fatal(err)
	}
	return userdomain.NewUser(id, name, display, time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC))
}

func bridge(t *testing.T, regErr, findErr error) *userbridge.Bridge {
	t.Helper()
	b, err := userbridge.New(
		fakeRegister{fn: func(context.Context, userusecase.RegisterUserCommand) (userdomain.User, error) {
			if regErr != nil {
				return userdomain.User{}, regErr
			}
			return alice(t), nil
		}},
		fakeFind{fn: func(context.Context, userusecase.FindByUsernameQuery) (userdomain.User, error) {
			if findErr != nil {
				return userdomain.User{}, findErr
			}
			return alice(t), nil
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestRegisterProfile(t *testing.T) {
	t.Parallel()
	p, err := bridge(t, nil, nil).RegisterProfile(context.Background(), "alice", "Алиса")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID.String() != aliceID || p.Username != "alice" || p.DisplayName != "Алиса" || !p.CreatedAt.Equal(alice(t).CreatedAt()) {
		t.Fatalf("профиль переведён неверно: %+v", p)
	}
}

func TestRegisterProfileErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		err   error
		check func(t *testing.T, got error)
	}{
		{
			name: "ошибка данных — в модель auth, с тем же полем",
			err:  fmt.Errorf("register user: %w", &userdomain.ValidationError{Field: "username", Reason: "только a–z, 0–9 и _"}),
			check: func(t *testing.T, got error) {
				var ve *authdomain.ValidationError
				if !errors.As(got, &ve) || ve.Field != "username" || ve.Reason != "только a–z, 0–9 и _" {
					t.Fatalf("ждали auth ValidationError по username, получили %v", got)
				}
				var userVE *userdomain.ValidationError
				if errors.As(got, &userVE) {
					t.Fatal("ошибка модели user просочилась в auth")
				}
			},
		},
		{
			name: "имя занято",
			err:  fmt.Errorf("register user: %w", userdomain.ErrUsernameTaken),
			check: func(t *testing.T, got error) {
				if !errors.Is(got, authdomain.ErrUsernameTaken) {
					t.Fatalf("ждали auth ErrUsernameTaken, получили %v", got)
				}
			},
		},
		{
			name: "сбой — обёрнут, причина видна",
			err:  fmt.Errorf("register user: %w", context.Canceled),
			check: func(t *testing.T, got error) {
				if !errors.Is(got, context.Canceled) || errors.Is(got, authdomain.ErrInvalid) {
					t.Fatalf("ждали обёрнутый context.Canceled, получили %v", got)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := bridge(t, tt.err, nil).RegisterProfile(context.Background(), "alice", "Алиса")
			tt.check(t, err)
		})
	}
}

// ValidateProfile — те же правила и то же поле в ошибке, что у RegisterProfile, но без
// создания профиля: регистрация зовёт её до дорогого хеша.
func TestValidateProfile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, username, displayName string
		wantField                   string // "" — имена корректны
	}{
		{name: "корректные имена", username: "alice", displayName: "Алиса"},
		{name: "имя не по правилам", username: "Не Имя!", displayName: "Алиса", wantField: "username"},
		{name: "имя короче 3 символов", username: "al", displayName: "Алиса", wantField: "username"},
		{name: "пустое отображаемое имя", username: "alice", displayName: "  ", wantField: "display_name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b := bridge(t, errors.New("RegisterProfile не должен вызываться"), nil)
			err := b.ValidateProfile(context.Background(), tt.username, tt.displayName)
			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("ждали nil, получили %v", err)
				}
				return
			}
			var ve *authdomain.ValidationError
			if !errors.As(err, &ve) || ve.Field != tt.wantField || ve.Reason == "" {
				t.Fatalf("ждали auth ValidationError по %s, получили %v", tt.wantField, err)
			}
			var userVE *userdomain.ValidationError
			if errors.As(err, &userVE) {
				t.Fatal("ошибка модели user просочилась в auth")
			}
		})
	}
}

func TestUserIDByUsername(t *testing.T) {
	t.Parallel()
	id, err := bridge(t, nil, nil).UserIDByUsername(context.Background(), "alice")
	if err != nil || id.String() != aliceID {
		t.Fatalf("UserIDByUsername = %q, %v", id, err)
	}
	tests := []struct {
		name     string
		err      error
		noSuch   bool
		wantWrap error
	}{
		{name: "не найден", err: fmt.Errorf("find by username: %w", userdomain.ErrUserNotFound), noSuch: true},
		{name: "некорректное имя — тоже «нет такого»", err: &userdomain.ValidationError{Field: "username", Reason: "только a–z, 0–9 и _"}, noSuch: true},
		{name: "сбой", err: fmt.Errorf("find by username: %w", context.DeadlineExceeded), wantWrap: context.DeadlineExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := bridge(t, nil, tt.err).UserIDByUsername(context.Background(), "x")
			if tt.noSuch && !errors.Is(err, authdomain.ErrNoSuchUser) {
				t.Fatalf("ждали ErrNoSuchUser, получили %v", err)
			}
			if tt.wantWrap != nil && (!errors.Is(err, tt.wantWrap) || errors.Is(err, authdomain.ErrNoSuchUser)) {
				t.Fatalf("ждали обёрнутый %v, получили %v", tt.wantWrap, err)
			}
		})
	}
}

func TestNewRejectsNil(t *testing.T) {
	t.Parallel()
	if _, err := userbridge.New(nil, fakeFind{}); err == nil {
		t.Error("New(nil, …): ждали ошибку")
	}
	if _, err := userbridge.New(fakeRegister{}, nil); err == nil {
		t.Error("New(…, nil): ждали ошибку")
	}
}
