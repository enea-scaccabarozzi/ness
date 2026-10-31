package session

import (
	"regexp"
	"slices"
	"strings"
)

// nameLimit is herdr's agent name length ([a-z][a-z0-9_-]{0,31}).
const nameLimit = 32

var nameBad = regexp.MustCompile(`[^a-z0-9_-]+`)

// AgentName is "<project>-<scope>" lowercased, truncated to herdr's limit, made unique against
// the live agents of every repository by appending characters of the session id (never by
// attaching to the other agent) (R-SESS-035, R-SESS-036).
func AgentName(project, scope, id string, live []string) string {
	base := nameBad.ReplaceAllString(strings.ToLower(project+"-"+scope), "-")
	base = strings.TrimLeft(base, "0123456789_-")
	if base == "" {
		base = "ness"
	}
	if len(base) > nameLimit {
		base = base[:nameLimit]
	}
	if !slices.Contains(live, base) {
		return base
	}
	chars := strings.TrimPrefix(id, "s-")
	for n := 1; n <= len(chars); n++ {
		suffix := "-" + chars[:n]
		stem := base
		if len(stem)+len(suffix) > nameLimit {
			stem = stem[:nameLimit-len(suffix)]
		}
		if name := stem + suffix; !slices.Contains(live, name) {
			return name
		}
	}
	return base[:nameLimit-len(chars)-1] + "-" + chars
}
