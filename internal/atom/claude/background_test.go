package claude_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/claude"
)

const shellFooter = "  ⏵⏵ bypass permissions on · 1 shell · ← for agents"

// footerFixture is the real footer captured 2026-10-15: an empty prompt box, a status line and
// the permission line that counts live background shells.
func footerFixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "footer-1-shell.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), shellFooter) {
		t.Fatal("the fixture lost its footer")
	}
	return string(data)
}

// Rejects a reader that ignores the count, or that matches only the singular.
func TestFooterWithShellsSetsBackground(t *testing.T) {
	// covers: R-WRK-095
	fixture := footerFixture(t)
	for name, footer := range map[string]string{
		"one shell":    shellFooter,
		"three shells": strings.Replace(shellFooter, "1 shell", "3 shells", 1),
	} {
		t.Run(name, func(t *testing.T) {
			sc := claude.Read(strings.Replace(fixture, shellFooter, footer, 1), time.Time{})
			if !sc.Background {
				t.Fatal("Background = false, want true")
			}
			if sc.NoPrompt || sc.Draft != "" {
				t.Fatalf("the box read changed: NoPrompt %v, Draft %q", sc.NoPrompt, sc.Draft)
			}
		})
	}
}

// Rejects a reader that reports a background task when the footer shows none, and one that scans
// agent prose instead of the footer.
func TestFooterWithoutShellsIsNotBackground(t *testing.T) {
	// covers: R-WRK-095
	fixture := footerFixture(t)
	none := strings.Replace(fixture, " · 1 shell", "", 1)
	prose := "The log says · 1 shell stayed alive after the run.\n" + none
	for name, screen := range map[string]string{
		"no shell count":                       none,
		"a shell in agent prose above the box": prose,
		"the word shell without a count":       strings.Replace(fixture, "1 shell", "shell", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if sc := claude.Read(screen, time.Time{}); sc.Background {
				t.Fatal("Background = true, want false")
			}
		})
	}
}

// Rejects a reader that needs a middot before the count (F10): the footer of the other permission
// modes may lead with it. The footers are synthetic: no capture of them exists yet, and a real one
// should replace each row as it is taken.
func TestFooterOfOtherPermissionModesSetsBackground(t *testing.T) {
	// covers: R-WRK-095
	fixture := footerFixture(t)
	for name, footer := range map[string]string{
		"default mode, count first": "  1 shell · ? for shortcuts",
		"plan mode":                 "  ⏸ plan mode on · 1 shell · ← for agents",
		"accept edits":              "  ⏵⏵ accept edits on · 2 shells",
		"the count alone":           "  1 shell",
	} {
		t.Run(name, func(t *testing.T) {
			if sc := claude.Read(strings.Replace(fixture, shellFooter, footer, 1), time.Time{}); !sc.Background {
				t.Fatal("Background = false, want true")
			}
		})
	}
}
