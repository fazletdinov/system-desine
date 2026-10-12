package udpstat

import (
	"testing"
	"time"
)

func TestAnalyze(t *testing.T) {
	tests := []struct {
		name    string
		sent    int
		replies []Reply
		want    Counts
	}{
		{"все дошли по порядку", 3, []Reply{{0, 1}, {1, 1}, {2, 1}}, Counts{Sent: 3, Received: 3}},
		{"одна потеря", 3, []Reply{{0, 1}, {2, 1}}, Counts{Sent: 3, Received: 2, Lost: 1}},
		{"переупорядочивание", 3, []Reply{{0, 1}, {2, 1}, {1, 1}}, Counts{Sent: 3, Received: 3, Reordered: 1}},
		{"дубликат", 2, []Reply{{0, 1}, {0, 1}, {1, 1}}, Counts{Sent: 2, Received: 2, Duplicates: 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Analyze(tt.sent, tt.replies)
			if got.Counts != tt.want {
				t.Fatalf("Analyze() = %+v, want %+v", got.Counts, tt.want)
			}
		})
	}
}

func TestAnalyzeCollectsRTTsOfUniqueReplies(t *testing.T) {
	got := Analyze(2, []Reply{{0, time.Millisecond}, {0, 5 * time.Millisecond}, {1, 2 * time.Millisecond}})
	if len(got.RTTs) != 2 {
		t.Fatalf("want 2 RTTs, got %v", got.RTTs)
	}
}
