package worker_test

import (
	"regexp"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

// stallFyis keeps the stall messages a child's sender sent: the fyis, not the replies around them.
func stallFyis(b *bench, to, from string) []store.Item {
	var out []store.Item
	for _, it := range fyis(b.listed(to)) {
		if it.Sender == from {
			out = append(out, it)
		}
	}
	return out
}

// Each case rejects the implementation the review reproduced (B3 owes-its-brief with no check on
// who the child waits for): a child idle by design is neither reported nor alerted.
func TestChildWaitingOnSomeoneElseIsNotStalled(t *testing.T) {
	// covers: R-WRK-050
	cases := []struct {
		name  string
		setup func(b *bench)
	}{
		{"a manager whose own child is working", func(b *bench) {
			b.session(sC, sB, "claude")
			b.pane(sC).status = "working"
		}},
		{"a manager whose grandchild has a background shell", func(b *bench) {
			b.session(sC, sB, "claude")
			b.session(sD, sC, "claude")
			b.pane(sD).screen = backgroundScreen
		}},
		{"a child waiting on a request to its manager", func(b *bench) {
			b.send(sB, sA, store.IntentRequest, "may I?", false)
		}},
		{"a child waiting on a decision to the operator", func(b *bench) {
			b.send(sB, store.Operator, store.IntentDecision, "which way?", false)
		}},
		{"a child waiting on a request to the operator", func(b *bench) {
			b.send(sB, store.Operator, store.IntentRequest, "may I?", false)
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
			b.passes(40, time.Minute)
			if got := stallFyis(b, sA, sB); len(got) != 0 {
				t.Fatalf("stall message for a waiting child: %+v", got)
			}
			if got := a.of(worker.AlertTrouble); len(got) != 0 {
				t.Fatalf("Trouble alert for a waiting child: %+v", got)
			}
		})
	}
}

// Rejects an implementation that waits on a request forever: once the manager answers, the child
// owes its brief again and stalls (and one that never counts the wait at all, see above).
func TestChildStallsAgainOnceItsRequestIsAnswered(t *testing.T) {
	// covers: R-WRK-050
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).status = "working"
	b.session(sB, sA, "claude")
	req := b.send(sB, sA, store.IntentRequest, "may I?", false)
	b.pass()
	b.passes(40, time.Minute)
	if _, err := b.s.Reply(req, sA, "yes", store.Resolved, b.now); err != nil {
		t.Fatal(err)
	}
	b.passes(40, time.Minute)
	if got := stallFyis(b, sA, sB); len(got) != 1 {
		t.Fatalf("after the answer: %+v", got)
	}
}

// Rejects an implementation that counts a returned handoff as delivered (the review's nit): a
// child sent back to rework that then sits idle is reported.
func TestChildWithReturnedHandoffStalls(t *testing.T) {
	// covers: R-WRK-050
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).status = "working"
	b.session(sB, sA, "claude")
	h := b.send(sB, sA, store.IntentHandoff, "done", false)
	if _, err := b.s.Reply(h, sA, "redo the tests", store.Returned, b.now); err != nil {
		t.Fatal(err)
	}
	b.pass()
	b.passes(40, time.Minute)
	if got := stallFyis(b, sA, sB); len(got) != 1 {
		t.Fatalf("a child with a returned handoff: %+v", got)
	}
}

// Rejects the review's missing "and": the sentence goes verbatim to the manager and into the
// Trouble alert, and the tests used to check only a substring.
func TestBriefStallSentenceMatchesTheSpec(t *testing.T) {
	// covers: R-WRK-050, R-WRK-094
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.pass()
	b.passes(70, time.Minute) // the Trouble alert comes at the second miss (R-WRK-094)
	fyi := regexp.MustCompile(`^ness worker: session ` + sB + ` \(x\) has been idle since \S+Z and has not handed off its brief$`)
	got := stallFyis(b, sA, sB)
	if len(got) != 1 || !fyi.MatchString(got[0].Body) {
		t.Fatalf("manager's message: %+v", got)
	}
	troubles := a.of(worker.AlertTrouble)
	text := regexp.MustCompile(`^has been idle since \S+Z and has not handed off its brief$`)
	if len(troubles) != 1 || !text.MatchString(troubles[0].Text) {
		t.Fatalf("alerts: %+v", a.got)
	}
}
