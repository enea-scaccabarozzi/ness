package worker_test

import (
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

// park records a boundary for id now, with until (zero: until something reaches it).
func (b *bench) park(id string, until time.Time) {
	b.t.Helper()
	if err := b.s.SetBoundary(id, "state written down", until, b.now); err != nil {
		b.t.Fatal(err)
	}
}

// Each case is a field false positive: a child that waits (an accepted request, a probe window, a
// boundary) idle for two hours. Rejects a stall rule that counts only unanswered items as waits,
// lets a wait excuse only the owed brief, or reports a session that holds an obligation while it
// waits.
func TestWaitingChildGetsNoStallReport(t *testing.T) {
	// covers: R-WRK-050, R-WRK-099
	cases := []struct {
		name  string
		setup func(b *bench)
	}{
		{"its request was accepted, the answer still owed", func(b *bench) {
			req := b.send(sB, sA, store.IntentRequest, "may I?", false)
			if _, err := b.s.Reply(req, sA, "on it", store.Accepted, b.now); err != nil {
				b.t.Fatal(err)
			}
		}},
		{"it owns an active probe", func(b *bench) {
			b.start(sB, "probes/window.md")
		}},
		{"it parked until something arrives", func(b *bench) {
			b.park(sB, time.Time{})
		}},
		{"it parked until a later time", func(b *bench) {
			b.park(sB, b.now.Add(3*time.Hour))
		}},
		{"it holds an obligation while its decision waits on the operator", func(b *bench) {
			b.send(sA, sB, store.IntentRequest, "do the thing", false)
			b.send(sB, store.Operator, store.IntentDecision, "which way?", false)
		}},
		{"it holds an obligation while its probe runs", func(b *bench) {
			b.send(sA, sB, store.IntentRequest, "do the thing", false)
			b.start(sB, "probes/window.md")
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := newBench(t)
			a := b.alerts()
			b.session(sA, store.Operator, "claude")
			b.pane(sA).status = "working"
			b.session(sB, sA, "claude")
			c.setup(b)
			b.pass()
			b.passes(120, time.Minute)
			if got := stallFyis(b, sA, sB); len(got) != 0 {
				t.Fatalf("stall report for a waiting child: %+v", got)
			}
			if got := a.of(worker.AlertTrouble); len(got) != 0 {
				t.Fatalf("Trouble alert for a waiting child: %+v", got)
			}
		})
	}
}

// Rejects a park that ignores its until-time (a report at 30 min) or outlives it (none after).
func TestParkedChildStallsOnceItsUntilPasses(t *testing.T) {
	// covers: R-WRK-050, R-WRK-099
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).status = "working"
	b.session(sB, sA, "claude")
	b.park(sB, b.now.Add(time.Hour))
	b.pass()
	b.passes(59, time.Minute)
	if got := stallFyis(b, sA, sB); len(got) != 0 {
		t.Fatalf("before the until: %+v", got)
	}
	b.passes(35, time.Minute)
	if got := stallFyis(b, sA, sB); len(got) != 1 {
		t.Fatalf("after the until: %+v, want one stall report", got)
	}
}

// Rejects an open-ended park that never ends: once the manager's item reaches the child it waits
// on nothing, and idle a stall window after the arrival it is reported.
func TestParkedChildStallsAfterAnArrival(t *testing.T) {
	// covers: R-WRK-050, R-WRK-099
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).status = "working"
	b.session(sB, sA, "claude")
	b.park(sB, time.Time{})
	b.pass()
	b.passes(120, time.Minute)
	if got := stallFyis(b, sA, sB); len(got) != 0 {
		t.Fatalf("parked: %+v", got)
	}
	b.send(sA, sB, store.IntentFYI, "the window is open, go on", false)
	b.passes(29, time.Minute)
	if got := stallFyis(b, sA, sB); len(got) != 0 {
		t.Fatalf("29 min after the arrival: %+v", got)
	}
	b.passes(2, time.Minute)
	if got := stallFyis(b, sA, sB); len(got) != 1 {
		t.Fatalf("31 min after the arrival: %+v, want one stall report", got)
	}
}

// Rejects a probe wait read from worker memory or never released: once the probe stops the child
// is reported like any other.
func TestChildStallsOnceItsProbeStops(t *testing.T) {
	// covers: R-WRK-050, R-WRK-099
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).status = "working"
	b.session(sB, sA, "claude")
	p := b.start(sB, "probes/window.md")
	b.pass()
	b.passes(60, time.Minute)
	if got := stallFyis(b, sA, sB); len(got) != 0 {
		t.Fatalf("with its probe active: %+v", got)
	}
	if err := b.s.SetProbeState(p.ID, store.ProbeStopped, "done", b.now); err != nil {
		t.Fatal(err)
	}
	b.passes(2, time.Minute)
	if got := stallFyis(b, sA, sB); len(got) != 1 {
		t.Fatalf("after the probe stopped: %+v, want one stall report", got)
	}
}
