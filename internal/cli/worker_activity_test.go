package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/claude"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// Rejects a worker whose claude client reads no transcript, or one looked up in the wrong project
// directory: the reading must come from the session's own worktree under the user's home.
func TestWorkerReadsTheSessionsTranscript(t *testing.T) {
	// covers: R-WRK-096
	home, primary := t.TempDir(), "/home/x/Projects/app"
	t.Setenv("HOME", home)
	sess := store.Session{ID: "s-1", ClientSession: "u-1", Worktree: ".worktrees/s-1"}
	path := claude.TranscriptPath(home, filepath.Join(primary, sess.Worktree), "u-1")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	launch := `{"type":"user","timestamp":"2026-10-18T01:00:00Z","message":{"content":[]},"toolUseResult":{"backgroundTaskId":"bq7x2kd3m"}}` + "\n"
	if err := os.WriteFile(path, []byte(launch), 0o600); err != nil {
		t.Fatal(err)
	}
	c := withActivity(workerClients(Deps{}), primary)["claude"]
	act, ok := c.Activity(sess, time.Date(2026, 10, 18, 2, 0, 0, 0, time.UTC))
	if !ok || !act.Pending {
		t.Fatalf("Activity = %+v, %v; want pending", act, ok)
	}
	if _, ok = c.Activity(store.Session{ID: "s-2"}, time.Time{}); ok {
		t.Fatal("a session with no conversation id has a reading")
	}
}
