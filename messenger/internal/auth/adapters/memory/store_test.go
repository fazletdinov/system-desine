package memory_test

import (
	"testing"

	"messenger/internal/auth/adapters/memory"
	"messenger/internal/auth/usecase/storetest"
)

func TestStoreContract(t *testing.T) {
	storetest.Run(t, func(*testing.T) storetest.Store { return memory.NewStore() })
}
