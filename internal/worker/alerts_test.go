package worker_test

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

// alerts records every alert the worker posts and fails the first fail posts.
type alerts struct {
	got  []worker.Alert
	fail int
}

func (a *alerts) post(al worker.Alert) error {
	if a.fail > 0 {
		a.fail--
		return errors.New("telegram: 502 Bad Gateway")
	}
	a.got = append(a.got, al)
	return nil
}

// of lists the recorded alerts of one kind.
func (a *alerts) of(kind worker.AlertKind) []worker.Alert {
	var out []worker.Alert
	for _, al := range a.got {
		if al.Kind == kind {
			out = append(out, al)
		}
	}
	return out
}

// alerts wires a recorder as the operator channel.
func (b *bench) alerts() *alerts {
	a := &alerts{}
	b.w.Notify = a.post
	return a
}

// restart replaces the worker by a fresh one over the same store, runtime, clock and channel: a
// process restart, which remembers nothing of the old one's passes.
func (b *bench) restart() {
	b.t.Helper()
	clock := func() time.Time { return b.now }
	b.w = &worker.Worker{
		Store: b.s, Runtime: b.rt, Project: "demo-app", Notify: b.w.Notify,
		Clients: map[string]worker.Client{"claude": b.client()},
		Version: "0.1.0", Now: clock, Log: &worker.Log{Path: b.log, Now: clock, Err: os.Stderr},
		Stall: 30 * time.Minute, Quiet: 60 * time.Second,
	}
}

// titled gives the session a title other than the bench's default.
func (b *bench) titled(id, title string) {
	b.t.Helper()
	b.sql(`UPDATE sessions SET title = ? WHERE id = ?`, title, id)
}

// errorLines counts the logged error lines about the operator channel.
func (b *bench) errorLines(sub string) int {
	n := 0
	for _, l := range b.logLines() {
		if strings.Contains(l, " error ") && strings.Contains(l, sub) {
			n++
		}
	}
	return n
}

func (b *bench) unalerted() int {
	b.t.Helper()
	items, err := b.s.UnalertedToOperator()
	if err != nil {
		b.t.Fatal(err)
	}
	return len(items)
}

// Rejects an implementation that waits for idle (the root here works throughout), repeats the
// alert on later passes, or does not stamp the item.
func TestNeedsYouFiresAtCreationOnce(t *testing.T) {
	// covers: R-WRK-080
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.pane(sA).status = "working"
	b.send(sA, store.Operator, store.IntentDecision, "Go on L0b?\nBand study over the cells", false)
	b.pass()
	want := worker.Alert{Kind: worker.AlertNeedsYou, Project: "demo-app", Title: "x", Text: "Go on L0b?", Root: sA}
	if len(a.got) != 1 || a.got[0] != want {
		t.Fatalf("alerts = %+v, want [%+v]", a.got, want)
	}
	b.passes(20, time.Minute)
	if len(a.got) != 1 || b.unalerted() != 0 {
		t.Fatalf("after 20 passes: %d alerts, %d unalerted", len(a.got), b.unalerted())
	}
}

// Rejects an implementation that alerts only for roots' items, names the root as the sender, or
// delivers newest first.
func TestChildItemAlertsUnderTheRootThread(t *testing.T) {
	// covers: R-WRK-080, R-WRK-082
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	b.titled(sA, "program")
	b.titled(sB, "child-b")
	b.send(sA, store.Operator, store.IntentRequest, "first", false)
	b.now = b.now.Add(time.Second)
	b.send(sB, store.Operator, store.IntentHandoff, "second", false)
	b.pass()
	if len(a.got) != 2 {
		t.Fatalf("alerts = %+v", a.got)
	}
	first := worker.Alert{Kind: worker.AlertNeedsYou, Project: "demo-app", Title: "program", Text: "first", Root: sA}
	second := worker.Alert{Kind: worker.AlertNeedsYou, Project: "demo-app", Title: "child-b", Text: "second", Root: sA}
	if a.got[0] != first || a.got[1] != second {
		t.Fatalf("alerts = %+v, want [%+v %+v]", a.got, first, second)
	}
}

// Rejects an implementation that alerts an fyi, or an item settled before its first pass.
func TestNoAlertForFYIOrSettledItems(t *testing.T) {
	// covers: R-WRK-080
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.send(sA, store.Operator, store.IntentFYI, "just so you know", false)
	replied := b.send(sA, store.Operator, store.IntentDecision, "answered at once", false)
	withdrawn := b.send(sA, store.Operator, store.IntentRequest, "withdrawn at once", false)
	if _, err := b.s.Reply(replied, store.Operator, "yes", store.Resolved, b.now); err != nil {
		t.Fatal(err)
	}
	if _, err := b.s.Withdraw(withdrawn, "did it myself", b.now); err != nil {
		t.Fatal(err)
	}
	b.passes(3, time.Minute)
	if len(a.got) != 0 {
		t.Fatalf("alerts = %+v", a.got)
	}
}

// Rejects an implementation that keeps the delivery state in memory (a restart repeats the alert).
func TestRestartRepeatsNoAlert(t *testing.T) {
	// covers: R-WRK-080
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.send(sA, store.Operator, store.IntentDecision, "q", false)
	b.pass()
	b.restart()
	b.passes(3, time.Minute)
	if len(a.got) != 1 {
		t.Fatalf("alerts = %+v", a.got)
	}
}

// Rejects an implementation that stamps the item before the post succeeded, retries without a
// bound, logs every failure, or logs nothing.
func TestFailedPostIsLoggedOnceAndRetried(t *testing.T) {
	// covers: R-WRK-083, R-WRK-080
	b := newBench(t)
	a := b.alerts()
	a.fail = 2
	b.session(sA, store.Operator, "claude")
	b.send(sA, store.Operator, store.IntentDecision, "q", false)
	b.passes(2, 5*time.Second)
	if len(a.got) != 0 || b.unalerted() != 1 || b.errorLines("notify") != 1 {
		t.Fatalf("alerts %+v, unalerted %d, log:\n%s", a.got, b.unalerted(), strings.Join(b.logLines(), "\n"))
	}
	b.passes(3, 5*time.Second)
	if len(a.got) != 1 || b.unalerted() != 0 {
		t.Fatalf("alerts %+v, unalerted %d", a.got, b.unalerted())
	}
}

// Rejects an implementation that tells a limited pane's operator nothing: the item is the
// operator's business, whatever the pane does.
func TestLimitedPaneStillAlerts(t *testing.T) {
	// covers: R-WRK-080
	b := newBench(t)
	a := b.alerts()
	b.w.Usage = func(now time.Time) (time.Time, bool) { return now.Add(time.Hour), true }
	b.session(sA, store.Operator, "claude")
	b.send(sA, store.Operator, store.IntentDecision, "q", false)
	b.passes(2, time.Minute)
	if len(a.got) != 1 {
		t.Fatalf("alerts = %+v", a.got)
	}
}

// Rejects an implementation that takes the whole first line, the wrong section, drops the page,
// mislabels a checkpoint with an Ask, or cuts the text by bytes.
func TestCheckpointAlertsCarryTheirSectionAndPage(t *testing.T) {
	// covers: R-WRK-080
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	plain := "## Done\nlanded rung one\n## Now\nscoring\n## Next\nrung two\n\nthen three\nPage: https://claude.ai/x\n"
	b.send(sA, store.Operator, store.IntentCheckpoint, plain, false)
	asking := "## Done\nd\n## Now\nn\n## Next\nx\n## Ask\nShip it?\nWith the caveats.\n"
	b.send(sA, store.Operator, store.IntentCheckpoint, asking, false)
	long := "## Done\nd\n## Now\nn\n## Next\n" + strings.Repeat("é", 300) + "\n" // one word under the cap
	b.send(sA, store.Operator, store.IntentCheckpoint, long, false)
	b.pass()
	if len(a.got) != 3 {
		t.Fatalf("alerts = %+v", a.got)
	}
	want := worker.Alert{Kind: worker.AlertCheckpoint, Project: "demo-app", Title: "x", Text: "rung two",
		Page: "https://claude.ai/x", Root: sA}
	if a.got[0] != want {
		t.Fatalf("plain checkpoint = %+v, want %+v", a.got[0], want)
	}
	if a.got[1].Kind != worker.AlertNeedsYou || words(a.got[1].Text) != "Ship it? With the caveats." || a.got[1].Page != "" {
		t.Fatalf("checkpoint with Ask = %+v", a.got[1])
	}
	if n := utf8.RuneCountInString(a.got[2].Text); a.got[2].Kind != worker.AlertCheckpoint || n != 300 ||
		!utf8.ValidString(a.got[2].Text) || strings.Trim(a.got[2].Text, "é") != "" {
		t.Fatalf("long checkpoint text: %d runes, %q", n, a.got[2].Text)
	}
}

// Rejects an Ask section that says "Nothing." turning a progress report into a Needs you (field-2:
// three false alerts).
func TestCheckpointWithAnEmptyAskIsACheckpoint(t *testing.T) {
	// covers: R-WRK-080
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.send(sA, store.Operator, store.IntentCheckpoint, "## Done\nd\n## Now\nn\n## Next\nrung two\n## Ask\nNothing.\n", false)
	b.pass()
	if len(a.got) != 1 || a.got[0].Kind != worker.AlertCheckpoint || a.got[0].Text != "rung two" {
		t.Fatalf("alerts = %+v", a.got)
	}
}

// Rejects an implementation that leaves operator items unalerted when nobody is listening (a
// channel enabled later would post the whole backlog, R1), or that logs anything for the absent
// channel.
func TestNoChannelStampsTheBacklogAndEnablingPostsNone(t *testing.T) {
	// covers: R-WRK-080, R-WRK-082
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.send(sA, store.Operator, store.IntentDecision, "old question", false)
	b.send(sA, store.Operator, store.IntentHandoff, "old handoff", false)
	b.passes(3, time.Minute)
	if b.unalerted() != 0 {
		t.Fatalf("%d items stay unalerted with no channel", b.unalerted())
	}
	if b.logCount("notify") != 0 {
		t.Fatalf("log:\n%s", strings.Join(b.logLines(), "\n"))
	}
	a := b.alerts()
	b.passes(3, time.Minute)
	if len(a.got) != 0 {
		t.Fatalf("enabling the channel posted the backlog: %+v", a.got)
	}
	b.send(sA, store.Operator, store.IntentRequest, "new question", false)
	b.pass()
	if len(a.got) != 1 || a.got[0].Text != "new question" {
		t.Fatalf("a new item after enabling: %+v", a.got)
	}
}

// Rejects an implementation that leaves the answer to an operator-sent item silent, alerts it for
// only some statuses, or alerts the notice of a withdrawn ask.
func TestReplyToAnOperatorItemAlertsForEveryStatus(t *testing.T) {
	// covers: R-WRK-080
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	for i, status := range []string{store.Resolved, store.Accepted, store.Declined, store.Returned, store.Deferred} {
		id := b.send(store.Operator, sA, store.IntentRequest, "ask "+status, false)
		if _, err := b.s.Reply(id, sA, "answer "+status, status, b.now.Add(time.Duration(i))); err != nil {
			t.Fatal(err)
		}
	}
	b.pass()
	if len(a.got) != 5 {
		t.Fatalf("alerts = %+v, want one per status", a.got)
	}
	for i, al := range a.got {
		if al.Kind != worker.AlertNeedsYou || al.Root != sA || !strings.HasPrefix(al.Text, "answer ") {
			t.Errorf("alert %d = %+v", i, al)
		}
	}
	b.passes(3, time.Minute)
	if len(a.got) != 5 {
		t.Fatalf("after 3 passes: %d alerts", len(a.got))
	}
}
