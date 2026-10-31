package merge_test

import (
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/merge"
	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestMergeContract(t *testing.T) {
	// covers: R-SESS-094, R-SESS-097, R-SESS-102, R-QA-024
	g := session.Git{Exec: cli.SystemExec}
	testkit.IntegratorContract(t, merge.Integrator{}, g)
}

func TestMergePreservesSourceCommits(t *testing.T) {
	// Every source commit is an ancestor of the target after the fold, through a merge commit.
	// covers: R-SESS-094
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	must := func(args ...string) string {
		t.Helper()
		out, err := g.Out(repo, args...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	must("checkout", "-q", "-b", "session/s-1")
	c1 := testkit.Commit(t, g, repo, "a.txt", "1\n", "one")
	c2 := testkit.Commit(t, g, repo, "b.txt", "2\n", "two")
	must("checkout", "-q", "main")
	testkit.Commit(t, g, repo, "c.txt", "3\n", "main moved") // Review Focus 3
	tmp := t.TempDir() + "/tmp"
	must("worktree", "add", "-q", "--detach", tmp, "main")
	ref, err := merge.Integrator{}.Fold(g, tmp, "session/s-1", "main")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{c1, c2} {
		if _, err := g.Out(repo, "merge-base", "--is-ancestor", c, ref); err != nil {
			t.Fatalf("%s is not an ancestor of %s", c, ref)
		}
	}
	if parents := must("rev-list", "--parents", "-n", "1", ref); len(strings.Fields(parents)) != 3 {
		t.Fatalf("not a merge commit: %q", parents)
	}
}

func TestMergeMakesAMergeCommitEvenWhenTheTargetDidNotMove(t *testing.T) {
	// A fast-forward would lose the "integrated" marker in the history.
	// covers: R-SESS-094
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	for _, args := range [][]string{{"checkout", "-q", "-b", "session/s-1"}} {
		if _, err := g.Out(repo, args...); err != nil {
			t.Fatal(err)
		}
	}
	testkit.Commit(t, g, repo, "a.txt", "1\n", "one")
	if _, err := g.Out(repo, "checkout", "-q", "main"); err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir() + "/tmp"
	if _, err := g.Out(repo, "worktree", "add", "-q", "--detach", tmp, "main"); err != nil {
		t.Fatal(err)
	}
	ref, err := merge.Integrator{}.Fold(g, tmp, "session/s-1", "main")
	if err != nil {
		t.Fatal(err)
	}
	parents, err := g.Out(repo, "rev-list", "--parents", "-n", "1", ref)
	if err != nil || len(strings.Fields(parents)) != 3 {
		t.Fatalf("not a merge commit: %q, %v", parents, err)
	}
}
