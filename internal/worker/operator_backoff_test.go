package worker_test

import (
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

// at runs one pass at start+d.
func (b *bench) at(start time.Time, d time.Duration) {
	b.t.Helper()
	b.now = start.Add(d)
	b.pass()
}

// blockedFyis keeps the manager's messages from id that say it is blocked.
func blockedFyis(b *bench, manager, id string) int {
	n := 0
	for _, it := range stallFyis(b, manager, id) {
		if strings.Contains(it.Body, "blocked") {
			n++
		}
	}
	return n
}

// Rejects the field's one phone alert at 2 minutes (the manager can answer a prompt first), a
// manager left untold, a phone told once and never again on a prompt that stays, a backoff that
// stops after 4 h or repeats faster, and an episode that does not reset once the prompt is gone.
func TestBlockedPromptTellsTheManagerThenThePhoneOnABackoff(t *testing.T) {
	// covers: R-WRK-081, R-WRK-094
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.pane(sA).status = "working"
	b.session(sB, sA, "claude")
	b.titled(sB, "child-b")
	b.pane(sB).status = "blocked"
	start := b.now
	b.pass()
	steps := []struct {
		at             time.Duration
		fyis, troubles int
	}{
		{2*time.Minute - time.Second, 0, 0},
		{2*time.Minute + 5*time.Second, 1, 0},
		{10*time.Minute - time.Second, 1, 0},
		{10*time.Minute + 5*time.Second, 1, 1},
		{30 * time.Minute, 1, 1},
		{time.Hour - time.Second, 1, 1},
		{time.Hour + 5*time.Second, 1, 2},
		{4*time.Hour - time.Second, 1, 2},
		{4*time.Hour + 5*time.Second, 1, 3},
		{6 * time.Hour, 1, 3},
		{8*time.Hour - time.Second, 1, 3},
		{8*time.Hour + 5*time.Second, 1, 4},
		{12*time.Hour + 5*time.Second, 1, 5},
	}
	for _, s := range steps {
		b.at(start, s.at)
		if got := blockedFyis(b, sA, sB); got != s.fyis {
			t.Fatalf("at %v the manager has %d blocked messages, want %d", s.at, got, s.fyis)
		}
		troubles := a.of(worker.AlertTrouble)
		if len(troubles) != s.troubles {
			t.Fatalf("at %v: %d Trouble alerts, want %d: %+v", s.at, len(troubles), s.troubles, troubles)
		}
		for _, al := range troubles {
			if !strings.Contains(trouble(t, al, "child-b", sA), "blocked") {
				t.Fatalf("at %v: %+v", s.at, al)
			}
		}
	}

	b.pane(sB).status = "working"
	b.at(start, 13*time.Hour)
	b.pane(sB).status = "blocked"
	again := start.Add(13*time.Hour + time.Minute)
	b.at(again, 0)
	b.at(again, 2*time.Minute+5*time.Second)
	if got := blockedFyis(b, sA, sB); got != 2 {
		t.Fatalf("second episode: the manager has %d blocked messages, want 2", got)
	}
	b.at(again, 10*time.Minute-time.Second)
	if got := a.of(worker.AlertTrouble); len(got) != 5 {
		t.Fatalf("second episode before 10 min: %+v", got)
	}
	b.at(again, 10*time.Minute+5*time.Second)
	if got := a.of(worker.AlertTrouble); len(got) != 6 {
		t.Fatalf("second episode at 10 min: %+v", got)
	}
}

// Rejects a manager message for a pane that herdr reads as blocked while it compacts: the
// compaction exception covers the manager's step as well as the phone's.
func TestBlockedWhileCompactingTellsNobody(t *testing.T) {
	// covers: R-WRK-081
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.pane(sA).status = "working"
	b.session(sB, sA, "claude")
	b.pane(sB).status = "blocked"
	b.pane(sB).screen = "✻ Compacting conversation… (15s)\n\n" + idleScreen
	b.pass()
	b.passes(15, time.Minute)
	if got := blockedFyis(b, sA, sB); got != 0 {
		t.Fatalf("the manager was told of a compaction: %d messages", got)
	}
	if len(a.got) != 0 {
		t.Fatalf("alerted during a compaction: %+v", a.got)
	}
}

// Rejects a backoff that, after a channel outage, sends every step it missed on consecutive passes:
// one alert when the channel is back, then the next step of the schedule (review L1).
func TestBlockedBackoffSkipsTheStepsAnOutageMissed(t *testing.T) {
	// covers: R-WRK-081
	b := newBench(t)
	a := b.alerts()
	a.fail = 1 << 20
	b.session(sA, store.Operator, "claude")
	b.pane(sA).status = "working"
	b.session(sB, sA, "claude")
	b.pane(sB).status = "blocked"
	start := b.now
	b.pass()
	for m := 1; m <= 300; m++ {
		b.at(start, time.Duration(m)*time.Minute)
	}
	a.fail = 0
	for m := 301; m <= 303; m++ {
		b.at(start, time.Duration(m)*time.Minute)
	}
	if got := a.of(worker.AlertTrouble); len(got) != 1 {
		t.Fatalf("after the outage: %d Trouble alerts, want 1", len(got))
	}
	b.at(start, 8*time.Hour-time.Second)
	if got := a.of(worker.AlertTrouble); len(got) != 1 {
		t.Fatalf("before 8 h: %d Trouble alerts, want 1", len(got))
	}
	b.at(start, 8*time.Hour+5*time.Second)
	if got := a.of(worker.AlertTrouble); len(got) != 2 {
		t.Fatalf("at 8 h: %d Trouble alerts, want 2", len(got))
	}
}
