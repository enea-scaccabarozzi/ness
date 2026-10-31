package worker_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
	_ "modernc.org/sqlite" // the test breaks the store through a second handle
)

func TestCompactsOnceAtTheBoundaryWhenIdle(t *testing.T) {
	// No boundary: never compacted. A boundary: /compact NOTE typed at the first idle,
	// empty-box pass, logged, the request cleared, nothing on the next pass. Compaction comes
	// before the hint of the same pass.
	// covers: R-WRK-030, R-WRK-031, R-WRK-032
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.passes(20, 5*time.Second)
	if n := len(b.rt.typed); n != 0 {
		t.Fatalf("typed without a boundary: %q", b.rt.typed)
	}
	b.pane(sA).status = "working"
	if err := b.s.SetBoundary(sA, "plan in docs/plan.md", time.Time{}, b.now); err != nil {
		t.Fatal(err)
	}
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.passes(3, 5*time.Second)
	if n := len(b.rt.typed); n != 0 {
		t.Fatalf("typed while working: %q", b.rt.typed)
	}
	b.pane(sA).status = "idle"
	b.passes(1, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 1 || got[0] != "/compact plan in docs/plan.md" {
		t.Fatalf("typed %q", got)
	}
	if sess, _ := b.s.Session(sA); sess.BoundaryNote != "" {
		t.Fatal("the boundary was not cleared")
	}
	if b.logCount(" compact "+sA+" plan in docs/plan.md") != 1 {
		t.Fatalf("log:\n%s", strings.Join(b.logLines(), "\n"))
	}
	b.passes(1, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 2 || got[1] != "ness: 1 unread (request); run ness inbox" {
		t.Fatalf("the pass after the compaction typed %q", got)
	}
	b.passes(10, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 2 {
		t.Fatalf("a second compaction: %q", got)
	}
}

func TestResumeComesBeforeCompaction(t *testing.T) {
	// A boundary set while the pane was limited: "continue" first, /compact on the next pass.
	// covers: R-WRK-030, R-WRK-043
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).screen = limitScreen("You've hit your weekly limit · resets 1pm")
	if err := b.s.SetBoundary(sA, "n", time.Time{}, b.now); err != nil {
		t.Fatal(err)
	}
	b.pass()
	b.passes(3*6+7, 10*time.Minute) // 13:10
	b.passes(1, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 2 || got[0] != "continue" || got[1] != "/compact n" {
		t.Fatalf("typed %q", got)
	}
}

func TestCompactIsNotTypedAgainWhenTheClearFails(t *testing.T) {
	// /compact went in but the store refused to clear the request: later passes retry the clear
	// only, the error is logged once, and nothing else is typed twice.
	// covers: R-WRK-032
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	if err := b.s.SetBoundary(sA, "n", time.Time{}, b.now); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.Join(filepath.Dir(b.log), "state.sqlite3")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `CREATE TRIGGER nb BEFORE UPDATE ON sessions
		BEGIN SELECT RAISE(ABORT, 'locked by test'); END`); err != nil {
		t.Fatal(err)
	}
	b.passes(4, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 1 || got[0] != "/compact n" {
		t.Fatalf("typed %q", got)
	}
	if b.logCount(" error "+sA+" compact") != 1 {
		t.Fatalf("log:\n%s", strings.Join(b.logLines(), "\n"))
	}
	if _, err := db.ExecContext(ctx, `DROP TRIGGER nb`); err != nil {
		t.Fatal(err)
	}
	b.passes(3, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 1 {
		t.Fatalf("typed after the clear succeeded: %q", got)
	}
	if sess, _ := b.s.Session(sA); sess.BoundaryNote != "" {
		t.Fatal("the boundary was never cleared")
	}
}
