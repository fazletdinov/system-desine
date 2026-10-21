package memory_test

import (
	"testing"

	"messenger/internal/chat/adapters/memory"
	"messenger/internal/chat/usecase/storetest"
)

// #region contract
func TestStoreContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) storetest.Store {
		t.Parallel() // у каждой проверки своё хранилище — делить нечего
		return memory.NewStore()
	})
}

// #endregion contract
