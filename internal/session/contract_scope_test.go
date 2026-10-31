package session_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

// fixture reads testdata/name.
func fixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// changedEntries counts the scenarios whose hash differs between two readings (a missing one counts).
func changedEntries(a, b map[string]string) int {
	n := 0
	for k, v := range a {
		if b[k] != v {
			n++
		}
	}
	return n
}

func TestFeatureScenariosHashesNameAndNormalisedSteps(t *testing.T) {
	// A scenario's identity is its name; its hash covers tags, steps, tables and Examples, and
	// the Feature's tags, the enclosing Rule's tags and every Background in scope. Layout and
	// comments are free.
	// covers: R-CTR-030
	feature := fixture(t, "pay.feature")
	base := session.FeatureScenarios(feature)
	if len(base) != 2 || base["pays"] == "" || base["refunds"] == "" {
		t.Fatalf("FeatureScenarios = %v; want pays, refunds", base)
	}
	reformatted := strings.NewReplacer("When they pay", "  When   they pay  \n    # a comment", "| 1 |", "|   1   |").Replace(feature)
	if got := session.FeatureScenarios(reformatted); changedEntries(base, got) != 0 {
		t.Errorf("whitespace and comments changed a hash: %v vs %v", got, base)
	}
	for _, c := range []struct {
		old, repl string
		want      int
	}{
		{"Then it works", "Then it fails", 1},
		{"| 2 |", "| 3 |", 1},
		{"@slow", "@skip", 1},
		{"@smoke", "@smoke @skip", 2},                                         // Feature tags reach every scenario
		{"Given a user", "Given nobody", 2},                                   // so does the Background
		{"Feature: pay", "Feature: pay\n  Background:\n    Given stubbed", 2}, // a Background added where none was
	} {
		got := session.FeatureScenarios(strings.Replace(feature, c.old, c.repl, 1))
		if n := changedEntries(base, got); n != c.want {
			t.Errorf("%q -> %q changed %d entries; want %d", c.old, c.repl, n, c.want)
		}
	}
}

func TestFeatureScenariosRuleTagsAndBackgroundReachTheirScenarios(t *testing.T) {
	// A tag above a Rule and a Background inside it show as edits of the scenarios they govern.
	// covers: R-CTR-030
	rule := fixture(t, "rule.feature")
	base := session.FeatureScenarios(rule)
	for name, text := range map[string]string{
		"rule tag":        strings.Replace(rule, "  Rule: r", "  @skip\n  Rule: r", 1),
		"rule background": strings.Replace(rule, "    Scenario: s", "    Background:\n      Given stubbed\n    Scenario: s", 1),
		"wrapping rule":   "Feature: f\n  Background:\n    Given stubbed\n  Rule: r\n    Scenario: s\n      Given a\n",
	} {
		if got := session.FeatureScenarios(text); changedEntries(base, got) != 1 {
			t.Errorf("%s: hash unchanged: %v vs %v", name, got, base)
		}
	}
}

func TestDocStringsAreOpaque(t *testing.T) {
	// A copy of a locked scenario hidden in a doc string, closed by a Rule line, must not stand in
	// for the real scenario; comment, tag and keyword lines and indentation inside a doc string
	// are scenario text.
	// covers: R-CTR-030
	locked := session.FeatureScenarios(fixture(t, "pay.feature"))
	got := session.FeatureScenarios(fixture(t, "smuggle.feature"))
	if len(got) != 2 || got["helper"] == "" || got["pays"] == locked["pays"] {
		t.Fatalf("smuggled copy stands in for the real scenario: %v", got)
	}
	doc := fixture(t, "docstring.feature")
	base := session.FeatureScenarios(doc)
	if len(base) != 1 {
		t.Fatalf("doc string lines became blocks: %v", base)
	}
	for old, repl := range map[string]string{
		"# Total: 5": "# Total: 9", "@x": "@y", "        indented": "      indented",
	} {
		if got = session.FeatureScenarios(strings.Replace(doc, old, repl, 1)); changedEntries(base, got) != 1 {
			t.Errorf("%q -> %q passed unnoticed", old, repl)
		}
	}
	fenced := strings.NewReplacer(`"""`, "```").Replace(doc)
	if got = session.FeatureScenarios(strings.Replace(fenced, "# Total: 5", "Scenario: x", 1)); len(got) != 1 {
		t.Errorf("a keyword in a fenced doc string opened a block: %v", got)
	}
}

func TestUnnamedScenariosGetStableNamesAndTheLockParses(t *testing.T) {
	// A Scenario with no name is "<unnamed N>"; the lock it makes parses back.
	// covers: R-CTR-030, R-CTR-031
	got := session.FeatureScenarios(fixture(t, "unnamed.feature"))
	if got["<unnamed 1>"] == "" || got["<unnamed 2>"] == "" || len(got) != 2 {
		t.Fatalf("FeatureScenarios = %v", got)
	}
	_, repo, l := scenarioRepo(t, fixture(t, "unnamed.feature"))
	if _, err := session.ParseLock(l.Bytes()); err != nil {
		t.Fatalf("the lock of an unnamed scenario does not parse: %v (%s)", err, repo)
	}
}

// scenarioRepo freezes text as a.feature in a fresh lock and returns the repository.
func scenarioRepo(t *testing.T, text string) (session.Git, string, session.Lock) {
	t.Helper()
	g, repo := repoWithLocks(t)
	testkit.Commit(t, g, repo, "a.feature", text, "feature")
	l := session.Lock{Dir: "docs/sessions/s", Files: map[string]string{}}
	if err := l.RecordScenarios(repo, "a.feature"); err != nil {
		t.Fatal(err)
	}
	if err := session.WriteLock(repo, l); err != nil {
		t.Fatal(err)
	}
	return g, repo, l
}

func diffs(t *testing.T, g session.Git, repo string) []session.Diff {
	t.Helper()
	locks, err := session.FindLocks(repo, "docs/sessions")
	if err != nil {
		t.Fatal(err)
	}
	d, err := session.CheckContracts(g, repo, "main", session.FrozenFiles(locks), config.Contracts{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var out []session.Diff
	for _, x := range d {
		if x.Path == "a.feature" || x.Path == "s.md" {
			out = append(out, x)
		}
	}
	return out
}

func TestScenarioLockToleratesAdditionsAndReordering(t *testing.T) {
	// Added scenarios and reordering are free; an edit or a removal of a locked scenario fails,
	// naming the scenario; a tag on the Feature fails every scenario it reaches.
	// covers: R-CTR-031, R-CTR-032
	feature := fixture(t, "pay.feature")
	g, repo, _ := scenarioRepo(t, feature)
	write := func(s string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, "a.feature"), []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if d := diffs(t, g, repo); len(d) != 0 {
		t.Fatalf("clean: %v", d)
	}
	write(feature + "\n  Scenario: new\n    Given more\n")
	if d := diffs(t, g, repo); len(d) != 0 {
		t.Fatalf("addition: %v", d)
	}
	head, rest, _ := strings.Cut(feature, "  Scenario: pays")
	pays, refunds, _ := strings.Cut(rest, "  @slow")
	write(head + "  @slow" + refunds + "\n  Scenario: pays" + pays)
	if d := diffs(t, g, repo); len(d) != 0 {
		t.Fatalf("reorder: %v", d)
	}
	write(strings.Replace(feature, "Then it works", "Then it passes", 1))
	if d := diffs(t, g, repo); len(d) != 1 || d[0].Path != "a.feature" || !strings.Contains(d[0].What, `"pays"`) {
		t.Fatalf("edit: %v; want a.feature naming pays", d)
	}
	write(strings.Replace(feature, "@smoke", "@smoke @skip", 1))
	if d := diffs(t, g, repo); len(d) != 1 || !strings.Contains(d[0].What, `"pays"`) || !strings.Contains(d[0].What, `"refunds"`) {
		t.Fatalf("feature tag: %v; want both scenarios named", d)
	}
	write(head + "  Scenario: pays" + pays)
	if d := diffs(t, g, repo); len(d) != 1 || !strings.Contains(d[0].What, `"refunds" removed`) {
		t.Fatalf("removal: %v; want refunds named", d)
	}
}

func TestLockRoundTripsScenariosAndRejectsTampering(t *testing.T) {
	// The scenario lines are bound to the file entry by a digest: an inconsistent hand edit, a
	// removed line or an orphan line is a malformed lock. (An edit that recomputes the digest is
	// caught by the reference-token comparison, R-CTR-021, not here.)
	// covers: R-CTR-031
	_, _, l := scenarioRepo(t, fixture(t, "pay.feature"))
	back, err := session.ParseLock(l.Bytes())
	if err != nil || len(back.Scenarios["a.feature"]) != 2 || back.Files["a.feature"] != l.Files["a.feature"] {
		t.Fatalf("ParseLock = %+v, %v", back, err)
	}
	var kept []string
	for _, ln := range strings.Split(string(l.Bytes()), "\n") {
		if !strings.HasPrefix(ln, "scenario ") || !strings.Contains(ln, "\trefunds") {
			kept = append(kept, ln)
		}
	}
	if _, err = session.ParseLock([]byte(strings.Join(kept, "\n"))); err == nil {
		t.Fatal("a lock with a scenario line removed parsed")
	}
	orphan := string(l.Bytes()) + "scenario " + strings.Repeat("a", 64) + " other.feature\tx\n"
	if _, err = session.ParseLock([]byte(orphan)); err == nil {
		t.Fatal("a scenario line with no entry parsed")
	}
}

func TestPrefixLockIsFormatterTolerantAndAppendOnly(t *testing.T) {
	// Wrapping, list markers, emphasis and trailing spaces are free; appended text passes;
	// an edit inside or before the locked text fails; an underscore inside a word counts.
	// covers: R-CTR-033, R-CTR-034
	spec := fixture(t, "spec.md")
	g, repo := repoWithLocks(t)
	testkit.Commit(t, g, repo, "s.md", spec, "spec")
	l := session.Lock{Dir: "docs/sessions/s", Files: map[string]string{}}
	if err := l.RecordPrefix(repo, "s.md"); err != nil {
		t.Fatal(err)
	}
	if err := session.WriteLock(repo, l); err != nil {
		t.Fatal(err)
	}
	for text, ok := range map[string]bool{
		spec: true,
		"# Spec\n\n* Hypothesis: X beats Y.   \n  \n1. Stop after\n   100 runs.\n": true,
		"# Spec\n\n- Hypothesis: __X__ beats *Y*.\n- Stop after 100 runs.\n":       true,
		spec + "\n## Amendment 2026-10-21\n\nMore.\n":                              true,
		strings.Replace(spec, "100", "200", 1):                                     false,
		strings.Replace(spec, "Stop", "St_op", 1):                                  false,
		"Preface\n" + spec: false,
		"# Spec\n":         false,
	} {
		if err := os.WriteFile(filepath.Join(repo, "s.md"), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		if d := diffs(t, g, repo); (len(d) == 0) != ok {
			t.Errorf("%q: diffs %v, want pass=%v", text, d, ok)
		}
	}
}

func TestPrefixLockRefusesAnEmptyFile(t *testing.T) {
	// An empty file would lock nothing, so any content would pass.
	// covers: R-CTR-033
	g, repo := repoWithLocks(t)
	testkit.Commit(t, g, repo, "e.md", "  \n", "empty")
	l := session.Lock{Dir: "docs/sessions/s"}
	if err := l.RecordPrefix(repo, "e.md"); err == nil {
		t.Fatal("RecordPrefix(empty) = nil; want a refusal")
	}
}
