package worker_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func limitScreen(notice string) string {
	return "assistant output\n  ⎿  " + notice + "\n╭────────╮\n│ > " + placeholder + "\n╰────────╯\n"
}

func TestLimitedPaneStaysSilentUntilReset(t *testing.T) {
	// A weekly limit with a pending item: nothing typed for 27 h of fake time, nothing at
	// reset + 59 s, the resume text once at the first pass after reset + 60 s, then the hint; the
	// notice still on screen is stale from then on.
	// covers: R-WRK-042, R-WRK-043
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).screen = limitScreen("You've hit your weekly limit · resets Oct 9, 1pm")
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	reset := time.Date(2026, 10, 9, 13, 0, 0, 0, time.UTC)
	b.pass()
	b.passes(60, time.Minute)
	b.passes(26*6, 10*time.Minute) // to 13:00 on Oct 9
	if !b.now.Equal(reset) || len(b.rt.typed) != 0 {
		t.Fatalf("at %s typed %q", b.now, b.rt.typed)
	}
	b.passes(1, 59*time.Second)
	if len(b.rt.typed) != 0 {
		t.Fatalf("at reset+59s typed %q", b.rt.typed)
	}
	b.passes(1, time.Second)
	if got := b.typedTo(sA); len(got) != 1 || got[0] != "continue" {
		t.Fatalf("at reset+60s typed %q", got)
	}
	if b.logCount(" resume "+sA) != 1 || b.logCount(" limited "+sA) != 1 {
		t.Fatalf("log:\n%s", strings.Join(b.logLines(), "\n"))
	}
	b.passes(1, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 2 || got[1] != "ness: 1 unread (request); run ness inbox" {
		t.Fatalf("after the resume typed %q", got)
	}
	b.passes(20, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 2 {
		t.Fatalf("resumed twice: %q", got)
	}
}

func TestUnknownResetRereadsEvery15Min(t *testing.T) {
	// covers: R-WRK-045
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).screen = limitScreen("You've reached your Fable limit")
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.pass()
	b.passes(12*60, 5*time.Second) // one hour
	if b.rt.reads != 5 || len(b.rt.typed) != 0 {
		t.Fatalf("reads = %d (want 0, 15, 30, 45, 60 min), typed %q", b.rt.reads, b.rt.typed)
	}
	b.pane(sA).screen = idleScreen
	b.passes(3*60, 5*time.Second) // the read at 75 min sees a clean screen: delivered in that pass
	if got := b.typedTo(sA); len(got) != 1 || b.rt.reads != 6 {
		t.Fatalf("typed %q, reads %d", got, b.rt.reads)
	}
}

func TestUsageFileSilencesWithoutANotice(t *testing.T) {
	// covers: R-WRK-047
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	until := t0.Add(time.Hour)
	b.w.Usage = func(now time.Time) (time.Time, bool) { return until, now.Before(until) }
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.passes(11, 5*time.Minute)
	if len(b.rt.typed) != 0 {
		t.Fatalf("typed under the file limit: %q", b.rt.typed)
	}
	b.passes(2, 5*time.Minute)
	if got := b.typedTo(sA); len(got) != 1 || got[0] != "ness: 1 unread (request); run ness inbox" {
		t.Fatalf("after the file limit typed %q", got)
	}
}

func TestFarResetIsHandledAsUnknown(t *testing.T) {
	// Review Focus 3: "resets Oct 1" read on Oct 8 resolves to next year; the pane is not silenced
	// for a year on a parse like that but treated as a notice without a reset: silent, re-read
	// every 15 min, delivered as soon as the notice is gone.
	// covers: R-WRK-044, R-WRK-045
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).screen = limitScreen("You've hit your weekly limit · resets Oct 1, 4am")
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.pass()
	b.passes(4*60, 5*time.Second) // 20 min
	if b.rt.reads != 2 || len(b.rt.typed) != 0 || b.logCount(" limited "+sA+" no reset time") != 1 {
		t.Fatalf("reads %d, typed %q, log:\n%s", b.rt.reads, b.rt.typed, strings.Join(b.logLines(), "\n"))
	}
	b.pane(sA).screen = idleScreen
	b.passes(2*60, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 1 {
		t.Fatalf("typed %q", got)
	}
}

func TestResumedNoticeStaysStale(t *testing.T) {
	// The notice is still on screen after the resume and, read later, names tomorrow's 1pm: the
	// pane is not silenced again and no second "continue" is typed; a new notice is a new limit.
	// covers: R-WRK-043 (H3)
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	notice := "You've hit your weekly limit · resets 1pm"
	b.pane(sA).screen = limitScreen(notice)
	b.pass()
	b.passes(3*6, 10*time.Minute) // 13:00 on Oct 8
	b.passes(1, 90*time.Second)
	b.passes(30, 10*time.Minute)
	if got := b.typedTo(sA); len(got) != 1 || got[0] != "continue" {
		t.Fatalf("typed %q", got)
	}
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.passes(2, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 2 {
		t.Fatalf("a stale notice silenced the pane: %q", got)
	}
	if b.logCount(" limited "+sA) != 1 {
		t.Fatalf("log:\n%s", strings.Join(b.logLines(), "\n"))
	}
	b.pane(sA).screen = idleScreen
	b.passes(1, 5*time.Second)
	b.pane(sA).screen = limitScreen("You've hit your weekly limit · resets 11pm")
	b.passes(1, 5*time.Second)
	if b.logCount(" limited "+sA) != 2 {
		t.Fatalf("a new notice was not noted:\n%s", strings.Join(b.logLines(), "\n"))
	}
}

func TestBoundaryAndResumeNeverTypeIntoABusyBox(t *testing.T) {
	// Compaction and the resume share the hint's gate: a draft, or a failed read. Focus is no gate.
	// covers: R-WRK-030, R-WRK-043
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	if err := b.s.SetBoundary(sA, "n", time.Time{}, b.now); err != nil {
		t.Fatal(err)
	}
	b.pane(sA).screen = draftScreen
	b.passes(2, 5*time.Second)
	if len(b.rt.typed) != 0 {
		t.Fatalf("typed %q", b.rt.typed)
	}
	b.pane(sA).screen = idleScreen
	b.passes(1, 5*time.Second)
	if len(b.rt.typed) != 0 {
		t.Fatalf("typed inside the quiet window: %q", b.rt.typed)
	}
	b.passes(12, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 1 || got[0] != "/compact n" {
		t.Fatalf("typed %q", got)
	}
}

func TestCompactAndResumeWaitOutTheQuietWindow(t *testing.T) {
	// The operator was in the pane 5 s ago (a draft, now gone): the resume and the compaction wait
	// for quiet_seconds like a hint, then go in order.
	// covers: R-WRK-018, R-WRK-030, R-WRK-043
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).screen = limitScreen("You've hit your weekly limit · resets 1pm")
	if err := b.s.SetBoundary(sA, "n", time.Time{}, b.now); err != nil {
		t.Fatal(err)
	}
	notice := "You've hit your weekly limit · resets 1pm"
	b.pass()
	b.passes(3*6, 10*time.Minute) // 13:00, the reset
	b.pane(sA).screen = strings.Replace(limitScreen(notice), placeholder, "fix the lint first", 1)
	b.passes(1, 61*time.Second) // the resume is due, the box holds a draft
	b.pane(sA).screen = limitScreen(notice)
	b.passes(10, 5*time.Second)
	if len(b.rt.typed) != 0 {
		t.Fatalf("typed inside the quiet window: %q", b.rt.typed)
	}
	b.passes(3, 5*time.Second) // 60 s after the draft was cleared
	if got := b.typedTo(sA); len(got) != 1 || got[0] != "continue" {
		t.Fatalf("typed %q", got)
	}
	b.passes(1, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 2 || got[1] != "/compact n" {
		t.Fatalf("typed %q", got)
	}
}

func TestManualResumeMakesTheNoticeStale(t *testing.T) {
	// The operator resumes by hand after the reset: the pane works with the notice still visible,
	// then idles. No "continue", no new silence, delivery goes on.
	// covers: R-WRK-043 (H3)
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).screen = limitScreen("You've hit your weekly limit · resets 1pm")
	b.pass()
	b.passes(3*6, 10*time.Minute) // 13:00, the reset
	b.pane(sA).status = "working"
	b.passes(1, 5*time.Second)
	b.pane(sA).status = "idle"
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.passes(2, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 1 || got[0] != "ness: 1 unread (request); run ness inbox" {
		t.Fatalf("typed %q", got)
	}
}

func TestFailedResumeDoesNotBlockHints(t *testing.T) {
	// covers: R-WRK-043
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).screen = limitScreen("You've hit your weekly limit · resets 1pm")
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.pass()
	b.passes(3*6, 10*time.Minute)
	b.rt.runErr = errors.New("herdr down")
	b.passes(3, 61*time.Second) // the resume fails once; the hint fails too while the runtime is down
	b.rt.runErr = nil
	b.passes(1, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 1 || got[0] != "ness: 1 unread (request); run ness inbox" {
		t.Fatalf("typed %q", got)
	}
	if b.logCount(" error "+sA+" resume") != 1 {
		t.Fatalf("log:\n%s", strings.Join(b.logLines(), "\n"))
	}
}

func TestLimitedSessionIsNotStallReportedAfterTheLimit(t *testing.T) {
	// A session limited for an hour with an open obligation was not idling: the idle clock restarts
	// when the limit ends, so the stall report comes a full stall window later, not at once.
	// covers: R-WRK-042, R-WRK-050
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.pane(sB).screen = limitScreen("You've reached your Fable limit")
	b.send(sA, sB, store.IntentRequest, "please", false)
	b.pass()
	b.passes(12*60, 5*time.Second) // one hour under the notice
	b.pane(sB).screen = idleScreen
	b.passes(3*60, 5*time.Second) // the read at 75 min sees a clean screen; 15 more minutes
	if got := b.listed(sA); len(got) != 0 {
		t.Fatalf("a stall report right after the limit: %+v", got)
	}
	b.passes(30*60/5, 5*time.Second)
	if got := b.listed(sA); len(got) != 1 {
		t.Fatalf("no stall report a window after the limit: %+v", got)
	}
}
