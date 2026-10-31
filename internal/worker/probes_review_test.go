package worker_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/shell"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

const wakeAlways = "---\nevery: 30m\nfloor: ./c.sh\nwake: always\n---\nscope\n"

func TestRestartDuringFloorIsNotBookedByTheOldRun(t *testing.T) {
	// A once probe restarted (same row, still active) while its old floor runs: the old floor's result
	// is dropped, so the restarted probe is neither stopped nor delayed by it (R-PRB-089).
	// covers: R-PRB-089
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, _ = b.probes(1)
	path := b.probeFile(sA, "a.md", "---\nonce: true\nfloor: ./c.sh\n---\nscope\n")
	p := b.start(sA, path)
	g := newGate(b)
	b.w.Probes.Run = g.run
	b.launch()
	g.waitStarted(1)
	b.now = b.now.Add(time.Minute)
	if again := b.start(sA, path); again.ID != p.ID || again.State != store.ProbeActive {
		t.Fatalf("restart = %+v", again)
	}
	g.open()
	b.w.Wait()
	if got := b.probe(p.ID); got.State != store.ProbeActive || got.Runs != 0 || !got.NextRunAt.Equal(b.now) {
		t.Fatalf("after the old floor: %+v", got)
	}
}

func TestFloorEndingAfterStopIsNotBooked(t *testing.T) {
	// The worker's stop kills its floors (exit 143, UNCLEAR): none of them is booked, woken or posted;
	// the probe stays due and runs again after a restart. A stopped worker starts no floor (R-PRB-089).
	// covers: R-PRB-089
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, _ = b.probes(1)
	p := b.start(sA, b.probeFile(sA, "a.md", wakeAlways))
	b.w.Probes.Run = func(shell.Cmd) (shell.Result, error) {
		b.w.Stop() // the signal lands, then the shell dies
		return shell.Result{Exit: 143}, nil
	}
	b.pass()
	if got := b.probe(p.ID); got.Runs != 0 || len(b.listed(sA)) != 0 || got.NextRunAt.After(b.now) {
		t.Fatalf("a floor killed at shutdown was booked: %+v, wakes %d", got, len(b.listed(sA)))
	}
	g := newGate(b)
	b.w.Probes.Run = g.run
	b.launch()
	if b.w.InFlight() != 0 || g.calls.Load() != 0 {
		t.Fatalf("a stopped worker started a floor")
	}
}

func TestSlowPostHoldsNeitherTheBookingNorThePass(t *testing.T) {
	// A channel post that hangs runs outside the booking lock: the next pass, with a floorless
	// probe to book, still returns (R-PRB-089).
	// covers: R-PRB-089
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, _ = b.probes(1)
	b.start(sA, b.probeFile(sA, "a.md", "---\nevery: 30m\nfloor: ./c.sh\nnotify: auto\nchannel: ops\n---\ns\n"))
	b.w.Probes.Run = func(shell.Cmd) (shell.Result, error) { return exit(1), nil }
	posting, release := make(chan struct{}, 1), make(chan struct{})
	b.w.Probes.Post = func(string, string, string) error {
		posting <- struct{}{}
		select {
		case <-release:
		case <-time.After(stuck):
		}
		return nil
	}
	t.Cleanup(func() { close(release); b.w.Wait() })
	b.launch()
	select {
	case <-posting:
	case <-time.After(stuck):
		t.Fatal("no post")
	}
	b.start(sA, b.probeFile(sA, "z.md", floorless)) // due now, to be booked inline by the pass
	done := make(chan error, 1)
	go func() { done <- b.w.Pass() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(stuck / 2):
		t.Fatal("the pass is held by a post")
	}
}

func TestOverdueProbesGetTheFreeSlotsFirst(t *testing.T) {
	// More due probes than slots: the longest overdue starts first, whatever its path (R-PRB-089).
	// covers: R-PRB-089
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, _ = b.probes(1)
	ids := make([]string, 0, 9)
	for i := range 9 {
		ids = append(ids, b.start(sA, b.probeFile(sA, "p"+strconv.Itoa(i)+".md", red)).ID)
	}
	g := newGate(b)
	b.w.Probes.Run = g.run
	b.launch()
	g.waitStarted(8)
	g.open()
	b.w.Wait() // p8 was left over: overdue since the start
	b.now = b.now.Add(31 * time.Minute)
	g2 := newGate(b)
	b.w.Probes.Run = g2.run
	b.launch()
	g2.waitStarted(8)
	g2.open()
	b.w.Wait()
	if last := b.probe(ids[8]); last.Runs != 1 {
		t.Fatalf("the longest overdue probe waited behind the others: %+v", last)
	}
}

func TestFloorOfAPausedProbeIsNotBooked(t *testing.T) {
	// covers: R-PRB-089
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, _ = b.probes(1)
	p := b.start(sA, b.probeFile(sA, "a.md", wakeAlways))
	g := newGate(b)
	b.w.Probes.Run = g.run
	b.launch()
	g.waitStarted(1)
	if err := b.s.SetProbeState(p.ID, store.ProbePaused, "owner closed", b.now); err != nil {
		t.Fatal(err)
	}
	g.open()
	b.w.Wait()
	if got := b.probe(p.ID); got.Runs != 0 || got.State != store.ProbePaused || len(b.listed(sA)) != 0 {
		t.Fatalf("probe = %+v, wakes %d", got, len(b.listed(sA)))
	}
}

func TestFloorOfAHandedOverProbeWakesTheNewOwner(t *testing.T) {
	// The owner closes while the floor runs and the probe moves to its manager: the run is booked and
	// the wake lands with the new owner (R-PRB-070, R-PRB-089).
	// covers: R-PRB-070, R-PRB-089
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	b.session(sB, sA, "claude")
	_, _ = b.probes(1)
	p := b.start(sB, b.probeFile(sB, "a.md", wakeAlways))
	g := newGate(b)
	b.w.Probes.Run = g.run
	b.launch()
	g.waitStarted(1)
	if _, err := b.s.TransferProbes(sB, sA, b.now); err != nil {
		t.Fatal(err)
	}
	g.open()
	b.w.Wait()
	if got := b.probe(p.ID); got.Runs != 1 || got.Owner != sA || len(b.listed(sA)) != 1 || len(b.listed(sB)) != 0 {
		t.Fatalf("probe = %+v, wakes %d / %d", got, len(b.listed(sA)), len(b.listed(sB)))
	}
}

func TestFloorThatCannotStartKeepsTheLastStateAndSchedulesTheNext(t *testing.T) {
	// The runner fails: no booked run, no wake, the error logged once, the next run in every
	// (R-PRB-089).
	// covers: R-PRB-089
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, _ = b.probes(1)
	p := b.start(sA, b.probeFile(sA, "a.md", wakeAlways))
	b.w.Probes.Run = func(shell.Cmd) (shell.Result, error) { return shell.Result{}, errors.New("no sh") }
	b.pass()
	got := b.probe(p.ID)
	if got.Runs != 0 || !got.NextRunAt.Equal(b.now.Add(30*time.Minute)) || len(b.listed(sA)) != 0 ||
		b.logCount("error "+p.ID+" floor no sh") != 1 {
		t.Fatalf("probe = %+v\n%s", got, strings.Join(b.logLines(), "\n"))
	}
}

func TestWindowEndsAfterTheFloorThatWasRunning(t *testing.T) {
	// The window elapses while a floor runs: that run is booked, then the next pass ends the window
	// with its final wake (R-PRB-042, R-PRB-089).
	// covers: R-PRB-042, R-PRB-089
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, _ = b.probes(1)
	p := b.start(sA, b.probeFile(sA, "a.md", "---\nevery: 30m\nwindow: 1h\nfloor: ./c.sh\n---\nscope\n"))
	g := newGate(b)
	b.w.Probes.Run = g.run
	b.launch()
	g.waitStarted(1)
	b.now = b.now.Add(2 * time.Hour)
	b.launch() // running: not touched, no final wake yet
	if got := b.probe(p.ID); got.FinalWake || got.State != store.ProbeActive || len(b.listed(sA)) != 0 {
		t.Fatalf("window ended under a running floor: %+v", got)
	}
	g.open()
	b.w.Wait()
	b.pass()
	if got := b.probe(p.ID); got.Runs != 1 || got.State != store.ProbeStopped || len(b.listed(sA)) != 1 {
		t.Fatalf("probe = %+v, wakes %d", got, len(b.listed(sA)))
	}
}
