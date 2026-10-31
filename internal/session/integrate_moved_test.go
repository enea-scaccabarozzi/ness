package session_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/merge"
	"github.com/enea-scaccabarozzi/ness/internal/atom/squash"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

// moveRun is one Integrate whose gate (testdata/move_gate.sh) moves main in repo while it runs.
// It folds with merge unless it is set, onto main checked out in repo unless away moved repo off it.
type moveRun struct {
	g                  session.Git
	it                 session.Integrator
	repo, common, mode string
	state              string
	log                strings.Builder
}

func newMoveRun(t *testing.T, mode string) *moveRun {
	t.Helper()
	g, repo, common := setupRepo(t)
	return &moveRun{g: g, it: merge.Integrator{}, repo: repo, common: common, mode: mode, state: t.TempDir()}
}

// away checks repo out on another branch, so main is checked out nowhere and moves by update-ref.
func (m *moveRun) away(t *testing.T) {
	t.Helper()
	if _, err := m.g.Out(m.repo, "checkout", "-q", "-b", "elsewhere"); err != nil {
		t.Fatal(err)
	}
}

func (m *moveRun) integrate(t *testing.T) (session.Result, error) {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("testdata", "move_gate.sh"))
	if err != nil {
		t.Fatal(err)
	}
	return session.Integrate(m.g, m.it, session.IntegrateSpec{
		Repo: m.repo, Common: m.common, TmpName: "tmp-s-1", Log: &m.log, Records: "docs/sessions",
		Gate:        "sh " + script + " " + m.repo + " " + m.state + " " + m.mode,
		Integration: session.Integration{Source: "session/s-1", Target: "main"},
	})
}

// file is a trimmed state file the gate wrote.
func (m *moveRun) file(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(m.state, name))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

// landed asserts main holds the move and the source's change, the checkout is clean, the result
// records main, and the throwaway is gone.
func (m *moveRun) landed(t *testing.T, res session.Result) {
	t.Helper()
	main, _ := m.g.Rev(m.repo, "main")
	if res.Ref != main {
		t.Fatalf("recorded %s, main is %s", res.Ref, main)
	}
	if ok, _ := m.g.IsAncestor(m.repo, m.file(t, "moved"), "main"); !ok {
		t.Fatal("the commit that moved main during the gate is gone from main")
	}
	if a, err := m.g.Out(m.repo, "show", "main:a.txt"); err != nil || a != "1" {
		t.Fatalf("the source's change is not on main: %q %v", a, err)
	}
	if dirty, _ := m.g.Out(m.repo, "status", "--porcelain"); dirty != "" {
		t.Fatalf("the checkout of main is not at the landed commit:\n%s", dirty)
	}
	m.swept(t)
}

func (m *moveRun) swept(t *testing.T) {
	t.Helper()
	if left, _ := filepath.Glob(filepath.Join(m.common, "ness", "tmp-s-1-*")); len(left) != 0 {
		t.Fatalf("throwaway left behind: %v", left)
	}
}

// Rejects an implementation that lands the old fold over a moved target (losing the record
// commit or failing the fast-forward), and one that re-runs the gate for a record-only move.
func TestIntegrateLandsARecordOnlyMoveWithoutASecondGate(t *testing.T) {
	// covers: R-SESS-165
	m := newMoveRun(t, "record")
	res, err := m.integrate(t)
	if err != nil {
		t.Fatalf("record-only move: %v\n%s", err, m.log.String())
	}
	if n := m.file(t, "count"); n != "1" {
		t.Fatalf("the gate ran %s times for a record-only move", n)
	}
	m.landed(t, res)
	if want := "ness: main moved during the gate by session records only; landing without a second gate"; !strings.Contains(m.log.String(), want) {
		t.Fatalf("log lacks %q:\n%s", want, m.log.String())
	}
}

// Rejects an implementation that treats any move as record-only (no second gate), and one whose
// second gate still sees the old target as NESS_BASE.
func TestIntegrateRunsTheGateAgainWhenCodeMoved(t *testing.T) {
	// covers: R-SESS-165, R-SESS-156
	m := newMoveRun(t, "code")
	res, err := m.integrate(t)
	if err != nil {
		t.Fatalf("code move: %v\n%s", err, m.log.String())
	}
	if n := m.file(t, "count"); n != "2" {
		t.Fatalf("the gate ran %s times for a code move, want 2", n)
	}
	m.landed(t, res)
	if want := "ness: main moved during the gate; running the gate again on the new main"; !strings.Contains(m.log.String(), want) {
		t.Fatalf("log lacks %q:\n%s", want, m.log.String())
	}
	bases := strings.Split(m.file(t, "bases"), "\n")
	if len(bases) != 2 || bases[1] != m.file(t, "moved") {
		t.Fatalf("NESS_BASE per gate run %q, want the second to be the moved main %s", bases, m.file(t, "moved"))
	}
}

// Rejects an implementation that surfaces raw git output for a re-fold conflict, moves main to a
// half-made fold, or leaves the throwaway behind.
func TestIntegrateRefusesPlainlyWhenTheMovedTargetConflicts(t *testing.T) {
	// covers: R-SESS-165
	m := newMoveRun(t, "conflict")
	_, err := m.integrate(t)
	if !errors.Is(err, session.ErrTargetMoved) {
		t.Fatalf("conflicting move: %v", err)
	}
	for _, raw := range []string{"exit status", "hint:"} {
		if strings.Contains(err.Error(), raw) {
			t.Fatalf("the error carries raw git text %q: %v", raw, err)
		}
	}
	if !strings.Contains(err.Error(), "main") {
		t.Fatalf("the error does not name the target: %v", err)
	}
	if main, _ := m.g.Rev(m.repo, "main"); main != m.file(t, "moved") {
		t.Fatalf("main is %s, want the moved %s untouched", main, m.file(t, "moved"))
	}
	m.swept(t)
}

// Rejects an unbounded retry loop and one that lands after a second code move without a gate.
func TestIntegrateGivesUpAfterASecondCodeMove(t *testing.T) {
	// covers: R-SESS-165
	m := newMoveRun(t, "code-always")
	_, err := m.integrate(t)
	if !errors.Is(err, session.ErrTargetMoved) {
		t.Fatalf("two code moves: %v", err)
	}
	if n := m.file(t, "count"); n != "2" {
		t.Fatalf("the gate ran %s times, want 2", n)
	}
	if main, _ := m.g.Rev(m.repo, "main"); main != m.file(t, "moved") {
		t.Fatalf("main is %s, want the last move %s", main, m.file(t, "moved"))
	}
	m.swept(t)
}

// Rejects an implementation whose record-only check lets rename detection hide a code file's old
// path: a code file renamed into the records moved code out of the gated tree.
func TestIntegrateRunsTheGateAgainWhenCodeIsRenamedIntoTheRecords(t *testing.T) {
	// covers: R-SESS-165
	m := newMoveRun(t, "rename")
	testkit.Commit(t, m.g, m.repo, "code.txt", "code\n", "code on main")
	res, err := m.integrate(t)
	if err != nil {
		t.Fatalf("rename into the records: %v\n%s", err, m.log.String())
	}
	if n := m.file(t, "count"); n != "2" {
		t.Fatalf("the gate ran %s times for a code file renamed into the records, want 2", n)
	}
	m.landed(t, res)
}

// Rejects a re-fold that only works where main is checked out (the update-ref path lands the old
// fold, or fails its compare-and-swap) or only with the merge integrator (squash, the default,
// re-folds onto the moved target).
func TestIntegrateRefoldsAMovedTargetWhereverItIsAndWhateverFolds(t *testing.T) {
	// covers: R-SESS-165
	for _, c := range []struct {
		name, mode, gates string
		squash, away      bool
	}{
		{"away record", "record", "1", false, true},
		{"away code", "code", "2", false, true},
		{"squash record", "record", "1", true, false},
		{"squash code", "code", "2", true, false},
		{"squash away code", "code", "2", true, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := newMoveRun(t, c.mode)
			if c.squash {
				m.it = squash.Integrator{Subject: "feat: x"}
			}
			if c.away {
				m.away(t)
			}
			res, err := m.integrate(t)
			if err != nil {
				t.Fatalf("%v\n%s", err, m.log.String())
			}
			if n := m.file(t, "count"); n != c.gates {
				t.Fatalf("the gate ran %s times, want %s", n, c.gates)
			}
			m.landed(t, res)
		})
	}
}
