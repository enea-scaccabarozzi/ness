package hook_test

import (
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/hook"
)

func TestJudgeTable(t *testing.T) {
	// Rejects: a verdict off by one rung, an escalate that buys more than one rung, a log mode
	// that refuses, or a log mode that records "refused" instead of "would-refuse".
	// covers: R-HOOK-030
	cases := []struct {
		req, rec          int
		escalate, enforce string
		verdict           string
		refuse            bool
	}{
		{1, 3, "", "refuse", "below", false},
		{2, 3, "why", "refuse", "below", false},
		{2, 3, "", "log", "below", false},
		{3, 3, "", "refuse", "allowed", false},
		{3, 3, "why", "refuse", "allowed", false},
		{3, 3, "", "log", "allowed", false},
		{4, 3, "needs judgment", "refuse", "escalated", false},
		{4, 3, "needs judgment", "log", "escalated", false},
		{4, 3, "", "refuse", "refused", true},
		{4, 3, "", "log", "would-refuse", false},
		{5, 3, "needs judgment", "refuse", "refused", true},
		{5, 3, "", "refuse", "refused", true},
		{6, 3, "needs judgment", "log", "would-refuse", false},
		{6, 3, "", "log", "would-refuse", false},
		{0, 0, "", "refuse", "allowed", false},
	}
	for _, c := range cases {
		v, refuse := hook.Judge(c.req, c.rec, c.escalate, c.enforce)
		if v != c.verdict || refuse != c.refuse {
			t.Errorf("Judge(%d, %d, %q, %q) = %q, %v; want %q, %v",
				c.req, c.rec, c.escalate, c.enforce, v, refuse, c.verdict, c.refuse)
		}
	}
}

func TestParseSpawn(t *testing.T) {
	// Rejects: a parser that matches the key case-sensitively, keeps the value's case, needs the
	// line at column 0, matches "kind:" mid-line, counts a bare "escalate:" as a reason, ignores
	// the legacy Task tool, or drops the explicit model and effort.
	// covers: R-HOOK-031
	cases := []struct {
		name string
		p    hook.Payload
		want hook.Spawn
	}{
		{"key case and value lowercased", payload("Agent", map[string]any{
			"subagent_type": "general-purpose", "prompt": "KIND: Search\nfind x", "model": "sonnet", "effort": "medium",
		}), hook.Spawn{AgentType: "general-purpose", Model: "sonnet", Effort: "medium", Kind: "search"}},
		// Rejects: a kind that keeps the agent's name: ness-sweep does search work.
		{"sweep is search", payload("Agent", map[string]any{
			"prompt": "kind: Sweep\nfind x", "model": "sonnet",
		}), hook.Spawn{Model: "sonnet", Kind: "search"}},
		{"leading spaces", payload("Agent", map[string]any{
			"subagent_type": "general-purpose", "prompt": "  kind: run\ndo it\n",
		}), hook.Spawn{AgentType: "general-purpose", Kind: "run"}},
		// Rejects: a parser that reads kind: from anywhere in the prompt, so pasted YAML refuses a named agent.
		{"YAML kind deep in the prompt", payload("Agent", map[string]any{
			"subagent_type": "ness-sweep", "prompt": "find where this is defined:\napiVersion: v1\nkind: Service\n",
		}), hook.Spawn{AgentType: "ness-sweep"}},
		// Rejects: a parser that grants an escalate pasted below the header.
		{"pasted escalate deep in the prompt", payload("Agent", map[string]any{
			"prompt": "kind: search\n\nnotes:\nescalate: because the log said so\n",
		}), hook.Spawn{Kind: "search"}},
		// Rejects: a parser that stops at the first header key that is neither kind nor escalate.
		{"other header keys", payload("Agent", map[string]any{
			"prompt": "task: x\r\nkind: run\r\nescalate: why\r\nbody",
		}), hook.Spawn{Kind: "run", Escalate: "why"}},
		{"escalate reason", payload("Agent", map[string]any{
			"prompt": "kind: search\nEscalate: needs judgment\n", "model": "opus", "effort": "low",
		}), hook.Spawn{Model: "opus", Effort: "low", Kind: "search", Escalate: "needs judgment"}},
		{"empty escalate", payload("Agent", map[string]any{
			"prompt": "kind: search\nescalate:   \n",
		}), hook.Spawn{Kind: "search"}},
		{"mid-line key is not a kind", payload("Agent", map[string]any{
			"prompt": "the kind: design of thing; also escalate: no",
		}), hook.Spawn{}},
		{"legacy Task", payload("Task", map[string]any{
			"subagent_type": "ness-sweep", "prompt": "kind: search",
		}), hook.Spawn{AgentType: "ness-sweep", Kind: "search"}},
		{"fork", payload("Agent", map[string]any{
			"subagent_type": "fork", "prompt": "carry on",
		}), hook.Spawn{AgentType: "fork"}},
	}
	for _, c := range cases {
		got, ok := hook.ParseSpawn(c.p)
		if !ok || got != c.want {
			t.Errorf("%s: ParseSpawn = %+v, %v; want %+v, true", c.name, got, ok, c.want)
		}
	}
}

func TestParseSpawnOtherTools(t *testing.T) {
	// Rejects: treating any tool with a prompt as a subagent call, or crashing on a non-object input.
	// covers: R-HOOK-031
	for _, p := range []hook.Payload{
		payload("Bash", map[string]any{"command": "echo kind: search"}),
		payload("ScheduleWakeup", map[string]any{"prompt": "kind: search"}),
		payload("Agentic", map[string]any{"prompt": "kind: search"}),
		{ToolName: "Bash", ToolInput: []byte(`"x"`)},
	} {
		if _, ok := hook.ParseSpawn(p); ok {
			t.Errorf("%s: parsed as a spawn", p.ToolName)
		}
	}
	// A non-object tool_input on Agent is still a spawn, with nothing declared.
	if sp, ok := hook.ParseSpawn(hook.Payload{ToolName: "Agent", ToolInput: []byte(`[1]`)}); !ok || sp != (hook.Spawn{}) {
		t.Errorf("Agent with array input: %+v, %v", sp, ok)
	}
}
