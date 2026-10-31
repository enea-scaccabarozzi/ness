package hook

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/rule"
)

// Guard is one project guard (R-HOOK-022): a refusal for the tool calls whose tool name matches
// Tool and whose Bash command, or edited file path, matches Match; a non-nil Content narrows it
// to calls whose new text matches too.
type Guard struct {
	Tool    *regexp.Regexp
	Match   *regexp.Regexp
	Content *regexp.Regexp // nil: the new text is not looked at
	Message string
	Pointer rule.Pointer
}

// ProjectGuards returns the refusal of the first guard that applies to p, message then pointer,
// or "" to allow (R-HOOK-022).
func ProjectGuards(p Payload, guards []Guard) string {
	var cmd string
	var in struct {
		Command string `json:"command"`
	}
	if json.Unmarshal(p.ToolInput, &in) == nil {
		cmd = in.Command
	}
	files := EditedFiles(p.ToolName, p.ToolInput)
	for _, g := range guards {
		if g.Tool.MatchString(p.ToolName) && matches(g, p, cmd, files) {
			return g.Message + " " + g.Pointer.String()
		}
	}
	return ""
}

// matches reports whether g's match (and content, when set) fit the call: Match on the Bash
// command, or on any edited file path.
func matches(g Guard, p Payload, cmd string, files []string) bool {
	hit := p.ToolName == "Bash" && g.Match.MatchString(cmd)
	for _, f := range files {
		hit = hit || g.Match.MatchString(f)
	}
	return hit && (g.Content == nil || g.Content.MatchString(newText(p.ToolInput)))
}

// newText is the text a call writes: new_string, content and every edits[].new_string; never
// old_string.
func newText(raw json.RawMessage) string {
	var in struct {
		NewString string `json:"new_string"`
		Content   string `json:"content"`
		Edits     []struct {
			NewString string `json:"new_string"`
		} `json:"edits"`
	}
	if json.Unmarshal(raw, &in) != nil {
		return ""
	}
	parts := []string{in.NewString, in.Content}
	for _, e := range in.Edits {
		parts = append(parts, e.NewString)
	}
	return strings.Join(parts, "\n")
}

// DefaultGuards returns the guards every project gets: the gate guard for a non-empty gate
// (R-HOOK-023) and the launch-limits guard (R-HOOK-024).
func DefaultGuards(gate string) []Guard {
	var out []Guard
	if gate != "" {
		out = append(out, Guard{
			Tool: regexp.MustCompile(`^Bash$`),
			// The gate at the start of the command or after a separator, ending at whitespace, a
			// separator or the end, so `make validate-docs` passes.
			Match: regexp.MustCompile(`(?m)(^|&&|\|\||;|\|)\s*` + regexp.QuoteMeta(gate) + `(\s|$|[;&|])`),
			Message: "`" + gate + "` is the gate: it runs once, inside ness integrate, under the gate lock; " +
				"run the tests of the packages you touch",
			Pointer: rule.Of(rule.GateGuard),
		})
	}
	return append(out, Guard{
		Tool:    regexp.MustCompile(`^(Edit|Write|MultiEdit)$`),
		Match:   regexp.MustCompile(`(^|/)\.ness/ness\.toml$`),
		Content: regexp.MustCompile(`max_children|\[(launch|tiers|worker)`),
		Message: "sessions do not change launch limits, tiers or worker settings, which are the " +
			"operator's to change: wait for a child to land, or ask your manager",
		Pointer: rule.Of(rule.LimitsGuard),
	})
}
