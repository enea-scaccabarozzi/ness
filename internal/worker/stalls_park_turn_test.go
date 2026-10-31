package worker_test

import (
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

// parkAndCompact has id take a real turn, park with until (zero: open-ended), and stay idle until
// the worker compacted it; it fails the test when the compaction did not consume the boundary.
func (b *bench) parkAndCompact(id string, until time.Time) {
	b.t.Helper()
	b.act(id, session.Activity{Turn: b.now})
	b.pass()
	b.passes(1, time.Minute)
	b.park(id, until)
	b.passes(3, time.Minute)
	sess, err := b.s.Session(id)
	if err != nil || sess.BoundaryNote != "" || sess.ParkedAt.IsZero() {
		b.t.Fatalf("after the compaction: %+v, %v", sess, err)
	}
}

// turn has id take a real assistant turn now: the transcript moves on and the pane works once.
func (b *bench) turn(id string) {
	b.act(id, session.Activity{Turn: b.now})
	b.workedOnce(id)
}

// Rejects an open-ended park that outlives the work (review H3): a root that parked, was compacted
// and then worked again by itself idles on nobody, and gets its Your move. The compaction itself is
// no turn: the park holds through it.
func TestOpenParkEndsAtTheRootsNextTurn(t *testing.T) {
	// covers: R-WRK-093, R-WRK-099
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.parkAndCompact(sA, time.Time{})
	b.passes(60, time.Minute)
	if got := a.of(worker.AlertYourMove); len(got) != 0 {
		t.Fatalf("parked and compacted: %+v", got)
	}
	b.turn(sA)
	b.passes(120, time.Minute)
	if got := a.of(worker.AlertYourMove); len(got) != 1 {
		t.Fatalf("idle after a turn past the park: %+v, want one Your move", a.got)
	}
}

// Rejects the same for a child that owes its brief (review H3), and a park with an until-time
// that a turn ends early: that one holds until its time.
func TestOpenParkEndsAtTheChildsNextTurn(t *testing.T) {
	// covers: R-WRK-050, R-WRK-099
	for name, c := range map[string]struct {
		until time.Duration
		want  int
	}{
		"open-ended":     {0, 1},
		"until 5 h away": {5 * time.Hour, 0},
	} {
		t.Run(name, func(t *testing.T) {
			b := newBench(t)
			b.session(sA, store.Operator, "claude")
			b.pane(sA).status = "working"
			b.session(sB, sA, "claude")
			var until time.Time
			if c.until > 0 {
				until = b.now.Add(c.until)
			}
			b.parkAndCompact(sB, until)
			b.turn(sB)
			b.passes(180, time.Minute)
			if got := stallFyis(b, sA, sB); len(got) != c.want {
				t.Fatalf("idle 3 h after a turn past the park: %+v, want %d stall reports", got, c.want)
			}
		})
	}
}
