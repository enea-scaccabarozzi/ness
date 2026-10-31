package worker_test

import (
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

// Rejects an implementation that reads only descendants (the review's probe: a root with its own
// background shell got Your move), and one that ignores the shell ending.
func TestYourMoveSuppressedByTheRootsOwnBackgroundShell(t *testing.T) {
	// covers: R-WRK-093
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.pane(sA).screen = backgroundScreen
	b.pass()
	b.passes(40, time.Minute)
	if got := a.of(worker.AlertYourMove); len(got) != 0 {
		t.Fatalf("Your move with a live shell: %+v", got)
	}
	b.pane(sA).screen = idleScreen
	b.passes(31, time.Minute)
	if got := a.of(worker.AlertYourMove); len(got) != 1 {
		t.Fatalf("after the shell ended: %+v", a.got)
	}
}

// Rejects an implementation that alerts before the draft check (the review's probe: a half-typed
// box got a phone alert), and one that forgets the alert once the box is clear and quiet.
func TestYourMoveWaitsForADraftAndTheQuietWindow(t *testing.T) {
	// covers: R-WRK-093, R-WRK-018
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.pane(sA).screen = draftScreen
	b.pass()
	b.passes(40, time.Minute)
	if got := a.of(worker.AlertYourMove); len(got) != 0 {
		t.Fatalf("Your move over a draft: %+v", got)
	}
	b.pane(sA).screen = idleScreen
	b.passes(1, 5*time.Second)
	if got := a.of(worker.AlertYourMove); len(got) != 0 {
		t.Fatalf("Your move inside the quiet window: %+v", got)
	}
	b.passes(13, 5*time.Second)
	if got := a.of(worker.AlertYourMove); len(got) != 1 {
		t.Fatalf("after the window: %+v", a.got)
	}
}

// Each case rejects the doubled alert the review reproduced: one cause, one alert.
func TestOneCauseOneAlert(t *testing.T) {
	// covers: R-WRK-093, R-WRK-094
	cases := []struct {
		name  string
		setup func(b *bench)
		want  worker.AlertKind
	}{
		{"an idle root holding a child's handoff: the stall covers it", func(b *bench) {
			b.session(sB, sA, "claude")
			b.send(sB, sA, store.IntentHandoff, "done", false)
		}, worker.AlertTrouble},
		{"a dead child owing its brief: its stall covers the idle root", func(b *bench) {
			b.session(sB, sA, "claude")
		}, worker.AlertTrouble},
		{"the overnight case: the last handoff accepted, the root asked only in chat", func(b *bench) {
			b.session(sB, sA, "claude")
			h := b.send(sB, sA, store.IntentHandoff, "done", false)
			if _, err := b.s.Reply(h, sA, "merged", store.Resolved, b.now); err != nil {
				b.t.Fatal(err)
			}
		}, worker.AlertYourMove},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := newBench(t)
			a := b.alerts()
			b.session(sA, store.Operator, "claude")
			c.setup(b)
			b.pass()
			b.passes(70, time.Minute) // a stall's Trouble comes at the second miss (R-WRK-094)
			if len(a.got) != 1 || a.got[0].Kind != c.want {
				t.Fatalf("alerts = %+v, want exactly one of kind %s", a.got, c.want)
			}
		})
	}
}

// Rejects an implementation that spends the stop alert on a failed post (the review: the tab is
// closed and the session never visited again) or that delays the stop behind the post.
func TestStopAlertIsRetriedAfterAFailedPost(t *testing.T) {
	// covers: R-WRK-083, R-WRK-094
	b := newBench(t)
	a := b.alerts()
	a.fail = 1
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.titled(sB, "child-b")
	b.pane(sB).screen = unavailScreen
	b.send(sA, sB, store.IntentRequest, "work", false)
	b.passes(3, 5*time.Second)
	if len(b.rt.closed) != 1 {
		t.Fatalf("the stop waited for the post: closed %q", b.rt.closed)
	}
	b.passes(10, 5*time.Second)
	troubles := a.of(worker.AlertTrouble)
	if len(troubles) != 1 || !strings.Contains(troubles[0].Text, "model") {
		t.Fatalf("alerts = %+v", a.got)
	}
	if got := stallFyis(b, sA, sB); len(got) != 1 {
		t.Fatalf("the retry sent the manager %d items, want 1", len(got))
	}
	b.passes(20, 5*time.Second)
	if len(a.of(worker.AlertTrouble)) != 1 {
		t.Fatalf("repeated: %+v", a.got)
	}
}

// Rejects the literal first line (the review: a Markdown heading or a blank line made the alert
// useless): the first non-blank line, leading # and spaces stripped.
func TestNeedsYouTextIsTheFirstNonBlankLineWithoutHeadingMarks(t *testing.T) {
	// covers: R-WRK-080
	for body, want := range map[string]string{
		"# Decision: go?\nmore":    "Decision: go?",
		"\n\n## Context\nbody":     "Context",
		"  \n   plain line  \nx":   "plain line",
		"#nospace heading\nx":      "nospace heading",
		"Is 2 # 3 ok?\n# not this": "Is 2 # 3 ok?",
	} {
		b := newBench(t)
		a := b.alerts()
		b.session(sA, store.Operator, "claude")
		b.pane(sA).status = "working"
		b.send(sA, store.Operator, store.IntentDecision, body, false)
		b.pass()
		if len(a.got) != 1 || a.got[0].Text != want {
			t.Fatalf("body %q: alerts = %+v, want text %q", body, a.got, want)
		}
	}
}
