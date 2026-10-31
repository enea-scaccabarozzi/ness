package probe_test

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/probe"
)

func TestVerdictNoteAndFirstRun(t *testing.T) {
	// covers: R-PRB-041, R-PRB-042, R-PRB-052
	if got := probe.VerdictNote(probe.Probe{Path: "probes/p.md"}, "", "red", "bad"); got != "probe probes/p.md verdict: RED\n\nbad\n" {
		t.Errorf("VerdictNote = %q", got)
	}
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	r := rand.New(rand.NewPCG(1, 1))
	for _, tc := range []struct {
		name string
		pf   probe.Probe
		lo   time.Duration
		hi   time.Duration
	}{
		{"every runs now", probe.Probe{Jitter: time.Hour}, 0, 0},
		{"once within jitter", probe.Probe{Once: true, Jitter: 20 * time.Minute}, 0, 20 * time.Minute},
		{"capped at the window", probe.Probe{Once: true, Jitter: time.Hour, Window: time.Nanosecond}, 0, time.Nanosecond},
	} {
		for range 50 {
			if d := probe.FirstRun(tc.pf, now, r).Sub(now); d < tc.lo || d > tc.hi {
				t.Fatalf("%s: first run after %v, want [%v, %v]", tc.name, d, tc.lo, tc.hi)
			}
		}
	}
}
