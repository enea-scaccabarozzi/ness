package worker_test

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

// trouble checks one Trouble alert's envelope and returns its text.
func trouble(t *testing.T, al worker.Alert, title, root string) string {
	t.Helper()
	if al.Kind != worker.AlertTrouble || al.Project != "demo-app" || al.Title != title || al.Session != "" ||
		al.Root != root || al.Page != "" {
		t.Fatalf("trouble alert = %+v, want title %q root %q", al, title, root)
	}
	if n := utf8.RuneCountInString(al.Text); n == 0 || n > alertCap+1 {
		t.Fatalf("trouble text of %d runes: %q", n, al.Text)
	}
	return al.Text
}

// Rejects an implementation that leaves a stall the manager did not act on to the manager alone,
// alerts twice per episode, or loses the alert after a failed post (and, retrying, sends the
// manager a second item). The alert comes at the second miss, a stall window after the report.
func TestStallReportAlsoAlertsTheOperatorOnce(t *testing.T) {
	// covers: R-WRK-050, R-WRK-094
	b := newBench(t)
	a := b.alerts()
	a.fail = 1
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.pane(sA).status = "working" // the manager is busy: the first post that fails is the Trouble one
	b.titled(sB, "child-b")
	b.send(sA, sB, store.IntentRequest, "please", false)
	b.pass()
	b.passes(29, time.Minute)
	if len(a.got) != 0 {
		t.Fatalf("at 29 min: %+v", a.got)
	}
	b.passes(40, time.Minute)
	troubles := a.of(worker.AlertTrouble)
	if len(troubles) != 1 || !strings.Contains(trouble(t, troubles[0], "child-b", sA), "idle since") {
		t.Fatalf("alerts = %+v", a.got)
	}
	if got := fyis(b.listed(sA)); len(got) != 1 {
		t.Fatalf("the manager got %d stall items, want 1", len(got))
	}
	b.pane(sB).status = "working"
	b.passes(1, time.Minute)
	b.pane(sB).status = "idle"
	b.passes(61, time.Minute)
	if len(a.of(worker.AlertTrouble)) != 2 {
		t.Fatalf("second episode: %+v", a.of(worker.AlertTrouble))
	}
}

// Rejects an implementation that reports the child to its manager but not to the operator
// (B3), alerts a child that has a live background shell, or ignores the shell ending.
func TestUnfinishedChildStallAlertsOnceAndBackgroundShellNever(t *testing.T) {
	// covers: R-WRK-050, R-WRK-094
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.titled(sB, "child-b")
	b.pass()
	b.passes(29, time.Minute)
	if len(a.of(worker.AlertTrouble)) != 0 {
		t.Fatalf("at 29 min: %+v", a.got)
	}
	b.passes(40, time.Minute)
	troubles := a.of(worker.AlertTrouble)
	if len(troubles) != 1 || !strings.Contains(trouble(t, troubles[0], "child-b", sA), "idle since") {
		t.Fatalf("alerts after the stall = %+v", a.got)
	}

	c := newBench(t)
	ac := c.alerts()
	c.session(sA, store.Operator, "claude")
	c.session(sB, sA, "claude")
	c.pane(sB).screen = backgroundScreen
	c.pass()
	c.passes(90, time.Minute)
	if len(ac.got) != 0 {
		t.Fatalf("a child with a live background shell caused %+v", ac.got)
	}
	c.pane(sB).screen = idleScreen
	c.passes(61, time.Minute)
	if got := ac.of(worker.AlertTrouble); len(got) != 1 {
		t.Fatalf("after the shell ended: %+v", ac.got)
	}
}

// Rejects an implementation that alerts the screen error before it is confirmed, or never.
func TestScreenErrorAlertsOnce(t *testing.T) {
	// covers: R-WRK-051, R-WRK-094
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.pane(sA).screen = errorScreen("API Error: 500 Internal server error")
	b.pass()
	if len(a.got) != 0 {
		t.Fatalf("one reading alerted: %+v", a.got)
	}
	b.passes(5, 5*time.Second)
	if len(a.got) != 1 || !strings.Contains(trouble(t, a.got[0], "x", sA), "API Error") {
		t.Fatalf("alerts = %+v", a.got)
	}
}

// Rejects an implementation that stops the unavailable model silently on the operator's side.
func TestUnavailableModelStopAlertsOnce(t *testing.T) {
	// covers: R-WRK-062, R-WRK-094
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.titled(sB, "child-b")
	b.pane(sB).screen = unavailScreen
	b.send(sA, sB, store.IntentRequest, "work", false)
	b.passes(10, 5*time.Second)
	if len(b.rt.closed) != 1 {
		t.Fatalf("closed %q", b.rt.closed)
	}
	troubles := a.of(worker.AlertTrouble)
	if len(troubles) != 1 || !strings.Contains(trouble(t, troubles[0], "child-b", sA), "model") {
		t.Fatalf("alerts = %+v", a.got)
	}
}
