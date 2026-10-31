package profiles_test

import (
	"io/fs"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/enea-scaccabarozzi/ness/internal/profile"
	"github.com/enea-scaccabarozzi/ness/internal/rule"
	"github.com/enea-scaccabarozzi/ness/profiles"
)

var (
	// pointerRE is the one pointer format (R-RULE-001).
	pointerRE = regexp.MustCompile(`\(skill ([a-z0-9-]+), section ([^)]+)\)`)
	// pairRE finds the quoted ("skill", "Section") pairs of audit.py's POINTERS table.
	pairRE = regexp.MustCompile(`["'](ness-[a-z-]+)["']\s*,\s*["']([^"'\n]+)["']`)
	nameRE = regexp.MustCompile(`(?m)^name:\s*(\S+)\s*$`)
)

// skillHeadings maps each skill under the roots to its headings; the name is the SKILL.md
// frontmatter name, so a skill in skills/NAME/ and one at features/X/ are found alike.
func skillHeadings(t *testing.T, roots ...fs.FS) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, root := range roots {
		err := fs.WalkDir(root, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || path.Base(p) != "SKILL.md" {
				return err
			}
			b, err := fs.ReadFile(root, p)
			if err != nil {
				return err
			}
			m := nameRE.FindSubmatch(b)
			if m == nil {
				t.Errorf("%s has no name in its frontmatter", p)
				return nil
			}
			out[string(m[1])] = rule.Headings(b)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

// unresolved scans every file of root for pointers (and audit.py for POINTERS pairs) and
// returns how many it saw plus one line per pointer that does not resolve.
func unresolved(t *testing.T, root fs.FS, skills map[string][]string) (seen int, bad []string) {
	t.Helper()
	err := fs.WalkDir(root, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(root, p)
		if err != nil {
			return err
		}
		res := pointerRE.FindAllSubmatch(b, -1)
		if path.Base(p) == "audit.py" {
			res = append(res, pairRE.FindAllSubmatch(b, -1)...)
		}
		for _, m := range res {
			seen++
			ptr := rule.Pointer{Skill: string(m[1]), Section: strings.TrimSpace(string(m[2]))}
			if rerr := rule.Resolve(skills, ptr); rerr != nil {
				bad = append(bad, p+": "+rerr.Error())
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return seen, bad
}

func embeddedSkills(t *testing.T) map[string][]string {
	t.Helper()
	return skillHeadings(t, profiles.Core(), profiles.Builtin(), profiles.Features())
}

func TestRegistryResolvesOverEmbeddedSkills(t *testing.T) {
	// covers: R-RULE-003, R-RULE-002
	skills := embeddedSkills(t)
	if len(rule.Table()) == 0 {
		t.Fatal("empty registry")
	}
	for _, e := range rule.Table() {
		if err := rule.Resolve(skills, e.Pointer); err != nil {
			t.Errorf("%s: %v", e.ID, err)
		}
	}
}

func TestShippedFilePointersResolve(t *testing.T) {
	// covers: R-RULE-003
	skills := embeddedSkills(t)
	for name, root := range map[string]fs.FS{"core": profiles.Core(), "builtin": profiles.Builtin(), "features": profiles.Features()} {
		_, bad := unresolved(t, root, skills)
		for _, b := range bad {
			t.Errorf("%s/%s", name, b)
		}
	}
}

func TestProfilesShipPointers(t *testing.T) {
	// covers: R-RULE-003
	// A scan that finds nothing proves nothing: these files must carry pointers (plan Task 2).
	skills := embeddedSkills(t)
	b := profiles.Builtin()
	for _, f := range []string{
		"git-conventional/commit-msg",
		"go-strict/golangci.tests.yml",
		"go-strict/Makefile",
	} {
		data, err := fs.ReadFile(b, f)
		if err != nil {
			t.Fatal(err)
		}
		one := fstest.MapFS{path.Base(f): {Data: data}}
		if seen, _ := unresolved(t, one, skills); seen == 0 {
			t.Errorf("%s carries no (skill NAME, section HEADING) pointer", f)
		}
	}
}

func TestAuditPointerTable(t *testing.T) {
	// covers: R-RULE-003
	skills := embeddedSkills(t)
	sub, err := fs.Sub(profiles.Builtin(), "go-strict")
	if err != nil {
		t.Fatal(err)
	}
	src, err := fs.ReadFile(sub, "audit.py")
	if err != nil {
		t.Fatal(err)
	}
	have := map[rule.Pointer]bool{}
	for _, m := range pairRE.FindAllSubmatch(src, -1) {
		have[rule.Pointer{Skill: string(m[1]), Section: string(m[2])}] = true
	}
	// RULE -> pointer of plan Task 2, headings checked against the shipped skills.
	for _, want := range []rule.Pointer{
		{Skill: "ness-test-discipline", Section: "Rules"},
		{Skill: "ness-documentation", Section: "Comments"},
		{Skill: "ness-interface-design", Section: "Time and test hooks"},
		{Skill: "ness-interface-design", Section: "Wiring"},
		{Skill: "ness-behavior-specs", Section: "Scenarios"},
	} {
		if !have[want] {
			t.Errorf("audit.py POINTERS lacks (%q, %q)", want.Skill, want.Section)
		}
		if err := rule.Resolve(skills, want); err != nil {
			t.Errorf("(%q, %q) does not resolve: %v", want.Skill, want.Section, err)
		}
	}
}

func TestBuiltinChecksPointersResolve(t *testing.T) {
	// covers: R-RULE-003
	skills := embeddedSkills(t)
	roots := map[string]fs.FS{"core": profiles.Core()}
	names, err := profiles.Names()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		sub, err := fs.Sub(profiles.Builtin(), n)
		if err != nil {
			t.Fatal(err)
		}
		roots[n] = sub
	}
	for n, dir := range roots {
		data, err := fs.ReadFile(dir, "profile.toml")
		if err != nil {
			t.Fatal(err)
		}
		p, err := profile.Parse(n, data, dir)
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		for _, c := range p.Checks {
			ptr := rule.Pointer{Skill: c.Skill, Section: c.Section}
			if err := rule.Resolve(skills, ptr); err != nil {
				t.Errorf("%s/%s: %v", n, c.ID, err)
			}
		}
	}
}

func TestPointerScanNegatives(t *testing.T) {
	// covers: R-RULE-003
	skills := skillHeadings(t, os.DirFS("testdata/skilltree"))
	files := os.DirFS("testdata/files")
	seen, bad := unresolved(t, files, skills)
	if seen != 5 {
		t.Fatalf("saw %d pointers in the fixtures, want 5", seen)
	}
	joined := strings.Join(bad, "\n")
	for _, want := range []string{"missing-skill.txt", "missing-section.txt", "fenced.txt", "audit.py"} {
		if !strings.Contains(joined, want) {
			t.Errorf("no failure reported for %s; got:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "good.txt") {
		t.Errorf("good.txt reported as failing:\n%s", joined)
	}
	if len(bad) != 4 {
		t.Errorf("got %d failures, want 4:\n%s", len(bad), joined)
	}
}

func TestCommunicateStatesEveryHookRule(t *testing.T) {
	// covers: R-RULE-005
	b, err := fs.ReadFile(profiles.Core(), "skills/ness-entry/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	_, body, ok := strings.Cut(body, "\n## Communicate\n")
	if !ok {
		t.Fatal("ness-entry has no Communicate section")
	}
	body, _, _ = strings.Cut(body, "\n## ")
	for _, want := range []string{"run_in_background", "pgrep -f", "tool result"} {
		if !strings.Contains(body, want) {
			t.Errorf("ness-entry Communicate does not state %q", want)
		}
	}
}
