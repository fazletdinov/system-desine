package memory_test

import (
	"testing"

	"messenger/internal/chat/adapters/memory"
	"messenger/internal/chat/usecase/storetest"
)

// #region contract
func TestStoreContract(t *testing.T) {
	storetest.Run(t, func(*testing.T) storetest.Store { return memory.NewStore() })
}

// #endregion contract
