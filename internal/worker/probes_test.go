package worker_test

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/probe"
	"github.com/enea-scaccabarozzi/ness/internal/shell"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

const (
	floorless = "---\nevery: 30m\n---\nLook at the dashboards.\n"
	red       = "---\nevery: 30m\nfloor: ./check.sh\n---\nscope\n"
)

func TestFloorlessProbeWakesEveryRunAndNeverStartsAnAgent(t *testing.T) {
	// A due floorless probe puts one request in the owner's inbox carrying the scope; nothing
	// is typed, no tab or client is started, no command runs (R-WRK-060, R-PRB-030, R-PRB-038).
	// covers: R-WRK-060, R-PRB-030, R-PRB-038
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	fr, _ := b.probes(1)
	p := b.start(sA, b.probeFile(sA, "look.md", floorless))
	b.pass()
	items := b.listed(sA)
	if len(items) != 1 || items[0].Intent != store.IntentRequest || items[0].Probe != p.ID || items[0].Sender != sA ||
		!strings.HasPrefix(items[0].Body, "probe probes/look.md run 1\n") ||
		!strings.Contains(items[0].Body, "Look at the dashboards.") ||
		!strings.Contains(items[0].Body, "ness probe verdict") {
		t.Fatalf("wake = %+v", items)
	}
	if len(fr.cmds) != 0 || len(b.rt.closed) != 0 {
		t.Fatalf("a floorless probe ran %d commands, closed %v", len(fr.cmds), b.rt.closed)
	}
	if got := b.typedTo(sA); len(got) != 1 || !strings.HasPrefix(got[0], "ness: 1 unread (request)") {
		t.Fatalf("the same pass hints the wake: typed %v", got)
	}
	b.passes(5, 5*time.Minute) // 25 min: not due
	if len(b.listed(sA)) != 1 {
		t.Fatal("woke before every elapsed")
	}
	b.passes(1, 5*time.Minute) // 30 min: due
	if items = b.listed(sA); len(items) != 2 || !strings.HasPrefix(items[1].Body, "probe probes/look.md run 2\n") {
		t.Fatalf("second wake = %+v", items)
	}
	if b.logCount("wake "+p.ID) != 2 || b.logCount("probe "+p.ID) != 2 {
		t.Fatalf("log:\n%s", strings.Join(b.logLines(), "\n"))
	}
}

func TestWakePolicies(t *testing.T) {
	// never: no wake; change: on a state change only; red: RED only; not-green: RED, UNCLEAR,
	// VACUOUS; always: every run (R-PRB-031..035). confirm: k consecutive non-GREEN runs before
	// a non-GREEN wake (R-PRB-037).
	// covers: R-PRB-031, R-PRB-032, R-PRB-033, R-PRB-034, R-PRB-035, R-PRB-037
	for _, tc := range []struct {
		wake    string
		results []int // exit codes per run; 7 = VACUOUS (exit 0, expect unmet)
		wakes   []int // runs (1-based) that wake
	}{
		{"never", []int{1, 1, 2, 7}, nil},
		{"change", []int{0, 0, 1, 1, 0}, []int{3, 5}},
		{"change", []int{1, 1}, []int{1}},                            // RED from the first run: a first confirmed non-GREEN wakes
		{"confirm: 2\nwake: change", []int{0, 1, 1, 0}, []int{3, 4}}, // compares with the last confirmed state
		{"confirm: 2\nwake: change", []int{0, 1, 0}, nil},            // an unconfirmed flap is no change
		{"red", []int{2, 7, 1, 1, 0}, []int{3, 4}},
		{"not-green", []int{1, 2, 7, 0}, []int{1, 2, 3}},
		{"always", []int{0, 0}, []int{1, 2}},
		{"confirm: 2\nwake: not-green", []int{1, 0, 1, 1}, []int{4}},
	} {
		b := newBench(t)
		b.session(sA, store.Operator, "claude")
		fr, _ := b.probes(1)
		file := "---\nevery: 10m\nfloor: ./f.sh\nexpect: n>=1\nsample: 0\n" + policyLine(tc.wake) + "\n---\nscope\n"
		b.start(sA, b.probeFile(sA, "p.md", file))
		var woke []int
		for i, code := range tc.results {
			fr.results = []shell.Result{floorResult(code)}
			if i > 0 {
				b.now = b.now.Add(10 * time.Minute)
			}
			before := len(b.listed(sA))
			b.pass()
			if len(b.listed(sA)) > before {
				woke = append(woke, i+1)
			}
		}
		if !slices.Equal(woke, tc.wakes) {
			t.Errorf("%q with %v woke at %v, want %v", tc.wake, tc.results, woke, tc.wakes)
		}
	}
}

// policyLine is "wake: X" or a multi-line policy given as is.
func policyLine(s string) string {
	if strings.Contains(s, ":") {
		return s
	}
	return "wake: " + s
}

// floorResult is a floor outcome by code: 7 is exit 0 with n=0 (VACUOUS under expect: n>=1).
func floorResult(code int) shell.Result {
	if code == 7 {
		return shell.Result{Output: []byte("n=0\n")}
	}
	return shell.Result{Exit: code, Output: []byte("n=3\n")}
}

func TestSampleWiringWakesGreensOnly(t *testing.T) {
	// The fraction itself is TestSampledGreenFraction; here a sample of 1 wakes every GREEN under
	// wake: never, and a RED does not wake (R-PRB-036, R-PRB-031).
	// covers: R-PRB-036, R-PRB-031
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	fr, _ := b.probes(42)
	b.start(sA, b.probeFile(sA, "p.md", "---\nevery: 1m\nfloor: ./f.sh\nwake: never\nsample: 1\n---\nscope\n"))
	for range 3 {
		fr.results = []shell.Result{exit(0)}
		b.pass()
		b.now = b.now.Add(time.Minute)
	}
	if n := len(b.listed(sA)); n != 3 {
		t.Fatalf("sample 1 woke %d times over 3 GREEN runs", n)
	}
	fr.results = []shell.Result{exit(1)}
	b.pass()
	if n := len(b.listed(sA)); n != 3 {
		t.Fatalf("wake: never woke on RED (%d items)", n)
	}
}

func TestEvidenceIsRedactedAndCappedInTheWake(t *testing.T) {
	// covers: R-PRB-039, R-PRB-083
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	fr, _ := b.probes(1)
	p := b.start(sA, b.probeFile(sA, "p.md", red))
	fr.results = []shell.Result{{Exit: 1, Output: []byte(strings.Repeat("x", 5000) + "\ntoken=hunter2\n")}}
	b.pass()
	items := b.listed(sA)
	if len(items) != 1 || strings.Contains(items[0].Body, "hunter2") || !strings.Contains(items[0].Body, "token=[redacted]") {
		t.Fatalf("wake body leaks or lacks the evidence: %+v", items)
	}
	if ev := b.probe(p.ID).Evidence; len(ev) != probe.EvidenceCap || strings.Contains(ev, "hunter2") {
		t.Fatalf("stored evidence: %d bytes, redacted %v", len(ev), !strings.Contains(ev, "hunter2"))
	}
	sess, _ := b.s.Session(sA)
	if c := fr.cmds[0]; c.Dir != filepath.Join(b.primary, sess.Worktree) || c.Command != "./check.sh" ||
		!slices.Contains(c.Env, "NESS_PROBE=probes/p.md") || !slices.Contains(c.Env, "NESS_SESSION="+sA) ||
		c.Timeout != time.Minute {
		t.Fatalf("floor cmd = %+v", c)
	}
}

func TestScheduleWithJitter(t *testing.T) {
	// every: 30m, jitter: 5m: each run comes within [30m, 35m] of the previous one, scheduled from
	// the run (R-PRB-040). The clock steps to each probe's next run.
	// covers: R-PRB-040
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, _ = b.probes(7)
	start := b.now
	p := b.start(sA, b.probeFile(sA, "p.md", "---\nevery: 30m\njitter: 5m\nfloor: true\nwake: always\n---\ns\n"))
	var last time.Time
	spread := map[time.Duration]bool{}
	for b.now.Before(start.Add(24 * time.Hour)) {
		b.pass()
		cur := b.probe(p.ID)
		if !last.IsZero() {
			gap := cur.LastRunAt.Sub(last)
			spread[gap] = true
			if gap < 30*time.Minute || gap > 35*time.Minute {
				t.Fatalf("run %d came %v after the previous", cur.Runs, gap)
			}
		}
		last = cur.LastRunAt
		b.now = cur.NextRunAt
	}
	if runs := b.probe(p.ID).Runs; runs < 40 || len(spread) < 2 {
		t.Fatalf("%d runs in a day with %d distinct gaps; want >= 40 runs and some jitter", runs, len(spread))
	}
}

func TestWindowCapsTheNextRun(t *testing.T) {
	// window: 5h, every: 2h: runs at 0, 2h, 4h; the next after 4h is the window's end, not 6h,
	// and the pass at 5h ends the window (R-PRB-042, P-13).
	// covers: R-PRB-042
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, _ = b.probes(1)
	start := b.now
	p := b.start(sA, b.probeFile(sA, "p.md", "---\nevery: 2h\nwindow: 5h\nfloor: true\nwake: never\n---\ns\n"))
	for range 5 { // 0h .. 4h in hour steps
		b.pass()
		b.now = b.now.Add(time.Hour)
	}
	cur := b.probe(p.ID)
	if cur.Runs != 3 || !cur.NextRunAt.Equal(start.Add(5*time.Hour)) || cur.State != store.ProbeActive {
		t.Fatalf("after 4h: runs %d, next %v, state %s", cur.Runs, cur.NextRunAt.Sub(start), cur.State)
	}
	b.pass() // 5h
	if cur = b.probe(p.ID); cur.State != store.ProbeStopped || cur.Runs != 3 || !cur.FinalWake || len(b.listed(sA)) != 1 {
		t.Fatalf("at 5h: %+v, %d wakes", cur, len(b.listed(sA)))
	}
}

func TestShortEveryRunsOncePerPass(t *testing.T) {
	// Review Focus 3: every: 1s runs at most once per pass, scheduled from the run.
	// covers: R-PRB-040
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	fr, _ := b.probes(1)
	p := b.start(sA, b.probeFile(sA, "p.md", "---\nevery: 1s\nfloor: true\n---\ns\n"))
	b.passes(3, worker.Interval)
	if cur := b.probe(p.ID); cur.Runs != 3 || len(fr.cmds) != 3 {
		t.Fatalf("runs = %d, commands %d; want 3 and 3", cur.Runs, len(fr.cmds))
	}
}

func TestOwnerGoneMeansPauseLoggedOnce(t *testing.T) {
	// Owner and manager closed: the probe pauses with one log line and never runs; a new session
	// opening does not resume it (R-WRK-061, R-PRB-071).
	// covers: R-WRK-061, R-PRB-071
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	fr, _ := b.probes(1)
	p := b.start(sB, b.probeFile(sB, "p.md", red))
	for _, id := range []string{sB, sA} {
		if err := b.s.CloseSession(id); err != nil {
			t.Fatal(err)
		}
	}
	b.passes(3, time.Hour)
	if cur := b.probe(p.ID); cur.State != store.ProbePaused || cur.Runs != 0 || len(fr.cmds) != 0 {
		t.Fatalf("probe = %+v, %d commands", cur, len(fr.cmds))
	}
	if b.logCount("pause "+p.ID) != 1 {
		t.Fatalf("pause logged %d times", b.logCount("pause "+p.ID))
	}
	b.session(sC, store.Operator, "claude")
	b.passes(2, time.Hour)
	if cur := b.probe(p.ID); cur.State != store.ProbePaused || cur.Owner != sB {
		t.Fatalf("a new session resumed it: %+v", cur)
	}
}

func TestBrokenFileIsLoggedOnceAndRetried(t *testing.T) {
	// Review Focus 1: a deleted or broken file keeps the row active, is logged once per text,
	// re-read every minute, and runs again once it parses (R-PRB-061, P-11: every is 1m).
	// covers: R-PRB-061
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, _ = b.probes(1)
	good := "---\nevery: 1m\nfloor: ./check.sh\n---\nscope\n"
	path := b.probeFile(sA, "p.md", good)
	p := b.start(sA, path)
	b.pass()
	b.probeFile(sA, "p.md", "---\nevry: 1m\n---\n")
	b.passes(30, worker.Interval) // 2.5 min: due at 1m, retried every minute
	if cur := b.probe(p.ID); cur.State != store.ProbeActive || cur.Runs != 1 ||
		!strings.Contains(cur.FileError, "unknown field evry") {
		t.Fatalf("probe = %+v", cur)
	}
	if n := b.logCount("error " + p.ID + " file"); n != 1 {
		t.Fatalf("file error logged %d times", n)
	}
	b.probeFile(sA, "p.md", good)
	b.passes(13, worker.Interval)
	if cur := b.probe(p.ID); cur.Runs != 2 || cur.FileError != "" {
		t.Fatalf("after the fix: %+v", cur)
	}
}

func TestSlowFloorHoldsNeitherThePassNorTheHeartbeat(t *testing.T) {
	// A floor runs beside the pass: a pass with a floor still running returns and writes its
	// heartbeat, so the worker never looks dead however long a floor takes (R-WRK-008, R-WRK-060,
	// R-PRB-089).
	// covers: R-WRK-060, R-WRK-008, R-PRB-089
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, _ = b.probes(1)
	b.start(sA, b.probeFile(sA, "a.md", red))
	g := newGate(b)
	b.w.Probes.Run = g.run
	b.launch()
	if b.w.InFlight() != 1 { // the pass returned with the floor still blocked
		t.Fatalf("%d floors running after the pass", b.w.InFlight())
	}
	g.waitStarted(1)
	b.now = b.now.Add(50 * time.Second)
	b.launch() // the floor is still running
	hb, err := b.s.Heartbeat()
	if err != nil || !hb.At.Equal(b.now) {
		t.Fatalf("heartbeat = %v, %v; want %v", hb.At, err, b.now)
	}
	g.open()
	b.w.Wait()
}

func TestFailedBookingStopsTheRun(t *testing.T) {
	// A run that could not be booked sends no wake: the probe is due again next pass and would
	// repeat it (review I2). A dropped probes table makes RecordRun fail.
	// covers: R-PRB-038
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, _ = b.probes(1)
	p := b.start(sA, b.probeFile(sA, "p.md", floorless))
	b.breakBooking(p.ID)
	b.passes(3, worker.Interval)
	if n := len(b.listed(sA)); n != 0 {
		t.Fatalf("%d wakes without a booked run", n)
	}
}

func TestCheckRefusalIsLoggedOnce(t *testing.T) {
	// A probe the configuration cannot serve (a github channel, P-17) is a file error: the row
	// stays active, the text is logged once, and the probe never runs.
	// covers: R-PRB-061
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, _ = b.probes(1)
	b.w.Probes.Check = func(probe.Probe) error { return errors.New("channel gh is a github channel") }
	p := b.start(sA, b.probeFile(sA, "p.md", "---\nevery: 1m\nfloor: ./check.sh\n---\nscope\n"))
	b.passes(30, worker.Interval)
	if cur := b.probe(p.ID); cur.State != store.ProbeActive || cur.Runs != 0 ||
		!strings.Contains(cur.FileError, "github channel") {
		t.Fatalf("probe = %+v", cur)
	}
	if n := b.logCount("error " + p.ID + " file"); n != 1 {
		t.Fatalf("logged %d times", n)
	}
}

func TestWakeAfterOwnerClosedMidFloorGoesToTheManager(t *testing.T) {
	// The owner closes while its floor runs: close moved the probe to the manager, so the wake
	// lands with the row's owner, never in the closed session's inbox (R-PRB-038, R-PRB-070).
	// covers: R-PRB-038, R-PRB-070
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	fr, _ := b.probes(1)
	p := b.start(sB, b.probeFile(sB, "p.md", red))
	fr.results = []shell.Result{exit(1)}
	inner := b.w.Probes.Run
	b.w.Probes.Run = func(c shell.Cmd) (shell.Result, error) {
		if _, err := b.s.TransferProbes(sB, sA, b.now); err != nil {
			t.Error(err)
		}
		if err := b.s.CloseSession(sB); err != nil {
			t.Error(err)
		}
		return inner(c)
	}
	b.pass()
	if cur := b.probe(p.ID); cur.Owner != sA {
		t.Fatalf("probe = %+v", cur)
	}
	if got := b.listed(sA); len(got) != 1 || got[0].Probe != p.ID {
		t.Fatalf("manager wakes = %+v", got)
	}
	if got := b.listed(sB); len(got) != 0 {
		t.Fatalf("closed owner wakes = %+v", got)
	}
}

func TestPausedProbeWithLiveOwnerDoesNotRun(t *testing.T) {
	// Only probe start resumes a paused probe: a live owner alone does not (R-PRB-071).
	// covers: R-PRB-071
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	fr, _ := b.probes(1)
	p := b.start(sA, b.probeFile(sA, "p.md", red))
	if err := b.s.SetProbeState(p.ID, store.ProbePaused, "paused", b.now); err != nil {
		t.Fatal(err)
	}
	b.passes(3, time.Hour)
	if cur := b.probe(p.ID); cur.Runs != 0 || len(fr.cmds) != 0 || len(b.listed(sA)) != 0 {
		t.Fatalf("probe = %+v, %d commands", cur, len(fr.cmds))
	}
}
