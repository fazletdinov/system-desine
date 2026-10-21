// Package storetest — контрактные тесты хранилища учётных данных: одна батарея проверок
// для любой реализации портов CredentialsSaver и CredentialsFinder. Её проходят
// memory.Store и Postgres-адаптер (auth/adapters/postgres).
package storetest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"messenger/internal/auth/domain"
	"messenger/internal/auth/usecase"
)

// Store — всё, что use case'ы auth ждут от хранилища учётных данных.
type Store interface {
	usecase.CredentialsSaver
	usecase.CredentialsFinder
}

// Run прогоняет контракт против хранилища, которое создаёт newStore. Каждая проверка
// получает своё, пустое хранилище. Идти ли проверкам параллельно, решает newStore
// (t.Parallel): независимые хранилища в памяти — да, а Postgres-адаптеры делят одну
// базу тестового пакета, которую каждая проверка восстанавливает из снимка, — нет.
func Run(t *testing.T, newStore func(t *testing.T) Store) {
	t.Helper()
	tests := []struct {
		name string
		fn   func(t *testing.T, s Store)
	}{
		{"сохранение и поиск", testSaveAndFind},
		{"повторное сохранение", testSaveTwice},
		{"нет учётных данных", testNotFound},
		{"одновременное сохранение для одного пользователя", testConcurrentSave},
		{"отменённый контекст", testCanceledContext},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { tt.fn(t, newStore(t)) })
	}
}

// savedUser — единственный пользователь, для которого батарея сохраняет учётные данные.
const savedUser = 1

// UserIDs — пользователи, для которых батарея сохраняет учётные данные. Хранилище, где
// учётные данные ссылаются на таблицу пользователей (в Postgres — внешний ключ на
// users.users), заводит их в newStore до проверки.
func UserIDs() []string { return []string{userID(savedUser)} }

func userID(n int) string { return fmt.Sprintf("00000000-0000-7000-a000-%012d", n) }

// creds — учётные данные пользователя n с хешем hash.
func creds(t *testing.T, n int, hash string) domain.Credentials {
	t.Helper()
	id, err := domain.ParseUserID(userID(n))
	if err != nil {
		t.Fatal(err)
	}
	h, err := domain.NewPasswordHash(hash)
	if err != nil {
		t.Fatal(err)
	}
	c, err := domain.NewCredentials(id, h)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func testSaveAndFind(t *testing.T, s Store) {
	ctx := context.Background()
	c := creds(t, savedUser, "hash-1")
	if err := s.SaveCredentials(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err := s.FindCredentials(ctx, c.UserID())
	if err != nil || got != c {
		t.Fatalf("FindCredentials: %+v, %v — ждали сохранённые", got, err)
	}
}

func testSaveTwice(t *testing.T, s Store) {
	ctx := context.Background()
	first := creds(t, savedUser, "hash-1")
	if err := s.SaveCredentials(ctx, first); err != nil {
		t.Fatal(err)
	}
	// Второе сохранение не перезаписывает хеш: смена пароля — отдельный сценарий,
	// а не побочный эффект повторной регистрации.
	if err := s.SaveCredentials(ctx, creds(t, savedUser, "hash-2")); !errors.Is(err, domain.ErrCredentialsExist) {
		t.Fatalf("ждали ErrCredentialsExist, получили %v", err)
	}
	if got, err := s.FindCredentials(ctx, first.UserID()); err != nil || got != first {
		t.Fatalf("первые учётные данные перезаписаны: %+v, %v", got, err)
	}
}

func testNotFound(t *testing.T, s Store) {
	if _, err := s.FindCredentials(context.Background(), creds(t, 99, "h").UserID()); !errors.Is(err, domain.ErrNoCredentials) {
		t.Fatalf("ждали ErrNoCredentials, получили %v", err)
	}
}

func testConcurrentSave(t *testing.T, s Store) {
	const writers = 50
	all := make([]domain.Credentials, writers)
	for i := range all {
		all[i] = creds(t, savedUser, fmt.Sprintf("hash-%d", i))
	}
	var (
		wg   sync.WaitGroup
		wins atomic.Int32
	)
	start := make(chan struct{})
	for _, c := range all {
		wg.Go(func() {
			<-start
			err := s.SaveCredentials(context.Background(), c)
			switch {
			case err == nil:
				wins.Add(1)
			case !errors.Is(err, domain.ErrCredentialsExist):
				t.Error(err)
			}
		})
	}
	close(start)
	wg.Wait()
	if n := wins.Load(); n != 1 {
		t.Fatalf("учётные данные одного пользователя сохранены %d раз, ждали ровно один", n)
	}
}

func testCanceledContext(t *testing.T, s Store) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := creds(t, savedUser, "hash-1")
	if err := s.SaveCredentials(ctx, c); !errors.Is(err, context.Canceled) {
		t.Fatalf("SaveCredentials: ждали context.Canceled, получили %v", err)
	}
	if _, err := s.FindCredentials(ctx, c.UserID()); !errors.Is(err, context.Canceled) {
		t.Fatalf("FindCredentials: ждали context.Canceled, получили %v", err)
	}
	if _, err := s.FindCredentials(context.Background(), c.UserID()); !errors.Is(err, domain.ErrNoCredentials) {
		t.Fatalf("отменённый SaveCredentials сохранил данные: %v", err)
	}
}
