package memory_test

import (
	"testing"

	"messenger/internal/user/adapters/memory"
	"messenger/internal/user/usecase/storetest"
)

func TestStoreContract(t *testing.T) {
	storetest.Run(t, func(*testing.T) storetest.Store { return memory.NewStore() })
}
