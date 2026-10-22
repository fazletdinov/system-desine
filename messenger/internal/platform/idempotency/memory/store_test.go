package memory_test

import (
	"testing"
	"time"

	"messenger/internal/platform/idempotency"
	"messenger/internal/platform/idempotency/memory"
	"messenger/internal/platform/idempotency/storetest"
)

func newStore(t *testing.T) *memory.Store {
	t.Helper()
	t.Parallel() // у каждой проверки своё хранилище — делить нечего
	s, err := memory.NewStore(storetest.ReservationTTL)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestStoreContract(t *testing.T) {
	t.Parallel()
	storetest.Run(t, func(t *testing.T) idempotency.Store { return newStore(t) })
}

func TestStoreSweeper(t *testing.T) {
	t.Parallel()
	storetest.RunSweeper(t, func(t *testing.T) storetest.SweepStore { return newStore(t) })
}

func TestNewStoreRejectsNonPositiveTTL(t *testing.T) {
	t.Parallel()
	for _, ttl := range []time.Duration{0, -time.Second} {
		if _, err := memory.NewStore(ttl); err == nil {
			t.Fatalf("срок %s: ждали ошибку", ttl)
		}
	}
}
