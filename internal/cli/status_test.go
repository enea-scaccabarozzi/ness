package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/ui"
)

func TestPaintOnlyOnATerminal(t *testing.T) {
	// A paused probe is red on a terminal and plain anywhere else, or with NO_COLOR.
	// covers: R-SESS-083
	if got := paint(Deps{}, "paused"); got != "paused" {
		t.Fatalf("pipe: %q", got)
	}
	tty := Deps{Term: ui.Term{StdoutTTY: true}}
	if got := paint(tty, "paused"); got != "\x1b[31mpaused\x1b[0m" {
		t.Fatalf("terminal: %q", got)
	}
	tty.Term.Getenv = func(k string) string { return map[string]string{"NO_COLOR": "1"}[k] }
	if got := paint(tty, "paused"); got != "paused" {
		t.Fatalf("NO_COLOR: %q", got)
	}
}

func TestProbeLines(t *testing.T) {
	// A probe line carries state flags, the last state or a dash, and a one-line excerpt; the
	// feature on with no probe says none.
	// covers: R-SESS-081
	var out bytes.Buffer
	d := Deps{Stdout: &out}
	printProbes(d, []probeJSON{}, true)
	printProbes(d, []probeJSON{{
		Path: "p.md", Owner: "s-1", State: "active", NeedsEdit: true, FileError: "x", Verdict: "ok",
		Evidence: "a\nb  " + strings.Repeat("c", 100),
	}}, true)
	want := "probes: none\nprobe\tp.md\ts-1\tactive needs-edit file error verdict ok\t-\ta b " +
		strings.Repeat("c", 76) + "\n"
	if out.String() != want {
		t.Fatalf("got %q\nwant %q", out.String(), want)
	}
}

func TestRichPausedProbeIsRed(t *testing.T) {
	// The rich status draws a paused probe in the fail color, an active one in the ok color.
	// covers: R-UI-027, R-SESS-083
	th := ui.Settings{Rich: true, Width: 100, Dark: true}.Theme()
	out := probeBlock(th, []probeJSON{
		{Path: "a.md", Owner: "s-1", State: store.ProbePaused},
		{Path: "b.md", Owner: "s-1", State: "active"},
	}, true)
	if !strings.Contains(out, th.Pill(store.ProbePaused, "fail")) || !strings.Contains(out, th.Pill("active", "ok")) {
		t.Fatalf("probe states not colored:\n%q", out)
	}
}

func TestTreePrefixes(t *testing.T) {
	// A root has no connector; the last child of a parent closes the branch; a grandchild under a
	// parent with a later sibling keeps the bar.
	// covers: R-UI-027
	rows := []sessionJSON{{Depth: 0}, {Depth: 1}, {Depth: 2}, {Depth: 1}, {Depth: 2}, {Depth: 0}}
	want := []string{"", "├─ ", "│  └─ ", "└─ ", "   └─ ", ""}
	got := treePrefixes(rows)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row %d: %q, want %q (all %q)", i, got[i], want[i], got)
		}
	}
}
