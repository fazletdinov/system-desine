package memory_test

import (
	"testing"

	"messenger/internal/platform/idempotency"
	"messenger/internal/platform/idempotency/memory"
	"messenger/internal/platform/idempotency/storetest"
)

func TestStoreContract(t *testing.T) {
	t.Parallel()
	storetest.Run(t, func(*testing.T) idempotency.Store { return memory.NewStore() })
}
