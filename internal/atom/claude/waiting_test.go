package claude_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/claude"
)

// Rejects a reader that ignores the client's own status line while a background agent runs (the
// pane reads idle then), and one that takes the same words in agent prose, or from history far
// above the box, for the status line. The fixture is the line as the client drew it
// (✻ Waiting for 1 background agent to finish) with its agent list, then the box.
func TestWaitingForBackgroundAgentSetsBackground(t *testing.T) {
	// covers: R-WRK-095
	data, err := os.ReadFile(filepath.Join("testdata", "waiting-agent.txt"))
	if err != nil {
		t.Fatal(err)
	}
	fixture := string(data)
	waiting := "✻ Waiting for 1 background agent to finish"
	if !strings.Contains(fixture, waiting) {
		t.Fatal("the fixture lost its status line")
	}
	cases := []struct {
		name, screen string
		want         bool
	}{
		{"one agent", fixture, true},
		{"three agents", strings.Replace(fixture, "1 background agent", "3 background agents", 1), true},
		{"no status line", strings.Replace(fixture, waiting+"\n", "", 1), false},
		{"in agent prose", strings.Replace(fixture, waiting, "● "+waiting, 1), false},
		{"quoted in a diff", strings.Replace(fixture, waiting, "+"+waiting, 1), false},
		{"far above the box", waiting + strings.Repeat("\nline", 20) + "\n" + strings.Join(strings.Split(fixture, "\n")[4:], "\n"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sc := claude.Read(c.screen, time.Time{})
			if sc.Background != c.want {
				t.Fatalf("Background = %v, want %v", sc.Background, c.want)
			}
			if c.want && (sc.NoPrompt || sc.Draft != "") {
				t.Fatalf("the box read changed: NoPrompt %v, Draft %q", sc.NoPrompt, sc.Draft)
			}
		})
	}
}

// Rejects a reader that misses the client's compaction status line, or takes the words in agent
// prose for it.
func TestCompactingConversationIsRead(t *testing.T) {
	// covers: R-WRK-081
	for name, c := range map[string]struct {
		screen string
		want   bool
	}{
		"status line":    {"some output\n✻ Compacting conversation… (15s)\n\n" + box(""), true},
		"in agent prose": {"● Compacting conversation… is what the client says\n\n" + box(""), false},
		"no line":        {box(""), false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := claude.Read(c.screen, time.Time{}).Compacting; got != c.want {
				t.Fatalf("Compacting = %v, want %v", got, c.want)
			}
		})
	}
}
