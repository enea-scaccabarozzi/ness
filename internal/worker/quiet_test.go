package worker_test

import (
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestQuietWindowRunsFromTheBoxChange(t *testing.T) {
	// A draft seen at t=0 and cleared at t=5 s: no non-urgent hint before t=65 s, the hint at
	// the first pass after. The window runs from the change, not from the draft's first sight.
	// covers: R-WRK-018, R-WRK-019
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).screen = draftScreen
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.pass() // t=0: the draft
	b.pane(sA).screen = idleScreen
	for sec := 5; sec <= 60; sec += 5 { // the box is cleared at t=5 s
		b.passes(1, 5*time.Second)
		if got := b.typedTo(sA); len(got) != 0 {
			t.Fatalf("hinted at t=%ds, inside the window: %q", sec, got)
		}
	}
	b.passes(2, 5*time.Second) // t=70 s at the latest
	if got := b.typedTo(sA); len(got) != 1 {
		t.Fatalf("not hinted after the window: %q", got)
	}
}

func TestEditingTheDraftExtendsTheWindow(t *testing.T) {
	// The window counts from the last change of the box, so a draft retyped at t=35 s holds a
	// hint that the first draft's clearing alone would have released at t=65 s.
	// covers: R-WRK-018
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.pane(sA).screen = draftScreen
	b.pass() // t=0
	b.pane(sA).screen = idleScreen
	b.passes(6, 5*time.Second) // cleared at t=5 s; now t=30 s
	b.pane(sA).screen = draftScreen
	b.passes(1, 5*time.Second) // retyped at t=35 s
	b.pane(sA).screen = idleScreen
	b.passes(1, 5*time.Second) // cleared at t=40 s
	b.passes(5, 5*time.Second) // t=65 s: past the first window, inside the second
	if got := b.typedTo(sA); len(got) != 0 {
		t.Fatalf("hinted inside the window of the last change: %q", got)
	}
	b.passes(8, 5*time.Second) // t=105 s, past the window of the change at t=40 s
	if got := b.typedTo(sA); len(got) != 1 {
		t.Fatalf("not hinted after the last change's window: %q", got)
	}
}

func TestFocusedEmptyBoxGetsEverythingInTheFirstPass(t *testing.T) {
	// A focused pane with an empty, never-changed box gets the compaction in the first pass,
	// with no log line about focus.
	// covers: R-WRK-017, R-WRK-019, R-WRK-030
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).focused = true
	if err := b.s.SetBoundary(sA, "n", time.Time{}, b.now); err != nil {
		t.Fatal(err)
	}
	b.pass()
	if got := b.typedTo(sA); len(got) != 1 || got[0] != "/compact n" {
		t.Fatalf("typed %q, want the compaction in the first pass", got)
	}
	if b.logCount("focused") != 0 {
		t.Fatalf("a focus line was logged:\n%v", b.logLines())
	}
}

func TestNothingTypedWhileThePromptIsPending(t *testing.T) {
	// A session with a ready mark whose launch is still at sending-brief has not been given its
	// first prompt (a trust dialog accepted after a stall): the worker types nothing into it.
	// covers: R-WRK-013, R-SESS-025
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	if err := b.s.SetPhase(sA, "sending-brief", b.now); err != nil {
		t.Fatal(err)
	}
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.passes(5, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 0 {
		t.Fatalf("typed into a session that has no prompt yet: %q", got)
	}
	if err := b.s.SetPhase(sA, "ready", b.now); err != nil {
		t.Fatal(err)
	}
	b.passes(1, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 1 {
		t.Fatalf("not typed once the launch is ready: %q", got)
	}
}

func TestSubmittedDraftStartsNoQuietWindow(t *testing.T) {
	// The operator typed a draft and sent it: the session works, and on its first idle pass the
	// box is empty. The submission is not a clearing; the hint lands in that pass.
	// covers: R-WRK-018, R-WRK-019
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.pane(sA).screen = draftScreen
	b.pass()
	b.pane(sA).status, b.pane(sA).screen = "working", idleScreen
	b.passes(24, 5*time.Second) // two minutes of work: the window the typing opened is over
	b.pane(sA).status = "idle"
	b.passes(1, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 1 {
		t.Fatalf("not hinted in the first idle pass after a submitted draft: %q", got)
	}
}

func TestLaunchPromptInTheBoxIsNeverTypedOverOrSubmitted(t *testing.T) {
	// A ready session's idle pane whose box holds its own launch prompt as normal-intensity text (the
	// client dropped the launch's Enter): the worker types nothing and presses nothing, however long
	// it waits and whether or not a hint is pending.
	// covers: R-SESS-151, R-WRK-014
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	prompt := session.Prompt(sA, "x", "docs/sessions/x")
	b.pane(sA).screen = "assistant output\n╭────────╮\n│ > " + prompt + "\n╰────────╯\n"
	b.send(store.Operator, sA, store.IntentRequest, "x", true)
	b.passes(30, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 0 {
		t.Fatalf("typed over the launch prompt: %q", got)
	}
	if len(b.rt.enters) != 0 || len(b.rt.typed) != 0 {
		t.Fatalf("worker acted on the box: enters %q, typed %q", b.rt.enters, b.rt.typed)
	}
}
