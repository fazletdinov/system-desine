package idgen_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"messenger/internal/platform/idgen"
)

func TestNewIDIsCanonicalUniqueV7(t *testing.T) {
	t.Parallel()
	const n = 1000
	seen := make(map[string]bool, n)
	for range n {
		id := idgen.UUIDv7{}.NewID()
		parsed, err := uuid.Parse(id)
		if err != nil || id != strings.ToLower(id) || len(id) != 36 {
			t.Fatalf("%q — не канонический UUID в нижнем регистре: %v", id, err)
		}
		if parsed.Version() != 7 {
			t.Fatalf("%q: версия %d, ждали 7", id, parsed.Version())
		}
		if seen[id] {
			t.Fatalf("повтор id %q", id)
		}
		seen[id] = true
	}
}

func TestNewIDSortsByTime(t *testing.T) {
	t.Parallel()
	ids := make([]string, 5)
	for i := range ids {
		ids[i] = idgen.UUIDv7{}.NewID()
		// Разные миллисекунды: порядок задают первые 48 бит — время создания.
		time.Sleep(time.Millisecond)
	}
	if !slices.IsSorted(ids) {
		t.Fatalf("строковый порядок id не совпал с порядком создания: %v", ids)
	}
}
