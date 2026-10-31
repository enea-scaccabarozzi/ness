package worker_test

import (
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/claude"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

const fableLimit = "You've reached your Fable limit"

// client is the claude client with a hand-written transcript reading: b.acts per session.
func (b *bench) client() worker.Client {
	return worker.Client{
		Read: claude.Read, Resume: claude.ResumeText, Rename: claude.RenameText,
		Activity: func(sess store.Session, _ time.Time) (session.Activity, bool) {
			if b.readFor == nil {
				b.readFor = map[string]string{}
			}
			b.readFor[sess.ID] = sess.ClientSession
			act, ok := b.acts[sess.ID]
			return act, ok
		},
	}
}

// act sets what the transcript of the session says from now on.
func (b *bench) act(id string, a session.Activity) {
	if b.acts == nil {
		b.acts = map[string]session.Activity{}
	}
	b.acts[id] = a
}

// working runs one pass with the pane working, then leaves it idle again.
func (b *bench) workedOnce(id string) {
	b.pane(id).status = "working"
	b.passes(1, time.Minute)
	b.pane(id).status = "idle"
}

// Rejects the field-2 false positives: 22 stall reports about sessions whose subagent or shell was
// running while the pane read idle. Whatever the report would be about (the owed brief, an open
// obligation), pending background work in the transcript excuses the child, and the report comes
// once the work is finished.
func TestPendingBackgroundWorkIsNotAStall(t *testing.T) {
	// covers: R-WRK-050, R-WRK-096
	for name, setup := range map[string]func(b *bench){
		"owed brief": func(*bench) {},
		"open obligation": func(b *bench) {
			b.send(sA, sB, store.IntentRequest, "do the thing", false)
		},
	} {
		t.Run(name, func(t *testing.T) {
			b := newBench(t)
			a := b.alerts()
			b.session(sA, store.Operator, "claude")
			b.session(sB, sA, "claude")
			setup(b)
			b.act(sB, session.Activity{Pending: true})
			b.pass()
			b.passes(90, time.Minute)
			if got := fyis(b.listed(sA)); len(got) != 0 {
				t.Fatalf("stall reports while a subagent runs: %+v", got)
			}
			if len(a.got) != 0 {
				t.Fatalf("alerts while a subagent runs (the root's Your move too): %+v", a.got)
			}
			b.act(sB, session.Activity{})
			b.passes(61, time.Minute) // the Trouble alert comes at the second miss (R-WRK-094)
			if got := fyis(b.listed(sA)); len(got) != 1 {
				t.Fatalf("after the work finished A has %+v, want one stall report", got)
			}
			if got := a.of(worker.AlertTrouble); len(got) != 1 {
				t.Fatalf("Trouble alerts = %+v, want one", a.got)
			}
		})
	}
}

// Rejects the loop that made the stall reports self-feeding: a report, the manager's hint, a
// brief turn of the child, and the next report 30 minutes later while its subagent runs. After the
// hint the child launches background work and idles: no second report while it runs.
func TestHintedChildWithPendingWorkIsNotReportedAgain(t *testing.T) {
	// covers: R-WRK-050, R-WRK-052, R-WRK-096
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.act(sB, session.Activity{Turn: b.now})
	b.pass()
	b.passes(31, time.Minute)
	if got := fyis(b.listed(sA)); len(got) != 1 {
		t.Fatalf("first stall: %+v", got)
	}
	b.send(sA, sB, store.IntentFYI, "go on", false) // the manager's answer, hinted into the child
	b.act(sB, session.Activity{Turn: b.now.Add(time.Minute), Pending: true})
	b.workedOnce(sB)
	b.passes(120, time.Minute)
	if got := fyis(b.listed(sA)); len(got) != 1 {
		t.Fatalf("a child waiting on its subagent was reported again: %+v", got)
	}
}

// Rejects an episode reset by any blip of work (field-2: an auto-compaction re-armed Your move and
// the stall report), and one that never resets: only a real assistant turn starts a new episode.
func TestOnlyARealTurnStartsANewEpisode(t *testing.T) {
	// covers: R-WRK-052, R-WRK-093, R-WRK-096
	t.Run("your move", func(t *testing.T) {
		b := newBench(t)
		a := b.alerts()
		b.session(sA, store.Operator, "claude")
		b.act(sA, session.Activity{Turn: b.now})
		b.pass()
		b.passes(31, time.Minute)
		if got := a.of(worker.AlertYourMove); len(got) != 1 {
			t.Fatalf("first Your move: %+v", a.got)
		}
		b.workedOnce(sA) // a compaction: the pane works, no assistant turn is written
		b.passes(40, time.Minute)
		if got := a.of(worker.AlertYourMove); len(got) != 1 {
			t.Fatalf("a compaction re-armed Your move: %+v", a.got)
		}
		b.act(sA, session.Activity{Turn: b.now}) // a real turn
		b.workedOnce(sA)
		b.passes(31, time.Minute)
		if got := a.of(worker.AlertYourMove); len(got) != 2 {
			t.Fatalf("after a real turn: %+v, want a second Your move", a.got)
		}
	})
	t.Run("stall", func(t *testing.T) {
		b := newBench(t)
		a := b.alerts()
		b.session(sA, store.Operator, "claude")
		b.session(sB, sA, "claude")
		b.act(sB, session.Activity{Turn: b.now})
		b.pass()
		b.passes(31, time.Minute)
		b.workedOnce(sB)
		b.passes(40, time.Minute)
		if got := fyis(b.listed(sA)); len(got) != 1 {
			t.Fatalf("a compaction made a second report: %+v", got)
		}
		if got := a.of(worker.AlertTrouble); len(got) != 1 {
			t.Fatalf("a compaction made a second alert: %+v", a.got)
		}
		b.act(sB, session.Activity{Turn: b.now})
		b.workedOnce(sB)
		b.passes(70, time.Minute) // the Trouble alert comes at the second miss (R-WRK-094)
		if got := fyis(b.listed(sA)); len(got) != 2 {
			t.Fatalf("after a real turn: %+v, want a second report", got)
		}
		if got := a.of(worker.AlertTrouble); len(got) != 2 {
			t.Fatalf("after a real turn: %+v, want a second alert", a.got)
		}
	})
}

// outOfUsage keeps the out-of-usage Trouble alerts.
func outOfUsage(a *alerts) (out []worker.Alert) {
	for _, al := range a.of(worker.AlertTrouble) {
		if strings.Contains(al.Text, "out of usage") {
			out = append(out, al)
		}
	}
	return out
}

// Rejects a worker that types into a pane out of usage (field-2: three hints into a limited root,
// each answered by an instant API error), reports it as stalled, tells the operator more than once
// per episode, or never tries again: the notice names no reset.
func TestUsageLimitInTheTranscriptSilencesThePane(t *testing.T) {
	// covers: R-WRK-097, R-WRK-042
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.act(sB, session.Activity{Turn: b.now, Limit: fableLimit, LimitAt: b.now})
	b.send(sA, sB, store.IntentRequest, "do the thing", false)
	b.pass()
	b.passes(29, time.Minute)
	if got := b.typedTo(sB); len(got) != 0 {
		t.Fatalf("typed into a limited pane: %q", got)
	}
	if got := fyis(b.listed(sA)); len(got) != 0 {
		t.Fatalf("stall report for a limited pane: %+v", got)
	}
	got := outOfUsage(a)
	if len(got) != 1 || got[0].Title != "x" || !strings.Contains(got[0].Text, "out of usage") ||
		!strings.Contains(got[0].Text, fableLimit) {
		t.Fatalf("alerts = %+v, want one out-of-usage Trouble", a.got)
	}
	if n := b.logCount("limited " + sB); n != 1 {
		t.Fatalf("limited logged %d times", n)
	}
	if n := b.logCount("screen " + sB + " usage limit suspected, last lines:"); n != 1 {
		t.Fatalf("the screen at the limit was logged %d times", n)
	}
	// After the hold the pane is tried again with one hint; the error comes back at once.
	b.passes(2, time.Minute)
	if got := b.typedTo(sB); len(got) != 1 {
		t.Fatalf("after 31 min typed = %q, want one hint", got)
	}
	for range 3 { // each retry brings the error back, a new record every time
		b.act(sB, session.Activity{Turn: b.now, Limit: fableLimit, LimitAt: b.now})
		b.passes(29, time.Minute)
	}
	if got := fyis(b.listed(sA)); len(got) != 0 {
		t.Fatalf("stall report for a limited pane: %+v", got)
	}
	if got := outOfUsage(a); len(got) != 1 {
		t.Fatalf("the retry repeated the alert: %+v", a.got)
	}
	// A real turn ends the episode; the next limit is a new one.
	b.act(sB, session.Activity{Turn: b.now})
	b.passes(1, time.Minute)
	b.act(sB, session.Activity{Turn: b.now, Limit: fableLimit, LimitAt: b.now})
	b.passes(1, time.Minute)
	if got := outOfUsage(a); len(got) != 2 {
		t.Fatalf("second episode: %+v", a.got)
	}
}

// Rejects a limit error read again after a worker restart that alerts again and mutes the pane for
// good: a limit older than the hold is not told, and the pane's stall is still reported.
func TestOldUsageLimitIsNeitherToldNorMutes(t *testing.T) {
	// covers: R-WRK-097
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.act(sB, session.Activity{Turn: b.now.Add(-6 * time.Hour), Limit: fableLimit, LimitAt: b.now.Add(-5 * time.Hour)})
	b.pass()
	b.passes(40, time.Minute)
	if got := outOfUsage(a); len(got) != 0 {
		t.Fatalf("a limit from 5 hours ago was told: %+v", got)
	}
	if got := fyis(b.listed(sA)); len(got) != 1 {
		t.Fatalf("the pane stayed muted: %+v, want one stall report", got)
	}
}

// Rejects an activity read of the stored conversation id when the agent runs another: after /clear
// the old transcript never advances, so the episode would freeze.
func TestActivityReadsTheLiveConversation(t *testing.T) {
	// covers: R-WRK-096
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).client = "live-conversation"
	b.pass()
	if got := b.readFor[sA]; got != "live-conversation" {
		t.Fatalf("read the conversation %q, want the live one", got)
	}
}

// Rejects a screen read on every pass while a pane stays blocked on a compaction.
func TestBlockedScreenIsReadRarely(t *testing.T) {
	// covers: R-WRK-081
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).status = "blocked"
	b.pane(sA).screen = "✻ Compacting conversation… (15s)\n\n" + idleScreen
	b.pass()
	b.passes(10, 5*time.Second) // two minutes: the first read
	b.passes(60, 5*time.Second) // five minutes of passes, a read every 30 s at most
	if b.rt.reads > 12 {
		t.Fatalf("%d screen reads in 5 minutes of blocked passes", b.rt.reads)
	}
}

// Rejects a blocked alert on herdr's word alone (field-2: a pane compacting its conversation read
// as blocked): once the screen shows the compaction the operator is not told, and is told when it
// ends and the pane is still blocked.
func TestBlockedWhileCompactingIsNotAlerted(t *testing.T) {
	// covers: R-WRK-081
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.pane(sA).status = "blocked"
	b.pane(sA).screen = "✻ Compacting conversation… (15s)\n\n" + idleScreen
	b.pass()
	b.passes(10, time.Minute)
	if len(a.got) != 0 {
		t.Fatalf("alerted during a compaction: %+v", a.got)
	}
	b.pane(sA).screen = idleScreen
	b.passes(11, time.Minute) // the first Trouble is due 10 min into the block (R-WRK-081)
	if got := a.of(worker.AlertTrouble); len(got) != 1 {
		t.Fatalf("after the compaction: %+v, want the blocked alert", a.got)
	}
}

// Rejects a stall report that leaves no trace of what the pane showed, a log line that carries
// escapes or the whole screen, and one that leaks a configured secret.
func TestStallLogsTheLastLinesOfTheScreen(t *testing.T) {
	// covers: R-WRK-050
	b := newBench(t)
	b.probes(1) // the probes slot carries the secrets to redact
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.pane(sB).screen = "alpha\n\x1b[1mtoken hunter2 here\x1b[0m\n" + idleScreen
	b.pass()
	b.passes(31, time.Minute)
	var line string
	for _, l := range b.logLines() {
		if strings.Contains(l, "screen "+sB+" stall, last lines:") {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("no screen line in the log: %q", b.logLines())
	}
	if strings.Contains(line, "\x1b") || strings.Contains(line, "alpha") || strings.Contains(line, "hunter2") ||
		!strings.Contains(line, "? for shortcuts") || !strings.Contains(line, "[redacted]") {
		t.Fatalf("log line = %q", line)
	}
}
