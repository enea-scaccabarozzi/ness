package session_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

const fmtText = "unit a: one\nunit b: two\n"

// fmtContracts is a project with one format, "tests", over *.fmt files, run by the fake
// extractor testdata/units.sh through its absolute path.
func fmtContracts(t *testing.T) config.Contracts {
	t.Helper()
	abs, err := filepath.Abs("testdata/units.sh")
	if err != nil {
		t.Fatal(err)
	}
	return config.Contracts{
		Formats:     map[string]config.Format{"tests": {Match: []string{"*.fmt"}, Units: "sh " + abs}},
		FormatOrder: []string{"tests"},
	}
}

func sumOf(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// extractor is the export-backed runner of a repository whose base is main.
func extractor(t *testing.T, g session.Git, repo string) *session.Extractor {
	t.Helper()
	x := session.NewExtractor(g, repo, "main")
	t.Cleanup(func() {
		if err := x.Close(); err != nil {
			t.Error(err)
		}
	})
	return x
}

func TestExtractUnitsSpeaksTheJSONLineProtocol(t *testing.T) {
	// The command gets the text on stdin and the path as its last argument, runs in the given
	// directory, and prints one {"id","body"} object per line; a failing command, invalid or loose
	// JSON, a repeated or oversized id, too much output or a command past its timeout fail closed.
	// covers: R-CTR-047, R-CTR-048
	f := fmtContracts(t).Formats["tests"]
	dir := t.TempDir()
	long := strings.Repeat("x", 1025)
	for _, tc := range []struct{ text, wantErr string }{
		{"#fail", "boom"},
		{"#junk", "invalid"},
		{"#dup", `repeats id "a"`},
		{"#loose", "invalid"},
		{"unit " + long + ": big", "id"},
	} {
		if _, err := session.ExtractUnits(context.Background(), dir, f, "x.fmt", tc.text); err == nil ||
			!strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), "x.fmt") {
			t.Errorf("%q: err = %v; want %q naming x.fmt", tc.text, err, tc.wantErr)
		}
	}
	got, err := session.ExtractUnits(context.Background(), dir, f, "dir/with space.fmt", "#path")
	if err != nil || got["path"] != sumOf("dir/with space.fmt") {
		t.Fatalf("path argument: %v, %v", got, err)
	}
	if got, err = session.ExtractUnits(context.Background(), dir, f, "x.fmt", fmtText); err != nil ||
		len(got) != 2 || got["a"] != sumOf("one") {
		t.Fatalf("units = %v, %v", got, err)
	}
	_, err = session.ExtractUnits(context.Background(), dir, config.Format{Units: "head -c 9000000 /dev/zero #"}, "x.fmt", "")
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("output past the cap: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = session.ExtractUnits(ctx, dir, config.Format{Units: "sleep 5 #"}, "x.fmt", "")
	if err == nil || time.Since(start) > 3*time.Second {
		t.Fatalf("a command past its timeout: %v after %v", err, time.Since(start))
	}
}

func TestFormatLockRoundTripsAndRejectsTampering(t *testing.T) {
	// A format lock keeps its units as lines bound to the entry by a digest, like a scenario lock.
	// covers: R-CTR-049
	g, repo, l, c := fmtRepo(t)
	if !strings.HasPrefix(l.Files["a.fmt"], session.FormatTag+"tests:") || !session.Lenient(l.Files["a.fmt"]) {
		t.Fatalf("token = %q", l.Files["a.fmt"])
	}
	back, err := session.ParseLock(l.Bytes())
	if err != nil || back.Files["a.fmt"] != l.Files["a.fmt"] || len(back.Scenarios["a.fmt"]) != 2 {
		t.Fatalf("round trip: %+v, %v", back, err)
	}
	tampered := bytes.Replace(l.Bytes(), []byte("\tb\n"), []byte("\tz\n"), 1)
	if _, err = session.ParseLock(tampered); err == nil {
		t.Error("a renamed unit line parsed")
	}
	write(t, repo, "empty.fmt", "nothing\n")
	if err = l.RecordUnits(extractor(t, g, repo), repo, "empty.fmt", "tests", c.Formats["tests"]); err == nil ||
		!strings.Contains(err.Error(), "no units") {
		t.Errorf("a file with no units froze: %v", err)
	}
}

// fmtRepo commits a.fmt and freezes it in a session lock under the "tests" format.
func fmtRepo(t *testing.T) (session.Git, string, session.Lock, config.Contracts) {
	t.Helper()
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	testkit.Commit(t, g, repo, "a.fmt", fmtText, "fmt")
	c := fmtContracts(t)
	l := session.Lock{Dir: "docs/sessions/s", Files: map[string]string{}}
	if err := l.RecordUnits(extractor(t, g, repo), repo, "a.fmt", "tests", c.Formats["tests"]); err != nil {
		t.Fatal(err)
	}
	return g, repo, l, c
}

func fmtDiff(t *testing.T, g session.Git, repo string, l session.Lock, c config.Contracts) []session.Diff {
	t.Helper()
	d, err := session.CheckContracts(g, repo, "main", session.FrozenFiles([]session.Lock{l}), c, extractor(t, g, repo))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestFormatLockNamesEachChangedOrRemovedUnit(t *testing.T) {
	// New units are free; a changed or removed locked unit is named; a file that matches another
	// format or none, and an extractor that fails, differ with a message.
	// covers: R-CTR-050, R-CTR-051
	g, repo, l, c := fmtRepo(t)
	if d := fmtDiff(t, g, repo, l, c); len(d) != 0 {
		t.Fatalf("clean: %v", d)
	}
	write(t, repo, "a.fmt", fmtText+"unit c: three\n")
	if d := fmtDiff(t, g, repo, l, c); len(d) != 0 {
		t.Fatalf("added unit: %v", d)
	}
	write(t, repo, "a.fmt", "unit a: ONE\nunit b: two\n")
	if d := fmtDiff(t, g, repo, l, c); len(d) != 1 || d[0].What != `unit "a" changed` {
		t.Fatalf("changed: %v", d)
	}
	write(t, repo, "a.fmt", "unit b: two\n")
	if d := fmtDiff(t, g, repo, l, c); len(d) != 1 || d[0].What != `unit "a" removed` {
		t.Fatalf("removed: %v", d)
	}
	write(t, repo, "a.fmt", "#fail\n")
	if d := fmtDiff(t, g, repo, l, c); len(d) != 1 || !strings.Contains(d[0].What, "boom") {
		t.Fatalf("failing extractor: %v", d)
	}
	write(t, repo, "a.fmt", fmtText)
	other := config.Contracts{
		Formats:     map[string]config.Format{"other": c.Formats["tests"]},
		FormatOrder: []string{"other"},
	}
	if d := fmtDiff(t, g, repo, l, other); len(d) != 1 || !strings.Contains(d[0].What, `format "tests"`) ||
		!strings.Contains(d[0].What, `"other"`) {
		t.Fatalf("other format: %v", d)
	}
	if d := fmtDiff(t, g, repo, l, config.Contracts{}); len(d) != 1 || !strings.Contains(d[0].What, `format "tests"`) {
		t.Fatalf("no format: %v", d)
	}
}

func TestFormatsMatchInConfigOrderBeforeTheBuiltins(t *testing.T) {
	// The first format whose globs match wins, in config order.
	// covers: R-CTR-047
	c := config.Contracts{
		Formats: map[string]config.Format{
			"late":  {Match: []string{"*.feature", "*.fmt"}, Units: "true"},
			"early": {Match: []string{"*.fmt"}, Units: "true"},
		},
		FormatOrder: []string{"early", "late"},
	}
	if name, _, ok := c.FormatOf("a.fmt"); !ok || name != "early" {
		t.Errorf("a.fmt -> %q, %v; want early", name, ok)
	}
	if name, _, ok := c.FormatOf("pay.feature"); !ok || name != "late" {
		t.Errorf("pay.feature -> %q, %v; want late", name, ok)
	}
	if _, _, ok := c.FormatOf("x.go"); ok {
		t.Error("x.go matched a format")
	}
}

func TestExtractorRunsTheBaseJudgeOnStdinOnly(t *testing.T) {
	// The command runs in an export of the merge-base tree: the branch's edit of the extractor is
	// not used, the file under test is not there to be read from disk, and an extractor that
	// prints units from empty stdin is refused. FromRepo says whether the first word is a file of
	// the merge-base.
	// covers: R-CTR-048, R-CTR-052
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	script, err := os.ReadFile("testdata/units.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Join(repo, "tools"), 0o750); err != nil {
		t.Fatal(err)
	}
	testkit.Commit(t, g, repo, "tools/units.sh", string(script), "judge")
	testkit.Commit(t, g, repo, "a.fmt", fmtText, "fmt")
	testkit.Commit(t, g, repo, "tools/hard.sh", "printf '{\"id\":\"h\",\"body\":\"1\"}\\n'\n", "hard")
	testkit.Commit(t, g, repo, "tools/disk.sh", "cat \"$1\" | sed -n 's/^unit \\(.*\\): \\(.*\\)$/{\"id\":\"\\1\",\"body\":\"\\2\"}/p'\n", "disk")
	x := extractor(t, g, repo)
	f := config.Format{Units: "sh tools/units.sh"}
	write(t, repo, "tools/units.sh", "exit 1\n") // a branch edit: not what runs
	got, err := x.Units("tests", f, "a.fmt", fmtText)
	if err != nil || len(got) != 2 {
		t.Fatalf("base judge on stdin: %v, %v", got, err)
	}
	if _, err = x.Units("hard", config.Format{Units: "sh tools/hard.sh"}, "a.fmt", fmtText); err == nil ||
		!strings.Contains(err.Error(), "stdin") {
		t.Errorf("an extractor that ignores stdin: %v", err)
	}
	if got, err = x.Units("disk", config.Format{Units: "sh tools/disk.sh"}, "a.fmt", "unit z: 1\n"); err == nil && len(got) > 0 {
		t.Errorf("an extractor that reads the file under test from disk got its units: %v", got)
	}
	if x.FromRepo(f) || !x.FromRepo(config.Format{Units: "tools/units.sh"}) ||
		x.FromRepo(config.Format{Units: "python3 -c 1"}) || x.FromRepo(config.Format{Units: `"tools/units.sh"`}) {
		t.Error("FromRepo: only a bare path of the merge-base is a repo file")
	}
}

// fmtProject is a repo whose main holds a.fmt and the judge tools/units.sh, with the "tests"
// format run by "sh tools/units.sh" and the project-wide glob over *.fmt.
func fmtProject(t *testing.T) (session.Git, string, config.Contracts) {
	t.Helper()
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	if err := os.MkdirAll(filepath.Join(repo, "tools"), 0o750); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile("testdata/units.sh")
	if err != nil {
		t.Fatal(err)
	}
	testkit.Commit(t, g, repo, "tools/units.sh", string(script), "judge")
	testkit.Commit(t, g, repo, "a.fmt", fmtText, "fmt")
	testkit.Commit(t, g, repo, "empty.fmt", "nothing\n", "no units")
	return g, repo, config.Contracts{
		Frozen:      []string{"*.fmt"},
		Formats:     map[string]config.Format{"tests": {Match: []string{"*.fmt"}, Units: "sh tools/units.sh"}},
		FormatOrder: []string{"tests"},
	}
}

func TestProjectFrozenFormatsLockUnitsFromTheBaseText(t *testing.T) {
	// A project-wide glob over a format's files locks the units of the text at base (untrimmed on
	// stdin, as the worktree file is): an added unit passes, a changed one is named, an edited
	// extractor on the branch changes nothing, and a file with no units at base takes additions.
	// covers: R-CTR-048, R-CTR-052, R-CTR-053
	g, repo, c := fmtProject(t)
	x := extractor(t, g, repo)
	proj, err := session.ProjectFrozen(g, repo, "main", c, x)
	if err != nil || !strings.HasPrefix(proj["a.fmt"].Sha, session.FormatTag+"tests:") ||
		len(proj["a.fmt"].Scenarios) != 2 || len(proj) != 2 {
		t.Fatalf("a.fmt: %+v, %v", proj, err)
	}
	if !strings.HasPrefix(proj["empty.fmt"].Sha, session.FormatTag) || len(proj["empty.fmt"].Scenarios) != 0 {
		t.Fatalf("empty.fmt must be a format lock with no units: %+v", proj["empty.fmt"])
	}
	check := func() map[string]string {
		t.Helper()
		d, cerr := session.CheckContracts(g, repo, "main", proj, c, x)
		if cerr != nil {
			t.Fatal(cerr)
		}
		out := map[string]string{}
		for _, y := range d {
			out[y.Path] = y.What
		}
		return out
	}
	if d := check(); len(d) != 0 {
		t.Fatalf("clean: %v", d)
	}
	write(t, repo, "a.fmt", fmtText+"unit c: three\n")
	write(t, repo, "empty.fmt", "unit new: x\n")
	if d := check(); len(d) != 0 {
		t.Fatalf("added units: %v", d)
	}
	write(t, repo, "a.fmt", "unit a: one\nunit b: 2\n")
	write(t, repo, "tools/units.sh", "#!/bin/sh\ncat >/dev/null\nprintf '{\"id\":\"a\",\"body\":\"1\"}\\n'\n")
	if d := check(); d["a.fmt"] != `unit "b" changed` || len(d) != 1 {
		t.Fatalf("changed (with an edited extractor): %v", d)
	}
}

func TestGuardWarnsOnFormatFilesAndNeverRefuses(t *testing.T) {
	// A format file, project-wide or in a session lock, is a soft verdict: the edit proceeds with
	// a warning. A file the glob never covered is silent.
	// covers: R-CTR-054
	g, repo, c := fmtProject(t)
	var warn bytes.Buffer
	gd := session.Guard{Git: g, Sessions: "docs/sessions", Project: c.Frozen, Formats: c, Base: "main", Warn: &warn}
	reason, soft := gd.Verdict("a.fmt", repo)
	if reason == "" || !soft || !strings.Contains(reason, "a.fmt") {
		t.Fatalf("project-wide format file: %q soft=%v", reason, soft)
	}
	if reason, _ = gd.Verdict("new.fmt", repo); reason != "" {
		t.Fatalf("a file new on the branch: %q", reason)
	}
	g2, repo2, l, c2 := fmtRepo(t)
	if err := session.WriteLock(repo2, l); err != nil {
		t.Fatal(err)
	}
	gd = session.Guard{Git: g2, Sessions: "docs/sessions", Formats: c2, Warn: &warn}
	if reason, soft = gd.Verdict("a.fmt", repo2); reason == "" || !soft {
		t.Fatalf("session-locked format file: %q soft=%v", reason, soft)
	}
}
