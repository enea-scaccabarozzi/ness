package worker_test

// The discovery regression corpus (R-WRK-070), one named test per case:
//   launch race               TestNothingTypedBeforeReady
//   draft vs placeholder      TestInputBoxTable (claude), TestPlaceholderDoesNotDelayDelivery, TestDraftBlocksDelivery
//   every usage-limit notice  TestParseLimitTable (claude), TestLimitedPaneStaysSilentUntilReset
//   fyi wake                  TestEveryIntentWakes
//   quiet window on operator  TestQuietWindowKeyedOnOperatorInput
//   zombie items              TestRetiresItemsToClosedSessions
//   repeated-error log storm  TestRuntimeErrorLoggedOnce, TestLogRepeatedErrorOnce
//   never-ran launch close    close_never_ran.txtar (P3b)
// covers: R-WRK-070

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
	_ "modernc.org/sqlite" // the test breaks the store through a second handle
)

func TestHeartbeatEveryPass(t *testing.T) {
	// covers: R-WRK-008
	b := newBench(t)
	b.pass()
	hb, err := b.s.Heartbeat()
	if err != nil || !hb.At.Equal(t0) || hb.Version != "0.1.0" {
		t.Fatalf("heartbeat = %+v, %v", hb, err)
	}
	b.passes(1, 5*time.Second)
	if hb, _ = b.s.Heartbeat(); !hb.At.Equal(t0.Add(5 * time.Second)) {
		t.Fatalf("second heartbeat = %+v", hb)
	}
}

func TestEveryIntentWakes(t *testing.T) {
	// One item per intent to an idle session, replies included: each pane gets one hint in the
	// same pass (F1 fyi-wake regression), and only one.
	// covers: R-WRK-010, R-WRK-019
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	for _, s := range []string{sB, sC, sD} {
		b.session(s, sA, "claude")
	}
	b.send(sA, sB, store.IntentFYI, "fyi", false)
	b.send(sA, sC, store.IntentDecision, "decide", false)
	b.send(sA, sD, store.IntentHandoff, "take it", false)
	req := b.send(sB, sA, store.IntentRequest, "a request", false)
	b.pass()
	for s, want := range map[string]string{
		sA: "ness: 1 unread (request); run ness inbox", sB: "ness: 1 unread (fyi); run ness inbox",
		sC: "ness: 1 unread (decision); run ness inbox", sD: "ness: 1 unread (handoff); run ness inbox",
	} {
		if got := b.typedTo(s); len(got) != 1 || got[0] != want {
			t.Errorf("%s typed %q, want %q", s, got, want)
		}
	}
	// A reply wakes the asker too.
	if _, err := b.s.Inbox(sA, b.now); err != nil {
		t.Fatal(err)
	}
	if _, err := b.s.Reply(req, sA, "done", store.Resolved, b.now); err != nil {
		t.Fatal(err)
	}
	b.passes(1, 5*time.Second)
	if got := b.typedTo(sB); len(got) != 2 || got[1] != "ness: 2 unread (fyi, reply); run ness inbox" {
		t.Fatalf("B after a reply typed %q", got)
	}
}

func TestHintsAreBatchedPerPass(t *testing.T) {
	// Three items → one hint; the same unread set is not hinted again; a new item is.
	// covers: R-WRK-011
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	for _, body := range []string{"one", "two", "three"} {
		b.send(store.Operator, sA, store.IntentRequest, body, false)
	}
	b.passes(3, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 1 || got[0] != "ness: 3 unread (request); run ness inbox" {
		t.Fatalf("typed %q", got)
	}
	b.send(store.Operator, sA, store.IntentFYI, "four", false)
	b.passes(1, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 2 || got[1] != "ness: 4 unread (fyi, request); run ness inbox" {
		t.Fatalf("typed %q", got)
	}
}

func TestPulledItemsAreNeverHinted(t *testing.T) {
	// covers: R-WRK-012
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.send(store.Operator, sA, store.IntentRequest, "pulled before the pass", false)
	if _, err := b.s.Inbox(sA, b.now); err != nil {
		t.Fatal(err)
	}
	b.passes(5, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 0 {
		t.Fatalf("a pulled item was hinted: %q", got)
	}
	b.send(store.Operator, sA, store.IntentRequest, "new", false)
	b.passes(1, 5*time.Second)
	if _, err := b.s.Inbox(sA, b.now); err != nil {
		t.Fatal(err)
	}
	b.passes(20, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 1 {
		t.Fatalf("hinted again after the pull: %q", got)
	}
}

func TestNothingTypedBeforeReady(t *testing.T) {
	// A session still launching (no ready mark, or no pane yet) gets nothing; the launch race.
	// covers: R-WRK-013
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	if err := b.s.ClearReady(sA); err != nil {
		t.Fatal(err)
	}
	b.send(store.Operator, sA, store.IntentRequest, "early", false)
	b.passes(3, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 0 {
		t.Fatalf("typed before ready: %q", got)
	}
	if err := b.s.MarkReady(sA, b.now); err != nil {
		t.Fatal(err)
	}
	b.passes(1, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 1 {
		t.Fatalf("typed after ready: %q", got)
	}
}

func TestNoTypingWhileWorking(t *testing.T) {
	// covers: R-WRK-013
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	for _, status := range []string{"working", "blocked", "unknown", "busy-in-a-future-herdr", ""} {
		b.pane(sA).status = status
		b.passes(2, 5*time.Second)
		if got := b.typedTo(sA); len(got) != 0 {
			t.Fatalf("typed while %s: %q", status, got)
		}
	}
	b.pane(sA).status = "idle"
	b.passes(1, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 1 {
		t.Fatalf("typed once idle: %q", got)
	}
}

func TestDraftBlocksDelivery(t *testing.T) {
	// A draft on the prompt line blocks typing; once it is cleared the pane is quiet for
	// quiet_seconds after that change, then the hint lands.
	// covers: R-WRK-014, R-WRK-018
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).screen = draftScreen
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.passes(3, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 0 {
		t.Fatalf("typed over a draft: %q", got)
	}
	b.pane(sA).screen = idleScreen
	b.passes(12, 5*time.Second) // cleared at the first of these; 55 s after the change
	if got := b.typedTo(sA); len(got) != 0 {
		t.Fatalf("typed inside the quiet window: %q", got)
	}
	b.passes(1, 5*time.Second) // 60 s after the change
	if got := b.typedTo(sA); len(got) != 1 {
		t.Fatalf("typed after the quiet window: %q", got)
	}
}

func TestPlaceholderDoesNotDelayDelivery(t *testing.T) {
	// The dim suggestion is an empty box: the hint is typed in the same pass, no interrupt key,
	// no wait; a grey (SGR 90) placeholder reads the same.
	// covers: R-WRK-015, R-WRK-016
	for name, screen := range map[string]string{
		"dim":  idleScreen,
		"grey": strings.ReplaceAll(idleScreen, "\x1b[2m", "\x1b[90m"),
	} {
		b := newBench(t)
		b.session(sA, store.Operator, "claude")
		b.pane(sA).screen = screen
		b.send(store.Operator, sA, store.IntentRequest, "x", false)
		b.pass()
		got := b.typedTo(sA)
		if len(got) != 1 || strings.ContainsRune(strings.Join(b.rt.typed, ""), '\x03') {
			t.Fatalf("%s: typed %q", name, got)
		}
	}
}

func TestFocusedPaneIsNotHeld(t *testing.T) {
	// herdr reports the selected tab as focused with nobody at the keyboard (CX-15): focus holds
	// nothing, and leaving it starts no quiet window.
	// covers: R-WRK-017, R-WRK-018, R-WRK-019
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).focused = true
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.pass()
	if got := b.typedTo(sA); len(got) != 1 || got[0] != "ness: 1 unread (request); run ness inbox" {
		t.Fatalf("the focused pane got %q in the first pass, want the hint", got)
	}
	b.pane(sA).focused = false
	b.send(store.Operator, sA, store.IntentFYI, "y", false)
	b.passes(1, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 2 {
		t.Fatalf("a hint right after the focus left was held: %q", got)
	}
}

func TestQuietWindowIgnoresWorkAndFocus(t *testing.T) {
	// The session's own work (working passes) never starts the window: a hint lands in the
	// first idle pass; neither does focus, which also never extends one (quiet_test.go has the
	// window a changed box opens).
	// covers: R-WRK-018
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.pane(sA).status = "working"
	b.passes(24, 5*time.Second) // two minutes of transcript writes
	b.pane(sA).status = "idle"
	b.passes(1, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 1 {
		t.Fatalf("after the session's own work: %q", got)
	}
	b.send(store.Operator, sA, store.IntentRequest, "y", false)
	b.pane(sA).focused = true
	b.passes(1, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 2 {
		t.Fatalf("focus held the hint: %q", got)
	}
}

func TestUrgentBypassesTheQuietWindowNotTheDraft(t *testing.T) {
	// covers: R-MSG-042
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).screen = draftScreen
	b.pass()
	b.pane(sA).screen = idleScreen
	b.passes(2, 5*time.Second) // the box changed 10 s ago
	b.send(store.Operator, sA, store.IntentRequest, "later", false)
	b.passes(1, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 0 {
		t.Fatalf("a non-urgent item inside the window was hinted: %q", got)
	}
	b.send(store.Operator, sA, store.IntentRequest, "now", true)
	b.passes(1, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 1 || got[0] != "ness: 2 unread, urgent (request); run ness inbox now" {
		t.Fatalf("urgent inside the window: %q", got)
	}
	b.pane(sA).screen = draftScreen
	b.send(store.Operator, sA, store.IntentRequest, "and this", true)
	b.passes(3, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 1 {
		t.Fatalf("urgent typed over a draft: %q", got)
	}
}

func TestRetiresItemsToClosedSessions(t *testing.T) {
	// covers: R-WRK-020, R-WRK-021
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	id := b.send(sA, sB, store.IntentRequest, "zombie", false)
	if err := b.s.RecordClose(sB, "", "", b.now); err != nil {
		t.Fatal(err)
	}
	delete(b.rt.panes, paneOf(sB))
	b.pass()
	if len(b.listed(sB)) != 0 {
		t.Fatal("the zombie is still listed")
	}
	if n := b.logCount(" retire " + id + " to " + sB + " "); n != 1 {
		t.Fatalf("retire lines = %d, log:\n%s", n, strings.Join(b.logLines(), "\n"))
	}
	b.passes(10, 5*time.Second)
	if n := b.logCount(" retire "); n != 1 || len(b.rt.typed) != 0 {
		t.Fatalf("later passes: %d retire lines, typed %q", n, b.rt.typed)
	}
}

func TestSuccessfulHintsAreLogged(t *testing.T) {
	// covers: R-WRK-023, R-WRK-021
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.passes(3, 5*time.Second)
	if n := b.logCount(" hint " + sA + " 1 unread"); n != 1 {
		t.Fatalf("hint lines = %d, log:\n%s", n, strings.Join(b.logLines(), "\n"))
	}
}

func TestRuntimeErrorLoggedOnce(t *testing.T) {
	// A pane herdr no longer has: one line for a hundred passes, nothing typed (log-storm regression).
	// covers: R-WRK-022
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	delete(b.rt.panes, paneOf(sA))
	b.passes(100, 5*time.Second)
	if n := b.logCount(" error " + sA + " "); n != 1 || len(b.rt.typed) != 0 {
		t.Fatalf("%d error lines, typed %q; log:\n%s", n, b.rt.typed, strings.Join(b.logLines(), "\n"))
	}
	b.rt.panes[paneOf(sA)] = &fakePane{status: "idle", agent: "agent-" + paneOf(sA), screen: idleScreen, live: true}
	b.passes(1, 5*time.Second)
	delete(b.rt.panes, paneOf(sA))
	b.passes(1, 5*time.Second)
	if n := b.logCount(" error " + sA + " "); n != 2 {
		t.Fatalf("after a recovery the error is not written again: %d lines", n)
	}
}

func TestForeignAgentIsNeverTyped(t *testing.T) {
	// Review Focus 2: the pane runs another agent; nothing typed, one log line naming both.
	// covers: R-WRK-013, R-WRK-022
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).agent, b.pane(sA).client = "someone-else", "c-other"
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.passes(20, 5*time.Second)
	if len(b.rt.typed) != 0 {
		t.Fatalf("typed into a foreign agent: %q", b.rt.typed)
	}
	if n := b.logCount("someone-else"); n != 1 || b.logCount("agent-"+paneOf(sA)) != 1 {
		t.Fatalf("log:\n%s", strings.Join(b.logLines(), "\n"))
	}
	// An adopted root has no bound agent: any live idle agent in its pane is delivered to.
	b.session(sB, store.Operator, "claude")
	if err := b.s.BindAgent(sB, "", ""); err != nil {
		t.Fatal(err)
	}
	b.pane(sB).agent = "whatever-the-operator-started"
	b.send(store.Operator, sB, store.IntentRequest, "y", false)
	b.passes(1, 5*time.Second)
	if got := b.typedTo(sB); len(got) != 1 {
		t.Fatalf("adopted root: %q", got)
	}
}

func TestRunFailureRetriesNextPass(t *testing.T) {
	// Review Focus 1: a failed Run leaves the item unhinted, logged once; the next pass retries.
	// covers: R-WRK-022, R-WRK-023
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.rt.runErr = errors.New("herdr pane run: exit status 1")
	b.passes(3, 5*time.Second)
	if n := b.logCount(" error " + sA + " hint herdr pane run"); n != 1 || b.logCount(" hint "+sA+" ") != 0 {
		t.Fatalf("log:\n%s", strings.Join(b.logLines(), "\n"))
	}
	b.rt.runErr = nil
	b.passes(1, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 1 || b.logCount(" hint "+sA+" ") != 1 {
		t.Fatalf("retry: typed %q, log:\n%s", got, strings.Join(b.logLines(), "\n"))
	}
}

func TestUnknownClientIsSkippedOnce(t *testing.T) {
	// A codex session (experimental, no screen reader in v1) is never typed into; one log line.
	// covers: R-WRK-022
	b := newBench(t)
	b.session(sA, store.Operator, "codex")
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.passes(10, 5*time.Second)
	if len(b.rt.typed) != 0 || b.logCount(" error "+sA+" client codex") != 1 {
		t.Fatalf("typed %q, log:\n%s", b.rt.typed, strings.Join(b.logLines(), "\n"))
	}
}

func TestRelaunchedPaneIsWokenAgain(t *testing.T) {
	// H5: after a resume (new run, new pane, same id) what the dead process was hinted is hinted again.
	// covers: R-WRK-011
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.passes(2, 5*time.Second)
	if err := b.s.BindTab(sA, "p-new", "t:p-new", 2); err != nil {
		t.Fatal(err)
	}
	b.rt.panes["p-new"] = &fakePane{status: "idle", agent: "", screen: idleScreen, live: true}
	b.passes(1, 5*time.Second)
	if n := len(b.rt.typed); n != 2 || !strings.HasPrefix(b.rt.typed[1], "p-new ") {
		t.Fatalf("typed %q", b.rt.typed)
	}
}

func TestUrgentAfterANonUrgentHint(t *testing.T) {
	// L6: a hinted non-urgent set, then an urgent item inside the quiet window: one more hint, urgent.
	// covers: R-MSG-042
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.pass()
	b.pane(sA).screen = draftScreen
	b.passes(1, 5*time.Second)
	b.pane(sA).screen = idleScreen
	b.passes(1, 5*time.Second) // the box changed 5 s ago
	b.send(store.Operator, sA, store.IntentFYI, "y", true)
	b.passes(1, 5*time.Second)
	got := b.typedTo(sA)
	if len(got) != 2 || got[1] != "ness: 2 unread, urgent (fyi, request); run ness inbox now" {
		t.Fatalf("typed %q", got)
	}
}

func TestDriftIsLoggedOnceAndFocusNever(t *testing.T) {
	// M6, L3: no prompt on screen gets one line per episode; a focused pane is nothing to log.
	// covers: R-WRK-014, R-WRK-017, R-WRK-022
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).screen = "no box at all\n"
	b.pane(sA).focused = true
	b.passes(20, 5*time.Second)
	if b.logCount(" error "+sA+" prompt no input box") != 1 || b.logCount("focused") != 0 {
		t.Fatalf("log:\n%s", strings.Join(b.logLines(), "\n"))
	}
}

func TestUrgentIsTypedIntoAFocusedPane(t *testing.T) {
	// covers: R-WRK-017, R-MSG-042
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).focused = true
	b.send(store.Operator, sA, store.IntentRequest, "now", true)
	b.passes(3, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 1 || got[0] != "ness: 1 unread, urgent (request); run ness inbox now" {
		t.Fatalf("urgent for the focused pane: %q", got)
	}
}

func TestStoreErrorLoggedOnce(t *testing.T) {
	// A persistent Unread failure is one log line over three passes, not one per pass.
	// covers: R-WRK-022
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	db, err := sql.Open("sqlite", "file:"+filepath.Join(filepath.Dir(b.log), "state.sqlite3")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(context.Background(), `DROP INDEX IF EXISTS items_unread`); err != nil {
		t.Log(err)
	}
	if _, err := db.ExecContext(context.Background(), `ALTER TABLE items DROP COLUMN pulled_at`); err != nil {
		t.Fatal(err)
	}
	b.passes(3, 5*time.Second)
	if n := b.logCount(" error " + sA + " unread "); n != 1 {
		t.Fatalf("%d unread error lines, log:\n%s", n, strings.Join(b.logLines(), "\n"))
	}
}

func TestDoneIsIdle(t *testing.T) {
	// herdr reports `done` for an agent that finished a turn in a tab the operator has not focused
	// since: the same idle prompt. The worker hints, compacts, resumes and stall-reports it.
	// covers: R-WRK-010, R-WRK-019, R-WRK-030, R-WRK-043, R-WRK-050
	t.Run("hint", func(t *testing.T) {
		b := newBench(t)
		b.session(sA, store.Operator, "claude")
		b.pane(sA).status = "done"
		b.send(store.Operator, sA, store.IntentRequest, "x", false)
		b.passes(1, 5*time.Second)
		if got := b.typedTo(sA); len(got) != 1 || got[0] != "ness: 1 unread (request); run ness inbox" {
			t.Fatalf("typed %q", got)
		}
	})
	t.Run("compaction", func(t *testing.T) {
		b := newBench(t)
		b.session(sA, store.Operator, "claude")
		b.pane(sA).status = "done"
		if err := b.s.SetBoundary(sA, "n", time.Time{}, b.now); err != nil {
			t.Fatal(err)
		}
		b.passes(1, 5*time.Second)
		if got := b.typedTo(sA); len(got) != 1 || got[0] != "/compact n" {
			t.Fatalf("typed %q", got)
		}
	})
	t.Run("resume", func(t *testing.T) {
		b := newBench(t)
		b.session(sA, store.Operator, "claude")
		b.pane(sA).status = "done"
		b.pane(sA).screen = limitScreen("You've hit your weekly limit · resets 1pm")
		b.pass()
		b.passes(3*6+7, 10*time.Minute)
		if got := b.typedTo(sA); len(got) != 1 || got[0] != "continue" {
			t.Fatalf("typed %q", got)
		}
	})
	t.Run("stall", func(t *testing.T) {
		b := newBench(t)
		b.session(sA, store.Operator, "claude")
		b.session(sB, sA, "claude")
		b.pane(sB).status = "done"
		b.send(sA, sB, store.IntentRequest, "please", false)
		b.pass()
		b.passes(30, time.Minute)
		if got := b.listed(sA); len(got) != 1 || got[0].Sender != sB {
			t.Fatalf("A has %+v", got)
		}
	})
}

func TestClearedNameStillDelivers(t *testing.T) {
	// Field run 2: herdr cleared a live child's agent name; its conversation id still matches, so
	// the hint is typed. A cleared name with another conversation id is still foreign.
	// covers: R-WRK-084
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.pane(sA).agent = ""
	b.send(store.Operator, sA, store.IntentRequest, "x", false)
	b.passes(20, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 1 {
		t.Fatalf("cleared name, same conversation: %q", got)
	}
	b.session(sB, store.Operator, "claude")
	b.pane(sB).agent, b.pane(sB).client = "", "c-other"
	b.send(store.Operator, sB, store.IntentRequest, "y", false)
	b.passes(20, 5*time.Second)
	if got := b.typedTo(sB); len(got) != 0 {
		t.Fatalf("cleared name, other conversation: %q", got)
	}
}
