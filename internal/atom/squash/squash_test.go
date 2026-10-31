package squash_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/squash"
	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestSquashContract(t *testing.T) {
	// covers: R-SESS-095, R-SESS-097, R-SESS-102, R-QA-024
	g := session.Git{Exec: cli.SystemExec}
	testkit.IntegratorContract(t, squash.Integrator{}, g)
}

func TestSquashIsOneCommitWithTheTrailer(t *testing.T) {
	// The target advances by exactly one commit carrying the combined diff and the trailer that
	// answers integrated? later, also when the target moved on (Review Focus 3).
	// covers: R-SESS-095, R-SESS-102
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
	testkit.Commit(t, g, repo, "a.txt", "1\n", "one")
	src := testkit.Commit(t, g, repo, "b.txt", "2\n", "two")
	must("checkout", "-q", "main")
	testkit.Commit(t, g, repo, "c.txt", "3\n", "main moved")
	before := must("rev-parse", "main")
	tmp := t.TempDir() + "/tmp"
	must("worktree", "add", "-q", "--detach", tmp, "main")
	ref, err := squash.Integrator{}.Fold(g, tmp, "session/s-1", "main")
	if err != nil {
		t.Fatal(err)
	}
	if n := must("rev-list", "--count", before+".."+ref); n != "1" {
		t.Fatalf("target advanced by %s commits", n)
	}
	if body := must("log", "-1", "--format=%B", ref); !strings.Contains(body, squash.Trailer+": "+src) ||
		!strings.HasPrefix(body, "integrate session/s-1 into main") {
		t.Fatalf("message %q", body)
	}
	if files := must("diff", "--name-only", before, ref); files != "a.txt\nb.txt" {
		t.Fatalf("diff %q", files)
	}
	must("update-ref", "refs/heads/main", ref, before)
	if ok, err := (squash.Integrator{}).Integrated(g, repo, "session/s-1", "main"); err != nil || !ok {
		t.Fatalf("Integrated = %v, %v", ok, err)
	}
}

func TestBodyListsFoldedSubjects(t *testing.T) {
	// A body that lists merge commits, or reverses the order, or lacks the markers, fails.
	// covers: R-SESS-141
	g := session.Git{Exec: cli.SystemExec}
	f := newFixture(t, g)
	f.git("checkout", "-q", "-b", "session/s-1")
	f.commit("a.txt", "1\n", "feat: first thing")
	f.git("checkout", "-q", "main")
	f.commit("m.txt", "m\n", "main moved")
	f.git("checkout", "-q", "session/s-1")
	f.git("merge", "-q", "--no-ff", "-m", "Merge main into the session", "main")
	src := f.commit("b.txt", "2\n", "fix: second thing")
	f.git("checkout", "-q", "main")
	ref, err := squash.Integrator{Subject: "feat(x): y"}.Fold(g, f.throwaway(), "session/s-1", "main")
	if err != nil {
		t.Fatal(err)
	}
	want := "feat(x): y\n\nBEGIN_NESTED_COMMIT\nfeat: first thing\nEND_NESTED_COMMIT\n" +
		"BEGIN_NESTED_COMMIT\nfix: second thing\nEND_NESTED_COMMIT\n\n" + squash.Trailer + ": " + src
	if body := f.git("log", "-1", "--format=%B", ref); body != want {
		t.Fatalf("body %q\nwant %q", body, want)
	}
}

func TestRefoldTakesOnlyWhatTheSourceGained(t *testing.T) {
	// Merge base = the tree git finds (source's old tip vs main's edit) would conflict; the
	// trailer's sha as base must not.
	// covers: R-SESS-142
	g := session.Git{Exec: cli.SystemExec}
	f := newFixture(t, g)
	it := squash.Integrator{Subject: "feat(x): y"}
	f.git("checkout", "-q", "-b", "session/s-1")
	f.commit("a.txt", "l1\nl2\n", "feat: landed")
	f.git("checkout", "-q", "main")
	ref, err := it.Fold(g, f.throwaway(), "session/s-1", "main")
	if err != nil {
		t.Fatal(err)
	}
	f.land(ref)
	f.commit("a.txt", "main\nl2\n", "main edits a landed line")
	f.git("checkout", "-q", "session/s-1")
	f.commit("c.txt", "c\n", "feat: gained later")
	f.git("checkout", "-q", "main")
	if ok, err := it.Integrated(g, f.repo, "session/s-1", "main"); err != nil || ok {
		t.Fatalf("Integrated before the second fold = %v, %v", ok, err)
	}
	ref, err = it.Fold(g, f.throwaway(), "session/s-1", "main")
	if err != nil {
		t.Fatalf("second fold: %v", err)
	}
	if a := f.git("show", ref+":a.txt"); a != "main\nl2" {
		t.Fatalf("a.txt = %q", a)
	}
	if c := f.git("show", ref+":c.txt"); c != "c" {
		t.Fatalf("c.txt = %q", c)
	}
	body := f.git("log", "-1", "--format=%B", ref)
	if !strings.Contains(body, "\nfeat: gained later\n") || strings.Contains(body, "feat: landed") {
		t.Fatalf("body %q", body)
	}
	f.land(ref)
	if ok, err := it.Integrated(g, f.repo, "session/s-1", "main"); err != nil || !ok {
		t.Fatalf("Integrated after = %v, %v", ok, err)
	}
}

func TestEmptySquashIsNothingToIntegrate(t *testing.T) {
	// A fold that commits an empty change, or fails with a bare git error, fails here.
	// covers: R-SESS-143
	g := session.Git{Exec: cli.SystemExec}
	f := newFixture(t, g)
	it := squash.Integrator{Subject: "feat(x): y"}
	f.git("checkout", "-q", "-b", "session/s-1")
	f.commit("a.txt", "1\n", "feat: landed")
	f.git("checkout", "-q", "main")
	ref, err := it.Fold(g, f.throwaway(), "session/s-1", "main")
	if err != nil {
		t.Fatal(err)
	}
	f.land(ref)
	f.git("checkout", "-q", "session/s-1")
	f.git("merge", "-q", "--no-ff", "-m", "Merge main", "main")
	f.git("checkout", "-q", "main")
	before := f.git("rev-parse", "main")
	if _, err = it.Fold(g, f.throwaway(), "session/s-1", "main"); !errors.Is(err, session.ErrNothingToIntegrate) {
		t.Fatalf("err = %v", err)
	}
	if after := f.git("rev-parse", "main"); after != before {
		t.Fatalf("main moved %s -> %s", before, after)
	}
}
