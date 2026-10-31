package check_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/check"
	"github.com/enea-scaccabarozzi/ness/internal/profile"
	"github.com/enea-scaccabarozzi/ness/internal/rule"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// realGit runs the real git in dir with the user's global and system config switched off, so only
// the repository's own .git/config shapes the output.
func realGit(t *testing.T) session.Exec {
	t.Helper()
	return func(dir, name string, args ...string) ([]byte, error) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		return cmd.Output()
	}
}

// Rejects a branch-forbids that reads the diff through the user's git config: with color.ui=always,
// diff.external set and a non-ASCII record path (quoted by core.quotePath), the added decision id
// must still be found (R-CHK-007: a kind that cannot decide fails, never passes).
func TestBranchForbidsIgnoresUserGitConfig(t *testing.T) {
	// covers: R-CHK-012, R-CHK-007
	dir := t.TempDir()
	g := session.Git{Exec: realGit(t)}
	must := func(args ...string) {
		t.Helper()
		if _, err := g.Out(dir, args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	must("init", "-q", "-b", "main")
	must("config", "user.email", "t@t")
	must("config", "user.name", "t")
	must("config", "color.ui", "always")
	must("config", "diff.external", "true") // prints nothing: an external diff hides the patch
	record := "docs/sessions/r/é"
	if err := os.MkdirAll(filepath.Join(dir, record), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, record, "decisions.md"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("notes\n")
	must("add", "-A")
	must("commit", "-q", "-m", "base")
	must("checkout", "-q", "-b", "session/fix-1")
	write("notes\n- **D7** an id was added\n")
	must("add", "-A")
	must("commit", "-q", "-m", "id")

	ctx := integrateCtx(dir)
	ctx.Into, ctx.Branch, ctx.Record = "main", "session/fix-1", record
	c := withEvent(kindCheck("branch-forbids", map[string]string{
		"path": `{record}/decisions\.md$`, "pattern": `(?m)^\s*[-*]\s*\*\*D\d+`,
	}), "integrate")
	got := check.Run(ctx, []profile.Check{c}, check.Deps{Git: g})
	wantFail(t, "an id added under user git config", got, "D7")
}

// Rejects a checkpoint kind that holds a child to a checkpoint it can never send (only a root
// sends one): a session that is not a root passes whatever landed.
func TestCheckpointSinceLandingPassesForANonRoot(t *testing.T) {
	// covers: R-CHK-014
	landed := fakeFacts{landing: base.Add(time.Hour), checkpoint: base}
	c := kindCheck("checkpoint-since-landing", nil)
	child := openCtx("/w")
	child.Session, child.Root = "s-child", "s-root"
	wantPass(t, "a child opening grandchildren", check.Run(child, []profile.Check{c}, check.Deps{Facts: landed}))
	root := openCtx("/w") // the same facts do fail a root
	wantFail(t, "a root", check.Run(root, []profile.Check{c}, check.Deps{Facts: landed}), "checkpoint")
}

// Rejects a checkpoint kind that asks the store about "every root" when the operator acts (Root is
// empty): it must pass without consulting the facts at all.
func TestCheckpointSinceLandingPassesForTheOperator(t *testing.T) {
	// covers: R-CHK-014
	facts := fakeFacts{landing: base.Add(time.Hour), err: errors.New("facts must not be read")}
	ctx := openCtx("/w")
	ctx.Session, ctx.Root = "", ""
	got := check.Run(ctx, []profile.Check{kindCheck("checkpoint-since-landing", nil)}, check.Deps{Facts: facts})
	wantPass(t, "an empty root", got)
}

// Rejects check kinds, rule.Headings and the Lessons check that read the same heading line
// differently. rule.Headings is the reference: 1-3 leading spaces are fine, 4 are code, trailing
// #s are part of the name, CRLF is trimmed.
func TestOneHeadingParser(t *testing.T) {
	// covers: R-CHK-011, R-RULE-001, R-SESS-153
	for _, tc := range []struct{ name, line, heading string }{
		{"plain", "## Lessons", "Lessons"},
		{"one space indent", " ## Lessons", "Lessons"},
		{"three space indent", "   ## Lessons", "Lessons"},
		{"four space indent is code", "    ## Lessons", "Lessons"},
		{"crlf", "## Lessons\r", "Lessons"},
		{"trailing hashes are not the name", "## Lessons ##", "Lessons"},
		{"trailing hashes are part of the name", "## Lessons ##", "Lessons ##"},
		{"hash in the name", "## C#", "C"},
		{"hash in the name, exact", "## C#", "C#"},
	} {
		text := tc.line + "\nbody\n"
		want := slices.Contains(rule.Headings([]byte(text)), tc.heading)

		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "f.md"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		c := kindCheck("section-matches", map[string]string{"file": "f.md", "heading": tc.heading}, `body`)
		ctx := openCtx(dir)
		got := len(check.Run(ctx, []profile.Check{c}, check.Deps{})) == 0
		if got != want {
			t.Errorf("%s: section-matches finds %q = %v, rule.Headings says %v", tc.name, tc.heading, got, want)
		}
		if tc.heading == "Lessons" {
			if has := store.HasLessons(text); has != want {
				t.Errorf("%s: HasLessons = %v, rule.Headings says %v", tc.name, has, want)
			}
		}
	}
}
