package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/claude"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// Rejects a transcript lookup that, for a root adopted with --here, only ever looks in the primary
// checkout's project directory: a conversation started inside the root's worktree keeps its
// transcript under the worktree's directory, and every reader must find it there.
func TestTranscriptDirOfAnAdoptedRoot(t *testing.T) {
	// covers: R-SESS-114
	primary := "/home/x/Projects/app"
	sess := store.Session{ID: "s-1", ClientSession: "u-1", Worktree: ".worktrees/s-1", Adopted: true}
	wt := filepath.Join(primary, sess.Worktree)
	for _, tc := range []struct {
		name, startedIn, want string
	}{
		{"started in the checkout", primary, primary},
		{"started in the worktree", wt, wt},
		{"no transcript yet", "", primary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			if tc.startedIn != "" {
				path := claude.TranscriptPath(home, tc.startedIn, sess.ClientSession)
				if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if got := transcriptDir(primary, sess); got != tc.want {
				t.Fatalf("transcriptDir = %s; want %s", got, tc.want)
			}
		})
	}
	if got := transcriptDir(primary, store.Session{Worktree: ".worktrees/s-2"}); got != filepath.Join(primary, ".worktrees/s-2") {
		t.Fatalf("a launched child reads %s; want its worktree", got)
	}
}
