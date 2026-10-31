package worker_test

import (
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

// Rejects an implementation that alerts an integrated quick root (it is done), or one that skips
// Your move for a full root with the same facts, or for a quick root that has not integrated.
func TestYourMoveSkipsAnIntegratedQuickRoot(t *testing.T) {
	// covers: R-WRK-098
	for _, tc := range []struct {
		name       string
		size       string
		integrated bool
		want       int
	}{
		{"quick and integrated is done", "quick", true, 0},
		{"quick, not integrated yet", "quick", false, 1},
		{"full with an integration still alerts", "full", true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newBench(t)
			a := b.alerts()
			b.session(sA, store.Operator, "claude")
			if err := b.s.SetSize(sA, tc.size); err != nil {
				t.Fatal(err)
			}
			if tc.integrated {
				if err := b.s.SetIntegration(sA, "abc1234", b.now); err != nil {
					t.Fatal(err)
				}
			}
			b.pass()
			b.passes(40, time.Minute)
			if got := a.of(worker.AlertYourMove); len(got) != tc.want {
				t.Fatalf("Your move alerts = %+v, want %d", got, tc.want)
			}
		})
	}
}

// Rejects a done state that outlives the turn that integrated: a follow-up turn after the
// integration makes the root a normal idle root again, so a chat-wait gets its Your move.
func TestYourMoveReturnsAfterAFollowUpTurn(t *testing.T) {
	// covers: R-WRK-098
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	if err := b.s.SetSize(sA, "quick"); err != nil {
		t.Fatal(err)
	}
	b.pane(sA).status = "working"
	b.passes(1, time.Minute)
	if err := b.s.SetIntegration(sA, "abc1234", b.now); err != nil { // integrated inside this turn
		t.Fatal(err)
	}
	b.passes(1, time.Minute)
	b.pane(sA).status = "idle"
	b.passes(40, time.Minute)
	if got := a.of(worker.AlertYourMove); len(got) != 0 {
		t.Fatalf("the integrating turn is done, got %+v", got)
	}
	b.pane(sA).status = "working" // the operator asks for more
	b.passes(2, time.Minute)
	b.pane(sA).status = "idle"
	b.passes(40, time.Minute)
	if got := a.of(worker.AlertYourMove); len(got) != 1 {
		t.Fatalf("a follow-up turn left the root waiting: %+v", got)
	}
}
