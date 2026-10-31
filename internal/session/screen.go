package session

import (
	"regexp"
	"time"
)

// Screen is what a client's pane shows that the worker acts on, as its client atom reads it:
// the operator's draft in the input box ("" when empty or when the box shows the client's dim
// placeholder, R-WRK-015), a live usage-limit notice (R-WRK-040), the first matched error
// pattern (R-WRK-051) and whether the tier's model is reported unavailable (R-SESS-009).
// NoPrompt is set when no input box was found at all: Draft is then "" (fail open), and the
// worker logs it once so a client that changed its prompt is noticed (R-WRK-014).
type Screen struct {
	NoPrompt    bool
	Draft       string
	Limit       *Limit
	Error       string
	Unavailable bool
	// Background is set when the client's footer shows a live background shell: the child is
	// working although the client reads as idle (R-WRK-050).
	Background bool
	// Compacting is set when the client shows it is compacting its conversation: herdr can read
	// that as a blocked prompt (R-WRK-081).
	Compacting bool
}

// Activity is what a client's transcript says about its conversation, read where the screen is
// silent (R-WRK-096): background work launched and not finished (a subagent shows no marker and the
// pane reads idle), the time of the last real assistant turn (a compaction is not one), and the notice
// of a usage-limit error that is still the client's last word, with its time, and the latest
// Remote Control URL that opens the conversation in the client's app, "" when none (R-WRK-082).
type Activity struct {
	Pending bool
	Turn    time.Time
	Limit   string
	LimitAt time.Time
	Remote  string
}

// Limit is a usage-limit notice: its text and the reset time it names, zero when it names none
// (R-WRK-045).
type Limit struct {
	Notice string
	Reset  time.Time
}

// ansiRe matches CSI sequences, OSC strings (title, hyperlink), charset designations (ESC ( B)
// and two-byte escapes.
var ansiRe = regexp.MustCompile(
	`\x1b\[[0-?]*[\x20-/]*[\x40-\x7e]|\x1b\][^\a\x1b]*(?:\a|\x1b\\)|\x1b[()*+][0-9A-Za-z]|\x1b[\x40-\x5a\\-_]`)

// StripANSI returns s without terminal escape sequences.
func StripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }
