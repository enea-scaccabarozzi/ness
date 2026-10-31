package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// ResumeText is typed into a pane once its usage limit reset (R-WRK-043): the turn the limit cut
// short goes on.
const ResumeText = "continue"

// RenameText renames the running conversation (R-WRK-071): Claude Code's /rename, the one way to
// name a session that is already running.
func RenameText(name string) string { return "/rename " + name }

// ReadScreen is the package's Read: with ResumeText and RenameText, what the worker needs to
// deliver to a Claude Code pane (R-WRK-090).
func (Client) ReadScreen(ansi string, now time.Time) session.Screen { return Read(ansi, now) }

// ResumeText is the package's ResumeText.
func (Client) ResumeText() string { return ResumeText }

// RenameText is the package's RenameText.
func (Client) RenameText(name string) string { return RenameText(name) }

// ErrorPattern is a screen text that means the session is stuck on the API; Name is what the
// worker reports, never the screen's own text.
type ErrorPattern struct {
	Name string
	Re   *regexp.Regexp
}

// lead starts a notice line: the client's glyphs and indentation, nothing else. Source, diffs,
// line numbers, quotes and the operator's own prompt line start with something else (H2). The
// client's agent-prose markers (● and ⏺) are not among them: a line after one is prose.
const lead = `(?mi)^[ \t│⎿✻✗!]*`

// noticeLines is how many non-blank lines above the input box can hold a live notice; older output
// is history, not a state of the session.
const noticeLines = 3

// ErrorPatterns are the client's error notices: a whole line in notice position that starts with
// the client's phrase, never a substring of agent prose (R-WRK-051, H2). The worker reports a stall
// to its manager under Name, not under screen text. Usage limits are not among them (D37).
// ponytail: line-start phrases only, no SGR color check; widen when a real notice shows another.
var ErrorPatterns = []ErrorPattern{
	{"API Error", regexp.MustCompile(lead + `API Error:`)},
	{"overloaded", regexp.MustCompile(lead + `(?:error: )?overloaded(?:_error\b|[ \t]*$|:)`)},
	{"capacity", regexp.MustCompile(lead + `(?:API Error: )?(?:the )?service is (?:currently )?(?:at|over) capacity`)},
}

// unavailableRe matches the two texts Claude Code 2.1.294 prints when the tier's model id does not
// resolve (R-SESS-009): "There's an issue with the selected model (X)..." and "The model X is not
// available on your ... deployment". Only column 0 matches: a tool result (⎿) and indented agent
// prose that quote the text are not the client's own notice (H2). This anchoring, not the worker's
// two-pass debounce, is what tells a notice from a quote on a screen that does not change.
// ponytail: the two real phrases only; widen when a new one shows.
var unavailableRe = regexp.MustCompile(
	`(?m)^(?:There's an issue with the selected model \(|The model \S+ is not available on your )`)

// lf normalises the line endings: herdr's pane read ends every line with CRLF, and the patterns
// anchor on "$" and "\n".
func lf(s string) string { return crlf.Replace(s) }

var crlf = strings.NewReplacer("\r\n", "\n", "\r", "\n")

// Read classifies a Claude Code pane's visible screen (R-WRK-014, R-WRK-015, R-WRK-040,
// R-WRK-051, R-SESS-009). A limit notice wins over the error patterns: a limited pane is silent,
// not stalled.
func Read(ansi string, now time.Time) session.Screen {
	ansi = lf(ansi)
	line, at, found := promptLine(ansi)
	sc := session.Screen{
		Draft: draftAt(ansi, line, at), NoPrompt: !found,
		Background: background(ansi) || waitingRe.MatchString(textAbove(ansi, at, waitingLines)),
		Compacting: compactingRe.MatchString(textAbove(ansi, at, noticeLines)),
	}
	text := noticeText(ansi, at)
	if lim, ok := ParseLimit(text, now); ok {
		sc.Limit = &lim
		return sc
	}
	sc.Unavailable = unavailableRe.MatchString(text)
	for _, p := range ErrorPatterns {
		if p.Re.MatchString(text) {
			sc.Error = p.Name
			break
		}
	}
	return sc
}

// shellsRe matches the footer count of live background shells, "1 shell" or "2 shells", with or
// without a leading middot (the footers of the permission modes differ).
var shellsRe = regexp.MustCompile(`\b\d+\s+shells?\b`)

// background reports whether the screen's last non-blank line, the footer, shows a live
// background shell.
func background(ansi string) bool {
	lines := strings.Split(session.StripANSI(ansi), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return shellsRe.MatchString(lines[i])
		}
	}
	return false
}

// waitingRe matches the status line the client draws above the box while a background agent runs,
// "✻ Waiting for 1 background agent to finish", which no footer count shows; waitingLines is how
// far above the box it can sit, under the agent list.
var waitingRe = regexp.MustCompile(lead + `Waiting for \d+ background agents? to finish`)

const waitingLines = 12

// compactingRe matches the status line of a conversation being compacted,
// "✻ Compacting conversation… (15s)".
var compactingRe = regexp.MustCompile(lead + `Compacting conversation`)

// blankRe matches a line with no text: spaces and box drawing only.
var blankRe = regexp.MustCompile(`^[\s─━│╭╮╰╯]*$`)

// ownRe matches a line carrying the worker's own notice.
var ownRe = regexp.MustCompile(`ness worker|ness open --resume`)

// noticeText is the plain text of the noticeLines non-blank lines above line index end (the input
// box, or the screen's end). The worker's own notices ("ness worker ...", "ness open --resume")
// are skipped whole, wrapped or not: the worker must never read its own report as a state (H1).
func noticeText(ansi string, end int) string { return textAbove(ansi, end, noticeLines) }

// textAbove is the plain text of the n non-blank lines above line index end, the worker's own
// notices skipped.
func textAbove(ansi string, end, n int) string {
	lines := strings.Split(session.StripANSI(ansi), "\n")
	var kept []string
	for i := min(end, len(lines)) - 1; i >= 0 && len(kept) < n; i-- {
		if !blankRe.MatchString(lines[i]) && !ownRe.MatchString(lines[i]) {
			kept = append(kept, lines[i])
		}
	}
	slices.Reverse(kept)
	return strings.Join(kept, "\n")
}

// promptRe is the input box's prompt on a plain line: optional box drawing and spaces, then the
// client's glyph, ">" in older renderings and "❯" in 2.1.x, alone or followed by a space or a
// no-break space. If the client changes its glyph, edit this one constant (M6).
var promptRe = regexp.MustCompile(`^[│ \t]*[>❯](?:[ \x{a0}]|$)`)

// escRe matches one escape sequence at the start of a string; the CSI parameters are captured.
var escRe = regexp.MustCompile(
	`^(?:\x1b\[([0-?]*)[\x20-/]*([\x40-\x7e])|\x1b\][^\a\x1b]*(?:\a|\x1b\\)|\x1b[\x40-\x5a\\-_])`)

// promptLine returns the last prompt line of the visible screen, escapes kept.
// at is its index, or the line count when there is none.
func promptLine(ansi string) (line string, at int, found bool) {
	lines := strings.Split(ansi, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if promptRe.MatchString(session.StripANSI(lines[i])) {
			return lines[i], i, true
		}
	}
	return "", len(lines), false
}

// draft is the operator's text on a prompt line: what is drawn at normal intensity after the
// prompt, "" when the box shows only the client's dim placeholder (R-WRK-015).
func draft(line string) string {
	bright := brightText(line)
	bright = strings.TrimLeft(bright, "│ \t")
	bright = strings.TrimPrefix(strings.TrimPrefix(bright, ">"), "❯")
	return strings.TrimSpace(strings.Trim(bright, "\u00a0"))
}

// draftAt is the draft of the prompt line at index at; when that line is empty, a draft that starts
// with a line break continues on the lines below it, down to the box's lower rule.
func draftAt(ansi, line string, at int) string {
	if d := draft(line); d != "" {
		return d
	}
	lines := strings.Split(ansi, "\n")
	for i := at + 1; i < len(lines) && at < len(lines) && !ruleRe.MatchString(session.StripANSI(lines[i])); i++ {
		if d := strings.TrimSpace(brightText(lines[i])); d != "" {
			return d
		}
	}
	return ""
}

// ruleRe starts the box's lower border: the lines below it are the client's status, not the draft.
var ruleRe = regexp.MustCompile(`^\s*[─━╰╭]`)

// InputBox returns the operator's draft: the text after the last prompt of the visible screen
// without escape sequences, or "" when the box is empty or shows only dim text (R-WRK-015, the P3
// placeholder regression). No prompt on the screen is an empty box; Read also reports that.
func InputBox(ansi string) string {
	ansi = lf(ansi)
	line, at, _ := promptLine(ansi)
	return draftAt(ansi, line, at)
}

// sgr tracks the drawing state: faint (SGR 2) and SGR 90 mean dim; color is the foreground set by
// SGR 30-37 or 38 ("" is the default).
// ponytail: the client's placeholder is dim by SGR 2/90 or painted in the same color as the prompt
// glyph on its line (read from the line, so no foreground guess); another dim style would read as a
// draft, which fails safe (the worker does not type).
type sgr struct {
	faint, grey bool
	color       string
	seen        bool   // the prompt glyph has been drawn (kept across resets)
	glyph       string // the color it was drawn in
}

// apply folds one SGR parameter list into the state.
func (s *sgr) apply(params string) {
	p := strings.Split(params, ";")
	for i := 0; i < len(p); i++ {
		if s.attribute(p[i]) {
			continue
		}
		switch {
		case p[i] == "38":
			end := skipColor(p, i)
			s.color = strings.Join(p[i:end+1], ";")
			i = end
		case p[i] == "48" || p[i] == "58":
			i = skipColor(p, i)
		case len(p[i]) == 2 && p[i][0] == '3' && p[i][1] <= '7':
			s.color = p[i]
		}
	}
}

// attribute applies a one-parameter attribute and reports whether p was one.
func (s *sgr) attribute(p string) bool {
	switch p {
	case "", "0":
		*s = sgr{seen: s.seen, glyph: s.glyph}
	case "2":
		s.faint = true
	case "22":
		s.faint = false
	case "90":
		s.grey = true
	case "39":
		s.grey, s.color = false, ""
	default:
		return false
	}
	return true
}

// skipColor returns the index of the last parameter of the extended color at p[i], so its
// channels (38;2;r;g;b, 38;5;n) are not read as attributes.
func skipColor(p []string, i int) int {
	switch {
	case i+1 < len(p) && p[i+1] == "2":
		return min(i+4, len(p)-1)
	case i+1 < len(p) && p[i+1] == "5":
		return min(i+2, len(p)-1)
	}
	return len(p) - 1
}

// draws reports whether rune r is drawn as text, and notes the prompt glyph's color.
func (s *sgr) draws(r rune) bool {
	dim := s.faint || s.grey || (s.seen && s.glyph != "" && s.color == s.glyph)
	if !s.seen && (r == '>' || r == '❯') {
		s.seen, s.glyph = true, s.color
	}
	return !dim
}

// brightText returns the characters of s drawn as text: not faint, not SGR 90, and, after the
// prompt glyph, not in the glyph's own color (the client's placeholder).
func brightText(s string) string {
	var out strings.Builder
	var st sgr
	for s != "" {
		if m := escRe.FindStringSubmatch(s); m != nil {
			if m[2] == "m" {
				st.apply(m[1])
			}
			s = s[len(m[0]):]
			continue
		}
		r, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		if st.draws(r) {
			out.WriteRune(r)
		}
	}
	return out.String()
}

var (
	limitRe = regexp.MustCompile(lead + `(?:Agent terminated early due to an API error: )?` +
		`You'?ve (?:hit|reached) your (session|weekly|Fable|Opus|Sonnet|Haiku) limit(?:\s*·\s*resets\s+([^\n]+))?`)
	resetRe = regexp.MustCompile(`(?i)^(?:(?P<mon>jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)\w*` +
		`\s+(?P<day>\d{1,2})(?:,|\s+at)?\s*)?` +
		`(?:(?P<h>\d{1,2})(?::(?P<m>\d{2}))?\s*(?P<ap>am|pm)?)?\s*(?:\((?P<tz>[^)]+)\))?`)
	months = map[string]time.Month{
		"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
		"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
	}
)

// ParseLimit finds the first usage-limit notice line in text (R-WRK-040, the F2 regression) and
// the reset it names (see resetTime). A notice without a usable reset has a zero Reset.
func ParseLimit(text string, now time.Time) (session.Limit, bool) {
	m := limitRe.FindStringSubmatch(text)
	if m == nil {
		return session.Limit{}, false
	}
	lim := session.Limit{Notice: strings.TrimSpace(m[0])}
	reset := strings.TrimRight(strings.TrimSpace(m[2]), ". ")
	if r := resetRe.FindStringSubmatch(reset); len(r) != 0 {
		lim.Reset = resetTime(r, now)
	}
	return lim, true
}

// resetTime is the reset a notice names, in the zone it prints or now's (R-WRK-046): a time
// without a date is today, or tomorrow when it is more than an hour past (a notice read right
// after its reset is that reset); a date is this year, a day already past is Unknown, never next
// year (R-WRK-044, H2). The time is 12h with am/pm or 24h with minutes; a date without one is
// 00:00. Zero (Unknown) for an impossible date or time, an unloadable zone, or a result more than
// 8 days ahead or (dated) a day already past: the caller then rechecks the screen (R-WRK-045).
func resetTime(r []string, now time.Time) time.Time {
	g := func(name string) string { return r[resetRe.SubexpIndex(name)] }
	if g("mon") == "" && g("h") == "" {
		return time.Time{}
	}
	loc := now.Location()
	if g("tz") != "" {
		var err error
		if loc, err = time.LoadLocation(g("tz")); err != nil {
			return time.Time{}
		}
	}
	now = now.In(loc)
	hour, minute, ok := clock(g("h"), g("m"), g("ap"))
	if !ok {
		return time.Time{}
	}
	var t time.Time
	if g("mon") == "" {
		t = time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, loc)
		if t.Before(now.Add(-time.Hour)) {
			t = t.AddDate(0, 0, 1)
		}
	} else if t, ok = onDate(months[strings.ToLower(g("mon"))], atoi(g("day")), hour, minute, now); !ok {
		return time.Time{}
	}
	if t.After(now.AddDate(0, 0, 8)) {
		return time.Time{}
	}
	return t
}

// onDate is month/day at hour:minute this year in now's zone; false for an impossible date or a
// day already past.
func onDate(month time.Month, day, hour, minute int, now time.Time) (time.Time, bool) {
	t := time.Date(now.Year(), month, day, hour, minute, 0, 0, now.Location())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return t, t.Month() == month && t.Day() == day && !t.Before(today)
}

// clock reads a notice's time: no time is 00:00; 12h needs am/pm, 24h needs minutes.
func clock(h, m, ap string) (hour, minute int, ok bool) {
	switch {
	case h == "":
		return 0, 0, true
	case ap != "":
		hour, minute = atoi(h), atoi(m)
		if hour < 1 || hour > 12 || minute > 59 {
			return 0, 0, false
		}
		hour %= 12
		if strings.EqualFold(ap, "pm") {
			hour += 12
		}
		return hour, minute, true
	case m != "":
		hour, minute = atoi(h), atoi(m)
		return hour, minute, hour < 24 && minute < 60
	}
	return 0, 0, false
}

// atoi parses digits the regexes guarantee; "" (an absent minute) is 0.
func atoi(s string) int {
	n, _ := strconv.Atoi(s) //nolint:errcheck // digits by construction, or empty
	return n
}

// Usage reads the client's usage snapshot, ~/.claude/usage-latest.json (the statusline input it
// last wrote; testkit.UsageGuard reads the same file): the latest end of a window at 100% that
// has not reset, or false (R-WRK-047). A missing, unreadable or malformed file is no limit.
func Usage(home string, now time.Time) (time.Time, bool) {
	path := filepath.Join(home, ".claude", "usage-latest.json")
	data, err := os.ReadFile(path) //nolint:gosec // the user's own snapshot
	if err != nil {
		return time.Time{}, false
	}
	var u struct {
		RateLimits map[string]struct {
			Used     float64 `json:"used_percentage"`
			ResetsAt float64 `json:"resets_at"`
		} `json:"rate_limits"`
	}
	if json.Unmarshal(data, &u) != nil {
		return time.Time{}, false
	}
	var until time.Time
	for _, w := range u.RateLimits {
		if r := time.Unix(int64(w.ResetsAt), 0); w.Used >= 100 && r.After(now) && r.After(until) {
			until = r
		}
	}
	return until, !until.IsZero()
}
