package session_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

// lessonCases are report bodies and whether the Lessons check lets them through (R-SESS-153).
var lessonCases = []struct {
	name, body string
	ok         bool
}{
	{"no lessons", "# Report\nwhat I did\n", false},
	{"deeper heading", "# Report\n### Lessons\nx\n", false},
	{"indented heading", "  ## Lessons\nx\n", true}, // CommonMark: up to three spaces of indent (R-CHK-005, one parser)
	{"code block indent", "    ## Lessons\nx\n", false},
	{"in prose", "I have lessons, see below\n", false},
	{"fenced heading", "```\n## Lessons\nx\n```\n", false},
	{"none", "## Lessons\nNone.\n", true},
	{"list", "# Report\n## Lessons\n- one\n- two\n", true},
	{"trailing blanks", "x\n## Lessons  \nNone.\n", true},
}

func openStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "ness.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.AddSession("s-1", "x", store.Operator, time.Now()); err != nil {
		t.Fatal(err)
	}
	return s
}

func wantLessons(t *testing.T, name string, ok bool, err error) {
	t.Helper()
	switch {
	case ok && err != nil:
		t.Errorf("%s: refused: %v", name, err)
	case !ok && (err == nil || !strings.Contains(err.Error(), "Lessons")):
		t.Errorf("%s: err = %v, want a refusal naming Lessons", name, err)
	}
}

// Rejects a close that checks only that report.md exists, with --discard too.
func TestCloseWantsALessonsHeadingInTheReport(t *testing.T) {
	// covers: R-SESS-153
	for _, c := range lessonCases {
		dir := t.TempDir()
		report := filepath.Join(dir, "wt", "docs/sessions/x", "report.md")
		if err := os.MkdirAll(filepath.Dir(report), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(report, []byte(c.body), 0o600); err != nil {
			t.Fatal(err)
		}
		spec := session.CloseSpec{Primary: dir, Discard: true, Sess: store.Session{
			ID: "s-1", Worktree: "wt", Record: "docs/sessions/x", Run: 2,
		}}
		wantLessons(t, c.name, c.ok, session.CheckClose(openStore(t), session.Git{}, spec))
	}
}

// Rejects a close that reads only the worktree: with the worktree deleted, the report on the
// session's branch is the one checked.
func TestCloseWantsALessonsHeadingInTheReportOnTheBranch(t *testing.T) {
	// covers: R-SESS-153, R-SESS-061
	g := session.Git{Exec: cli.SystemExec}
	for _, c := range lessonCases {
		repo := testkit.NewRepo(t, g)
		if _, err := g.Out(repo, "checkout", "-q", "-b", "session/s-1"); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(repo, "docs/sessions/x"), 0o700); err != nil {
			t.Fatal(err)
		}
		testkit.Commit(t, g, repo, "docs/sessions/x/report.md", c.body, "report")
		spec := session.CloseSpec{Primary: repo, Discard: true, Sess: store.Session{
			ID: "s-1", Worktree: "gone", Branch: "session/s-1", Record: "docs/sessions/x", Run: 2,
		}}
		wantLessons(t, c.name, c.ok, session.CheckClose(openStore(t), g, spec))
	}
}

func TestCheckReportNeverReadsAnUninspectableCheckoutAsGone(t *testing.T) {
	// covers: R-SESS-061
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "afile"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// stat of afile/wt is ENOTDIR, not "does not exist"; Git{} would panic if the branch were asked.
	spec := session.CloseSpec{Primary: dir, Discard: true, Sess: store.Session{
		ID: "s-1", Worktree: "afile/wt", Record: "docs/sessions/x", Run: 2,
	}}
	err := session.CheckClose(nil, session.Git{}, spec)
	if err == nil || !strings.Contains(err.Error(), "cannot inspect") {
		t.Fatalf("CheckClose = %v, want a cannot-inspect refusal", err)
	}
}

func TestNeverRan(t *testing.T) {
	// A session migrated from before first_ready_at (no mark, run bumped) did run (R-SESS-066).
	// covers: R-SESS-066
	for _, c := range []struct {
		name string
		sess store.Session
		want bool
	}{
		{"fresh", store.Session{Run: 1}, true},
		{"relaunched", store.Session{Run: 2}, false},
		{"adopted", store.Session{Run: 1, Adopted: true}, false},
	} {
		if got := (session.CloseSpec{Sess: c.sess}).NeverRan(); got != c.want {
			t.Errorf("%s: NeverRan = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestContainedIsAMergeThatChangesNothingInTheBase(t *testing.T) {
	// covers: R-SESS-167
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	branch := func(name, path, content string) {
		t.Helper()
		if _, err := g.Out(repo, "checkout", "-q", "-b", name, "main"); err != nil {
			t.Fatal(err)
		}
		if path != "" {
			testkit.Commit(t, g, repo, path, content, name)
		}
	}
	branch("ancestor", "", "")
	branch("folded", "a.txt", "a\n")
	branch("extra", "b.txt", "b\n")
	branch("conflict", "a.txt", "other\n")
	if _, err := g.Out(repo, "checkout", "-q", "main"); err != nil {
		t.Fatal(err)
	}
	testkit.Commit(t, g, repo, "a.txt", "a\n", "hand fold") // the same change, made by hand on main
	for name, want := range map[string]bool{"ancestor": true, "folded": true, "extra": false, "conflict": false} {
		spec := session.CloseSpec{Primary: repo, Sess: store.Session{Branch: name, Base: "main"}}
		if got, err := spec.Contained(g); err != nil || got != want {
			t.Errorf("%s: Contained = %v, %v; want %v", name, got, err, want)
		}
	}
}
