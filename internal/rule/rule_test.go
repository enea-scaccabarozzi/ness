package rule_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/rule"
)

// refusals is R-RULE-002's list, written here and not read from the code, with the skill
// that states each rule.
var refusals = map[rule.ID]string{
	rule.HookPolling:    "ness-entry",
	rule.HookBackground: "ness-entry",
	rule.HookOutput:     "ness-entry",
	rule.ChildCap:       "ness-launch",
	rule.Checkpoint:     "ness-entry",
	rule.Handoff:        "ness-entry",
	rule.Subject:        "ness-launch",
	rule.DirtyWorktree:  "ness-launch",
	rule.RootClose:      "ness-entry",
	rule.RootOpen:       "ness-launch",
	rule.ReportMissing:  "ness-entry",
	rule.ContractGuard:  "ness-contracts",
	rule.Size:           "ness-entry",
	rule.QuickRoot:      "ness-entry",
	rule.ModelEffort:    "ness-entry",
}

func TestPointerString(t *testing.T) {
	// covers: R-RULE-001
	got := rule.Pointer{Skill: "ness-entry", Section: "Wait, then fold"}.String()
	if want := "(skill ness-entry, section Wait, then fold)"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestRefIsSpaceThenPointer(t *testing.T) {
	// covers: R-RULE-001
	for _, e := range rule.Table() {
		if got, want := rule.Ref(e.ID), " "+e.Pointer.String(); got != want {
			t.Errorf("Ref(%s) = %q, want %q", e.ID, got, want)
		}
	}
	if len(rule.Table()) == 0 {
		t.Fatal("empty registry: Ref has nothing to be checked against")
	}
}

func TestRefUnknownPanics(t *testing.T) {
	// covers: R-RULE-001
	defer func() {
		if recover() == nil {
			t.Fatal("Ref of an unregistered id returned instead of panicking")
		}
	}()
	rule.Ref("no-such-refusal")
}

func TestTableCoversTheRefusals(t *testing.T) {
	// covers: R-RULE-002
	seen := map[rule.ID]rule.Pointer{}
	for _, e := range rule.Table() {
		if _, dup := seen[e.ID]; dup {
			t.Errorf("id %s registered twice", e.ID)
		}
		seen[e.ID] = e.Pointer
		if e.Pointer.Skill == "" || e.Pointer.Section == "" {
			t.Errorf("id %s has an empty pointer %+v", e.ID, e.Pointer)
		}
	}
	for id, skill := range refusals {
		p, ok := seen[id]
		if !ok {
			t.Errorf("no registry entry for %s", id)
			continue
		}
		if p.Skill != skill {
			t.Errorf("%s points at skill %q, want %q", id, p.Skill, skill)
		}
	}
}

func TestTableIsFresh(t *testing.T) {
	// covers: R-RULE-001
	a := rule.Table()
	if len(a) == 0 {
		t.Fatal("empty registry")
	}
	a[0].Pointer.Section = "mutated"
	if rule.Table()[0].Pointer.Section == "mutated" {
		t.Fatal("Table shares state between calls")
	}
}

const sample = "# Title\n\nintro\n\n## Communicate\n- a\n\n### Waiting rule  \n\n```md\n## Hidden\n```\n\n~~~\n## Tilde\n~~~\n\n## Wait, then fold\n"

func TestHeadings(t *testing.T) {
	// covers: R-RULE-003
	got := rule.Headings([]byte(sample))
	want := []string{"Title", "Communicate", "Waiting rule", "Wait, then fold"}
	if !slices.Equal(got, want) {
		t.Fatalf("Headings = %q, want %q", got, want)
	}
}

func TestResolve(t *testing.T) {
	// covers: R-RULE-003
	skills := map[string][]string{"ness-entry": rule.Headings([]byte(sample))}
	tests := []struct {
		name string
		p    rule.Pointer
		want []string // substrings of the error; nil means success
	}{
		{"h2", rule.Pointer{Skill: "ness-entry", Section: "Communicate"}, nil},
		{"h3", rule.Pointer{Skill: "ness-entry", Section: "Waiting rule"}, nil},
		{"punctuated", rule.Pointer{Skill: "ness-entry", Section: "Wait, then fold"}, nil},
		{"missing skill", rule.Pointer{Skill: "ness-nope", Section: "Communicate"}, []string{"ness-nope"}},
		{"missing section", rule.Pointer{Skill: "ness-entry", Section: "Gone"}, []string{"Gone", "Communicate", "Wait, then fold"}},
		{"heading in a code fence", rule.Pointer{Skill: "ness-entry", Section: "Hidden"}, []string{"Hidden"}},
		{"heading in a tilde fence", rule.Pointer{Skill: "ness-entry", Section: "Tilde"}, []string{"Tilde"}},
		{"case differs", rule.Pointer{Skill: "ness-entry", Section: "communicate"}, []string{"communicate"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := rule.Resolve(skills, tc.p)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("Resolve: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Resolve(%+v) = nil, want an error", tc.p)
			}
			for _, s := range tc.want {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("error %q lacks %q", err, s)
				}
			}
		})
	}
}
