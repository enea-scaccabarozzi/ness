package worker_test

import (
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// backgroundScreen is an idle prompt whose footer shows one live background shell.
const backgroundScreen = "assistant output\n╭────────╮\n│ > " + placeholder + "\n╰────────╯\n" +
	"  ⏵⏵ bypass permissions on · 1 shell · ← for agents\n"

// fyis keeps the fyi items: the stall messages, not the handoffs and requests around them.
func fyis(items []store.Item) []store.Item {
	var out []store.Item
	for _, it := range items {
		if it.Intent == store.IntentFYI {
			out = append(out, it)
		}
	}
	return out
}

// Rejects an implementation that waits for an obligation, reports before the idle clock, reports
// again inside an episode, or never starts a second episode.
func TestChildWithoutHandoffStallsOncePerEpisode(t *testing.T) {
	// covers: R-WRK-050, R-WRK-052
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.pass()
	b.passes(29, time.Minute)
	if got := fyis(b.listed(sA)); len(got) != 0 {
		t.Fatalf("at 29 min A has %+v", got)
	}
	b.passes(1, time.Minute)
	got := fyis(b.listed(sA))
	if len(got) != 1 || got[0].Sender != sB || !strings.Contains(got[0].Body, "has not handed off") {
		t.Fatalf("at 30 min A has %+v", got)
	}
	b.passes(30, time.Minute)
	if got := fyis(b.listed(sA)); len(got) != 1 {
		t.Fatalf("at 60 min A has %d fyi items", len(got))
	}
	b.pane(sB).status = "working"
	b.passes(1, time.Minute)
	b.pane(sB).status = "idle"
	b.passes(31, time.Minute)
	if got := fyis(b.listed(sA)); len(got) != 2 {
		t.Fatalf("after a second episode A has %d fyi items", len(got))
	}
}

// Rejects an implementation that nudges the child it reports on.
func TestChildStallReportNeverTypesIntoTheChild(t *testing.T) {
	// covers: R-WRK-050
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.pass()
	b.passes(40, time.Minute)
	if got := fyis(b.listed(sA)); len(got) != 1 {
		t.Fatalf("A has %d fyi items, want the stall message", len(got))
	}
	for _, text := range b.typedTo(sB) {
		if strings.Contains(text, "idle since") {
			t.Fatalf("stall text typed into the child: %q", text)
		}
	}
}

// Rejects an implementation that treats a handed-off child as still owing its brief, in any
// disposition of the handoff.
func TestHandedOffChildDoesNotStall(t *testing.T) {
	// covers: R-WRK-050
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.send(sB, sA, store.IntentHandoff, "done", false)
	b.pass()
	b.passes(90, time.Minute)
	if got := fyis(b.listed(sA)); len(got) != 0 {
		t.Fatalf("a handed-off child stalled: %+v", got)
	}
}

// Rejects an implementation that applies the brief rule to a root, whose manager is the operator.
func TestRootWithNothingOwedDoesNotStall(t *testing.T) {
	// covers: R-WRK-050
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pass()
	b.passes(90, time.Minute)
	if got := b.listed(store.Operator); len(got) != 0 {
		t.Fatalf("the operator got %+v for an idle root", got)
	}
}

// Rejects an implementation that starts the idle clock on a child that is still launching.
func TestLaunchingChildDoesNotStall(t *testing.T) {
	// covers: R-WRK-050
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	if err := b.s.ClearReady(sB); err != nil {
		t.Fatal(err)
	}
	b.pass()
	b.passes(90, time.Minute)
	if got := fyis(b.listed(sA)); len(got) != 0 {
		t.Fatalf("a launching child stalled: %+v", got)
	}
}

// Rejects an implementation that ignores the background shell on the screen (R2), and one that
// ignores it forever: once the shell is gone the child is idle again and stalls.
func TestChildWithLiveBackgroundShellIsNotStalled(t *testing.T) {
	// covers: R-WRK-050
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.pane(sB).screen = backgroundScreen
	b.pass()
	b.passes(90, time.Minute)
	if got := fyis(b.listed(sA)); len(got) != 0 {
		t.Fatalf("a child with a live background shell stalled: %+v", got)
	}
	if got := b.listed(store.Operator); len(got) != 0 {
		t.Fatalf("the operator was alerted for a working child: %+v", got)
	}
	b.pane(sB).screen = idleScreen
	b.passes(31, time.Minute)
	if got := fyis(b.listed(sA)); len(got) != 1 {
		t.Fatalf("after the shell ended A has %d fyi items, want 1", len(got))
	}
}
