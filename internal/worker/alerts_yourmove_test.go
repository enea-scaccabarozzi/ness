package worker_test

import (
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

const yourMoveText = "idle for 30 minutes; nothing appears to be running"

// Rejects an implementation that fires early or late, repeats inside the episode, never starts a
// second one, or words it differently from the spec.
func TestYourMoveOncePerIdleRootEpisode(t *testing.T) {
	// covers: R-WRK-093
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.pass()
	b.passes(29, time.Minute)
	if len(a.got) != 0 {
		t.Fatalf("at 29 min: %+v", a.got)
	}
	b.passes(1, time.Minute)
	want := worker.Alert{Kind: worker.AlertYourMove, Project: "demo-app", Title: "x", Text: yourMoveText, Root: sA}
	if len(a.got) != 1 || a.got[0] != want {
		t.Fatalf("at 30 min: %+v, want [%+v]", a.got, want)
	}
	b.passes(30, time.Minute)
	if len(a.got) != 1 {
		t.Fatalf("at 60 min: %+v", a.got)
	}
	b.pane(sA).status = "working"
	b.passes(1, time.Minute)
	b.pane(sA).status = "idle"
	b.passes(1, time.Minute)
	b.passes(29, time.Minute)
	if len(a.got) != 1 {
		t.Fatalf("29 min into the second episode: %+v", a.got)
	}
	b.passes(1, time.Minute)
	if len(a.of(worker.AlertYourMove)) != 2 {
		t.Fatalf("second episode: %+v", a.got)
	}
}

// Rejects an implementation that does not look below the root, or looks at the wrong things:
// each case is one reason the root is not waiting on the operator.
func TestYourMoveSuppressed(t *testing.T) {
	// covers: R-WRK-093
	cases := []struct {
		name  string
		setup func(b *bench)
	}{
		{"the root holds an open request to the operator", func(b *bench) {
			b.send(sA, store.Operator, store.IntentRequest, "may I?", false)
		}},
		{"a child is working", func(b *bench) {
			b.session(sB, sA, "claude")
			b.pane(sB).status = "working"
		}},
		{"a child is blocked", func(b *bench) {
			b.session(sB, sA, "claude")
			b.pane(sB).status = "blocked"
		}},
		{"a grandchild is working under an idle child", func(b *bench) {
			b.session(sB, sA, "claude")
			b.session(sC, sB, "claude")
			b.pane(sC).status = "working"
		}},
		{"a child has a live background shell", func(b *bench) {
			b.session(sB, sA, "claude")
			b.pane(sB).screen = backgroundScreen
		}},
		{"a child is still launching", func(b *bench) {
			b.session(sB, sA, "claude")
			if err := b.s.ClearReady(sB); err != nil {
				b.t.Fatal(err)
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := newBench(t)
			a := b.alerts()
			b.session(sA, store.Operator, "claude")
			c.setup(b)
			b.pass()
			b.passes(40, time.Minute)
			if got := a.of(worker.AlertYourMove); len(got) != 0 {
				t.Fatalf("Your move fired: %+v", got)
			}
		})
	}
}

// Rejects an implementation that treats any child as proof of life (a dead child leaves the root
// waiting, the census case: it surfaces once, as Trouble, per F4), and one that gives a child
// Your move.
func TestYourMoveFiresForADeadChildAndOnlyForRoots(t *testing.T) {
	// covers: R-WRK-093
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.pass()
	b.passes(70, time.Minute) // the Trouble alert comes at the second miss (R-WRK-094)
	if got := a.of(worker.AlertTrouble); len(got) != 1 || got[0].Root != sA {
		t.Fatalf("a dead child owing its brief: want one Trouble on the root's thread, got %+v", got)
	}
	got := a.of(worker.AlertYourMove)
	if len(got) != 0 {
		t.Fatalf("a dead child owing its brief also fired Your move: %+v", got)
	}

	c := newBench(t)
	ac := c.alerts()
	c.session(sA, store.Operator, "claude")
	c.session(sB, sA, "claude")
	c.pane(sA).status = "working"
	c.pass()
	c.passes(90, time.Minute)
	if got = ac.of(worker.AlertYourMove); len(got) != 0 {
		t.Fatalf("an idle child under a working root got Your move: %+v", got)
	}
}
