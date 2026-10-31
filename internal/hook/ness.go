package hook

import (
	"encoding/json"
	"strings"
)

// NessCommand reports whether p is a Bash call of one simple ness command: its first word is
// ness and it holds no shell operator, substitution, redirection, escape or newline. It is the
// only call that passes while a skew blocks the hooks, so the session can still repair it
// (R-UPD-021).
func NessCommand(p Payload) bool {
	var in toolInput
	if p.ToolName != "Bash" || json.Unmarshal(p.ToolInput, &in) != nil {
		return false
	}
	words := strings.Fields(in.Command)
	return len(words) > 0 && words[0] == "ness" && !strings.ContainsAny(in.Command, ";&|`$<>()\\\n")
}
