package pr_test

import (
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/pr"
	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestFakeForgeContract(t *testing.T) {
	// covers: R-SESS-096, R-QA-024
	f := testkit.FakeForge{Dir: t.TempDir()}
	if err := testkit.WriteFakeDefaults(f.Dir); err != nil {
		t.Fatal(err)
	}
	testkit.ForgeContract(t, f, func(url string) {
		if err := f.Merge(url); err != nil {
			t.Fatal(err)
		}
	})
}

func TestGHForgeOverTheTable(t *testing.T) {
	// gh pr create prints the url; gh pr view reports the state.
	// covers: R-SESS-096
	dir, repo, g := ghFixture(t)
	const url = "https://github.com/o/r/pull/7"
	listRows(t, dir, `[{"url":"https://github.com/o/r/pull/3","state":"CLOSED","headRefOid":"x"}]`)
	set(t, dir, "gh pr create --head session/x-1 --base main --title integrate session/x-1 into main"+
		" --body opened by ness integrate", url)
	set(t, dir, "gh pr view "+url+" --json state", `{"state":"OPEN"}`)
	got, err := g.OpenPR(repo, "session/x-1", "main", "integrate session/x-1 into main")
	if err != nil || got != url {
		t.Fatalf("OpenPR = %q, %v (a closed PR must not be reused)", got, err)
	}
	if ok, merr := g.Merged(repo, url); merr != nil || ok {
		t.Fatalf("Merged open = %v, %v", ok, merr)
	}
	set(t, dir, "gh pr view "+url+" --json state", `{"state":"MERGED"}`)
	if ok, merr := g.Merged(repo, url); merr != nil || !ok {
		t.Fatalf("Merged after = %v, %v", ok, merr)
	}
}

func TestGHFindPRChoosesByStateAndTip(t *testing.T) {
	// An open PR wins in either row order; a merged PR counts only at the source's current tip, so
	// new commits after a merge open a new PR; at the tip the pair is integrated.
	// covers: R-SESS-096, R-SESS-102
	dir, repo, g := ghFixture(t)
	tip, err := g.Exec(repo, "git", "rev-parse", "session/x-1")
	if err != nil {
		t.Fatal(err)
	}
	cur := strings.TrimSpace(string(tip))
	row := func(url, state, oid string) string {
		return `{"url":"` + url + `","state":"` + state + `","headRefOid":"` + oid + `"}`
	}
	merged, open := row("m", "MERGED", cur), row("o", "OPEN", cur)
	for name, c := range map[string]struct{ rows, want string }{
		"merged then open":    {merged + "," + open, "o"},
		"open then merged":    {open + "," + merged, "o"},
		"merged at old tip":   {row("m", "MERGED", "old"), ""},
		"merged at the tip":   {merged, "m"},
		"closed at the tip":   {row("c", "CLOSED", cur), ""},
		"old merged, current": {row("m", "MERGED", "old") + "," + merged, "m"},
	} {
		listRows(t, dir, "["+c.rows+"]")
		if got, ferr := g.FindPR(repo, "session/x-1", "main"); ferr != nil || got != c.want {
			t.Errorf("%s: FindPR = %q, %v; want %q", name, got, ferr, c.want)
		}
	}
	// Integrated follows: merged at the tip is integrated, merged at an old tip is not.
	it := pr.Integrator{Forge: g}
	set(t, dir, "gh pr view m --json state", `{"state":"MERGED"}`)
	listRows(t, dir, "["+row("m", "MERGED", "old")+"]")
	if ok, ierr := it.Integrated(session.Git{Exec: cli.SystemExec}, repo, "session/x-1", "main"); ierr != nil || ok {
		t.Fatalf("Integrated at an old merged tip = %v, %v", ok, ierr)
	}
	listRows(t, dir, "["+merged+"]")
	if ok, ierr := it.Integrated(session.Git{Exec: cli.SystemExec}, repo, "session/x-1", "main"); ierr != nil || !ok {
		t.Fatalf("Integrated at the merged tip = %v, %v", ok, ierr)
	}
}

// ghFixture is a real repo with branch session/x-1 and a fake gh table.
func ghFixture(t *testing.T) (dir, repo string, g pr.GH) {
	t.Helper()
	dir = t.TempDir()
	if err := testkit.WriteFakeDefaults(dir); err != nil {
		t.Fatal(err)
	}
	sg := session.Git{Exec: cli.SystemExec}
	repo = testkit.NewRepo(t, sg)
	if _, err := sg.Out(repo, "checkout", "-q", "-b", "session/x-1"); err != nil {
		t.Fatal(err)
	}
	testkit.Commit(t, sg, repo, "a.txt", "1\n", "one")
	if _, err := sg.Out(repo, "checkout", "-q", "main"); err != nil {
		t.Fatal(err)
	}
	return dir, repo, pr.GH{Exec: testkit.FakeExec{Dir: dir}.Run}
}

func set(t *testing.T, dir, k, v string) {
	t.Helper()
	if err := testkit.SetFake(dir, "output", k, v+"\n"); err != nil {
		t.Fatal(err)
	}
}

func listRows(t *testing.T, dir, json string) {
	t.Helper()
	set(t, dir, "gh pr list --head session/x-1 --base main --state all --json url,state,headRefOid", json)
}

func TestPRIntegratorPreviewsLocallyAndPublishesAfterTheGate(t *testing.T) {
	// Fold is a local merge preview: it pushes nothing and opens nothing. Publish, which the
	// caller runs only after the gate, pushes the source and opens the PR; the ref is its url;
	// integrated? follows the forge.
	// covers: R-SESS-092, R-SESS-096, R-SESS-102
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	remote := t.TempDir()
	must := func(dir string, args ...string) string {
		t.Helper()
		out, err := g.Out(dir, args...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	must(remote, "init", "-q", "--bare")
	must(repo, "remote", "add", "origin", remote)
	must(repo, "push", "-q", "origin", "main")
	must(repo, "checkout", "-q", "-b", "session/x-1")
	testkit.Commit(t, g, repo, "a.txt", "1\n", "one")
	must(repo, "checkout", "-q", "main")
	f := testkit.FakeForge{Dir: t.TempDir()}
	if err := testkit.WriteFakeDefaults(f.Dir); err != nil {
		t.Fatal(err)
	}
	it := pr.Integrator{Forge: f}
	if ok, _ := it.Integrated(g, repo, "session/x-1", "main"); ok {
		t.Fatal("integrated before any PR")
	}
	tmp := t.TempDir() + "/tmp"
	must(repo, "worktree", "add", "-q", "--detach", tmp, "main")
	if _, err := it.Fold(g, tmp, "session/x-1", "main"); err != nil {
		t.Fatal(err)
	}
	if out := must(tmp, "show", "HEAD:a.txt"); out != "1" {
		t.Fatalf("preview lacks the fold: %q", out)
	}
	if out := must(remote, "branch", "--list", "session/x-1"); out != "" {
		t.Fatalf("Fold pushed the source: %q", out)
	}
	if url, _ := f.FindPR(repo, "session/x-1", "main"); url != "" {
		t.Fatalf("Fold opened a PR: %q", url)
	}
	ref, err := it.Publish(g, repo, "session/x-1", "main")
	if err != nil || !strings.HasPrefix(ref, "https://") {
		t.Fatalf("Publish = %q, %v", ref, err)
	}
	if must(remote, "rev-parse", "session/x-1") != must(repo, "rev-parse", "session/x-1") {
		t.Fatal("source not pushed")
	}
	if again, aerr := it.Publish(g, repo, "session/x-1", "main"); aerr != nil || again != ref {
		t.Fatalf("second Publish = %q, %v; want %q", again, aerr, ref)
	}
	if ok, _ := it.Integrated(g, repo, "session/x-1", "main"); ok {
		t.Fatal("integrated while the PR is open")
	}
	if err := f.Merge(ref); err != nil {
		t.Fatal(err)
	}
	if ok, err := it.Integrated(g, repo, "session/x-1", "main"); err != nil || !ok {
		t.Fatalf("Integrated after merge = %v, %v", ok, err)
	}
}
