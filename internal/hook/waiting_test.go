package hook_test

import (
	"os"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/hook"
)

const communicate = "skill ness-entry, section Communicate"

func bashCmd(cmd string) hook.Payload { return payload("Bash", map[string]any{"command": cmd}) }

// Rejects a polling guard that fires on a loop and a ness read merely sharing a command line
// (the field saw `for b in a b; do git log $b; done; ness status` refused as a poll).
func TestPollingNeedsTheNessReadInsideTheLoop(t *testing.T) {
	// covers: R-HOOK-011, R-HOOK-025
	allowed := []string{
		"for b in a b; do git log $b; done; ness status",
		"for b in a b; do git log $b; done && ness inbox",
		"for b in a b; do git log $b; done\nness status",
		"while read b; do git log $b; done < branches; ness status",
		"until git fetch; do :; done; ness inbox",
		"for b in a b; do git log $b; done | head; ness status --json",
		"for b in a b; do git log $b; ness status; done",
		"for b in a b; do\n  git log $b\n  ness inbox\ndone",
	}
	for _, c := range allowed {
		if reason := hook.PreTool(bashCmd(c)); reason != "" {
			t.Errorf("%q: blocked: %q", c, reason)
		}
	}
	blocked := []string{
		"for b in a b; do git log $b; done; while true; do ness inbox; done",
		"git log; while ! ness inbox | grep m-; do sleep 5; done",
		"while true; do ness status; sleep 30; done; echo ok",
		"for b in a b; do ness status; sleep 30; done",
	}
	for _, c := range blocked {
		if reason := hook.PreTool(bashCmd(c)); !strings.Contains(reason, "end your turn") {
			t.Errorf("%q: allowed or no waiting rule: %q", c, reason)
		}
	}
}

// Rejects a polling guard that refuses a finite read: the field saw 54 refusals in a week, all of
// them `for` loops over item ids after a wake, one a python script read as shell. Also rejects
// one that stops refusing real waits, or that stops scanning a script handed to a shell's -c.
func TestFiniteReadsPassAndWaitsStayRefused(t *testing.T) {
	// covers: R-HOOK-011, R-HOOK-012
	allowed := []string{
		"for i in m-a m-b m-c; do ness inbox --show $i; done",
		"for i in m-a m-b; do\n  ness inbox --show $i\n  echo ---\ndone",
		"ness inbox --show m-a m-b m-c",
		"ness inbox --unread",
		"ness inbox --json | python3 -c \"import json,sys\nfor it in json.load(sys.stdin)['items']: print(it['id'])\"",
		"python3 -c \"import os\nwhile ids: os.system('ness inbox --show ' + ids.pop())\"",
		"python3 - <<'EOF'\nwhile ids: os.system('ness inbox --show ' + ids.pop())\nEOF",
		"node -e 'while (q.length) run(\"ness status\")'",
	}
	for _, c := range allowed {
		if reason := hook.PreTool(bashCmd(c)); reason != "" {
			t.Errorf("%q: blocked: %q", c, reason)
		}
	}
	blocked := []string{
		"for i in m-a m-b; do ness inbox --show $i; sleep 30; done",
		"while true; do ness inbox --show m-a; done",
		"until ness inbox | grep -q m-; do :; done",
		"bash -lc 'while true; do ness inbox; done'",
		"sudo bash -c 'until ness status | grep d; do :; done'",
		"zsh -c 'for i in 1 2; do ness inbox; sleep 5; done'",
		"bash -o pipefail -c 'while true; do ness inbox; done'",
		"bash --noprofile -euo pipefail -c 'until ness status | grep d; do :; done'",
	}
	for _, c := range blocked {
		reason := hook.PreTool(bashCmd(c))
		if !strings.Contains(reason, "end your turn") || !strings.Contains(reason, "ness inbox --show A B C") ||
			!strings.Contains(reason, "--unread") {
			t.Errorf("%q: allowed or no pointer to --show/--unread: %q", c, reason)
		}
	}
}

// Rejects a hook that lets a sleep loop retry ness integrate, send or open, or one that refuses
// every loop or every sleep.
func TestSleepLoopAroundNessCommandsIsRefused(t *testing.T) {
	// covers: R-HOOK-025
	blocked := []string{
		"for i in $(seq 40); do ness integrate && break; sleep 60; done",
		"while ! ness integrate; do sleep 30; done",
		"until ness integrate --subject 'feat: x'; do sleep 30; done",
		"while true; do ness send --to manager --body @h.md --intent handoff && break; sleep 5; done",
		"for i in 1 2 3; do ness open --scope x --tier standard --brief b.md && break; sleep 10; done",
		"bash -c 'for i in 1 2; do ness integrate && break; sleep 60; done'",
		"while true; do ness  integrate && break; sleep 60; done",
	}
	for _, c := range blocked {
		reason := hook.PreTool(bashCmd(c))
		if reason == "" || !strings.Contains(reason, communicate) || !strings.Contains(reason, "sleep") {
			t.Errorf("%q: allowed or no pointer: %q", c, reason)
		}
	}
	allowed := []string{
		"ness integrate",
		"ness integrate --subject 'feat: x'",
		"for b in a b; do ness integrate --from $b; done",
		"for i in 1 2 3; do sleep 1; done; ness integrate",
		"for i in 1 2 3; do make test; sleep 5; done",
		"make test && ness integrate",
		"ness send --to manager --body 'sleep well'",
		"echo 'ness integrate'; sleep 5",
	}
	for _, c := range allowed {
		if reason := hook.PreTool(bashCmd(c)); reason != "" {
			t.Errorf("%q: blocked: %q", c, reason)
		}
	}
}

// Rejects a guard that reads the wrong tool name or field, blocks other subagents, or omits
// the replacement and the pointer. The fixture is the shape Claude Code sends for a subagent call.
func TestExploreSubagentIsRefused(t *testing.T) {
	// covers: R-HOOK-026
	f, err := os.Open("testdata/agent_explore.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	p, err := hook.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	reason := hook.PreTool(p)
	if !strings.Contains(reason, "ness-sweep") || !strings.Contains(reason, communicate) {
		t.Errorf("fixture: reason = %q, want ness-sweep and the pointer", reason)
	}
	for _, tool := range []string{"Agent", "Task"} {
		in := map[string]any{"description": "d", "prompt": "p", "subagent_type": "Explore"}
		if reason := hook.PreTool(payload(tool, in)); !strings.Contains(reason, "ness-sweep") {
			t.Errorf("%s Explore: allowed or no replacement: %q", tool, reason)
		}
	}
	for _, kind := range []string{"ness-sweep", "ness-review", "general-purpose", "Plan", ""} {
		in := map[string]any{"description": "d", "prompt": "Explore the repo", "subagent_type": kind}
		if reason := hook.PreTool(payload("Agent", in)); reason != "" {
			t.Errorf("subagent_type %q: blocked: %q", kind, reason)
		}
	}
	other := payload("Bash", map[string]any{"command": "echo", "subagent_type": "Explore"})
	if reason := hook.PreTool(other); reason != "" {
		t.Errorf("Bash with a subagent_type field: blocked: %q", reason)
	}
}
