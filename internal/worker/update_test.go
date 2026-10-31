package worker_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

func TestUpdateItemAtIdle(t *testing.T) {
	// covers: R-UPD-070, R-UPD-071
	b := newBench(t)
	b.session("s-r", store.Operator, "claude")
	b.session("s-c", "s-r", "claude")
	stale := worker.Stale{
		From: "0.1.0", To: "0.2.0", Base: "session/x-s-r",
		Notes: []string{"ness 0.2.0: * merge"}, Skills: []string{"ness-entry"},
	}
	b.w.Stale = func(s store.Session) (worker.Stale, bool, error) { return stale, s.ID == "s-c", nil }
	b.pane("s-c").status = "working"
	b.pass()
	if n := len(b.listed("s-c")); n != 0 {
		t.Fatalf("busy pane got %d items", n)
	}
	b.pane("s-c").status = "idle"
	b.pass()
	got := b.listed("s-c")
	if len(got) != 1 || got[0].Sender != "s-c" || got[0].Intent != store.IntentRequest || got[0].Body != stale.Body() {
		t.Fatalf("%+v", got)
	}
	fyi := b.listed("s-r")
	if len(fyi) != 1 || fyi[0].Intent != store.IntentFYI || !strings.Contains(fyi[0].Body, stale.Body()) {
		t.Fatalf("root fyi %+v", fyi)
	}
	b.pass()
	if n := len(b.listed("s-r")); n != 1 {
		t.Fatalf("root has %d items after a further pass, want 1", n)
	}
}

func TestUpdateItemOncePerPair(t *testing.T) {
	// covers: R-UPD-070, R-UPD-071
	b := newBench(t)
	b.session("s-r", store.Operator, "claude") // a root: no fyi to itself
	pair := worker.Stale{From: "0.1.0", To: "0.2.0", Base: "main"}
	b.w.Stale = func(store.Session) (worker.Stale, bool, error) { return pair, true, nil }
	b.pass()
	b.pass()
	b.restart()
	b.pass()
	if n := len(b.listed("s-r")); n != 1 {
		t.Fatalf("items = %d after restart, want 1", n)
	}
	if n := len(b.listed(store.Operator)); n != 0 {
		t.Fatalf("operator got %d items, want none", n)
	}
	pair.To = "0.3.0"
	b.pass()
	if n := len(b.listed("s-r")); n != 2 {
		t.Fatalf("items = %d after a base hop, want 2", n)
	}
	b.w.Stale = func(store.Session) (worker.Stale, bool, error) { return worker.Stale{}, false, errors.New("no lock") }
	for range 3 {
		b.pass()
	}
	if n := b.logCount("no lock"); n != 1 {
		t.Fatalf("logged %d times, want once", n)
	}
}

func TestStaleBody(t *testing.T) {
	// covers: R-UPD-070
	got := worker.Stale{From: "0.1.0", To: "0.2.0", Base: "main"}.Body()
	want := "ness update: this worktree's ness files are 0.1.0; main has 0.2.0.\nAt your next stopping point:\n" +
		"1. git merge main\n2. ness sync --check (expect exit 0)\n3. re-load these skills: none: skip\n" +
		"Running sessions notes:\nnone"
	if got != want {
		t.Fatalf("Body =\n%s\nwant\n%s", got, want)
	}
	full := worker.Stale{
		From: "0.1.0", To: "0.3.0", Base: "main",
		Skills: []string{"a", "b"}, Notes: []string{"ness 0.2.0: x", "ness 0.3.0: y"},
	}.Body()
	for _, s := range []string{"3. re-load these skills: a, b\n", "Running sessions notes:\nness 0.2.0: x\nness 0.3.0: y"} {
		if !strings.Contains(full, s) {
			t.Errorf("Body lacks %q:\n%s", s, full)
		}
	}
}

func TestPassRunsUpdateAndLogsOnce(t *testing.T) {
	// covers: R-UPD-080
	b := newBench(t)
	calls := 0
	b.w.Update = func(time.Time) error { calls++; return errors.New("auto-install held: x") }
	for range 3 {
		b.pass()
	}
	if calls != 3 {
		t.Fatalf("Update ran %d times, want each pass", calls)
	}
	if n := b.logCount("auto-install held: x"); n != 1 {
		t.Fatalf("logged %d", n)
	}
}
