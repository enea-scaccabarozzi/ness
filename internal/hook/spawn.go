package hook

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Spawn is what a subagent call declares: its agent type, the explicit model and effort, and the
// kind and escalate reason read from the prompt's leading header block (R-HOOK-031).
type Spawn struct{ AgentType, Model, Effort, Kind, Escalate string }

var headerLine = regexp.MustCompile(`^[ \t]*([A-Za-z][\w-]*)[ \t]*:[ \t]*(.*?)[ \t]*$`)

// ParseSpawn reads a subagent call; false for any tool other than Agent or its legacy name Task.
// A tool_input that is not an object is still a spawn, declaring nothing.
func ParseSpawn(p Payload) (Spawn, bool) {
	if p.ToolName != "Agent" && p.ToolName != "Task" {
		return Spawn{}, false
	}
	var in toolInput
	if err := json.Unmarshal(p.ToolInput, &in); err != nil {
		in = toolInput{}
	}
	sp := Spawn{AgentType: in.SubagentType, Model: in.Model, Effort: in.Effort}
	// Only the header counts: consecutive key: value lines at the start; the first other line
	// ends it, so a kind: or escalate: in pasted YAML or quoted text below is not read.
	for _, line := range strings.Split(in.Prompt, "\n") {
		m := headerLine.FindStringSubmatch(strings.TrimSuffix(line, "\r"))
		if m == nil {
			break
		}
		switch {
		case strings.EqualFold(m[1], "kind") && sp.Kind == "":
			sp.Kind = CanonicalKind(strings.ToLower(m[2]))
		case strings.EqualFold(m[1], "escalate") && sp.Escalate == "":
			sp.Escalate = m[2]
		}
	}
	return sp, true
}

// kindAliases maps a kind name agents write to the kind it means: ness-sweep does search work.
// ponytail: global alias; a project that declares its own `sweep` kind loses it, check models.kinds if one does.
var kindAliases = map[string]string{"sweep": "search"}

// CanonicalKind is a declared kind with its alias resolved, case kept (R-HOOK-031, R-MOD-046).
func CanonicalKind(k string) string {
	if c, ok := kindAliases[k]; ok {
		return c
	}
	return k
}

// Judge returns the verdict on a requested rung against the recommended one (R-HOOK-030):
// below, allowed, escalated (one rung up with a reason), else refused, or would-refuse when
// enforce is "log". refuse is true only for a refusal under enforce "refuse".
func Judge(requested, recommended int, escalate, enforce string) (verdict string, refuse bool) {
	switch {
	case requested < recommended:
		return "below", false
	case requested == recommended:
		return "allowed", false
	case requested == recommended+1 && escalate != "":
		return "escalated", false
	case enforce == "log":
		return "would-refuse", false
	}
	return "refused", true
}
