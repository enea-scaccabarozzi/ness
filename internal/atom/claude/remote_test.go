package claude_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/claude"
)

// remoteLine is a remote_session_change attachment as Claude Code 2.1 writes it (trimmed).
func remoteLine(url string, sidechain bool) string {
	side := "false"
	if sidechain {
		side = "true"
	}
	return `{"isSidechain":` + side + `,"attachment":{"type":"remote_session_change","url":"` + url +
		`","commit":"x"},"type":"attachment","timestamp":"2026-10-29T16:16:05.773Z"}`
}

// Rejects a reader that misses the Remote Control URL, keeps the first one instead of the last,
// takes one from a subagent or one that would break a Markdown link, or forgets it on a later read
// that appends nothing new about it.
func TestActivityRemoteURL(t *testing.T) {
	// covers: R-WRK-096
	const a, b = "https://claude.ai/code/session_01AAA", "https://claude.ai/code/session_01BBB"
	const turn = `{"type":"assistant","timestamp":"2026-10-29T16:20:00Z","message":{"content":[{"type":"text","text":"x"}]}}`
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"none", []string{turn}, ""},
		{"one", []string{remoteLine(a, false), turn}, a},
		{"the last wins", []string{remoteLine(a, false), turn, remoteLine(b, false)}, b},
		{"a subagent's is ignored", []string{remoteLine(a, false), remoteLine(b, true)}, a},
		{"not https", []string{remoteLine("javascript:alert(1)", false)}, ""},
		{"would break a link", []string{remoteLine(a, false), remoteLine("https://x.invalid/a)b", false)}, a},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "u-1.jsonl")
			var body string
			for _, l := range c.lines {
				body += l + "\n"
			}
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			var tr claude.Transcripts
			if act, ok := tr.Activity(path, activityNow); !ok || act.Remote != c.want {
				t.Fatalf("Remote = %q, %v; want %q", act.Remote, ok, c.want)
			}
			if err := os.WriteFile(path, []byte(body+turn+"\n"), 0o600); err != nil { // appended: read incrementally
				t.Fatal(err)
			}
			if act, _ := tr.Activity(path, activityNow); act.Remote != c.want {
				t.Fatalf("after an append Remote = %q; want %q", act.Remote, c.want)
			}
		})
	}
}
