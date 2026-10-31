package worker_test

import (
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

// Rejects the review's S1: a busy or unseen descendant silencing a session that owes an answer.
// Each case leaves root A holding an open obligation (B's handoff or request) while something
// below it looks busy; A is reported after the stall time and the operator gets the Trouble.
func TestOwedAnswerIsReportedWhateverDescendantsDo(t *testing.T) {
	// covers: R-WRK-050, R-WRK-094
	cases := []struct {
		name  string
		setup func(b *bench)
	}{
		{"a sibling shows a background shell", func(b *bench) {
			b.session(sC, sA, "claude")
			b.pane(sC).screen = backgroundScreen
			b.send(sB, sA, store.IntentHandoff, "done", false)
		}},
		{"a child pane was never seen", func(b *bench) {
			b.session(sC, sA, "claude")
			if err := b.s.ClearReady(sC); err != nil {
				t.Fatal(err)
			}
			b.send(sB, sA, store.IntentHandoff, "done", false)
		}},
		{"the child that asked waits in a background shell", func(b *bench) {
			b.pane(sB).screen = backgroundScreen
			b.send(sB, sA, store.IntentRequest, "may I?", false)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := newBench(t)
			a := b.alerts()
			b.session(sA, store.Operator, "claude")
			b.session(sB, sA, "claude")
			c.setup(b)
			b.pass()
			b.passes(70, time.Minute) // the Trouble alert comes at the second miss (R-WRK-094)
			if got := stallFyis(b, store.Operator, sA); len(got) != 1 {
				t.Fatalf("stall report for the root: %+v", got)
			}
			if got := a.of(worker.AlertTrouble); len(got) == 0 {
				t.Fatalf("no Trouble alert, got %+v", a.got)
			}
		})
	}
}

// Keeps the brief path as it was: a child owing its brief while a descendant works is not reported.
func TestOwedBriefWithBusyDescendantIsNotReported(t *testing.T) {
	// covers: R-WRK-050
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).status = "working"
	b.session(sB, sA, "claude")
	b.session(sC, sB, "claude")
	b.pane(sC).screen = backgroundScreen
	b.pass()
	b.passes(40, time.Minute)
	if got := stallFyis(b, sA, sB); len(got) != 0 {
		t.Fatalf("stall report for a child with a busy descendant: %+v", got)
	}
}
