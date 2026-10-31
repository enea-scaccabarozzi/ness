package stats_test

import (
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/stats"
)

func TestParseSince(t *testing.T) {
	// covers: R-STAT-004
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	for in, want := range map[string]time.Time{
		"2026-10-01":           time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		"2026-10-01T08:30:00Z": time.Date(2026, 10, 1, 8, 30, 0, 0, time.UTC),
		"7d":                   now.Add(-7 * 24 * time.Hour),
		"36h":                  now.Add(-36 * time.Hour),
	} {
		got, err := stats.ParseSince(in, now)
		if err != nil || !got.Equal(want) {
			t.Errorf("ParseSince(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := stats.ParseSince("999999999d", now); err == nil || !strings.Contains(err.Error(), "range") {
		t.Errorf("the cause is lost: %v", err)
	}
	for _, bad := range []string{"yesterday", "", "0s", "-1d", "2026-13-01", "1w"} {
		if _, err := stats.ParseSince(bad, now); err == nil {
			t.Errorf("ParseSince(%q) accepted", bad)
		}
	}
}
