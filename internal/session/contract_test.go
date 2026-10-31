package session_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

const sha1 = "0123456789abcdef0123456789abcdef01234567"

func TestLockFormatRoundTrips(t *testing.T) {
	// One line per file, sorted by path: "<sha> <path>" or "amend <path>" (design §6: the lock
	// is a committed file in the session record, so a diff shows what was frozen or opened).
	// covers: R-CTR-001, R-CTR-005
	l := session.Lock{Dir: "docs/sessions/r", Files: map[string]string{"b/x.feature": session.Amended, "a.json": sha1}}
	got := string(l.Bytes())
	if got != sha1+" a.json\namend b/x.feature\n" {
		t.Fatalf("Bytes = %q", got)
	}
	parsed, err := session.ParseLock(l.Bytes())
	files := parsed.Files
	if err != nil || files["a.json"] != sha1 || files["b/x.feature"] != session.Amended {
		t.Fatalf("ParseLock = %v, %v", files, err)
	}
}

func TestParseLockRejectsMalformedLines(t *testing.T) {
	// Review Focus 2: a hand-edited lock with a line that is not "<sha> <path>" is an error that
	// names the line, never a silently unfrozen file.
	// covers: R-CTR-003
	for _, bad := range []string{"a.json\n", "zz a.json\n", sha1 + "\n", "amend\n", sha1 + " \n"} {
		if _, err := session.ParseLock([]byte(bad)); err == nil || !strings.Contains(err.Error(), "line 1") {
			t.Errorf("ParseLock(%q) = %v; want an error naming line 1", bad, err)
		}
	}
	if parsed, err := session.ParseLock([]byte("\n# comment\n" + sha1 + " a.json\n")); err != nil || len(parsed.Files) != 1 {
		t.Fatalf("blank and comment lines: %v, %v", parsed, err)
	}
}

// repoWithLocks is a repository with two session locks and one project-wide frozen path:
// r freezes a.json, c freezes b.txt and has c.txt open for amendment; p.txt is project-wide.
func repoWithLocks(t *testing.T) (session.Git, string) {
	t.Helper()
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	for _, f := range []string{"a.json", "b.txt", "c.txt", "p.txt"} {
		testkit.Commit(t, g, repo, f, f+"\n", "add "+f)
	}
	sha := func(p string) string {
		s, err := g.BlobSha(repo, p)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	for _, l := range []session.Lock{
		{Dir: "docs/sessions/r", Files: map[string]string{"a.json": sha("a.json")}},
		{Dir: "docs/sessions/r/c", Files: map[string]string{"b.txt": sha("b.txt"), "c.txt": session.Amended}},
	} {
		if err := session.WriteLock(repo, l); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := g.Out(repo, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Out(repo, "commit", "-q", "-m", "locks"); err != nil {
		t.Fatal(err)
	}
	return g, repo
}

func TestFindLocksAndFrozenFiles(t *testing.T) {
	// covers: R-CTR-003
	_, repo := repoWithLocks(t)
	locks, err := session.FindLocks(repo, "docs/sessions")
	if err != nil || len(locks) != 2 || locks[0].Dir != "docs/sessions/r" || locks[1].Dir != "docs/sessions/r/c" {
		t.Fatalf("FindLocks = %+v, %v", locks, err)
	}
	frozen := session.FrozenFiles(locks)
	if len(frozen) != 3 || frozen["c.txt"].Sha != session.Amended || frozen["b.txt"].LockDir != "docs/sessions/r/c" ||
		frozen["a.json"].LockDir != "docs/sessions/r" {
		t.Fatalf("FrozenFiles = %+v", frozen)
	}
	if err = os.WriteFile(filepath.Join(repo, "docs/sessions/r/contract.lock"), []byte("nope\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = session.FindLocks(repo, "docs/sessions"); err == nil || !strings.Contains(err.Error(), "docs/sessions/r/contract.lock") {
		t.Fatalf("a malformed lock: %v; want ErrBadLock naming it", err)
	}
}

func TestCheckContractsNamesEveryDifferingFile(t *testing.T) {
	// A frozen file whose blob differs from its recorded sha, or from base for a project-wide
	// path, is named; an amended file is not; a missing frozen file differs (R-CTR-003).
	// covers: R-CTR-003, R-CTR-004
	g, repo := repoWithLocks(t)
	locks, _ := session.FindLocks(repo, "docs/sessions")
	frozen := session.FrozenFiles(locks)
	frozen["p.txt"] = session.Frozen{Path: "p.txt"} // project-wide: compared with its blob at base
	diff, err := session.CheckContracts(g, repo, "main", frozen, config.Contracts{}, nil)
	if err != nil || len(diff) != 0 {
		t.Fatalf("clean tree: %v, %v", diff, err)
	}
	for _, f := range []string{"a.json", "c.txt", "p.txt"} { // c.txt is amended: allowed
		if werr := os.WriteFile(filepath.Join(repo, f), []byte("changed\n"), 0o600); werr != nil {
			t.Fatal(werr)
		}
	}
	if rerr := os.Remove(filepath.Join(repo, "b.txt")); rerr != nil {
		t.Fatal(rerr)
	}
	diff, err = session.CheckContracts(g, repo, "main", frozen, config.Contracts{}, nil)
	if err != nil || !slices.Equal(diff, []session.Diff{{Path: "a.json"}, {Path: "b.txt"}, {Path: "p.txt"}}) {
		t.Fatalf("CheckContracts = %v, %v; want a.json, b.txt, p.txt", diff, err)
	}
}

func TestGuardRefusesFrozenSpellingsAndAllowsTheRest(t *testing.T) {
	// The three spellings of one frozen file (relative, absolute, through a symlink) are all
	// refused naming the file and ness contract amend; an amended file, an unfrozen file and a
	// project-wide path behave as the lock says (R-CTR-009, R-CTR-011, R-CTR-012).
	// covers: R-CTR-009, R-CTR-011, R-CTR-012
	g, repo := repoWithLocks(t)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(repo, link); err != nil {
		t.Fatal(err)
	}
	gd := session.Guard{Git: g, Sessions: "docs/sessions", Project: []string{"p.txt"}, Warn: os.Stderr}
	for _, spelling := range []string{"a.json", filepath.Join(repo, "a.json"), filepath.Join(link, "a.json")} {
		reason := gd.Refuse(spelling, repo)
		if !strings.Contains(reason, "a.json is frozen") || !strings.Contains(reason, "ness contract amend a.json") {
			t.Errorf("Refuse(%q) = %q", spelling, reason)
		}
	}
	if r := gd.Refuse("p.txt", repo); !strings.Contains(r, "p.txt is frozen") || !strings.Contains(r, ".ness/ness.toml") {
		t.Errorf("project-wide: %q", r)
	}
	for _, open := range []string{"c.txt", "free.txt"} {
		if r := gd.Refuse(open, repo); r != "" {
			t.Errorf("Refuse(%q) = %q; want allowed", open, r)
		}
	}
}

func TestGuardRefusesEditsToTheLockItself(t *testing.T) {
	// An agent must not drop its own entry by editing contract.lock (preflight F25).
	// covers: R-CTR-009
	g, repo := repoWithLocks(t)
	gd := session.Guard{Git: g, Sessions: "docs/sessions", Warn: os.Stderr}
	if r := gd.Refuse("docs/sessions/r/c/contract.lock", repo); !strings.Contains(r, "ness contract") {
		t.Errorf("Refuse(lock) = %q", r)
	}
}

func TestGuardLockRuleOnlyAppliesUnderTheSessionsDir(t *testing.T) {
	// A contract.lock outside the sessions dir is an ordinary file.
	// covers: R-CTR-009
	g, repo := repoWithLocks(t)
	gd := session.Guard{Git: g, Sessions: "docs/sessions", Warn: os.Stderr}
	for p, want := range map[string]bool{"docs/sessions/r/contract.lock": true, "testdata/contract.lock": false} {
		if r := gd.Refuse(p, repo); (r != "") != want {
			t.Errorf("Refuse(%q) = %q; refused want %v", p, r, want)
		}
	}
}

func TestGuardAllowsPathsOutsideAnyRepoAndNewFiles(t *testing.T) {
	// Review Focus 1: a path outside every repository and a new file in a directory that does
	// not exist yet are allowed silently; the resolver never fails the tool call.
	// covers: R-CTR-011
	g, repo := repoWithLocks(t)
	gd := session.Guard{Git: g, Sessions: "docs/sessions", Warn: os.Stderr}
	outside := filepath.Join(t.TempDir(), "notes.md")
	for _, p := range []string{outside, "new/dir/file.go", filepath.Join(repo, "a", "b", "c.txt"), ""} {
		if r := gd.Refuse(p, repo); r != "" {
			t.Errorf("Refuse(%q) = %q; want allowed", p, r)
		}
	}
}

func TestGuardFailsOpenOnAnUnreadableLock(t *testing.T) {
	// Review Focus 2, the guard's half: a malformed lock is reported once on Warn and the edit
	// is allowed; contract check fails closed on it (TestFindLocksAndFrozenFiles).
	// covers: R-CTR-011
	g, repo := repoWithLocks(t)
	if err := os.WriteFile(filepath.Join(repo, "docs/sessions/r/contract.lock"), []byte("nope\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var warn strings.Builder
	gd := session.Guard{Git: g, Sessions: "docs/sessions", Warn: &warn}
	if r := gd.Refuse("a.json", repo); r != "" || !strings.Contains(warn.String(), "docs/sessions/r/contract.lock") {
		t.Fatalf("Refuse = %q, warn %q", r, warn.String())
	}
}

func TestFrozenFilesAnOpenAmendmentWinsOverASha(t *testing.T) {
	// A path frozen in one lock and amended in another is open: the amendment opens the path.
	// covers: R-CTR-005
	locks := []session.Lock{
		{Dir: "a", Files: map[string]string{"x": sha1}},
		{Dir: "b", Files: map[string]string{"x": session.Amended}},
	}
	if f := session.FrozenFiles(locks)["x"]; f.Sha != session.Amended || f.LockDir != "b" {
		t.Fatalf("FrozenFiles = %+v", f)
	}
}

func TestGuardResolvesSymlinksBeforeDotDot(t *testing.T) {
	// deep/inner/up points at z, so the OS opens deep/inner/up/../a.json as the frozen a.json at
	// the root; a lexical clean first would read it as deep/inner/a.json and let the edit through.
	// covers: R-CTR-009
	g, repo := repoWithLocks(t)
	for _, d := range []string{"z", filepath.Join("deep", "inner")} {
		if err := os.MkdirAll(filepath.Join(repo, d), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(repo, "z"), filepath.Join(repo, "deep", "inner", "up")); err != nil {
		t.Fatal(err)
	}
	gd := session.Guard{Git: g, Sessions: "docs/sessions", Project: []string{"p.txt"}, Warn: os.Stderr}
	spelling := "deep/inner/up/../a.json"
	if r := gd.Refuse(spelling, repo); !strings.Contains(r, "a.json is frozen") {
		t.Errorf("Refuse(%q) = %q; want the frozen a.json refused", spelling, r)
	}
}
