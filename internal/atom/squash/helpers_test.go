package squash_test

import (
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

// fixture is a fresh repo on main with git helpers bound to it.
type fixture struct {
	t    *testing.T
	g    session.Git
	repo string
}

func newFixture(t *testing.T, g session.Git) fixture {
	t.Helper()
	return fixture{t: t, g: g, repo: testkit.NewRepo(t, g)}
}

// git runs git in the repo and returns its output.
func (f fixture) git(args ...string) string {
	f.t.Helper()
	out, err := f.g.Out(f.repo, args...)
	if err != nil {
		f.t.Fatal(err)
	}
	return out
}

// commit adds one commit on the current branch.
func (f fixture) commit(file, content, msg string) string {
	f.t.Helper()
	return testkit.Commit(f.t, f.g, f.repo, file, content, msg)
}

// throwaway makes a detached worktree on main, as ness integrate does.
func (f fixture) throwaway() string {
	f.t.Helper()
	dir := f.t.TempDir() + "/tmp"
	f.git("worktree", "add", "-q", "--detach", dir, "main")
	return dir
}

// land makes main (checked out in the repo) point at ref, working tree included.
func (f fixture) land(ref string) {
	f.t.Helper()
	f.git("checkout", "-q", "main")
	f.git("reset", "-q", "--hard", ref)
}
