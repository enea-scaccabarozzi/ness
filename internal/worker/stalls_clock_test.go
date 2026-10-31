package worker_test

import (
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// Rejects the field-2 race: a child idle for 30 minutes whose answer was disposed 3 seconds ago
// was reported as stalled, before the hint carrying the answer could reach it. The clock runs from
// the answer; the child stalls only after Stall more minutes of idling.
func TestAnswerJustDeliveredRestartsTheStallClock(t *testing.T) {
	// covers: R-WRK-050
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	req := b.send(sB, sA, store.IntentRequest, "may I?", false)
	b.pass()
	b.passes(29, time.Minute)
	b.passes(1, time.Minute+3*time.Second) // t = 30m03s
	if _, err := b.s.Reply(req, sA, "yes", store.Resolved, b.now); err != nil {
		t.Fatal(err)
	}
	b.passes(1, time.Second)
	if got := fyis(b.listed(sA)); len(got) != 0 {
		t.Fatalf("stalled 1 s after its answer: %+v", got)
	}
	b.passes(29, time.Minute)
	if got := fyis(b.listed(sA)); len(got) != 0 {
		t.Fatalf("stalled 29 min after its answer: %+v", got)
	}
	b.passes(2, time.Minute)
	if got := fyis(b.listed(sA)); len(got) != 1 {
		t.Fatalf("31 min after its answer A has %+v, want one stall", got)
	}
}
