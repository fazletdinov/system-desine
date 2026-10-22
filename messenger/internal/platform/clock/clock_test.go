package clock_test

import (
	"testing"
	"time"

	"messenger/internal/platform/clock"
)

func TestSystemNowIsMicrosecondUTC(t *testing.T) {
	for range 100 {
		now := clock.System{}.Now()
		if now.Location() != time.UTC {
			t.Fatalf("часовой пояс %v, ждали UTC", now.Location())
		}
		if now.Nanosecond()%int(time.Microsecond) != 0 {
			t.Fatalf("%v: время точнее микросекунды — Postgres его округлит, и ответ разойдётся с базой", now)
		}
	}
}
