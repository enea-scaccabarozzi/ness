package rebase_test

import (
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/rebase"
	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestRebaseContract(t *testing.T) {
	// covers: R-SESS-118, R-SESS-097, R-SESS-102
	g := session.Git{Exec: cli.SystemExec}
	testkit.IntegratorContract(t, rebase.Integrator{}, g)
}

func TestRebaseKeepsTheTargetLinearAndFoldsOnlyNewCommits(t *testing.T) {
	// The target gains one commit per source commit and no merge commit, also after it moved on;
	// a second fold after more session work replays only the new commit.
	// covers: R-SESS-118
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	must := func(dir string, args ...string) string {
		t.Helper()
		out, err := g.Out(dir, args...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	must(repo, "checkout", "-q", "-b", "session/s-1")
	testkit.Commit(t, g, repo, "a.txt", "1\n", "one")
	testkit.Commit(t, g, repo, "b.txt", "2\n", "two")
	must(repo, "checkout", "-q", "main")
	testkit.Commit(t, g, repo, "c.txt", "3\n", "main moved")
	fold := func(want string) {
		t.Helper()
		before := must(repo, "rev-parse", "main")
		tmp := t.TempDir() + "/tmp"
		must(repo, "worktree", "add", "-q", "--detach", tmp, "main")
		ref, err := rebase.Integrator{}.Fold(g, tmp, "session/s-1", "main")
		if err != nil {
			t.Fatal(err)
		}
		must(repo, "update-ref", "refs/heads/main", ref, before)
		if n := must(repo, "rev-list", "--count", before+"..main"); n != want {
			t.Fatalf("main advanced by %s commits, want %s", n, want)
		}
		if merges := must(repo, "rev-list", "--merges", "main"); merges != "" {
			t.Fatalf("merge commits on main: %s", merges)
		}
	}
	fold("2")
	if subj := must(repo, "log", "-2", "--format=%s", "main"); subj != "two\none" {
		t.Fatalf("replayed subjects = %q", subj)
	}
	must(repo, "checkout", "-q", "session/s-1")
	testkit.Commit(t, g, repo, "d.txt", "4\n", "three")
	must(repo, "checkout", "-q", "main")
	if ok, err := (rebase.Integrator{}).Integrated(g, repo, "session/s-1", "main"); err != nil || ok {
		t.Fatalf("Integrated with a new commit = %v, %v", ok, err)
	}
	fold("1")
	if ok, err := (rebase.Integrator{}).Integrated(g, repo, "session/s-1", "main"); err != nil || !ok {
		t.Fatalf("Integrated after the second fold = %v, %v", ok, err)
	}
}
