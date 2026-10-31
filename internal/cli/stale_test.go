package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestStaleLockWaitsForAWorkerOnTheBasesNess(t *testing.T) {
	// covers: R-UPD-070
	primary := t.TempDir()
	wt := filepath.Join(primary, ".worktrees", "x", ".ness")
	if err := os.MkdirAll(wt, 0o750); err != nil {
		t.Fatal(err)
	}
	mine, err := os.ReadFile(filepath.Join("testdata", "stale", "lock-0.1.0.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(wt, "lock.json"), mine, 0o600); err != nil {
		t.Fatal(err)
	}
	base, err := os.ReadFile(filepath.Join("testdata", "stale", "lock-0.3.0.json"))
	if err != nil {
		t.Fatal(err)
	}
	git := func(_, _ string, args ...string) ([]byte, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --verify main^{commit}":
			return []byte("abc\n"), nil
		case "show abc:.ness/lock.json":
			return base, nil
		}
		return nil, fmt.Errorf("unexpected git %v", args)
	}
	sess := store.Session{ID: "s-r", Worktree: ".worktrees/x", Base: "main"}
	d := Deps{Version: "0.2.0", Exec: git, Notes: "# Changelog\n"}
	if _, stale, serr := staleLock(d, primary, sess, map[string]baseLock{}); serr != nil || stale {
		t.Fatalf("worker 0.2.0, base 0.3.0: stale=%v err=%v, want not stale until the restart", stale, serr)
	}
	d.Version = "0.3.0"
	st, stale, err := staleLock(d, primary, sess, map[string]baseLock{})
	if err != nil || !stale || st.From != "0.1.0" || st.To != "0.3.0" {
		t.Fatalf("worker 0.3.0: %+v stale=%v err=%v", st, stale, err)
	}
}

func TestStaleLockBaseCheckout(t *testing.T) {
	// covers: R-UPD-070 — a root working in the base checkout (no worktree) cannot lag its own
	// base: not stale and no error, so the worker logs nothing for it on every start
	sess := store.Session{ID: "s-fixture", Base: "main"}
	if _, stale, err := staleLock(Deps{Version: "0.2.0"}, t.TempDir(), sess, map[string]baseLock{}); err != nil || stale {
		t.Fatalf("stale=%v err=%v, want neither", stale, err)
	}
}
