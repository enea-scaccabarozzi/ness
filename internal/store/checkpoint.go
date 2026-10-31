package store

import (
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/rule"
)

// IntentCheckpoint is a root's progress report to the operator (R-MSG-065).
const IntentCheckpoint = "checkpoint"

var checkpointHeadings = []string{"Done", "Now", "Next", "Ask", "Lessons"}

var pageRe = regexp.MustCompile(`(?m)^Page: (\S+)[ \t]*$`)

// CheckpointMissing returns the required headings (Done, Now, Next) body lacks, in that order
// (R-MSG-065).
func CheckpointMissing(body string) (missing []string) {
	have := map[string]bool{}
	for _, h := range headings(body) {
		have[h] = true
	}
	for _, h := range []string{"Done", "Now", "Next"} {
		if !have[h] {
			missing = append(missing, h)
		}
	}
	return missing
}

// noAskRe matches the first line of an Ask section that asks nothing: "Nothing.", "None", "N/A",
// "Nothing blocking", "Nothing is asked of you today", in any case and markup.
var noAskRe = regexp.MustCompile(
	`(?i)^\W*(?:(?:nothing|none|n/a)|nothing (?:is )?(?:blocking|asked(?: of you)?(?: today)?|needed|to ask))\W*$`)

// HasAsk reports whether body has an `## Ask` heading line with something under it: an empty
// section, or one that says "Nothing" or "None", asks nothing (R-MSG-040).
func HasAsk(body string) bool {
	if !slices.Contains(headings(body), "Ask") {
		return false
	}
	first := Section(body, "Ask")
	return first != "" && !noAskRe.MatchString(first)
}

// HasLessons reports whether body has an `## Lessons` heading line outside fenced code
// (R-SESS-153).
func HasLessons(body string) bool { return slices.Contains(headings(body), "Lessons") }

// headings lists the checkpoint headings of body that lie outside fenced code blocks (``` or ~~~).
func headings(body string) (out []string) {
	lines := strings.Split(body, "\n")
	for _, h := range rule.HeadingLines([]byte(body)) {
		// An indented heading counts only for Lessons (rule.Headings' reading); the others stay strict.
		if h.Level == 2 && slices.Contains(checkpointHeadings, h.Text) &&
			(h.Text == "Lessons" || strings.HasPrefix(lines[h.Line], "## ")) {
			out = append(out, h.Text)
		}
	}
	return out
}

// unfenced returns the lines of body outside fenced code blocks (``` or ~~~), fence lines dropped.
func unfenced(body string) (out []string) {
	var fence string
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimLeft(line, " ")
		switch {
		case fence != "" && strings.HasPrefix(t, fence):
			fence = ""
		case fence == "" && (strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")):
			fence = t[:3]
		case fence == "":
			out = append(out, line)
		}
	}
	return out
}

// Section returns the first non-blank line under the `## heading` line, "" when none; text inside
// fenced code blocks does not count.
func Section(body, heading string) string {
	in := false
	for _, line := range unfenced(body) {
		line = strings.TrimRight(line, " \t\r")
		switch {
		case in && strings.HasPrefix(line, "## "):
			return ""
		case in && line != "":
			return line
		case line == "## "+heading:
			in = true
		}
	}
	return ""
}

// SectionLines returns the lines under the `## heading` line up to the next `## ` heading, right-
// trimmed, nil when the heading is absent; text inside fenced code blocks does not count.
func SectionLines(body, heading string) (out []string) {
	in := false
	for _, line := range unfenced(body) {
		line = strings.TrimRight(line, " \t\r")
		switch {
		case in && strings.HasPrefix(line, "## "):
			return out
		case in:
			out = append(out, line)
		case line == "## "+heading:
			in = true
		}
	}
	return out
}

// PageOf returns the URL of the first `Page: URL` line, "" when none.
func PageOf(body string) string {
	if m := pageRe.FindStringSubmatch(body); m != nil {
		return m[1]
	}
	return ""
}

// UnalertedToOperator lists the open operator-bound request, decision, handoff and checkpoint
// items, and the replies to the operator's own items not yet shown, that are not yet alerted,
// oldest first (R-MSG-067).
func (s *Store) UnalertedToOperator() ([]Item, error) {
	return queryItems(s.db, selectItem+` WHERE recipient IS NULL AND alerted_at IS NULL AND retired_at IS NULL
		AND (disposition IS NULL OR disposition = 'deferred')
		AND intent IN ('request','decision','handoff','checkpoint','reply')
		AND NOT (intent = 'reply' AND shown_at IS NOT NULL)`+inOrder)
}

// MarkAlerted stamps the item as alerted; the first stamp is kept (R-MSG-067).
func (s *Store) MarkAlerted(id string, now time.Time) error {
	return s.touch(ErrNoItem, `UPDATE items SET alerted_at = COALESCE(alerted_at, ?) WHERE id = ?`, stamp(now), id)
}
