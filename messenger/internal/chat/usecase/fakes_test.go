package usecase_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"messenger/internal/chat/domain"
)

var t0 = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return t0 }

// seqIDs выдаёт предсказуемые UUID: тесты сравнивают id, а не случайные строки.
type seqIDs struct{ n atomic.Int64 }

func (g *seqIDs) NewID() string {
	return fmt.Sprintf("00000000-0000-7000-b000-%012d", g.n.Add(1))
}

// knownUsers — фейк UserChecker: «существуют» только перечисленные пользователи.
type knownUsers struct {
	mu  sync.Mutex
	ids map[domain.UserID]bool
}

func newKnownUsers(ids ...domain.UserID) *knownUsers {
	k := &knownUsers{ids: make(map[domain.UserID]bool)}
	for _, id := range ids {
		k.ids[id] = true
	}
	return k
}

func (k *knownUsers) MissingUsers(_ context.Context, ids []domain.UserID) ([]domain.UserID, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	var missing []domain.UserID
	for _, id := range ids {
		if !k.ids[id] {
			missing = append(missing, id)
		}
	}
	return missing, nil
}

// garbageIDs — сломанный генератор: проверяем, что его сбой не выдаётся за ошибку клиента.
type garbageIDs struct{}

func (garbageIDs) NewID() string { return "не-uuid" }
