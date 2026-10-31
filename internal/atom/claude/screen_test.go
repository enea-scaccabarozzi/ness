package claude_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/claude"
)

const (
	dim   = "\x1b[2mTry \"fix the failing test\"\x1b[22m"
	gray  = "\x1b[90mTry \"fix the failing test\"\x1b[39m"
	grey  = "\x1b[0m\x1b[38;2;153;153;153mTry \"fix the failing test\"\x1b[0m"
	rule  = "\x1b[0m\x1b[38;2;136;136;136m────────────────\x1b[0m\n"
	nbsp  = " "
	reset = "\x1b[0m"
)

// box renders the client's input box around line: a bordered box, the prompt, a status line.
func box(line string) string {
	return "assistant output above\n╭────────────╮\n│ > " + line + "\n╰────────────╯\n  ? for shortcuts\n"
}

// realBox is the box as Claude Code 2.1.294 draws it: rules above and below, the prompt glyph
// followed by a no-break space, a status line (synthesized text, the client's rendering).
func realBox(line string) string {
	return "some output\n" + rule + "❯" + nbsp + line + "\n" + rule + "  status line\n"
}

// greyBox is the box as the real client paints it: the prompt glyph in truecolor grey 153.
func greyBox(after string) string {
	return "some output\n" + rule + "\x1b[0m\x1b[38;2;153;153;153m❯" + nbsp + after + "\x1b[0m\n" + rule + "  status\n"
}

func TestInputBoxTable(t *testing.T) {
	// The box is read as ANSI: the dim placeholder is an empty box, normal-intensity text is a
	// draft, a draft typed over the placeholder keeps only the bright part (P3 placeholder regression).
	// covers: R-WRK-014, R-WRK-015
	for name, c := range map[string]struct{ screen, want string }{
		"empty":                          {box(""), ""},
		"dim placeholder":                {box(dim), ""},
		"gray placeholder":               {box(gray), ""},
		"draft":                          {box("fix the lint first"), "fix the lint first"},
		"draft with color":               {box("\x1b[1mfix\x1b[22m the lint"), "fix the lint"},
		"draft over placeholder":         {box("fix \x1b[2m the failing test\x1b[22m"), "fix"},
		"bare prompt no space":           {"│ >\n", ""},
		"last prompt-like line":          {"> quoted line\nno box here\n", "quoted line"},
		"no prompt line":                 {"nothing\n", ""},
		"last prompt wins":               {"│ > old\n│ > new draft\n", "new draft"},
		"real empty":                     {realBox(""), ""},
		"real truecolor grey":            {realBox(grey), "Try \"fix the failing test\""}, // glyph is default: a draft
		"glyph grey, nothing after":      {greyBox(""), ""},
		"glyph grey, same grey text":     {greyBox("Try \"fix the failing test\""), ""},
		"glyph grey, default text":       {greyBox("\x1b[39mfix it"), "fix it"},
		"glyph grey, reset text":         {greyBox("\x1b[0mfix it"), "fix it"},
		"glyph grey, other color text":   {greyBox("\x1b[38;2;255;255;255mfix it"), "fix it"},
		"glyph grey, then same grey set": {greyBox("\x1b[0m\x1b[38;2;153;153;153mTry it"), ""},
		"draft starts on a later line":   {"❯" + nbsp + "\n  second line\n" + rule + "  status\n", "second line"},
		"empty box status below rule":    {"❯" + nbsp + "\n" + rule + "  status text\n", ""},
		"truecolor 2 channel":            {realBox("\x1b[38;2;2;2;2mfix\x1b[0m"), "fix"},
		"real faint":                     {realBox(dim), ""},
		"real draft":                     {realBox("fix the lint first"), "fix the lint first"},
		"real draft after reset":         {realBox(reset + "fix it"), "fix it"},
		"glyph alone on its line":        {"some output\n" + rule + "❯\n" + rule + "  status\n", ""},
		"glyph alone, draft below":       {"❯\n  second line\n" + rule + "  status\n", "second line"},
		"bare prompt then text":          {"│ >\nmore\n", "more"},
		"real glyph is dim":              {"\x1b[2m❯" + nbsp + "\x1b[22mfix it\n", "fix it"},
	} {
		for _, eol := range eols {
			if got := claude.InputBox(eol.conv(c.screen)); got != c.want {
				t.Errorf("%s (%s): InputBox = %q, want %q", name, eol.name, got, c.want)
			}
		}
	}
}

// eols runs a screen table twice: the fixtures as written (LF) and as herdr's pane read prints
// them (CRLF, verified against herdr 0.8.2).
var eols = []struct {
	name string
	conv func(string) string
}{
	{"LF", func(s string) string { return s }},
	{"CRLF", func(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }},
}

func TestReadFlagsAScreenWithoutAPrompt(t *testing.T) {
	// A prompt glyph the reader no longer recognises fails open (an empty box); NoPrompt makes
	// that visible so the worker can log it once (M6, R-WRK-014).
	// covers: R-WRK-014
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	if sc := claude.Read("nothing here\n", now); !sc.NoPrompt || sc.Draft != "" {
		t.Errorf("no prompt: %+v", sc)
	}
	if sc := claude.Read(realBox(""), now); sc.NoPrompt {
		t.Errorf("prompt present: %+v", sc)
	}
}

func rome(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Rome")
	if err != nil {
		t.Skip("no tzdata:", err)
	}
	return loc
}

func newYork(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("no tzdata:", err)
	}
	return loc
}

func TestParseLimitTable(t *testing.T) {
	// Every notice format of the spec (F2 regression), interpreted in the box's zone with the
	// next-occurrence rules.
	// covers: R-WRK-040, R-WRK-044, R-WRK-046
	loc := rome(t)
	at := func(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, loc) }
	seen := at(2026, time.September, 28, 10, 0)
	oct4 := at(2026, time.October, 4, 10, 0)
	const session, weekly = "You've hit your session limit · resets ", "You've hit your weekly limit · resets "
	for _, c := range []struct {
		name, text string
		now        time.Time
		limited    bool
		reset      time.Time
	}{
		{"session pm", session + "2:10pm", seen, true, at(2026, time.September, 28, 14, 10)},
		{"session am", session + "4am", seen, true, at(2026, time.September, 29, 4, 0)},
		{"session am minutes", session + "3:40am", seen, true, at(2026, time.September, 29, 3, 40)},
		{"weekly date time", weekly + "Oct 1, 4am", seen, true, at(2026, time.October, 1, 4, 0)},
		{"weekly date", weekly + "Oct 8", oct4, true, at(2026, time.October, 8, 0, 0)},
		{"api error wrapper", "Agent terminated early due to an API error: " + weekly + "Oct 8",
			oct4, true, at(2026, time.October, 8, 0, 0)},
		{"no reset", "You've reached your Fable limit", seen, true, time.Time{}},
		{"late night", session + "4am", at(2026, time.September, 28, 23, 50), true, at(2026, time.September, 29, 4, 0)},
		{"just reset", session + "4am", at(2026, time.September, 28, 4, 0).Add(30 * time.Second), true,
			at(2026, time.September, 28, 4, 0)},
		{"date this year", weekly + "Oct 1", seen, true, at(2026, time.October, 1, 0, 0)},
		{"date passed this year", weekly + "Oct 1", at(2026, time.October, 2, 9, 0), true, time.Time{}},
		{"more than 8 days away", weekly + "Oct 20", seen, true, time.Time{}},
		{"8 days away", weekly + "Oct 6", seen, true, at(2026, time.October, 6, 0, 0)},
		{"impossible date", weekly + "Feb 30", at(2026, time.February, 25, 9, 0), true, time.Time{}},
		{"impossible date with time", weekly + "Feb 30, 4am", at(2026, time.February, 25, 9, 0), true, time.Time{}},
		{"24h time", session + "16:30", seen, true, at(2026, time.September, 28, 16, 30)},
		{"24h time midnight", session + "00:15", seen, true, at(2026, time.September, 29, 0, 15)},
		{"zero am", session + "0am", seen, true, time.Time{}},
		{"bad 24h time", session + "25:00", seen, true, time.Time{}},
		{"dated at time", weekly + "Oct 1 at 4am", seen, true, at(2026, time.October, 1, 4, 0)},
		{"printed zone", session + "2:10pm (America/New_York)", seen, true,
			time.Date(2026, time.September, 28, 14, 10, 0, 0, newYork(t))},
		{"zone date differs", session + "9pm (America/New_York)", at(2026, time.September, 28, 2, 0), true,
			time.Date(2026, time.September, 27, 21, 0, 0, 0, newYork(t))},
		{"unknown zone", session + "2:10pm (Mars/Olympus)", seen, true, time.Time{}},
		{"dst end", session + "4am", time.Date(2026, time.October, 24, 23, 0, 0, 0, loc), true,
			time.Date(2026, time.October, 25, 3, 0, 0, 0, time.UTC)},
		{"dst start", session + "4am", time.Date(2026, time.March, 28, 23, 0, 0, 0, loc), true,
			time.Date(2026, time.March, 29, 2, 0, 0, 0, time.UTC)},
		{"date today", weekly + "Oct 1", at(2026, time.October, 1, 9, 0), true, at(2026, time.October, 1, 0, 0)},
		{"inside output", "some text\n  ⎿  " + weekly + "Oct 8.\n│ > \n", oct4, true, at(2026, time.October, 8, 0, 0)},
		{"not a limit", "the limit of this function is 4am", seen, false, time.Time{}},
		{"empty", "", seen, false, time.Time{}},
	} {
		lim, ok := claude.ParseLimit(c.text, c.now)
		if ok != c.limited {
			t.Errorf("%s: limited = %v, want %v", c.name, ok, c.limited)
			continue
		}
		if ok && !lim.Reset.Equal(c.reset) {
			t.Errorf("%s: reset = %s, want %s", c.name, lim.Reset, c.reset)
		}
		if ok && lim.Notice == "" {
			t.Errorf("%s: empty notice", c.name)
		}
	}
}

func TestReadClassifiesErrorsAndExcludesLimits(t *testing.T) {
	// covers: R-WRK-051, R-SESS-009
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	notFound := "There's an issue with the selected model (claude-opus-5-5). It may not exist."
	for name, c := range map[string]struct {
		screen      string
		err         string
		unavailable bool
		limited     bool
	}{
		"api error":             {"  ⎿  API Error: 500 internal\n" + box(dim), "API Error", false, false},
		"overloaded":            {"Error: overloaded_error: Overloaded\n" + box(dim), "overloaded", false, false},
		"overloaded type only":  {"Error: overloaded_error\n" + box(dim), "overloaded", false, false},
		"label not screen text": {"  ⎿  Overloaded\n" + box(dim), "overloaded", false, false},
		"capacity":              {"The service is at capacity, retrying\n" + box(dim), "capacity", false, false},
		"limit wins": {"Agent terminated early due to an API error: You've hit your weekly limit · resets Oct 8\n" +
			box(dim), "", false, true},
		"unavailable": {notFound + "\n" + box(dim), "", true, false},
		"clean":       {"all good\n" + box("draft"), "", false, false},
	} {
		for _, eol := range eols {
			sc := claude.Read(eol.conv(c.screen), now)
			if sc.Error != c.err || sc.Unavailable != c.unavailable || (sc.Limit != nil) != c.limited {
				t.Errorf("%s (%s): Read = %+v", name, eol.name, sc)
			}
		}
	}
	for _, eol := range eols {
		if sc := claude.Read(eol.conv(box("draft")), now); sc.Draft != "draft" {
			t.Fatalf("Read keeps the draft (%s): %+v", eol.name, sc)
		}
	}
}

func TestNoticesAreOnlyTheClientsOwnLines(t *testing.T) {
	// A notice is a whole line in notice position (the last lines above the input box) that starts with
	// the client's phrase; prose, diffs and source that quote one are not (H2).
	// covers: R-WRK-040, R-WRK-051, R-SESS-009
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	quiet := func(name, screen string) {
		t.Helper()
		if sc := claude.Read(screen, now); sc.Error != "" || sc.Unavailable || sc.Limit != nil {
			t.Errorf("%s: read as a notice: %+v", name, sc)
		}
	}
	for name, line := range map[string]string{
		"overloaded prose":   "The queue was overloaded, so I retried the job.",
		"overloaded start":   "Overloaded workers are drained first.",
		"capacity prose":     "Raise the capacity of the buffer to 64.",
		"model file":         "The model file was not found in the cache directory.",
		"model prose":        "model: the flag is not found in this parser",
		"rate limit":         "Add a rate limit of 10 requests per second.",
		"limit prose":        "You've hit your target, the limit is 4am",
		"diff added":         "+\t\"You've hit your weekly limit · resets Oct 8\"",
		"diff removed":       "-  API Error: 500 internal",
		"numbered":           "  42  API Error: 500",
		"quoted":             "> API Error: 500",
		"agent prose marker": "● API Error responses are retried with backoff by the client.",
		"wrapped prose":      "  API Error 500 means the server failed, see the docs.",
		"prose model":        "● API Error: model name is not available in the docs",
		"own limit":          "  You've hit your own limit of patience",
		"limit after marker": "● You've hit your weekly limit · resets Oct 8",
		"worker line":        "  ⎿  ness worker: session stalled: API Error: 500",
		"worker limit":       "ness worker: You've hit your weekly limit · resets Oct 8",
		"typed hint":         "❯\u00a0You've hit your weekly limit · resets Oct 8",
	} {
		quiet(name, line+"\n"+box(dim))
	}
	for _, f := range []string{"screen.go", "screen_test.go"} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(src), "\n")
		for i := 0; i+12 <= len(lines); i += 6 {
			chunk := lines[i : i+12]
			numbered := make([]string, len(chunk))
			for j, l := range chunk {
				numbered[j] = fmt.Sprintf("%5d  %s", i+j+1, l)
			}
			quiet(f, strings.Join(chunk, "\n")+"\n"+box(dim))
			quiet(f+" numbered", strings.Join(numbered, "\n")+"\n"+box(dim))
		}
	}
}

func TestNoticeMustSitAboveTheBox(t *testing.T) {
	// A notice scrolled far above the box is old output; one just above it counts (H2).
	// covers: R-WRK-051
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	filler := strings.Repeat("work line\n", 20)
	fourth := "  ⎿  API Error: 500\na\nb\nc\n" + box(dim)
	if sc := claude.Read(fourth, now); sc.Error != "" {
		t.Errorf("a notice 4 lines up is old output: %+v", sc)
	}
	blanks := "  ⎿  API Error: 500\n\n" + rule + "\n\n" + box(dim)
	if sc := claude.Read(blanks, now); sc.Error != "API Error" {
		t.Errorf("blank and rule lines do not count: %+v", sc)
	}
	if sc := claude.Read("  ⎿  API Error: 500\n"+filler+box(dim), now); sc.Error != "" {
		t.Errorf("stale notice: %+v", sc)
	}
	if sc := claude.Read("  ⎿  API Error: 500\nsecond\n"+box(dim), now); sc.Error != "API Error" {
		t.Errorf("fresh notice: %+v", sc)
	}
	stale := "You've hit your weekly limit · resets Oct 8\n" + filler + box(dim)
	if sc := claude.Read(stale, now); sc.Limit != nil {
		t.Errorf("stale limit: %+v", sc)
	}
}

func TestUsageFileReportsAnActiveLimit(t *testing.T) {
	// covers: R-WRK-047
	home := t.TempDir()
	path := filepath.Join(home, ".claude", "usage-latest.json")
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	if _, ok := claude.Usage(home, now); ok {
		t.Fatal("no file reads as limited")
	}
	write := func(s string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	reset := now.Add(90 * time.Minute).Unix()
	at := strconv.FormatInt(reset, 10)
	write(`{"rate_limits":{"five_hour":{"used_percentage":100,"resets_at":` + at +
		`},"seven_day":{"used_percentage":87,"resets_at":1791777600}}}`)
	until, ok := claude.Usage(home, now)
	if !ok || until.Unix() != reset {
		t.Fatalf("Usage = %s, %v; want limited until %d", until, ok, reset)
	}
	if _, ok = claude.Usage(home, now.Add(2*time.Hour)); ok {
		t.Fatal("a reset in the past still limits")
	}
	write(`{"rate_limits":{"five_hour":{"used_percentage":40,"resets_at":` + at + `}}}`)
	if _, ok = claude.Usage(home, now); ok {
		t.Fatal("40% reads as limited")
	}
	write(`not json`)
	if _, ok = claude.Usage(home, now); ok {
		t.Fatal("garbage reads as limited")
	}
}

func FuzzRead(f *testing.F) {
	// The parsers never panic on any screen (R-WRK-041); seeds are the real notice formats.
	// covers: R-WRK-041
	for _, s := range []string{
		"You've hit your session limit · resets 2:10pm", "You've hit your session limit · resets 4am",
		"You've hit your session limit · resets 3:40am", "You've hit your weekly limit · resets Oct 1, 4am",
		"You've hit your weekly limit · resets Oct 8", "You've reached your Fable limit",
		"Agent terminated early due to an API error: You've hit your weekly limit · resets Oct 8",
		box(dim), box("draft"), realBox(grey), "\x1b[", "\x1b]0;",
		"You've hit your weekly limit · resets Feb 30, 25pm", "",
	} {
		f.Add(s)
	}
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	f.Fuzz(func(_ *testing.T, s string) {
		_ = claude.Read(s, now)
		_ = claude.InputBox(s)
		_, _ = claude.ParseLimit(s, now)
	})
}

func TestUnavailableIsTheClientsOwnLineOnly(t *testing.T) {
	// Each real phrase matches at column 0; the same text in a tool result, in indented prose, or
	// quoted by the worker's own report does not (H1, H2, R-SESS-009).
	// covers: R-SESS-009, R-WRK-051
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	for _, phrase := range []string{
		"There's an issue with the selected model (claude-opus-5-5). It may not exist or you may not have access.",
		"The model claude-opus-5-5 is not available on your Bedrock deployment.",
	} {
		if sc := claude.Read(phrase+"\n"+box(dim), now); !sc.Unavailable {
			t.Errorf("%q: not read as unavailable", phrase)
		}
		for name, line := range map[string]string{
			"tool result":    "  ⎿  " + phrase,
			"tool result 2":  "⎿ " + phrase,
			"indented prose": "  " + phrase,
			"own notice":     "ness worker: " + phrase,
			"bulleted":       "● " + phrase,
		} {
			if sc := claude.Read(line+"\n"+box(dim), now); sc.Unavailable {
				t.Errorf("%s: %q read as unavailable", name, line)
			}
		}
	}
	if sc := claude.Read("API Error: 404 not_found_error: model x not found\n"+box(dim), now); sc.Unavailable {
		t.Error("an API Error line alone is an error, not the unavailable notice")
	}
}
