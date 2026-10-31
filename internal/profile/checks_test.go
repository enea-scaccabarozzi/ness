package profile_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/profile"
)

// fixture reads a testdata manifest.
func fixture(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", rel))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// parseFixture parses testdata/<rel> as the profile directory dir.
func parseFixture(t *testing.T, rel, dir string) (*profile.Profile, error) {
	t.Helper()
	return profile.Parse(dir, fixture(t, rel), fstest.MapFS{})
}

func TestParseChecksGood(t *testing.T) {
	// covers: R-PROF-045, R-PROF-001
	p, err := parseFixture(t, "checks/good.toml", "local")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Checks) != 5 {
		t.Fatalf("got %d checks, want 5: %+v", len(p.Checks), p.Checks)
	}
	sm, bf, ml, cp, run := p.Checks[0], p.Checks[1], p.Checks[2], p.Checks[3], p.Checks[4]
	if sm.ID != "autonomy-recorded" || sm.On != "open" || sm.For != "child" || sm.Skill != "ness-entry" ||
		sm.Section != "Autonomy" || sm.Message != "Record the autonomy mode." || sm.Kind != "section-matches" ||
		sm.Args["file"] != "docs/program/roadmap.md" || sm.Args["heading"] != "Autonomy" ||
		len(sm.Require) != 2 || sm.Require[1] != `\d{4}-\d{2}-\d{2}` || sm.Run != "" {
		t.Errorf("section-matches = %+v", sm)
	}
	if _, leaked := sm.Args["require"]; leaked {
		t.Errorf("require must be in Require, not Args: %+v", sm.Args)
	}
	if bf.Kind != "branch-forbids" || bf.Args["path"] != "^{record}/" || bf.Args["pattern"] != "D[0-9]+" || bf.For != "" {
		t.Errorf("branch-forbids = %+v", bf)
	}
	if ml.Kind != "match-last" || ml.For != "root" || ml.Args["against_file"] != "docs/program/decisions.md" ||
		ml.Args["pattern"] != `Recovery line: (D\d+)` || ml.Args["against_pattern"] != `^## (D\d+)` || ml.Args["file"] == "" {
		t.Errorf("match-last = %+v", ml)
	}
	if cp.Kind != "checkpoint-since-landing" || len(cp.Args) != 0 || cp.Require != nil {
		t.Errorf("checkpoint-since-landing = %+v", cp)
	}
	if run.Kind != "" || run.Run != "sh .ness/checks/x.sh" || run.Timeout != 45*time.Second || run.Message != "" {
		t.Errorf("run = %+v", run)
	}
}

func TestParseChecksRefused(t *testing.T) {
	// covers: R-PROF-045, R-PROF-005
	files, err := filepath.Glob("testdata/checks/bad/*.toml")
	if err != nil || len(files) < 20 {
		t.Fatalf("fixtures: %v %v", len(files), err)
	}
	for _, f := range files {
		rel := strings.TrimPrefix(f, "testdata/")
		t.Run(strings.TrimSuffix(filepath.Base(f), ".toml"), func(t *testing.T) {
			// The field the error must name is the first line: "# want: <field>".
			want, _, _ := strings.Cut(strings.TrimPrefix(string(fixture(t, rel)), "# want: "), "\n")
			_, err := parseFixture(t, rel, "local")
			if err == nil {
				t.Fatal("accepted")
			}
			// The field is named as a TOML path, checks[N].<field>; a duplicate id says "duplicate".
			field := regexp.MustCompile(`checks\[\d+\]\.` + regexp.QuoteMeta(want) + `\b`)
			if want == "duplicate" {
				field = regexp.MustCompile(`duplicate`)
			}
			if !field.MatchString(err.Error()) {
				t.Errorf("error %q does not match %s", err, field)
			}
			for _, w := range []string{"profile local", "profile.toml"} {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
		})
	}
}

func TestParseChecksDuplicateNamesID(t *testing.T) {
	// covers: R-PROF-045
	_, err := parseFixture(t, "checks/bad/duplicate_id.toml", "local")
	if err == nil || !strings.Contains(err.Error(), `"c1"`) {
		t.Fatalf("duplicate error should quote the id: %v", err)
	}
}

func TestResolveChecksOrder(t *testing.T) {
	// covers: R-PROF-047
	load := func(name string) *profile.Profile {
		t.Helper()
		p, err := parseFixture(t, "resolve/"+name+".toml", name)
		if err != nil {
			t.Fatal(err)
		}
		p.Local = name == "local"
		return p
	}
	r, err := profile.Resolve([]*profile.Profile{load("local"), load("alpha"), load("beta")}, none)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range r.Checks {
		got = append(got, c.ID)
	}
	if want := "alpha/a1 alpha/a2 beta/b1 local/l1"; strings.Join(got, " ") != want {
		t.Fatalf("ids = %v, want %s", got, want)
	}
	if c := r.Checks[1]; c.Run != "true" || c.On != "close" || c.Skill != "ness-entry" || c.Section != "Lifecycle" {
		t.Errorf("fields lost in resolve: %+v", c)
	}
}

func TestParseCheckSizeList(t *testing.T) {
	// covers: R-PROF-084
	p, err := parseFixture(t, "checks/size.toml", "local")
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Checks[0].Size; len(got) != 2 || got[0] != "program" || got[1] != "full" {
		t.Errorf("size = %v, want [program full]", got)
	}
	if got := p.Checks[1].Size; got != nil {
		t.Errorf("an undeclared size = %v, want nil (every size)", got)
	}
}
