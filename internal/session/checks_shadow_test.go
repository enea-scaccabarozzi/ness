package session_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// covers: R-SESS-005, R-SESS-006, R-SESS-007, R-DOC-008
func TestShadowed(t *testing.T) {
	tmp := t.TempDir()
	project, global := filepath.Join(tmp, "proj"), filepath.Join(tmp, "global")
	for _, d := range []string{"proj/deploy", "proj/lint", "proj/p:x", "global/deploy", "global/lint", "global/p:x", "global/other"} {
		if err := os.MkdirAll(filepath.Join(tmp, d), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(tmp, "elsewhere")
	if err := os.MkdirAll(link, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(link, filepath.Join(project, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(global, "linked"), 0o750); err != nil {
		t.Fatal(err)
	}
	// The project's own skills dir is also a root (project at $HOME): no self-shadowing.
	got, err := session.Shadowed(project, []string{global, project})
	if err != nil {
		t.Fatal(err)
	}
	want := []session.Shadow{
		{Global: filepath.Join(global, "deploy"), Project: filepath.Join(project, "deploy")},
		{Global: filepath.Join(global, "linked"), Project: filepath.Join(project, "linked")},
		{Global: filepath.Join(global, "lint"), Project: filepath.Join(project, "lint")},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
