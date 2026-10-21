package memory_test

import (
	"testing"

	"messenger/internal/user/adapters/memory"
	"messenger/internal/user/usecase/storetest"
)

func TestStoreContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) storetest.Store {
		t.Parallel() // у каждой проверки своё хранилище — делить нечего
		return memory.NewStore()
	})
}
