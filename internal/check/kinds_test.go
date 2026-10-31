package check_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/check"
	"github.com/enea-scaccabarozzi/ness/internal/profile"
)

// run runs one check against the fixture tree testdata/<tree> and returns its failures.
func run(t *testing.T, tree string, c profile.Check, deps check.Deps) []check.Failure {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("testdata", tree))
	if err != nil {
		t.Fatal(err)
	}
	ctx := openCtx(dir)
	ctx.Event = c.On
	return check.Run(ctx, []profile.Check{c}, deps)
}

func wantPass(t *testing.T, name string, got []check.Failure) {
	t.Helper()
	if len(got) != 0 {
		t.Errorf("%s: failures %+v, want none", name, got)
	}
}

func wantFail(t *testing.T, name string, got []check.Failure, detail string) {
	t.Helper()
	if len(got) != 1 {
		t.Errorf("%s: failures %+v, want exactly one", name, got)
		return
	}
	if got[0].Message != "Fix "+got[0].ID[strings.Index(got[0].ID, "/")+1:]+"." || got[0].Pointer.Skill != "ness-program" {
		t.Errorf("%s: failure %+v lacks the declared message or pointer", name, got[0])
	}
	if !strings.Contains(got[0].Detail, detail) {
		t.Errorf("%s: detail %q does not contain %q", name, got[0].Detail, detail)
	}
}

func TestSectionMatches(t *testing.T) {
	// covers: R-CHK-011
	args := func(file, heading string) map[string]string {
		return map[string]string{"file": file, "heading": heading}
	}
	const roadmap = "docs/roadmap.md"
	for _, tc := range []struct {
		name, tree, heading string
		require             []string
		fail                string // empty: passes; otherwise the detail
	}{
		{"every require matches in the body", "roadmap", "Autonomy", []string{`(?i)\b(gate|bands)\b`, `\d{4}-\d{2}-\d{2}`}, ""},
		{"a deeper heading belongs to the body", "roadmap", "Autonomy", []string{`bands for the first wave`}, ""},
		{"the next heading of equal level ends the body", "roadmap", "Autonomy", []string{`to checkpoint`}, "to checkpoint"},
		{"a require that matches nothing", "roadmap", "Autonomy", []string{`gate`, `custom`}, "custom"},
		{"no such heading", "roadmap", "Ghost", []string{`gate`}, "Ghost"},
		{"a heading in a code fence is not a heading", "fenced", "Autonomy", []string{`gate`}, "Autonomy"},
	} {
		c := kindCheck("section-matches", args(roadmap, tc.heading), tc.require...)
		got := run(t, tc.tree, c, check.Deps{})
		if tc.fail == "" {
			wantPass(t, tc.name, got)
			continue
		}
		wantFail(t, tc.name, got, tc.fail)
	}
}

func TestSectionMatchesFailsClosedOnAMissingFile(t *testing.T) {
	// covers: R-CHK-007, R-CHK-011
	c := kindCheck("section-matches", map[string]string{"file": "docs/none.md", "heading": "Autonomy"}, "gate")
	wantFail(t, "missing file", run(t, "roadmap", c, check.Deps{}), "none.md")
}

func TestMatchLast(t *testing.T) {
	// covers: R-CHK-013
	args := func(file, against string) map[string]string {
		return map[string]string{
			"file": "docs/" + file, "pattern": `last decision id (D\d+)`,
			"against_file": "docs/" + against, "against_pattern": `(?m)^- \*\*(D\d+)\*\*`,
		}
	}
	for _, tc := range []struct {
		name, file, against, fail string
	}{
		{"equal ids", "roadmap-equal.md", "decisions.md", ""},
		{"the ledger moved on", "roadmap-stale.md", "decisions.md", "D12"},
		{"no recovery line", "roadmap-none.md", "decisions.md", "roadmap-none.md"},
		{"no decision in the ledger", "roadmap-equal.md", "decisions-empty.md", "decisions-empty.md"},
		{"missing file", "roadmap-gone.md", "decisions.md", "roadmap-gone.md"},
		{"missing against file", "roadmap-equal.md", "decisions-gone.md", "decisions-gone.md"},
	} {
		got := run(t, "ledger", kindCheck("match-last", args(tc.file, tc.against)), check.Deps{})
		if tc.fail == "" {
			wantPass(t, tc.name, got)
			continue
		}
		wantFail(t, tc.name, got, tc.fail)
	}
}

const diffText = `diff --git a/docs/sessions/root/fix/decisions.md b/docs/sessions/root/fix/decisions.md
--- a/docs/sessions/root/fix/decisions.md
+++ b/docs/sessions/root/fix/decisions.md
@@ -1,2 +1,2 @@
 context - **D9** is context, not added
-- **D8** removed line
+a plain note
+- **D7** an id was added
diff --git a/docs/other.md b/docs/other.md
--- a/docs/other.md
+++ b/docs/other.md
@@ -1 +1 @@
+- **D5** in another file
`

func TestBranchForbids(t *testing.T) {
	// covers: R-CHK-012
	args := map[string]string{"path": `{record}/decisions\.md$`, "pattern": `(?m)^\s*[-*]\s*\*\*D\d+`}
	g := func(diff string) check.Deps {
		return check.Deps{Git: fakeGit{dir: "/w", rng: "session/root-1...session/fix-1", diff: diff}}
	}
	run := func(c profile.Check, d check.Deps) []check.Failure {
		return check.Run(integrateCtx("/w"), []profile.Check{withEvent(c, "integrate")}, d)
	}
	c := kindCheck("branch-forbids", args)
	wantFail(t, "an added id", run(c, g(diffText)), "D7")
	wantPass(t, "a clean diff", run(c, g("")))
	onlyOther := strings.Replace(diffText, "+- **D7** an id was added\n", "", 1)
	wantPass(t, "a removed line, a context line and another path", run(c, g(onlyOther)))
	other := kindCheck("branch-forbids", map[string]string{"path": `docs/other\.md$`, "pattern": `D5`})
	wantFail(t, "path is a regexp over the diff's files", run(other, g(diffText)), "D5")
	// {record} is quoted: a dot in the record dir is no wildcard.
	ctx := integrateCtx("/w")
	ctx.Record = "docs/sessions/r.ot/fix"
	got := check.Run(ctx, []profile.Check{withEvent(c, "integrate")}, g(diffText))
	wantPass(t, "a quoted record dir", got)
}

func TestBranchForbidsFailsClosedOnAGitError(t *testing.T) {
	// covers: R-CHK-007, R-CHK-012
	c := withEvent(kindCheck("branch-forbids", map[string]string{"path": `x$`, "pattern": "y"}), "integrate")
	d := check.Deps{Git: fakeGit{err: errors.New("bad revision")}}
	wantFail(t, "git error", check.Run(integrateCtx("/w"), []profile.Check{c}, d), "bad revision")
}

func TestCheckpointSinceLanding(t *testing.T) {
	// covers: R-CHK-014
	later, earlier := base.Add(time.Hour), base
	for _, tc := range []struct {
		name  string
		facts fakeFacts
		fail  string
	}{
		{"nothing landed", fakeFacts{}, ""},
		{"nothing landed, a checkpoint", fakeFacts{checkpoint: base}, ""},
		{"checkpoint after the landing", fakeFacts{landing: earlier, checkpoint: later}, ""},
		{"checkpoint at the landing", fakeFacts{landing: earlier, checkpoint: earlier}, ""},
		{"checkpoint before the landing", fakeFacts{landing: later, checkpoint: earlier}, "checkpoint"},
		{"landed, never checkpointed", fakeFacts{landing: later}, "checkpoint"},
		{"facts error", fakeFacts{err: errors.New("store locked")}, "store locked"},
	} {
		got := run(t, "roadmap", kindCheck("checkpoint-since-landing", nil), check.Deps{Facts: tc.facts})
		if tc.fail == "" {
			wantPass(t, tc.name, got)
			continue
		}
		wantFail(t, tc.name, got, tc.fail)
	}
}
