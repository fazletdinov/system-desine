// Package storetest — контрактные тесты хранилища пользователей: одна батарея
// проверок для любой реализации портов UserCreator и UserFinder. Сейчас её проходит
// memory.Store, в модуле 2a — Postgres-адаптер.
package storetest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"messenger/internal/user/domain"
	"messenger/internal/user/usecase"
)

// Store — всё, что use case'ы пользователей ждут от хранилища.
type Store interface {
	usecase.UserCreator
	usecase.UserFinder
}

var t0 = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

// Run прогоняет контракт против хранилища, которое создаёт newStore. Каждая проверка
// получает своё, пустое хранилище: параллельные проверки не делят состояние.
func Run(t *testing.T, newStore func(t *testing.T) Store) {
	t.Helper()
	tests := []struct {
		name string
		fn   func(t *testing.T, s Store)
	}{
		{"создание и поиск", testCreateAndFind},
		{"занятое имя", testUsernameTaken},
		{"отменённый контекст", testCanceledContext},
		{"одновременная регистрация одного имени", testConcurrentSameUsername},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.fn(t, newStore(t))
		})
	}
}

// newUser — пользователь номер n с именем username.
func newUser(t *testing.T, n int, username string) domain.User {
	t.Helper()
	userID, err := domain.ParseUserID(fmt.Sprintf("00000000-0000-7000-8000-%012d", n))
	if err != nil {
		t.Fatal(err)
	}
	name, err := domain.NewUsername(username)
	if err != nil {
		t.Fatal(err)
	}
	display, err := domain.NewDisplayName(fmt.Sprintf("Пользователь %d", n))
	if err != nil {
		t.Fatal(err)
	}
	return domain.NewUser(userID, name, display, t0)
}

// sameUser сравнивает пользователей по полям, а время — через Equal: хранилище вправе
// вернуть тот же момент в другой локации (Postgres отдаёт timestamptz в локальной зоне
// драйвера), и == у time.Time это различие увидел бы.
func sameUser(a, b domain.User) bool {
	return a.ID() == b.ID() && a.Username() == b.Username() &&
		a.DisplayName() == b.DisplayName() && a.CreatedAt().Equal(b.CreatedAt())
}

func testCreateAndFind(t *testing.T, s Store) {
	ctx := context.Background()
	u := newUser(t, 1, "alice")
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	got, err := s.FindUser(ctx, u.ID())
	if err != nil || !sameUser(got, u) {
		t.Fatalf("FindUser: %+v, %v — ждали сохранённого пользователя", got, err)
	}
	if _, err := s.FindUser(ctx, newUser(t, 99, "nobody").ID()); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("неизвестный id: ждали ErrUserNotFound, получили %v", err)
	}
}

func testUsernameTaken(t *testing.T, s Store) {
	ctx := context.Background()
	first := newUser(t, 1, "alice")
	if err := s.CreateUser(ctx, first); err != nil {
		t.Fatal(err)
	}
	second := newUser(t, 2, "alice")
	if err := s.CreateUser(ctx, second); !errors.Is(err, domain.ErrUsernameTaken) {
		t.Fatalf("ждали ErrUsernameTaken, получили %v", err)
	}
	if _, err := s.FindUser(ctx, second.ID()); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("отклонённый пользователь не должен сохраниться: %v", err)
	}
	if got, err := s.FindUser(ctx, first.ID()); err != nil || !sameUser(got, first) {
		t.Fatalf("первый пользователь перезаписан: %+v, %v", got, err)
	}
}

// Отменённый или истёкший контекст останавливает каждый метод, и ни один ничего не пишет.
func testCanceledContext(t *testing.T, s Store) {
	stored := newUser(t, 1, "alice")
	if err := s.CreateUser(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	fresh := newUser(t, 2, "bob")
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelExpired()

	for _, done := range []struct {
		name string
		ctx  context.Context
		want error
	}{
		{"отменённый", canceled, context.Canceled},
		{"истёкший", expired, context.DeadlineExceeded},
	} {
		if err := s.CreateUser(done.ctx, fresh); !errors.Is(err, done.want) {
			t.Errorf("CreateUser, %s: ждали %v, получили %v", done.name, done.want, err)
		}
		if _, err := s.FindUser(done.ctx, stored.ID()); !errors.Is(err, done.want) {
			t.Errorf("FindUser, %s: ждали %v, получили %v", done.name, done.want, err)
		}
	}
	if _, err := s.FindUser(context.Background(), fresh.ID()); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("отменённый CreateUser сохранил пользователя: %v", err)
	}
}

// Окно гонки «проверил имя — записал» длится наносекунды, и один забег его может
// не задеть. Поэтому забегов несколько, каждый — за своё имя.
func testConcurrentSameUsername(t *testing.T, s Store) {
	const (
		rounds  = 100
		writers = 16
	)
	for r := range rounds {
		// Пользователей готовим до горутин: t.Fatal из чужой горутины запрещён.
		users := make([]domain.User, writers)
		for w := range writers {
			users[w] = newUser(t, r*writers+w+1, fmt.Sprintf("name_%02d", r))
		}
		winner := raceForUsername(t, s, users)
		for _, u := range users {
			_, err := s.FindUser(context.Background(), u.ID())
			if u.ID() == winner.ID() && err != nil {
				t.Fatalf("победитель не найден: %v", err)
			}
			if u.ID() != winner.ID() && !errors.Is(err, domain.ErrUserNotFound) {
				t.Fatalf("проигравший %s сохранён: %v", u.ID(), err)
			}
		}
	}
}

// raceForUsername регистрирует всех users одновременно и возвращает единственного,
// кого хранилище приняло.
func raceForUsername(t *testing.T, s Store, users []domain.User) domain.User {
	t.Helper()
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		wins []domain.User
	)
	// Все горутины ждут общего старта: так их вызовы действительно идут одновременно,
	// а не по очереди, пока следующая горутина только создаётся.
	start := make(chan struct{})
	for _, u := range users {
		wg.Go(func() {
			<-start
			err := s.CreateUser(context.Background(), u)
			if err != nil && !errors.Is(err, domain.ErrUsernameTaken) {
				t.Error(err)
			}
			if err == nil {
				mu.Lock()
				wins = append(wins, u)
				mu.Unlock()
			}
		})
	}
	close(start)
	wg.Wait()
	if len(wins) != 1 {
		t.Fatalf("имя %s заняли %d раз, ждали ровно один", users[0].Username(), len(wins))
	}
	return wins[0]
}
