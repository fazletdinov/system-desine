package userchecker_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"messenger/internal/chat/adapters/userchecker"
	chatdomain "messenger/internal/chat/domain"
	userdomain "messenger/internal/user/domain"
	userusecase "messenger/internal/user/usecase"
)

const (
	aliceID = "00000000-0000-7000-b000-000000000001"
	bobID   = "00000000-0000-7000-b000-000000000002"
	eveID   = "00000000-0000-7000-b000-000000000003"
)

var errStorage = errors.New("хранилище недоступно")

// fakeGetter — фейк use case GetUser: известные id — есть, failOn — сбой инфраструктуры.
type fakeGetter struct {
	known  map[string]bool
	failOn string
}

func (f fakeGetter) Execute(_ context.Context, q userusecase.GetUserQuery) (userdomain.User, error) {
	switch {
	case q.ID == f.failOn:
		return userdomain.User{}, errStorage
	case f.known[q.ID]:
		return userdomain.NewUser(mustUserID(q.ID), userdomain.Username{}, userdomain.DisplayName{}, time.Time{}), nil
	default:
		return userdomain.User{}, userdomain.ErrUserNotFound
	}
}

func mustUserID(s string) userdomain.UserID {
	id, err := userdomain.ParseUserID(s)
	if err != nil {
		panic(err)
	}
	return id
}

func chatIDs(t *testing.T, ss ...string) []chatdomain.UserID {
	t.Helper()
	out := make([]chatdomain.UserID, len(ss))
	for i, s := range ss {
		id, err := chatdomain.ParseUserID(s)
		if err != nil {
			t.Fatal(err)
		}
		out[i] = id
	}
	return out
}

func TestMissingUsers(t *testing.T) {
	tests := []struct {
		name string
		ids  []string
		want []string
	}{
		{name: "все существуют", ids: []string{aliceID, bobID}, want: nil},
		{name: "одного нет", ids: []string{aliceID, eveID}, want: []string{eveID}},
		{name: "пустой список", ids: nil, want: nil},
	}
	getter := fakeGetter{known: map[string]bool{aliceID: true, bobID: true}}
	c, err := userchecker.New(getter)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.MissingUsers(context.Background(), chatIDs(t, tt.ids...))
			if err != nil {
				t.Fatalf("MissingUsers: %v", err)
			}
			if want := chatIDs(t, tt.want...); !slices.Equal(got, want) {
				t.Errorf("MissingUsers = %v, want %v", got, want)
			}
		})
	}
}

func TestMissingUsersInfrastructureError(t *testing.T) {
	c, err := userchecker.New(fakeGetter{known: map[string]bool{aliceID: true}, failOn: bobID})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.MissingUsers(context.Background(), chatIDs(t, aliceID, bobID))
	if !errors.Is(err, errStorage) {
		t.Fatalf("MissingUsers: %v, want wrapped errStorage", err)
	}
}

func TestNewNilDependency(t *testing.T) {
	if _, err := userchecker.New(nil); err == nil {
		t.Fatal("New(nil): want error")
	}
}
