package hook_test

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/hook"
	"github.com/enea-scaccabarozzi/ness/internal/rule"
)

func call(t *testing.T, tool string, input any) hook.Payload {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return hook.Payload{ToolName: tool, ToolInput: raw}
}

func shell(t *testing.T, cmd string) hook.Payload {
	t.Helper()
	return call(t, "Bash", map[string]any{"command": cmd})
}

type row struct {
	name    string
	p       hook.Payload
	blocked bool
}

func check(t *testing.T, guards []hook.Guard, rows []row, wantPointer string) {
	t.Helper()
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			got := hook.ProjectGuards(r.p, guards)
			switch {
			case r.blocked && got == "":
				t.Fatal("allowed, want refused")
			case r.blocked && !strings.Contains(got, wantPointer):
				t.Errorf("refusal %q lacks pointer %q", got, wantPointer)
			case !r.blocked && got != "":
				t.Errorf("refused %q, want allowed", got)
			}
		})
	}
}

func TestGateGuard(t *testing.T) {
	// covers: R-HOOK-023
	rows := []row{
		{"plain", shell(t, "make validate"), true},
		{"after cd", shell(t, "cd x && make validate"), true},
		{"before echo", shell(t, "make validate; echo"), true},
		{"validate-docs", shell(t, "make validate-docs"), false},
		{"go test", shell(t, "go test ./internal/hook"), false},
		{"not bash", call(t, "Read", map[string]any{"file_path": "make validate"}), false},
	}
	check(t, hook.DefaultGuards("make validate"), rows, "(skill ness-entry, section Tests)")
}

func TestGateGuardEmptyGate(t *testing.T) {
	// covers: R-HOOK-023
	check(t, hook.DefaultGuards(""), []row{{"make validate", shell(t, "make validate"), false}}, "")
}

func TestLimitsGuard(t *testing.T) {
	// covers: R-HOOK-024
	cfg := ".ness/ness.toml"
	rows := []row{
		{"edit max_children", call(t, "Edit", map[string]any{"file_path": cfg, "old_string": "max_children = 0", "new_string": "max_children = 8"}), true},
		{"write launch table", call(t, "Write", map[string]any{"file_path": "/repo/" + cfg, "content": "x = 1\n[launch]\n"}), true},
		{"edit tiers", call(t, "Edit", map[string]any{"file_path": cfg, "old_string": "a", "new_string": "[tiers.standard]"}), true},
		{"edit worker", call(t, "Edit", map[string]any{"file_path": cfg, "old_string": "a", "new_string": "[worker]"}), true},
		{"multiedit second", call(t, "MultiEdit", map[string]any{"file_path": cfg, "edits": []map[string]string{
			{"old_string": "a", "new_string": "b"}, {"old_string": "c", "new_string": "max_children = 3"}}}), true},
		{"other key", call(t, "Edit", map[string]any{"file_path": cfg, "old_string": "gate = \"a\"", "new_string": "gate = \"b\""}), false},
		{"other file", call(t, "Edit", map[string]any{"file_path": "docs/notes.md", "old_string": "a", "new_string": "max_children is 8"}), false},
		{"old_string only", call(t, "Edit", map[string]any{"file_path": cfg, "old_string": "[launch]\nmax_children = 0", "new_string": ""}), false},
	}
	check(t, hook.DefaultGuards("make validate"), rows, "(skill ness-launch, section Child or subagent)")
}

func TestConfiguredGuards(t *testing.T) {
	// covers: R-HOOK-022
	ptr := rule.Pointer{Skill: "ness-entry", Section: "Tests"}
	guards := []hook.Guard{
		{Tool: regexp.MustCompile(`^Bash$`), Match: regexp.MustCompile(`^pytest\s*$`), Message: "Run the one test you changed.", Pointer: ptr},
		{Tool: regexp.MustCompile(`^(Edit|Write)$`), Match: regexp.MustCompile(`\.py$`), Content: regexp.MustCompile(`print\(`),
			Message: "No debug prints.", Pointer: ptr},
	}
	rows := []row{
		{"whole suite", shell(t, "pytest"), true},
		{"one test", shell(t, "pytest tests/test_x.py"), false},
		{"other tool", call(t, "Read", map[string]any{"command": "pytest"}), false},
		{"print added", call(t, "Edit", map[string]any{"file_path": "a.py", "old_string": "x", "new_string": "print(x)"}), true},
		{"print only removed", call(t, "Edit", map[string]any{"file_path": "a.py", "old_string": "print(x)", "new_string": "x"}), false},
		{"other file", call(t, "Write", map[string]any{"file_path": "a.go", "content": "print("}), false},
	}
	check(t, guards, rows, ptr.String())
	got := hook.ProjectGuards(shell(t, "pytest"), guards)
	if !strings.Contains(got, "Run the one test you changed.") {
		t.Errorf("refusal %q lacks the message", got)
	}
}

func TestGuardOrderAndNone(t *testing.T) {
	// covers: R-HOOK-022
	p := rule.Pointer{Skill: "ness-entry", Section: "Tests"}
	first := hook.Guard{Tool: regexp.MustCompile(`Bash`), Match: regexp.MustCompile(`x`), Message: "first", Pointer: p}
	second := hook.Guard{Tool: regexp.MustCompile(`Bash`), Match: regexp.MustCompile(`x`), Message: "second", Pointer: p}
	if got := hook.ProjectGuards(shell(t, "x"), []hook.Guard{first, second}); !strings.Contains(got, "first") || strings.Contains(got, "second") {
		t.Errorf("refusal %q, want the first guard only", got)
	}
	if got := hook.ProjectGuards(shell(t, "x"), nil); got != "" {
		t.Errorf("no guards refused %q", got)
	}
}
