package claude_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/claude"
)

// activityNow is a clock shortly after the background fixture's records.
var activityNow = time.Date(2026, 10, 18, 3, 0, 0, 0, time.UTC)

// fixtureLines reads a testdata transcript as its lines.
func fixtureLines(t *testing.T, name string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

// writeLines writes the 1-based lines of the fixture, in the order given, as a transcript.
func writeLines(t *testing.T, lines []string, which ...int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "u-1.jsonl")
	var b strings.Builder
	for _, n := range which {
		b.WriteString(lines[n-1] + "\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// Rejects a reader that misses a kind of launch, counts a completion twice or not at all, treats an
// agent that stopped with background work of its own as finished, or reads a request blob that only
// quotes a launch, or a subagent's own records. Lines of testdata/transcript-background.jsonl:
// 1 async agent A launched, 2 background command B, 3 blob quoting a launch, 4 A finished (enqueued),
// 5 A finished (delivered), 6 agent C launched, 7 C waiting on its own background work, 8 A resumed,
// 9 B failed, 10 a sidechain launch, 11 a real turn, 12 a Read result quoting launches and
// notifications, 13 B stopped by TaskStop, 14 the client killed its agents, 15 a workflow launched,
// 16 the workflow finished (queued command).
func TestActivityPendingBackgroundWork(t *testing.T) {
	// covers: R-WRK-096
	lines := fixtureLines(t, "transcript-background.jsonl")
	cases := []struct {
		name  string
		which []int
		want  bool
	}{
		{"async agent launched", []int{1}, true},
		{"background command launched", []int{2}, true},
		{"agent finished, enqueued", []int{1, 4}, false},
		{"agent finished, delivered", []int{1, 5}, false},
		{"agent finished, one of two", []int{1, 2, 4}, true},
		{"agent and command finished", []int{1, 2, 4, 9}, false},
		{"failed command", []int{2, 9}, false},
		{"stopped with background work of its own", []int{6, 7}, true},
		{"resumed after finishing", []int{1, 4, 8}, true},
		{"resumed and finished again", []int{1, 4, 8, 5}, false},
		{"a request blob quoting a launch", []int{3}, false},
		{"a subagent's own launch", []int{10}, false},
		{"a turn alone", []int{11}, false},
		{"whole transcript", []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}, true}, // C and the resumed A
		{"a tool result quoting launches and notifications", []int{12}, false},
		{"quoted notifications do not finish a task", []int{1, 12}, true},
		{"command stopped", []int{2, 13}, false},
		{"stopping one leaves the other", []int{1, 2, 13}, true},
		{"the client killed its agents", []int{1, 2, 6, 14}, false},
		{"work launched after the kill", []int{1, 14, 15}, true},
		{"workflow launched", []int{15}, true},
		{"workflow finished", []int{15, 16}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			act, ok := (&claude.Transcripts{}).Activity(writeLines(t, lines, c.which...), activityNow)
			if !ok || act.Pending != c.want {
				t.Fatalf("Activity = %+v, %v; want Pending %v", act, ok, c.want)
			}
		})
	}
}

// Rejects launches that stay pending for ever: a notification lost to a client restart must not
// excuse the session from every report.
func TestActivityPendingExpires(t *testing.T) {
	// covers: R-WRK-096
	path := writeLines(t, fixtureLines(t, "transcript-background.jsonl"), 1)
	var tr claude.Transcripts
	if act, _ := tr.Activity(path, activityNow); !act.Pending {
		t.Fatal("a launch two hours ago is not pending")
	}
	if act, _ := tr.Activity(path, activityNow.Add(4*time.Hour)); act.Pending {
		t.Fatal("a launch 6 hours ago is still pending")
	}
}

// Rejects a reader that parses the file again on every call, or that consumes a line still being
// written: the worker reads every 5 seconds. The reader must pick up appended records, wait for the
// newline of a partial one, and start over when the file shrinks.
func TestActivityReadsAppendedRecordsOnly(t *testing.T) {
	// covers: R-WRK-096
	lines := fixtureLines(t, "transcript-background.jsonl")
	path := writeLines(t, lines, 1)
	var tr claude.Transcripts
	if act, _ := tr.Activity(path, activityNow); !act.Pending {
		t.Fatal("not pending after the launch")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck // test file
	done := lines[3]
	if _, err = f.WriteString(done[:len(done)/2]); err != nil { // half a line, no newline
		t.Fatal(err)
	}
	if act, _ := tr.Activity(path, activityNow); !act.Pending {
		t.Fatal("a partial line was taken as a record")
	}
	if _, err = f.WriteString(done[len(done)/2:] + "\n"); err != nil {
		t.Fatal(err)
	}
	if act, _ := tr.Activity(path, activityNow); act.Pending {
		t.Fatal("the completion was not read once its line was whole")
	}
	if err = os.WriteFile(path, []byte(lines[1]+"\n"), 0o600); err != nil { // shorter: a new file
		t.Fatal(err)
	}
	if act, _ := tr.Activity(path, activityNow); !act.Pending {
		t.Fatal("a rewritten transcript kept the old state")
	}
}

// Rejects a reader that keeps what it read of a file for ever: a transcript not asked for in an
// hour is forgotten, so the next read starts from its first line.
func TestActivityForgetsIdleFiles(t *testing.T) {
	// covers: R-WRK-096
	lines := fixtureLines(t, "transcript-background.jsonl")
	path := writeLines(t, lines, 2)
	var tr claude.Transcripts
	first := time.Date(2026, 10, 18, 2, 0, 0, 0, time.UTC)
	if act, _ := tr.Activity(path, first); !act.Pending {
		t.Fatal("not pending after the launch")
	}
	// a different, longer file at the same path: only a reader that forgot the first one reads it whole
	if err := os.WriteFile(path, []byte(lines[11]+"\n"+lines[11]+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if act, _ := tr.Activity(path, first.Add(90*time.Minute)); act.Pending {
		t.Fatal("state of a file not asked for in an hour was kept")
	}
}

// Rejects a missing transcript read as "no activity": the worker must fall back to the screen.
func TestActivityMissingFile(t *testing.T) {
	// covers: R-WRK-096
	if _, ok := (&claude.Transcripts{}).Activity(filepath.Join(t.TempDir(), "none.jsonl"), activityNow); ok {
		t.Fatal("ok for a missing transcript")
	}
}

// Rejects a limit detector that stays on after a real turn, that reads any API error as a limit, or
// that misses the real record (a synthetic assistant message with isApiErrorMessage and 429). Lines
// of testdata/transcript-fable-limit.jsonl: 1 a turn, 2 the hint, 3 the Fable limit error, 4 a
// system record, 5 an overloaded error, 6 a turn after the reset.
func TestActivityUsageLimit(t *testing.T) {
	// covers: R-WRK-097
	lines := fixtureLines(t, "transcript-fable-limit.jsonl")
	cases := []struct {
		name      string
		which     []int
		limit     bool
		turn      string
		limitedAt string
	}{
		{"only a turn", []int{1}, false, "2026-10-19T21:50:00Z", ""},
		{"the limit error ends it", []int{1, 2, 3}, true, "2026-10-19T21:50:00Z", "2026-10-19T21:58:11.734Z"},
		{"system records after it", []int{1, 2, 3, 4}, true, "2026-10-19T21:50:00Z", "2026-10-19T21:58:11.734Z"},
		{"another API error after it", []int{1, 2, 3, 4, 5}, true, "2026-10-19T21:50:00Z", "2026-10-19T21:58:11.734Z"},
		{"an overloaded error alone", []int{1, 5}, false, "2026-10-19T21:50:00Z", ""},
		{"a real turn clears it", []int{1, 2, 3, 4, 5, 6}, false, "2026-10-20T08:00:00Z", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			act, ok := (&claude.Transcripts{}).Activity(writeLines(t, lines, c.which...), activityNow)
			if !ok || (act.Limit != "") != c.limit {
				t.Fatalf("Activity = %+v, %v; want limit %v", act, ok, c.limit)
			}
			if c.limit && (!strings.HasPrefix(act.Limit, "You've reached your Fable limit") ||
				!act.LimitAt.Equal(mustTime(t, c.limitedAt))) {
				t.Fatalf("limit %q at %v, want Fable at %s", act.Limit, act.LimitAt, c.limitedAt)
			}
			if !act.Turn.Equal(mustTime(t, c.turn)) {
				t.Fatalf("Turn = %v, want %s: an error is not a turn", act.Turn, c.turn)
			}
		})
	}
}

// Rejects a compaction counted as a real turn: the summary and the boundary are not the agent
// working, and a re-armed alert follows when they are.
func TestActivityCompactionIsNotATurn(t *testing.T) {
	// covers: R-WRK-096
	const turn = `{"type":"assistant","timestamp":"2026-10-15T14:00:00Z","message":{"content":[{"type":"text","text":"x"}]}}`
	const summary = `{"type":"assistant","isCompactSummary":true,"timestamp":"2026-10-15T14:05:00Z",` +
		`"message":{"content":[{"type":"text","text":"summary"}]}}`
	const boundary = `{"type":"system","subtype":"compact_boundary","timestamp":"2026-10-15T14:05:01Z"}`
	path := filepath.Join(t.TempDir(), "u-1.jsonl")
	if err := os.WriteFile(path, []byte(turn+"\n"+summary+"\n"+boundary+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	act, ok := (&claude.Transcripts{}).Activity(path, activityNow)
	if !ok || !act.Turn.Equal(mustTime(t, "2026-10-15T14:00:00Z")) {
		t.Fatalf("Turn = %v, %v; want the turn before the compaction", act.Turn, ok)
	}
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	if s == "" {
		return time.Time{}
	}
	at, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatal(err)
	}
	return at
}
