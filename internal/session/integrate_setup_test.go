package session_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/merge"
	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

// setupRepo returns a repo on main with session/s-1 holding one commit, main checked out.
func setupRepo(t *testing.T) (g session.Git, repo, common string) {
	t.Helper()
	g = session.Git{Exec: cli.SystemExec}
	repo = testkit.NewRepo(t, g)
	if _, err := g.Out(repo, "checkout", "-q", "-b", "session/s-1"); err != nil {
		t.Fatal(err)
	}
	testkit.Commit(t, g, repo, "a.txt", "1\n", "one")
	if _, err := g.Out(repo, "checkout", "-q", "main"); err != nil {
		t.Fatal(err)
	}
	common, err := g.CommonDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	return g, repo, common
}

// Rejects an implementation that runs setup after the gate, in the real checkout, or not at all.
func TestIntegrateRunsSetupInTheThrowawayBeforeTheGate(t *testing.T) {
	// covers: R-SESS-146
	g, repo, common := setupRepo(t)
	_, err := session.Integrate(g, merge.Integrator{}, session.IntegrateSpec{
		Repo: repo, Common: common, TmpName: "tmp-s-1",
		Setup: "touch linked", Gate: "test -f linked",
		Integration: session.Integration{Source: "session/s-1", Target: "main"},
	})
	if err != nil {
		t.Fatalf("integrate with setup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "linked")); err == nil {
		t.Fatal("setup ran in the real checkout")
	}
}

// Rejects an implementation that ignores a setup failure, runs the gate anyway, drops the setup
// output from the log, or leaves the target moved or the throwaway behind.
func TestIntegrateSetupFailureFailsLikeTheGateWithoutRunningIt(t *testing.T) {
	// covers: R-SESS-146
	g, repo, common := setupRepo(t)
	before, _ := g.Rev(repo, "main")
	marker := filepath.Join(t.TempDir(), "gate-ran")
	var log strings.Builder
	_, err := session.Integrate(g, merge.Integrator{}, session.IntegrateSpec{
		Repo: repo, Common: common, TmpName: "tmp-s-1",
		Setup: "echo boom; exit 3", Gate: "touch " + marker, Log: &log,
		Integration: session.Integration{Source: "session/s-1", Target: "main"},
	})
	if !errors.Is(err, session.ErrGate) {
		t.Fatalf("err = %v, want ErrGate", err)
	}
	if !strings.Contains(log.String(), "boom") {
		t.Errorf("log lacks the setup output: %q", log.String())
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("the gate ran after a failed setup")
	}
	if now, _ := g.Rev(repo, "main"); now != before {
		t.Error("target moved although setup failed")
	}
	if left, _ := filepath.Glob(filepath.Join(common, "ness", "tmp-*")); len(left) != 0 {
		t.Errorf("throwaway left behind: %v", left)
	}
}
