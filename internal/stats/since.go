package stats

import (
	"fmt"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/probe"
)

// ParseSince reads --since: a date (2026-10-01), an RFC 3339 time, or a positive duration in
// Go syntax plus d, counted back from now (R-STAT-004). A bare date is midnight UTC: the spec names
// no zone, and UTC keeps a transcript filter the same on every machine.
func ParseSince(s string, now time.Time) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	d, err := probe.ParseDuration(s)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"--since %q: want a date (2026-10-01), an RFC 3339 time or a duration (7d, 36h): %w", s, err)
	}
	return now.Add(-d), nil
}
