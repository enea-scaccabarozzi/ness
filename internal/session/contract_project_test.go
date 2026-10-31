package session_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

const projFeature = "Feature: pay\n  Scenario: pays\n    Given a card\n    Then it works\n"

// projectRepo commits a feature, a spec and a step file to main and returns the patterns and
// prefix globs a project would configure.
func projectRepo(t *testing.T) (g session.Git, repo string, patterns, prefix []string) {
	t.Helper()
	g = session.Git{Exec: cli.SystemExec}
	repo = testkit.NewRepo(t, g)
	for _, d := range []string{"features", "exp/one"} {
		if err := os.MkdirAll(filepath.Join(repo, d), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	testkit.Commit(t, g, repo, "features/pay.feature", projFeature, "feature")
	testkit.Commit(t, g, repo, "exp/one/spec.md", "# Spec\n\nOne two three.\n", "spec")
	testkit.Commit(t, g, repo, "steps.go", "package s\n", "steps")
	return g, repo, []string{"features/*.feature", "exp/*/spec.md", "steps.go", "none/*.feature"},
		[]string{"exp/*/spec.md"}
}

func write(t *testing.T, repo, path, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, path), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func projectDiff(t *testing.T, g session.Git, repo string, patterns, prefix []string) map[string]string {
	t.Helper()
	proj, err := session.ProjectFrozen(g, repo, "main", config.Contracts{Frozen: patterns, Prefix: prefix}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d, err := session.CheckContracts(g, repo, "main", proj, config.Contracts{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, x := range d {
		out[x.Path] = x.What
	}
	return out
}

func TestProjectFrozenGlobsFollowTheSessionLockModes(t *testing.T) {
	// A project-wide glob over .feature is scenario-locked against base, a matching .md is an
	// append-only prefix, anything else whole-file; a file new on the branch is not frozen (it is
	// ProjectAdded's).
	// covers: R-CTR-040, R-CTR-041, R-CTR-042
	g, repo, patterns, prefix := projectRepo(t)
	if d := projectDiff(t, g, repo, patterns, prefix); len(d) != 0 {
		t.Fatalf("clean tree: %v", d)
	}
	write(t, repo, "features/pay.feature", projFeature+"\n  Scenario: extra\n    Given more\n")
	write(t, repo, "features/new.feature", "Feature: n\n  Scenario: n\n    Given x\n") // not at base
	write(t, repo, "exp/one/spec.md", "# Spec\n\nOne two three.\n\n## Amendment\n\nFour.\n")
	if d := projectDiff(t, g, repo, patterns, prefix); len(d) != 0 {
		t.Fatalf("additions: %v; want none", d)
	}
	write(t, repo, "features/pay.feature", strings.Replace(projFeature, "works", "fails", 1))
	write(t, repo, "exp/one/spec.md", "# Spec\n\nOne 2 three.\n")
	write(t, repo, "steps.go", "package t\n")
	d := projectDiff(t, g, repo, patterns, prefix)
	if len(d) != 3 || !strings.Contains(d["features/pay.feature"], `scenario "pays" edited`) ||
		!strings.Contains(d["exp/one/spec.md"], "start of the file") || d["steps.go"] != "" {
		t.Fatalf("edits: %v; want pay.feature, spec.md and steps.go named", d)
	}
}

func TestProjectFrozenLiteralMissingAtBaseStillFails(t *testing.T) {
	// A literal [contracts] frozen path absent at base fails closed, whatever its mode.
	// covers: R-CTR-003
	g, repo, _, _ := projectRepo(t)
	for _, p := range []string{"gone.txt", "gone.feature"} {
		proj, err := session.ProjectFrozen(g, repo, "main", config.Contracts{Frozen: []string{p}}, nil)
		if err == nil {
			_, err = session.CheckContracts(g, repo, "main", proj, config.Contracts{}, nil)
		}
		if err == nil || !strings.Contains(err.Error(), p) {
			t.Errorf("%s: err = %v; want a failure naming it", p, err)
		}
	}
}

func TestGuardWarnsOnProjectScenarioAndPrefixLocksAndRefusesWholeFile(t *testing.T) {
	// The guard applies the session-lock rule to project-wide paths: soft for .feature and prefix
	// .md, refusal for the rest; a glob match absent at base is not frozen.
	// covers: R-CTR-043
	g, repo, patterns, prefix := projectRepo(t)
	var warn bytes.Buffer
	gd := session.Guard{Git: g, Sessions: "docs/sessions", Project: patterns, Prefix: prefix, Base: "main", Warn: &warn}
	for _, p := range []string{"features/pay.feature", "exp/one/spec.md"} {
		if r, soft := gd.Verdict(p, repo); r == "" || !soft || !strings.Contains(r, p) {
			t.Errorf("Verdict(%s) = %q, %v", p, r, soft)
		}
	}
	if r := gd.Refuse("steps.go", repo); !strings.Contains(r, "frozen project-wide") {
		t.Errorf("steps.go: %q", r)
	}
	if r, _ := gd.Verdict("features/new.feature", repo); r != "" {
		t.Errorf("new feature not at base: %q; want allowed", r)
	}
	if warn.Len() != 0 { // "" from an error also reads as allowed: no error was reported
		t.Errorf("guard reported %q", warn.String())
	}
}

func TestGuardDecidesALiteralLikeCheck(t *testing.T) {
	// A literal .feature or prefix .md is soft only when check can lock its text at base; absent
	// or empty there check fails closed, so the guard refuses rather than say "adding is fine".
	// covers: R-CTR-043
	g, repo, _, _ := projectRepo(t)
	testkit.Commit(t, g, repo, "exp/one/empty.md", "", "empty")
	gd := session.Guard{Git: g, Sessions: "docs/sessions", Base: "main", Warn: os.Stderr,
		Project: []string{"features/pay.feature", "gone.feature", "exp/one/empty.md"}, Prefix: []string{"exp/*/*.md"}}
	if r, soft := gd.Verdict("features/pay.feature", repo); r == "" || !soft {
		t.Errorf("literal at base: %q, %v; want soft", r, soft)
	}
	for _, p := range []string{"gone.feature", "exp/one/empty.md"} {
		if r, soft := gd.Verdict(p, repo); r == "" || soft {
			t.Errorf("%s: %q, %v; want a refusal", p, r, soft)
		}
	}
}

func TestProjectPathsAreCheckedBesideASessionLockEntry(t *testing.T) {
	// A session lock that holds the same path, even one open for amendment, neither hides the
	// project-wide check nor the guard's verdict.
	// covers: R-CTR-045
	g, repo, patterns, prefix := projectRepo(t)
	write(t, repo, "features/pay.feature", strings.Replace(projFeature, "works", "fails", 1))
	proj, err := session.ProjectFrozen(g, repo, "main", config.Contracts{Frozen: patterns, Prefix: prefix}, nil)
	if err != nil {
		t.Fatal(err)
	}
	locks := []session.Lock{{Dir: "docs/sessions/x", Files: map[string]string{"features/pay.feature": session.Amended}}}
	d, err := session.CheckContracts(g, repo, "main", proj, config.Contracts{}, nil)
	if err != nil || d[0].Path != "features/pay.feature" {
		t.Fatalf("project check = %v, %v; want pay.feature", d, err)
	}
	if got := session.FrozenFiles(locks)["features/pay.feature"]; got.Sha != session.Amended {
		t.Fatalf("lock entry = %v", got)
	}
	if err = os.MkdirAll(filepath.Join(repo, "docs/sessions/x"), 0o750); err != nil {
		t.Fatal(err)
	}
	write(t, repo, "docs/sessions/x/contract.lock", "amend features/pay.feature\n")
	gd := session.Guard{Git: g, Sessions: "docs/sessions", Project: patterns, Prefix: prefix, Base: "main", Warn: os.Stderr}
	if r, soft := gd.Verdict("features/pay.feature", repo); r == "" || !soft {
		t.Errorf("guard beside an amend entry: %q, %v; want soft", r, soft)
	}
}

func TestProjectFrozenCatchesADeletedOrRenamedFile(t *testing.T) {
	// covers: R-CTR-040
	g, repo, patterns, prefix := projectRepo(t)
	if err := os.Remove(filepath.Join(repo, "features/pay.feature")); err != nil {
		t.Fatal(err)
	}
	if d := projectDiff(t, g, repo, patterns, prefix); !strings.Contains(d["features/pay.feature"], "missing") {
		t.Errorf("deleted: %v; want pay.feature missing", d)
	}
	write(t, repo, "steps2.go", "package s\n")
	if err := os.Rename(filepath.Join(repo, "steps.go"), filepath.Join(repo, "moved.go")); err != nil {
		t.Fatal(err)
	}
	if d := projectDiff(t, g, repo, patterns, prefix); len(d) != 2 {
		t.Errorf("renamed out of the glob: %v; want pay.feature and steps.go", d)
	}
}

const legacyLock = `{"files":{"a.txt":"abc"}}` + "\n"

func legacyRepo(t *testing.T) (session.Git, string) {
	t.Helper()
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	if err := os.MkdirAll(filepath.Join(repo, "docs/sessions/old"), 0o750); err != nil {
		t.Fatal(err)
	}
	testkit.Commit(t, g, repo, "docs/sessions/old/contract.lock", legacyLock, "legacy json lock")
	return g, repo
}

func TestFindLocksTolerantSkipsALegacyLockUnchangedFromBase(t *testing.T) {
	// A malformed lock identical at base is skipped and named; one the branch modifies or adds
	// still fails closed.
	// covers: R-CTR-044
	g, repo := legacyRepo(t)
	locks, skipped, err := session.FindLocksTolerant(g, repo, "docs/sessions", "main")
	if err != nil || len(locks) != 0 || len(skipped) != 1 || skipped[0] != "docs/sessions/old/contract.lock" {
		t.Fatalf("unchanged: %v, %v, %v", locks, skipped, err)
	}
	write(t, repo, "docs/sessions/old/contract.lock", legacyLock+"more\n")
	if _, _, err = session.FindLocksTolerant(g, repo, "docs/sessions", "main"); err == nil {
		t.Error("modified legacy lock: want an error")
	}
	write(t, repo, "docs/sessions/old/contract.lock", legacyLock)
	if err = os.MkdirAll(filepath.Join(repo, "docs/sessions/new"), 0o750); err != nil {
		t.Fatal(err)
	}
	write(t, repo, "docs/sessions/new/contract.lock", legacyLock)
	if _, _, err = session.FindLocksTolerant(g, repo, "docs/sessions", "main"); err == nil ||
		!strings.Contains(err.Error(), "docs/sessions/new/contract.lock") {
		t.Errorf("added legacy lock: %v; want an error naming it", err)
	}
}

func TestHistoricalFrozenSkipsAMalformedLockAtTheMergeBase(t *testing.T) {
	// The history walk reads the merge-base's locks too; a legacy one there is no failure, and
	// removing it on the branch is none either.
	// covers: R-CTR-044
	g, repo := legacyRepo(t)
	mb, err := session.MergeBase(g, repo, "main")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = g.Out(repo, "rm", "-q", "docs/sessions/old/contract.lock"); err != nil {
		t.Fatal(err)
	}
	if _, err = g.Out(repo, "-c", "user.email=t@x", "-c", "user.name=t", "commit", "-q", "-m", "drop"); err != nil {
		t.Fatal(err)
	}
	hist, err := session.HistoricalFrozen(g, repo, "docs/sessions", mb, nil)
	if err != nil || len(hist) != 0 {
		t.Fatalf("HistoricalFrozen = %v, %v", hist, err)
	}
}

func TestHistoricalFrozenSkipsAMalformedLockInBranchHistory(t *testing.T) {
	// A branch committed a legacy JSON lock; base never had it. The history walk reads the
	// branch's commits too: a lock that does not parse holds no paths and is no failure.
	// covers: R-CTR-044
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	if _, err := g.Out(repo, "checkout", "-q", "-b", "feat"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "docs/sessions/old"), 0o750); err != nil {
		t.Fatal(err)
	}
	testkit.Commit(t, g, repo, "docs/sessions/old/contract.lock", legacyLock, "branch adds a legacy lock")
	mb, err := session.MergeBase(g, repo, "main")
	if err != nil {
		t.Fatal(err)
	}
	hist, err := session.HistoricalFrozen(g, repo, "docs/sessions", mb, nil)
	if err != nil || len(hist) != 0 {
		t.Fatalf("HistoricalFrozen = %v, %v", hist, err)
	}
}

func TestProjectAddedAndAmendedOpenAProjectWidePath(t *testing.T) {
	// A worktree file new under a frozen glob is added; a literal or a file outside every glob is
	// not. A "(project-wide)" amend line new in a decisions.md under sessions opens the path, for
	// the guard too.
	// covers: R-CTR-055, R-CTR-056
	g, repo, patterns, prefix := projectRepo(t)
	write(t, repo, "features/new.feature", "Feature: n\n")
	write(t, repo, "other.go", "package s\n")
	added, err := session.ProjectAdded(g, repo, "main", patterns)
	if err != nil || len(added) != 1 || added["features/new.feature"] != "features/*.feature" {
		t.Fatalf("ProjectAdded = %v, %v; want features/new.feature under its glob", added, err)
	}
	gd := session.Guard{Git: g, Sessions: "docs/sessions", Project: patterns, Prefix: prefix, Base: "main", Warn: os.Stderr}
	if r := gd.Refuse("steps.go", repo); r == "" {
		t.Fatal("steps.go: allowed before the amend")
	}
	if err = os.MkdirAll(filepath.Join(repo, "docs/sessions/r"), 0o750); err != nil {
		t.Fatal(err)
	}
	write(t, repo, "docs/sessions/r/decisions.md", "Amend steps.go: a lock amend\n")
	if ok, aerr := session.ProjectAmended(g, repo, "docs/sessions", "main", "steps.go"); ok || aerr != nil {
		t.Fatalf("lock amend line: %v, %v; want not project-wide", ok, aerr)
	}
	write(t, repo, "docs/sessions/r/decisions.md", `Amend steps.go (project-wide): why [operator: "yes"] (2026-10-29)`+"\n")
	if ok, aerr := session.ProjectAmended(g, repo, "docs/sessions", "main", "steps.go"); !ok || aerr != nil {
		t.Fatalf("ProjectAmended = %v, %v; want true", ok, aerr)
	}
	if r := gd.Refuse("steps.go", repo); r != "" {
		t.Errorf("steps.go after the amend: %q; want allowed", r)
	}
}
