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

func TestIntegrateRunsTheGateInAThrowawayAndMovesTheTarget(t *testing.T) {
	// covers: R-SESS-091, R-SESS-092, R-SESS-093, R-SESS-099
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	if _, err := g.Out(repo, "checkout", "-q", "-b", "session/s-1"); err != nil {
		t.Fatal(err)
	}
	testkit.Commit(t, g, repo, "a.txt", "1\n", "one")
	if _, err := g.Out(repo, "checkout", "-q", "main"); err != nil {
		t.Fatal(err)
	}
	common, _ := g.CommonDir(repo)
	before, _ := g.Rev(repo, "main")
	cwdFile := filepath.Join(t.TempDir(), "cwd")
	in := session.Integration{Source: "session/s-1", Target: "main"}
	res, err := session.Integrate(g, merge.Integrator{}, session.IntegrateSpec{
		Repo: repo, Common: common, TmpName: "tmp-s-1",
		Gate: "pwd > " + cwdFile, Integration: in, Log: os.Stderr,
	})
	if err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.ReadFile(cwdFile)
	if !strings.HasPrefix(strings.TrimSpace(string(cwd)), filepath.Join(common, "ness", "tmp-s-1")) {
		t.Fatalf("gate ran in %q", cwd)
	}
	if left, _ := filepath.Glob(filepath.Join(common, "ness", "tmp-s-1-*")); len(left) != 0 {
		t.Fatal("throwaway worktree not removed")
	}
	after, _ := g.Rev(repo, "main")
	if after == before || res.Ref != after {
		t.Fatalf("main %s → %s, recorded %s", before, after, res.Ref)
	}
	// A failing gate: nothing moves, the throwaway is gone.
	if _, err := g.Out(repo, "checkout", "-q", "-b", "session/s-2"); err != nil {
		t.Fatal(err)
	}
	testkit.Commit(t, g, repo, "b.txt", "2\n", "two")
	if _, err := g.Out(repo, "checkout", "-q", "main"); err != nil {
		t.Fatal(err)
	}
	_, err = session.Integrate(g, merge.Integrator{}, session.IntegrateSpec{
		Repo: repo, Common: common, TmpName: "tmp-s-2",
		Gate: "false", Integration: session.Integration{Source: "session/s-2", Target: "main"},
	})
	if !errors.Is(err, session.ErrGate) {
		t.Fatalf("failing gate: %v", err)
	}
	if now, _ := g.Rev(repo, "main"); now != after {
		t.Fatal("target moved although the gate failed")
	}
	if left, _ := filepath.Glob(filepath.Join(common, "ness", "tmp-s-2-*")); len(left) != 0 {
		t.Fatal("throwaway worktree not removed after a failed gate")
	}
}

func TestIntegrateRefusesWhenThereIsNothingToIntegrate(t *testing.T) {
	// covers: R-SESS-092
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	if _, err := g.Out(repo, "branch", "session/s-1"); err != nil {
		t.Fatal(err)
	}
	common, _ := g.CommonDir(repo)
	_, err := session.Integrate(g, merge.Integrator{}, session.IntegrateSpec{
		Repo: repo, Common: common, TmpName: "tmp-s-1",
		Gate: "true", Integration: session.Integration{Source: "session/s-1", Target: "main"},
	})
	if !errors.Is(err, session.ErrNothingToIntegrate) {
		t.Fatalf("nothing to integrate: %v", err)
	}
}

func TestIntegrateGateOutputGoesToTheLog(t *testing.T) {
	// A26: a failing gate shows its output.
	// covers: R-SESS-091
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	if _, err := g.Out(repo, "checkout", "-q", "-b", "session/s-1"); err != nil {
		t.Fatal(err)
	}
	testkit.Commit(t, g, repo, "a.txt", "1\n", "one")
	if _, err := g.Out(repo, "checkout", "-q", "main"); err != nil {
		t.Fatal(err)
	}
	common, _ := g.CommonDir(repo)
	var log strings.Builder
	_, err := session.Integrate(g, merge.Integrator{}, session.IntegrateSpec{
		Repo: repo, Common: common, TmpName: "tmp-s-1",
		Gate: "x=gate; echo said-$x; exit 3", Integration: session.Integration{Source: "session/s-1", Target: "main"},
		Log: &log,
	})
	if !errors.Is(err, session.ErrGate) || !strings.Contains(log.String(), "said-gate") {
		t.Fatalf("err %v, log %q", err, log.String())
	}
}

func TestIntegrateRefusesAnEmptyGate(t *testing.T) {
	// An empty gate would pass everything.
	// covers: R-SESS-105
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	common, _ := g.CommonDir(repo)
	for _, gate := range []string{"", "  \n\t"} {
		_, err := session.Integrate(g, merge.Integrator{}, session.IntegrateSpec{
			Repo: repo, Common: common,
			TmpName: "tmp-s-1", Gate: gate, Integration: session.Integration{Source: "session/s-1", Target: "main"},
		})
		if !errors.Is(err, session.ErrNoGate) {
			t.Fatalf("gate %q: %v", gate, err)
		}
	}
}

func TestIntegrateSweepsAKilledRunsThrowaway(t *testing.T) {
	// covers: R-SESS-093
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	if _, err := g.Out(repo, "checkout", "-q", "-b", "session/s-1"); err != nil {
		t.Fatal(err)
	}
	testkit.Commit(t, g, repo, "a.txt", "1\n", "one")
	if _, err := g.Out(repo, "checkout", "-q", "main"); err != nil {
		t.Fatal(err)
	}
	common, _ := g.CommonDir(repo)
	stale := filepath.Join(common, "ness", "tmp-s-1-deadbeef")
	if _, err := g.Out(repo, "worktree", "add", "-q", "--detach", stale, "main"); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(common, "ness", "tmp-s-1-other-cafef00d") // another run's, not ours
	if _, err := g.Out(repo, "worktree", "add", "-q", "--detach", sibling, "main"); err != nil {
		t.Fatal(err)
	}
	_, err := session.Integrate(g, merge.Integrator{}, session.IntegrateSpec{
		Repo: repo, Common: common, TmpName: "tmp-s-1",
		Gate: "true", Integration: session.Integration{Source: "session/s-1", Target: "main"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, serr := os.Stat(stale); !errors.Is(serr, os.ErrNotExist) {
		t.Fatal("leftover throwaway still on disk")
	}
	if _, serr := os.Stat(sibling); serr != nil {
		t.Fatalf("a sibling prefix's throwaway was swept: %v", serr)
	}
	if list, _ := g.Out(repo, "worktree", "list", "--porcelain"); strings.Contains(list, "deadbeef") {
		t.Fatalf("leftover worktree entry remains:\n%s", list)
	}
}

// publishing wraps the merge fold with a Publish step, as the pr integrator has.
type publishing struct {
	merge.Integrator
	published *int
}

// Integrated is false until a PR is merged, as for pr: ancestry alone must not decide it.
func (publishing) Integrated(session.Git, string, string, string) (bool, error) { return false, nil }

func (p publishing) Publish(session.Git, string, string, string) (string, error) {
	*p.published++
	return "https://example.invalid/pr/1", nil
}

func TestIntegratePublishesOnlyAfterTheGateAndLeavesTheTargetAlone(t *testing.T) {
	// covers: R-SESS-092, R-SESS-096
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	if _, err := g.Out(repo, "checkout", "-q", "-b", "session/p-1"); err != nil {
		t.Fatal(err)
	}
	testkit.Commit(t, g, repo, "a.txt", "1\n", "one")
	if _, err := g.Out(repo, "checkout", "-q", "main"); err != nil {
		t.Fatal(err)
	}
	common, _ := g.CommonDir(repo)
	before, _ := g.Rev(repo, "main")
	n := 0
	run := func(gate string) (session.Result, error) {
		return session.Integrate(g, publishing{published: &n}, session.IntegrateSpec{
			Repo: repo, Common: common, TmpName: "tmp-p-1", Gate: gate,
			Integration: session.Integration{Source: "session/p-1", Target: "main"},
		})
	}
	if _, err := run("false"); !errors.Is(err, session.ErrGate) || n != 0 {
		t.Fatalf("failing gate: err %v, published %d times", err, n)
	}
	res, err := run("true")
	if err != nil || n != 1 || res.Ref != "https://example.invalid/pr/1" || res.MovedRef || !res.Published {
		t.Fatalf("passing gate: %+v, %v, published %d times", res, err, n)
	}
	if after, _ := g.Rev(repo, "main"); after != before {
		t.Fatal("the pr integrator moved the local target")
	}
}

func TestIntegrateRefusesASourceAlreadyInTheTargetBeforeAnyPublish(t *testing.T) {
	// covers: R-SESS-092, R-SESS-102
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	if _, err := g.Out(repo, "branch", "session/q-1", "main"); err != nil {
		t.Fatal(err)
	}
	common, _ := g.CommonDir(repo)
	n := 0
	_, err := session.Integrate(g, publishing{published: &n}, session.IntegrateSpec{
		Repo: repo, Common: common, TmpName: "tmp-q-1", Gate: "true",
		Integration: session.Integration{Source: "session/q-1", Target: "main"},
	})
	if !errors.Is(err, session.ErrNothingToIntegrate) || n != 0 {
		t.Fatalf("err %v, published %d times", err, n)
	}
}

func TestIntegrateTargetCheckoutRefusesTrackedChangesButNotUntrackedFiles(t *testing.T) {
	// covers: R-SESS-104
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	testkit.Commit(t, g, repo, "README.md", "base\n", "base")
	if _, err := g.Out(repo, "checkout", "-q", "-b", "session/s-1"); err != nil {
		t.Fatal(err)
	}
	testkit.Commit(t, g, repo, "a.txt", "1\n", "one")
	if _, err := g.Out(repo, "checkout", "-q", "main"); err != nil {
		t.Fatal(err)
	}
	common, _ := g.CommonDir(repo)
	run := func() error {
		_, err := session.Integrate(g, merge.Integrator{}, session.IntegrateSpec{
			Repo: repo, Common: common, TmpName: "tmp-s-1",
			Gate: "true", Integration: session.Integration{Source: "session/s-1", Target: "main"},
		})
		return err
	}
	// A tracked file modified in the target's checkout refuses.
	readme := filepath.Join(repo, "README.md")
	if err := os.WriteFile(readme, []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(); err == nil || !strings.Contains(err.Error(), "uncommitted changes") {
		t.Fatalf("tracked change: %v", err)
	}
	if _, err := g.Out(repo, "checkout", "--", "README.md"); err != nil {
		t.Fatal(err)
	}
	// An untracked file (a manager's brief) does not.
	if err := os.WriteFile(filepath.Join(repo, "brief.md"), []byte("b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(); err != nil {
		t.Fatalf("untracked file refused: %v", err)
	}
}

// opaque folds to something that is not a commit and publishes nothing.
type opaque struct{ merge.Integrator }

func (opaque) Fold(session.Git, string, string, string) (string, error) { return "not-a-commit", nil }

func TestIntegrateResultIsNeitherPublishedNorMovedForANonCommitRef(t *testing.T) {
	// covers: R-SESS-099
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	if _, err := g.Out(repo, "checkout", "-q", "-b", "session/o-1"); err != nil {
		t.Fatal(err)
	}
	testkit.Commit(t, g, repo, "a.txt", "1\n", "one")
	if _, err := g.Out(repo, "checkout", "-q", "main"); err != nil {
		t.Fatal(err)
	}
	common, _ := g.CommonDir(repo)
	res, err := session.Integrate(g, opaque{}, session.IntegrateSpec{
		Repo: repo, Common: common, TmpName: "tmp-o-1", Gate: "true",
		Integration: session.Integration{Source: "session/o-1", Target: "main"},
	})
	if err != nil || res.Published || res.MovedRef || res.Ref != "not-a-commit" {
		t.Fatalf("%+v, %v", res, err)
	}
}

func TestDirtyFilesCountsSubmoduleChangesWhateverTheConfigIgnores(t *testing.T) {
	// covers: R-SESS-103
	g := session.Git{Exec: cli.SystemExec}
	sub := testkit.NewRepo(t, g)
	repo := testkit.NewRepo(t, g)
	for _, args := range [][]string{
		{"-c", "protocol.file.allow=always", "submodule", "add", "-q", sub, "sub"},
		{"commit", "-q", "-m", "add sub"},
		{"config", "diff.ignoreSubmodules", "all"},
	} {
		if _, err := g.Out(repo, args...); err != nil {
			t.Fatal(args, err)
		}
	}
	testkit.Commit(t, g, filepath.Join(repo, "sub"), "n.txt", "1\n", "new commit in the submodule")
	files, err := g.DirtyFiles(repo)
	if err != nil || len(files) != 1 || !strings.Contains(files[0], "sub") {
		t.Fatalf("DirtyFiles = %v, %v; want the submodule listed", files, err)
	}
}
