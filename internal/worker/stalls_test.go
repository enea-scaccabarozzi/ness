package worker_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func errorScreen(line string) string {
	return "assistant output\n  ⎿  " + line + "\n╭────────╮\n│ > " + placeholder + "\n╰────────╯\n"
}

func TestStallCountsFromTheObligation(t *testing.T) {
	// B, a child that has handed off, is parked idle for two hours with nothing owed, then A hands
	// it work: no report on the pass the request arrives, none at 29 min after it, one at 30. Its
	// open handoff is no wait (R-WRK-099), so the follow-up obligation stalls.
	// covers: R-WRK-050, R-WRK-099
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.send(sB, sA, store.IntentHandoff, "done", false)
	b.pass()
	b.passes(120, time.Minute)
	b.send(sA, sB, store.IntentRequest, "your turn", false)
	b.passes(29, time.Minute)
	if got := fyis(b.listed(sA)); len(got) != 0 {
		t.Fatalf("29 min after the request A has %+v", got)
	}
	b.passes(1, time.Minute)
	if got := fyis(b.listed(sA)); len(got) != 1 {
		t.Fatalf("30 min after the request A has %d fyi items", len(got))
	}
}

func TestStallReportOncePerEpisode(t *testing.T) {
	// B idle with an open obligation from A: nothing at 29 min, one fyi from B in A's inbox at
	// 30 min, still one at 60; after B worked and idled 30 min again, a second one.
	// covers: R-WRK-050, R-WRK-052
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.send(sA, sB, store.IntentRequest, "please", false)
	b.pass()
	b.passes(29, time.Minute)
	if got := b.listed(sA); len(got) != 0 {
		t.Fatalf("at 29 min A has %d items", len(got))
	}
	b.passes(1, time.Minute)
	got := b.listed(sA)
	if len(got) != 1 || got[0].Sender != sB || got[0].Intent != store.IntentFYI || !strings.Contains(got[0].Body, "idle since") {
		t.Fatalf("at 30 min A has %+v", got)
	}
	b.passes(30, time.Minute)
	if got := b.listed(sA); len(got) != 1 {
		t.Fatalf("at 60 min A has %d items", len(got))
	}
	if b.logCount(" stall "+sB) != 1 {
		t.Fatalf("log:\n%s", strings.Join(b.logLines(), "\n"))
	}
	b.pane(sB).status = "working"
	b.passes(1, time.Minute)
	b.pane(sB).status = "idle"
	b.passes(31, time.Minute)
	if got := b.listed(sA); len(got) != 2 {
		t.Fatalf("after a second episode A has %d items", len(got))
	}
	// No obligation, no report: A idle for an hour with only an fyi.
	b.send(sB, sA, store.IntentFYI, "note", false)
	b.passes(60, time.Minute)
	if got := b.listed(store.Operator); len(got) != 0 {
		t.Fatalf("the operator got %+v without an obligation on A", got)
	}
}

func TestScreenErrorReportsOncePerEpisode(t *testing.T) {
	// One message per error pattern; a limit notice is not an error (D37).
	// covers: R-WRK-051, R-WRK-052
	b := newBench(t)
	for s, line := range map[string]string{
		sA: "API Error: 500 Internal server error", sB: "Error: overloaded_error", sC: "The service is at capacity",
		sD: "You've hit your weekly limit · resets Oct 9, 1pm",
	} {
		b.session(s, store.Operator, "claude")
		b.pane(s).screen = errorScreen(line)
	}
	b.passes(5, 5*time.Second)
	got := b.listed(store.Operator)
	if len(got) != 3 {
		t.Fatalf("operator got %d reports, want 3: %+v", len(got), got)
	}
	for _, it := range got {
		if it.Sender == sD || it.Intent != store.IntentFYI || !strings.Contains(it.Body, "shows") {
			t.Fatalf("report %+v", it)
		}
	}
	b.pane(sA).status = "working"
	b.passes(1, 5*time.Second)
	b.pane(sA).status = "idle"
	b.passes(2, 5*time.Second)
	if got := b.listed(store.Operator); len(got) != 4 {
		t.Fatalf("after A worked and errs again: %d reports", len(got))
	}
}

func TestModelUnavailableStopsTheSession(t *testing.T) {
	// The tab is closed, the row stays open, the manager gets one urgent fyi naming the fix and
	// the resume; later passes find the pane gone and stay quiet.
	// covers: R-WRK-062, R-SESS-009
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.pane(sB).screen = unavailScreen
	b.send(sA, sB, store.IntentRequest, "work", false)
	b.pass()
	if len(b.rt.closed) != 0 {
		t.Fatalf("one reading closed %q", b.rt.closed)
	}
	b.passes(1, 5*time.Second)
	if len(b.rt.closed) != 1 || b.rt.closed[0] != "t:"+paneOf(sB) {
		t.Fatalf("closed %q", b.rt.closed)
	}
	got := b.listed(sA)
	if len(got) != 1 || !got[0].Urgent || !strings.Contains(got[0].Body, "ness open --resume "+sB) ||
		!strings.Contains(got[0].Body, "tiers.standard") {
		t.Fatalf("A got %+v", got)
	}
	if sess, _ := b.s.Session(sB); sess.Closed {
		t.Fatal("the row was closed")
	}
	if got := b.typedTo(sB); len(got) != 0 { // no hint while the reading awaits its second pass, none after the stop
		t.Fatalf("typed into the stopped session: %q", got)
	}
	b.passes(10, 5*time.Second)
	if got := b.listed(sA); len(got) != 1 || b.logCount(" stop "+sB) != 1 || b.logCount(" error "+sB+" ") != 1 {
		t.Fatalf("later passes: %d items, log:\n%s", len(got), strings.Join(b.logLines(), "\n"))
	}
}

func TestWorkerNeverChangesModelOrEffort(t *testing.T) {
	// The worker half of R-MOD-012: across a hint, a compaction and a resume nothing typed is a
	// /model or effort command.
	// covers: R-MOD-012
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.pass()
	if err := b.s.SetBoundary(sA, "n", time.Time{}, b.now); err != nil {
		t.Fatal(err)
	}
	b.passes(1, 5*time.Second)
	b.pane(sA).screen = limitScreen("You've hit your session limit · resets 11am")
	b.passes(1, 5*time.Second)
	b.passes(62, time.Minute)
	if len(b.rt.typed) != 3 {
		t.Fatalf("typed %q", b.rt.typed)
	}
	for _, l := range b.rt.typed {
		if strings.Contains(l, "/model") || strings.Contains(l, "--effort") || strings.Contains(l, "/effort") {
			t.Fatalf("typed %q", l)
		}
	}
}

const notFound = "There's an issue with the selected model (claude-opus-5-5). It may not exist."

// unavailScreen is the client's own notice at column 0 above the box.
var unavailScreen = "assistant output\n" + notFound + "\n╭────────╮\n│ > " + placeholder + "\n╰────────╯\n"

func TestOneAmbiguousScreenActsOnNothing(t *testing.T) {
	// An error or unavailable reading that is gone on the next pass reports and closes nothing;
	// nor does one the operator is sitting in front of, for the tab.
	// covers: R-WRK-051, R-WRK-062, R-SESS-009
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	for _, screen := range []string{unavailScreen, errorScreen("API Error: 500 x"), unavailScreen, idleScreen} {
		b.pane(sB).screen = screen
		b.passes(1, 5*time.Second)
		b.pane(sB).screen = idleScreen
		b.passes(1, 5*time.Second)
	}
	if len(b.rt.closed) != 0 || len(b.listed(sA)) != 0 {
		t.Fatalf("closed %q, items %+v", b.rt.closed, b.listed(sA))
	}
}

func TestStopWaitsForTheOperator(t *testing.T) {
	// Focused or holding a draft, the tab stays; the manager hears of the error; when the
	// operator leaves, the tab is closed.
	// covers: R-WRK-062, R-SESS-009
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.pane(sB).screen = unavailScreen
	b.pane(sB).focused = true
	b.passes(4, 5*time.Second)
	if len(b.rt.closed) != 0 {
		t.Fatalf("closed %q while focused", b.rt.closed)
	}
	if got := b.listed(sA); len(got) != 1 || got[0].Urgent {
		t.Fatalf("manager got %+v, want the one error report", got)
	}
	b.pane(sB).focused = false
	b.pane(sB).screen = strings.Replace(unavailScreen, "│ > "+placeholder, "│ > half a thought", 1)
	b.passes(2, 5*time.Second)
	if len(b.rt.closed) != 0 {
		t.Fatalf("closed %q over a draft", b.rt.closed)
	}
	b.pane(sB).screen = unavailScreen
	b.passes(1, 5*time.Second)
	if len(b.rt.closed) != 1 {
		t.Fatalf("closed %q after the operator left", b.rt.closed)
	}
}

func TestOwnReportsOnAManagersScreenDoNothing(t *testing.T) {
	// The stop notice and the error report, shown in the manager's pane as it reads its inbox,
	// are not a state of the manager (H1).
	// covers: R-WRK-051, R-WRK-062
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.pane(sB).screen = unavailScreen
	b.passes(2, 5*time.Second)
	for _, it := range b.listed(sA) {
		b.pane(sA).screen = errorScreen("API Error: " + it.Body)
		b.passes(3, 5*time.Second)
		b.pane(sA).screen = errorScreen(it.Body)
		b.passes(3, 5*time.Second)
	}
	if len(b.rt.closed) != 1 || len(b.listed(store.Operator)) != 0 {
		t.Fatalf("closed %q, operator got %+v", b.rt.closed, b.listed(store.Operator))
	}
}

func TestStopNeverClosesAnOperatorsTab(t *testing.T) {
	// An adopted root's tab is the operator's own: the operator is told, nothing is closed, and the
	// text does not offer --resume, which is refused for an adopted session.
	// covers: R-WRK-062, R-SESS-009
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.sql(`UPDATE sessions SET adopted = 1 WHERE id = ?`, sA)
	b.pane(sA).screen = unavailScreen
	b.passes(6, 5*time.Second)
	got := b.listed(store.Operator)
	if len(b.rt.closed) != 0 || len(got) != 1 || !got[0].Urgent || strings.Contains(got[0].Body, "--resume") ||
		!strings.Contains(got[0].Body, "restart the client") {
		t.Fatalf("closed %q, operator got %+v", b.rt.closed, got)
	}
}

func TestStopOfASessionWithoutATabOnlyTells(t *testing.T) {
	// covers: R-WRK-062
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.sql(`UPDATE sessions SET tab = '' WHERE id = ?`, sB)
	b.pane(sB).screen = unavailScreen
	b.passes(6, 5*time.Second)
	if got := b.listed(sA); len(b.rt.closed) != 0 || len(got) != 1 || strings.Contains(got[0].Body, "--resume") {
		t.Fatalf("closed %q, A got %+v", b.rt.closed, got)
	}
	if b.logCount(" stop "+sB) != 0 || b.logCount(" told "+sB) != 1 { // m2: the log claims no close
		t.Fatalf("log:\n%s", strings.Join(b.logLines(), "\n"))
	}
}

func TestStopSendsFirstAndOncePerRun(t *testing.T) {
	// A failed send leaves the tab open and is retried; a tab that is already gone (ErrGone) while
	// the pane still reads live gives exactly one message.
	// covers: R-WRK-062, R-SESS-009
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.pane(sB).screen = unavailScreen
	b.denyFYI(sB)
	b.passes(4, 5*time.Second)
	if len(b.rt.closed) != 0 || len(b.listed(sA)) != 0 {
		t.Fatalf("closed %q with the send failing, A has %+v", b.rt.closed, b.listed(sA))
	}
	b.allowFYI()
	b.rt.closeErr = errors.New("herdr is busy")
	b.passes(3, 5*time.Second)
	if got := b.listed(sA); len(got) != 1 || len(b.rt.closed) != 3 { // a failed close is retried, not re-told
		t.Fatalf("A has %d items, close attempts %q", len(got), b.rt.closed)
	}
	b.rt.closed = nil
	b.rt.closeErr = fmt.Errorf("%w: tab gone", session.ErrGone)
	b.passes(6, 5*time.Second)
	if got := b.listed(sA); len(got) != 1 || len(b.rt.closed) != 1 {
		t.Fatalf("A has %d items, close attempts %q", len(got), b.rt.closed)
	}
}

// sql runs a statement on a second connection to the bench's store: state the store's API does not
// let a test set.
func (b *bench) sql(stmt string, args ...any) {
	b.t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(filepath.Dir(b.log), "state.sqlite3")+
		"?_pragma=busy_timeout(5000)")
	if err != nil {
		b.t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // test helper
	if _, err := db.ExecContext(context.Background(), stmt, args...); err != nil {
		b.t.Fatal(err)
	}
}

// denyFYI makes every insert of an fyi from sender fail, as a broken store would; allowFYI undoes it.
func (b *bench) denyFYI(sender string) {
	b.t.Helper()
	b.sql(`CREATE TRIGGER deny BEFORE INSERT ON items WHEN NEW.intent = 'fyi' AND NEW.sender = '` + sender +
		`' BEGIN SELECT RAISE(ABORT, 'denied'); END`)
}

func (b *bench) allowFYI() { b.t.Helper(); b.sql(`DROP TRIGGER deny`) }
