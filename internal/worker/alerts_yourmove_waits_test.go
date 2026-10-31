package worker_test

import (
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

// Rejects the field's silenced Your move (an old open ask covered every later idle spell) and one
// that fires while the root idles because it just asked: an ask from the root's latest turn covers
// the idle root; after the root worked past it and idles with it still open, Your move fires once.
func TestYourMoveCoveredOnlyByAnAskFromTheLatestTurn(t *testing.T) {
	// covers: R-WRK-093
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.pane(sA).status = "working"
	b.pass()
	b.passes(1, time.Minute)
	b.send(sA, store.Operator, store.IntentDecision, "ship rung two?", false)
	b.passes(1, time.Minute)
	b.pane(sA).status = "idle"
	b.passes(60, time.Minute)
	if got := a.of(worker.AlertYourMove); len(got) != 0 {
		t.Fatalf("the root idles on the ask it just sent, got %+v", got)
	}
	b.workedOnce(sA) // the operator chatted; the ask stays open
	b.passes(29, time.Minute)
	if got := a.of(worker.AlertYourMove); len(got) != 0 {
		t.Fatalf("29 min into the new spell: %+v", got)
	}
	b.passes(3, time.Minute)
	if got := a.of(worker.AlertYourMove); len(got) != 1 {
		t.Fatalf("after the root worked past its ask: %+v, want one Your move", got)
	}
	b.passes(60, time.Minute)
	if got := a.of(worker.AlertYourMove); len(got) != 1 {
		t.Fatalf("later in the episode: %+v", got)
	}
}

// Rejects a Your move that counts only operator items as the root's reason to idle: a root that
// waits on its probe or parked itself is not idle on the operator.
func TestYourMoveCoveredByTheRootsOtherWaits(t *testing.T) {
	// covers: R-WRK-093, R-WRK-099
	cases := []struct {
		name  string
		setup func(b *bench)
	}{
		{"an active probe", func(b *bench) { b.start(sA, "probes/window.md") }},
		{"parked until something arrives", func(b *bench) { b.park(sA, time.Time{}) }},
		{"parked until a later time", func(b *bench) { b.park(sA, b.now.Add(3*time.Hour)) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := newBench(t)
			a := b.alerts()
			b.session(sA, store.Operator, "claude")
			c.setup(b)
			b.pass()
			b.passes(90, time.Minute)
			if got := a.of(worker.AlertYourMove); len(got) != 0 {
				t.Fatalf("Your move for a waiting root: %+v", got)
			}
		})
	}
}

// Rejects a child that owes its brief covering Your move while it waits (here on the root's
// "on it"): its own stall report cannot fire, so the tree went silent overnight (review H1).
func TestYourMoveNotCoveredByAWaitingChildThatOwesItsBrief(t *testing.T) {
	// covers: R-WRK-093
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	req := b.send(sB, sA, store.IntentRequest, "may I?", false)
	if _, err := b.s.Reply(req, sA, "on it", store.Accepted, b.now); err != nil {
		t.Fatal(err)
	}
	b.pass()
	b.passes(240, time.Minute)
	if got := a.of(worker.AlertYourMove); len(got) != 1 {
		t.Fatalf("both idle 4 h: %+v, want one Your move", a.got)
	}
}

// Rejects an obligation covering Your move while the root's only wait is an ask it worked past:
// that ask silences the stall report, so the obligation would be told by nobody (review H2).
func TestYourMoveFiresForAnObligationBehindAnOldAsk(t *testing.T) {
	// covers: R-WRK-093
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.pane(sA).status = "working"
	b.pass()
	b.send(sA, store.Operator, store.IntentDecision, "ship rung two?", false)
	b.pane(sA).status = "idle"
	b.passes(1, time.Minute)
	b.workedOnce(sA) // a later turn: the ask is old
	b.send(store.Operator, sA, store.IntentRequest, "and look at the logs", false)
	b.passes(240, time.Minute)
	if got := a.of(worker.AlertYourMove); len(got) != 1 {
		t.Fatalf("an obligation behind an old ask: %+v, want one Your move", a.got)
	}
}
