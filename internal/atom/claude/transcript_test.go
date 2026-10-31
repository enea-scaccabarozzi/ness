package claude_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// transcriptAt writes content as the transcript of a session and sets its mtime.
func transcriptAt(t *testing.T, content string, mtime time.Time) (home, dir string) {
	t.Helper()
	home, dir = t.TempDir(), "/home/x/Projects/app/.worktrees/s-1"
	p := filepath.Join(home, ".claude", "projects", "-home-x-Projects-app--worktrees-s-1", "u-1.jsonl")
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	return home, dir
}

func readTranscript(t *testing.T, content string, mtime time.Time) session.Transcript {
	t.Helper()
	home, dir := transcriptAt(t, content, mtime)
	tr, err := fakeClient(t).Transcript(home, dir, "u-1")
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

// Rejects an implementation that keeps the file mtime, or the last timestamp of any entry
// (bookkeeping, meta, compaction summary), as the activity time.
func TestTranscriptActivityIsTheLastRealWorkEntry(t *testing.T) {
	// covers: R-SESS-080
	fixture, err := os.ReadFile(filepath.Join("testdata", "transcript-idle.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	mtime := time.Date(2026, 10, 13, 21, 25, 0, 0, time.UTC)
	tr := readTranscript(t, string(fixture), mtime)
	if want := time.Date(2026, 10, 13, 16, 14, 56, 0, time.UTC); !tr.LastActive.Equal(want) {
		t.Fatalf("LastActive = %v, want %v", tr.LastActive, want)
	}
	if tr.Context != 1010 {
		t.Fatalf("Context = %d, want 1010", tr.Context)
	}
}

// Rejects an implementation that returns the zero time (or fails) when no entry has a timestamp.
func TestTranscriptWithoutTimestampsFallsBackToMtime(t *testing.T) {
	// covers: R-SESS-080
	mtime := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	plain := strings.Join([]string{
		`{"type":"user","message":{"role":"user","content":"hi"}}`,
		`{"type":"assistant","message":{"usage":{"input_tokens":2,"cache_read_input_tokens":98}}}`,
		`not json at all`,
	}, "\n")
	if tr := readTranscript(t, plain, mtime); !tr.LastActive.Equal(mtime) || tr.Context != 100 {
		t.Fatalf("plain: %+v, want mtime %v and context 100", tr, mtime)
	}
	meta := `{"type":"last-prompt"}` + "\n" + `{"type":"custom-title"}` + "\n" +
		`{"type":"system","subtype":"turn_duration","timestamp":"2026-10-08T12:00:00Z"}`
	if tr := readTranscript(t, meta, mtime); !tr.LastActive.Equal(mtime) {
		t.Fatalf("metadata only: LastActive = %v, want mtime %v", tr.LastActive, mtime)
	}
}
