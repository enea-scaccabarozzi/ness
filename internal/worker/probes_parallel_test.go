package worker_test

import (
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/shell"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

// stuck bounds a wait that only an implementation without the property would never end.
const stuck = 5 * time.Second

// gate is a floor runner that tells the test each floor started and blocks it until opened.
type gate struct {
	t       *testing.T
	started chan struct{}
	opened  chan struct{}
	once    sync.Once
	calls   atomic.Int32
	cur     atomic.Int32
	max     atomic.Int32
}

// newGate is a gate that opens and drains when the test ends, however it ends, before the store closes.
func newGate(b *bench) *gate {
	b.t.Helper()
	g := &gate{t: b.t, started: make(chan struct{}, 64), opened: make(chan struct{})}
	b.t.Cleanup(func() { g.open(); b.w.Wait() })
	return g
}

func (g *gate) run(shell.Cmd) (shell.Result, error) {
	g.calls.Add(1)
	n := g.cur.Add(1)
	defer g.cur.Add(-1)
	for m := g.max.Load(); n > m && !g.max.CompareAndSwap(m, n); m = g.max.Load() {
	}
	g.started <- struct{}{}
	select {
	case <-g.opened:
		return exit(0), nil
	case <-time.After(stuck):
		return shell.Result{}, errors.New("floor was never let go")
	}
}

// waitStarted returns once n more floors have started.
func (g *gate) waitStarted(n int) {
	g.t.Helper()
	for range n {
		select {
		case <-g.started:
		case <-time.After(stuck):
			g.t.Fatal("floors did not start together")
		}
	}
}

func (g *gate) open() { g.once.Do(func() { close(g.opened) }) }

func TestDueFloorsRunConcurrently(t *testing.T) {
	// Two floors that each wait for the other to have started finish only if they run at the
	// same time; run one after another the first never sees the second start (R-PRB-089).
	// covers: R-PRB-089
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, _ = b.probes(1)
	a := b.start(sA, b.probeFile(sA, "a.md", red))
	c := b.start(sA, b.probeFile(sA, "b.md", red))
	g := newGate(b)
	b.w.Probes.Run = g.run
	b.launch()
	g.waitStarted(2)
	g.open()
	b.w.Wait()
	for _, p := range []store.Probe{b.probe(a.ID), b.probe(c.ID)} {
		if p.Runs != 1 || p.LastState != "GREEN" {
			t.Fatalf("probe %s = %+v", p.ID, p)
		}
	}
	if b.w.InFlight() != 0 {
		t.Fatalf("%d floors still marked running", b.w.InFlight())
	}
}

func TestAtMostMaxFloorsRunAtOnce(t *testing.T) {
	// More due floors than the cap: only the cap start, the rest stay due and run once slots are
	// free; none is lost or run twice (R-PRB-089).
	// covers: R-PRB-089
	const probes = worker.MaxFloors + 2
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, _ = b.probes(1)
	ids := make([]string, 0, probes)
	for i := range probes {
		ids = append(ids, b.start(sA, b.probeFile(sA, "p"+strconv.Itoa(i)+".md", red)).ID)
	}
	g := newGate(b)
	b.w.Probes.Run = g.run
	b.launch()
	g.waitStarted(worker.MaxFloors)
	b.launch() // full: starts nothing more
	if n := b.w.InFlight(); n != worker.MaxFloors {
		t.Fatalf("%d floors running, want %d", n, worker.MaxFloors)
	}
	g.open()
	b.w.Wait()
	b.pass() // the two left over
	for _, id := range ids {
		if p := b.probe(id); p.Runs != 1 {
			t.Fatalf("probe %s ran %d times", id, p.Runs)
		}
	}
	if m := g.max.Load(); m > worker.MaxFloors || g.calls.Load() != probes {
		t.Fatalf("peak %d floors at once over %d runs", m, g.calls.Load())
	}
}

func TestRunningFloorIsNotStartedAgain(t *testing.T) {
	// A floor still running when its probe is due again (the next pass, past every) is not started
	// a second time (R-PRB-089).
	// covers: R-PRB-089
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, _ = b.probes(1)
	p := b.start(sA, b.probeFile(sA, "a.md", red))
	g := newGate(b)
	b.w.Probes.Run = g.run
	b.launch()
	g.waitStarted(1)
	b.now = b.now.Add(31 * time.Minute) // every is 30m: due again
	b.launch()
	if n := b.w.InFlight(); n != 1 {
		t.Fatalf("%d floors running for one probe", n)
	}
	g.open()
	b.w.Wait()
	if got := b.probe(p.ID); got.Runs != 1 || g.calls.Load() != 1 {
		t.Fatalf("runs = %d, floor calls = %d, want 1 and 1", got.Runs, g.calls.Load())
	}
}

func TestFloorOfAProbeStoppedMeanwhileIsNotBooked(t *testing.T) {
	// A probe stopped while its floor runs gets no booking, wake or post from that run (R-PRB-089).
	// covers: R-PRB-089
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, posts := b.probes(1)
	p := b.start(sA, b.probeFile(sA, "a.md",
		"---\nevery: 30m\nfloor: ./c.sh\nwake: always\nnotify: auto\nchannel: ops\n---\nscope\n"))
	g := newGate(b)
	b.w.Probes.Run = g.run
	b.launch()
	g.waitStarted(1)
	if err := b.s.SetProbeState(p.ID, store.ProbeStopped, "done", b.now); err != nil {
		t.Fatal(err)
	}
	g.open()
	b.w.Wait()
	if got := b.probe(p.ID); got.Runs != 0 || got.State != store.ProbeStopped || len(b.listed(sA)) != 0 || len(*posts) != 0 {
		t.Fatalf("probe = %+v, wakes %d, posts %v", got, len(b.listed(sA)), *posts)
	}
}
