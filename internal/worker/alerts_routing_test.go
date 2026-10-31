package worker_test

import (
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

// Rejects a stall report that also goes to the phone at once (the field's phone noise), one that
// never reaches the phone, and one that reaches it more than once per episode. A root's first miss
// goes to the operator's inbox, never the phone.
func TestStallGoesToTheManagerFirstAndThePhoneOnTheSecondMiss(t *testing.T) {
	// covers: R-WRK-050, R-WRK-094
	cases := []struct {
		name, who, manager string
		setup              func(b *bench)
	}{
		{"a child owing its brief", sB, sA, func(b *bench) {
			b.pane(sA).status = "working"
			b.session(sB, sA, "claude")
		}},
		{"a root owing the operator an answer", sA, store.Operator, func(b *bench) {
			b.send(store.Operator, sA, store.IntentRequest, "please look", false)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := newBench(t)
			a := b.alerts()
			b.session(sA, store.Operator, "claude")
			c.setup(b)
			b.pass()
			b.passes(31, time.Minute)
			if got := stallFyis(b, c.manager, c.who); len(got) != 1 {
				t.Fatalf("at 31 min the manager has %+v, want the stall report", got)
			}
			if got := a.of(worker.AlertTrouble); len(got) != 0 {
				t.Fatalf("the first miss reached the phone: %+v", got)
			}
			b.passes(27, time.Minute) // 58 min: the report is 28 min old
			if got := a.of(worker.AlertTrouble); len(got) != 0 {
				t.Fatalf("before the second miss: %+v", got)
			}
			b.passes(4, time.Minute) // 62 min
			troubles := a.of(worker.AlertTrouble)
			if len(troubles) != 1 || troubles[0].Root != sA {
				t.Fatalf("at the second miss: %+v, want one Trouble on %s's thread", a.got, sA)
			}
			b.passes(120, time.Minute)
			if got := a.of(worker.AlertTrouble); len(got) != 1 {
				t.Fatalf("later in the episode: %+v", got)
			}
			if got := stallFyis(b, c.manager, c.who); len(got) != 1 {
				t.Fatalf("later the manager has %+v, want still one report", got)
			}
		})
	}
}

// Rejects a second-miss alert that ignores the manager's answer: something reached the child after
// the report, so the phone stays quiet for the rest of the episode.
func TestStallTroubleNotSentOnceTheManagerAnswered(t *testing.T) {
	// covers: R-WRK-094
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.pane(sA).status = "working"
	b.session(sB, sA, "claude")
	b.pass()
	b.passes(31, time.Minute)
	if got := stallFyis(b, sA, sB); len(got) != 1 {
		t.Fatalf("at 31 min: %+v, want the stall report", got)
	}
	b.passes(9, time.Minute)
	b.send(sA, sB, store.IntentFYI, "keep going on the tests", false)
	b.passes(120, time.Minute)
	if got := a.of(worker.AlertTrouble); len(got) != 0 {
		t.Fatalf("the manager answered and the phone still rang: %+v", got)
	}
}

// Rejects a second-miss alert after the operator answered a root's stall report: LastInbound
// compared the operator's NULL with <> and counted no answer of theirs (review H4).
func TestRootStallTroubleNotSentOnceTheOperatorAnswered(t *testing.T) {
	// covers: R-WRK-094
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.send(store.Operator, sA, store.IntentRequest, "please look", false)
	b.pass()
	b.passes(31, time.Minute)
	report := stallFyis(b, store.Operator, sA)
	if len(report) != 1 {
		t.Fatalf("at 31 min the operator has %+v, want the stall report", report)
	}
	b.passes(9, time.Minute)
	if _, err := b.s.Reply(report[0].ID, store.Operator, "seen, keep going", store.Resolved, b.now); err != nil {
		t.Fatal(err)
	}
	b.passes(120, time.Minute)
	if got := a.of(worker.AlertTrouble); len(got) != 0 {
		t.Fatalf("the operator answered and the phone still rang: %+v", got)
	}
}
