package worker

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/probe"
	"github.com/enea-scaccabarozzi/ness/internal/shell"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// ReloadEvery is how soon a probe whose file does not parse is read again.
const ReloadEvery = time.Minute

// MaxFloors is how many floors run at once (R-PRB-089).
const MaxFloors = 8

// Probes is the scheduler slot's dependencies (R-WRK-060): where probe files and floors live,
// what to redact, the runner, the random source for jitter and sample, and the poster (nil:
// the notify feature is off).
type Probes struct {
	Primary string
	Redact  []string
	Getenv  func(string) string
	Run     shell.Runner
	Rand    *rand.Rand
	Post    func(channel, markdown, thread string) error
	// Check refuses a parsed probe the project's configuration cannot serve (nil: none), e.g. a
	// github channel; its text is logged once like a file error.
	Check func(probe.Probe) error

	floors   sync.WaitGroup  // the floor goroutines in flight
	mu       sync.Mutex      // guards inflight
	inflight map[string]bool // probe ids whose floor is running
	book     sync.Mutex      // one booking at a time: Rand and the wake are not for two goroutines
	send     sync.Mutex      // one post at a time, outside book so a slow channel holds no booking
	stopping atomic.Bool     // the worker is shutting down: no floor starts or is booked
}

// probes runs every due probe. A probe whose owner is not an open session pauses, logged once
// (R-WRK-061); nothing here starts an agent (D10). Floors run beside the pass (R-PRB-089).
func (w *Worker) probes(now time.Time, live map[string]store.Session) {
	list, err := w.Store.Probes(false)
	if err != nil {
		w.Log.Error("probes", err)
		return
	}
	w.Log.Clear("probes")
	// Longest overdue first, so a probe past the cap cannot starve behind the ones before it in path
	// order (R-PRB-089).
	slices.SortStableFunc(list, func(a, b store.Probe) int { return a.NextRunAt.Compare(b.NextRunAt) })
	for _, p := range list {
		owner, ok := live[p.Owner]
		switch {
		case p.State == store.ProbePaused:
			continue
		case !ok:
			note := "owner " + p.Owner + " is not live"
			if perr := w.Store.SetProbeState(p.ID, store.ProbePaused, note, now); perr != nil {
				w.Log.Error(p.ID+" pause", perr)
				continue
			}
			w.Log.Action("pause", p.ID, note)
		case !p.NextRunAt.After(now) && !w.Probes.running(p.ID):
			w.runProbe(p, owner, w.Now())
		}
	}
}

// Wait blocks until every floor a pass started has run and been booked: what a one-shot pass needs.
func (w *Worker) Wait() {
	if w.Probes != nil {
		w.Probes.floors.Wait()
	}
}

// Stop is the loop's shutdown: no floor starts, and one that ends later is not booked (its probe is
// still due, so it runs again after a restart). It does not wait (R-PRB-089).
func (w *Worker) Stop() {
	if w.Probes != nil {
		w.Probes.stopping.Store(true)
	}
}

// running reports whether the probe's floor is still running: its next run is not booked until the
// floor ends, so it stays due and the pass must not pick it again (R-PRB-089).
func (p *Probes) running(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.inflight[id]
}

// claim marks the probe's floor running, or refuses when MaxFloors are (the probe stays due for
// the next pass).
func (p *Probes) claim(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.inflight) >= MaxFloors || p.stopping.Load() {
		return false
	}
	if p.inflight == nil {
		p.inflight = map[string]bool{}
	}
	p.inflight[id] = true
	return true
}

func (p *Probes) release(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.inflight, id)
}

// runProbe is the sequence for one due probe: reload the file (R-PRB-061), end the window
// (R-PRB-042), then start the floor in the background and book it when it ends (floor), or book a
// floorless run at once.
func (w *Worker) runProbe(p store.Probe, owner store.Session, now time.Time) {
	root := filepath.Join(w.Probes.Primary, owner.Worktree)
	pf, ok := w.reload(p, root, now)
	if !ok {
		return
	}
	if pf.Window > 0 && !now.Before(p.StartedAt.Add(pf.Window)) {
		w.endWindow(p, pf, now)
		return
	}
	if pf.Floorless() {
		w.settle(p, pf, probe.Run{}, nil, now)
		return
	}
	if !w.Probes.claim(p.ID) {
		return
	}
	w.Probes.floors.Add(1)
	go w.floor(p, pf, root, now)
}

// floor runs the probe's floor, books it and then posts. A floor that ends after Stop is dropped.
func (w *Worker) floor(p store.Probe, pf probe.Probe, root string, now time.Time) {
	defer w.Probes.floors.Done()
	defer w.Probes.release(p.ID) // after the post: the probe is not due and unclaimed in between
	f := probe.Floor{Dir: root, Owner: p.Owner, Label: p.Label, Redact: w.Probes.Redact, Getenv: w.Probes.Getenv}
	run, err := probe.RunFloor(w.Probes.Run, pf, f)
	if post := w.settle(p, pf, run, err, now); post != nil {
		post()
	}
}

// settle books a run, one booking at a time (Rand and the wakes have one user), and returns the
// post that follows, to be sent outside the lock. The probe is read again first: p is the row the
// run was made for, and a stop, pause or restart (a new start or next run) while the floor ran
// means nothing is written (RecordRun checks the same, atomically).
func (w *Worker) settle(p store.Probe, pf probe.Probe, run probe.Run, err error, now time.Time) func() {
	w.Probes.book.Lock()
	defer w.Probes.book.Unlock()
	cur, perr := w.Store.Probe(p.ID)
	switch {
	case w.Probes.stopping.Load():
		w.Log.Action("floor dropped", p.ID, "worker is stopping")
		return nil
	case perr != nil || cur.State != store.ProbeActive ||
		!cur.StartedAt.Equal(p.StartedAt) || !cur.NextRunAt.Equal(p.NextRunAt):
		w.Log.Action("floor dropped", p.ID, "probe changed while its floor ran")
		return nil
	case err != nil:
		w.floorFailed(cur, pf, err, now)
		return nil
	}
	w.Log.Clear(p.ID + " floor")
	return w.finish(cur, pf, run, now)
}

// finish books a run and decides its wake and stop; it returns the post, if any.
func (w *Worker) finish(p store.Probe, pf probe.Probe, run probe.Run, now time.Time) func() {
	rec, confirmed := w.bookOf(p, pf, run, now)
	if !w.record(p.ID, rec) {
		return nil // not booked: no wake, post or stop, or the next pass would repeat them
	}
	next := "-"
	if !rec.Next.IsZero() {
		next = rec.Next.UTC().Format(time.RFC3339)
	}
	w.Log.Action("probe", p.ID, fmt.Sprintf("%s (run %d, next %s)", stateOrDash(run.State), rec.Runs, next))
	return w.decide(p, pf, rec, run, confirmed, now)
}

// bookOf is what a run writes back, and whether its state is confirmed: GREEN and a floorless
// run at once, a non-GREEN state when its streak reaches confirm (R-PRB-037). A confirmed state
// becomes the confirmed state (R-PRB-032).
func (w *Worker) bookOf(p store.Probe, pf probe.Probe, run probe.Run, now time.Time) (store.ProbeRun, bool) {
	streak := 0
	if run.State != "" && run.State != probe.Green {
		streak = p.Streak + 1
	}
	confirmed := pf.Floorless() || run.State == probe.Green || streak >= pf.Confirm
	rec := store.ProbeRun{
		State: run.State, Evidence: run.Evidence, Hash: pf.Hash, Streak: streak, Runs: p.Runs + 1,
		At: now, Next: w.next(p, pf, now), Confirmed: p.ConfirmedState, StartedAt: p.StartedAt, Due: p.NextRunAt,
	}
	if confirmed && run.State != "" {
		rec.Confirmed = run.State
	}
	return rec, confirmed
}

// reload reads and parses the probe file. A file that does not parse keeps the row active, is
// logged once per text and read again after ReloadEvery (R-PRB-061).
func (w *Worker) reload(p store.Probe, root string, now time.Time) (probe.Probe, bool) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p.Path))) //nolint:gosec // the owner's own file
	var pf probe.Probe
	if err == nil {
		pf, err = probe.Parse(p.Path, data)
	}
	if err == nil && w.Probes.Check != nil {
		err = w.Probes.Check(pf)
	}
	if err != nil {
		w.Log.Error(p.ID+" file", err)
		w.record(p.ID, store.ProbeRun{FileError: err.Error(), At: now, Next: now.Add(ReloadEvery)})
		return pf, false
	}
	w.Log.Clear(p.ID + " file")
	return pf, true
}

// floorFailed handles a floor that cannot start: it keeps the last state and schedules the next run.
func (w *Worker) floorFailed(p store.Probe, pf probe.Probe, err error, now time.Time) {
	w.Log.Error(p.ID+" floor", err)
	next := w.next(p, pf, now)
	if next.IsZero() {
		next = now.Add(ReloadEvery) // a once probe has no next run, but this one did not happen
	}
	w.record(p.ID, store.ProbeRun{
		State: p.LastState, Evidence: p.Evidence, Hash: pf.Hash, Streak: p.Streak, Runs: p.Runs,
		At: now, Next: next, Confirmed: p.ConfirmedState, StartedAt: p.StartedAt, Due: p.NextRunAt,
	})
}

// decide is what a booked run causes: the wake and the end of a once probe now, and the returned
// channel post (nil: none) for the caller to send outside the booking lock.
func (w *Worker) decide(
	p store.Probe, pf probe.Probe, rec store.ProbeRun, run probe.Run, confirmed bool, now time.Time,
) func() {
	if w.wakes(pf, p.ConfirmedState, run.State, confirmed) {
		w.wake(p, pf, rec.Runs, run, false, now)
	}
	if pf.Once {
		w.stopProbe(p.ID, "once: ran", now)
	}
	if !pf.Floorless() && pf.Notify == "auto" && confirmed {
		return func() { w.autoPost(p, pf, run) }
	}
	return nil
}

func stateOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// wakes decides a wake (R-PRB-030..037): a floorless run always; otherwise a sampled GREEN, or
// the wake policy for a state that confirm lets through.
func (w *Worker) wakes(pf probe.Probe, confirmedBefore, state string, confirmed bool) bool {
	switch {
	case pf.Floorless(), w.sampledGreen(pf, state):
		return true
	case state != probe.Green && !confirmed:
		return false
	}
	return policyWakes(pf, confirmedBefore, state)
}

// sampledGreen is the random fraction of GREEN runs that wake whatever the policy (R-PRB-036).
func (w *Worker) sampledGreen(pf probe.Probe, state string) bool {
	return state == probe.Green && pf.Sample > 0 && w.Probes.Rand.Float64() < pf.Sample
}

// policyWakes applies the wake policy to a confirmed run (R-PRB-031..035). change compares with
// the last confirmed state, so a flap confirm suppressed is neither a wake nor a baseline; a first
// confirmed GREEN is a silent baseline, a first confirmed non-GREEN wakes.
func policyWakes(pf probe.Probe, confirmedBefore, state string) bool {
	switch pf.Wake {
	case "always":
		return true
	case "change":
		return state != confirmedBefore && (confirmedBefore != "" || state != probe.Green)
	case "red":
		return state == probe.Red
	case "not-green":
		return state != probe.Green
	}
	return false
}

// next is the next run: every plus a random delay within jitter, counted from this run, not from
// a fixed slot (R-PRB-040), and no later than the end of the window (R-PRB-042). A once probe
// has none.
func (w *Worker) next(p store.Probe, pf probe.Probe, now time.Time) time.Time {
	if pf.Once {
		return time.Time{}
	}
	d := pf.Every
	if pf.Jitter > 0 {
		d += time.Duration(w.Probes.Rand.Int64N(int64(pf.Jitter)))
	}
	at := now.Add(d)
	if end := p.StartedAt.Add(pf.Window); pf.Window > 0 && at.After(end) {
		return end
	}
	return at
}

// wake puts the request in the owner's inbox (R-PRB-038).
func (w *Worker) wake(p store.Probe, pf probe.Probe, run int, r probe.Run, final bool, now time.Time) bool {
	id, err := w.Store.SendWake(p.ID, WakeBody(pf, p.Label, run, r, final), now)
	if errors.Is(err, store.ErrProbeNotActive) {
		w.Log.Action("wake skipped", p.ID, "probe is no longer active")
		return false
	}
	if err != nil {
		w.Log.Error(p.ID+" wake", err)
		return false
	}
	w.Log.Clear(p.ID + " wake")
	w.Log.Action("wake", p.ID, id)
	return true
}

// record books a run and reports whether it was written.
func (w *Worker) record(id string, r store.ProbeRun) bool {
	err := w.Store.RecordRun(id, r)
	if errors.Is(err, store.ErrNoProbe) && !r.StartedAt.IsZero() {
		w.Log.Action("floor dropped", id, "probe changed while its floor ran")
		return false
	}
	if err != nil {
		w.Log.Error(id+" record", err)
		return false
	}
	w.Log.Clear(id + " record")
	return true
}

// stopProbe ends a probe for good (Worker.stop is the stop of a session's pane).
func (w *Worker) stopProbe(id, reason string, now time.Time) {
	if err := w.Store.SetProbeState(id, store.ProbeStopped, reason, now); err != nil {
		w.Log.Error(id+" stop", err)
		return
	}
	w.Log.Action("stop", id, reason)
}

// WakeBody is the wake item's text: the probe (with its label) and run, the scope, the evidence,
// the answer.
func WakeBody(pf probe.Probe, label string, run int, r probe.Run, final bool) string {
	var b strings.Builder
	name := probe.Name(pf.Path, label)
	switch {
	case final:
		fmt.Fprintf(&b, "probe %s: window elapsed, final verdict requested\n", name)
	case pf.Floorless():
		fmt.Fprintf(&b, "probe %s run %d\n", name, run)
	default:
		fmt.Fprintf(&b, "probe %s run %d: %s\n", name, run, r.State)
	}
	b.WriteString("\n" + pf.Scope + "\n")
	if r.Evidence != "" {
		b.WriteString("\nEvidence:\n```\n" + r.Evidence)
		if !strings.HasSuffix(r.Evidence, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n")
	}
	b.WriteString("\nAnswer with: ness probe verdict ITEM --state green|red --note TEXT [--probe-bug]\n")
	return b.String()
}

// endWindow ends a probe whose window elapsed: one final wake asking for a verdict, then the
// stop (R-PRB-042). A probe already asked is left alone; a wake that failed is tried next pass.
func (w *Worker) endWindow(p store.Probe, pf probe.Probe, now time.Time) {
	if !p.FinalWake {
		if !w.wake(p, pf, p.Runs, probe.Run{Evidence: p.Evidence}, true, now) {
			return
		}
		if err := w.Store.MarkFinalWake(p.ID); err != nil {
			w.Log.Error(p.ID+" final", err)
			return
		}
	}
	w.stopProbe(p.ID, "window elapsed", now)
}

// post sends markdown to the probe's channel in the probe's own thread and reports whether it
// went out; with notify off, one log line per probe says so and the probe keeps running (R-PRB-051).
func (w *Worker) post(p store.Probe, pf probe.Probe, markdown string) bool {
	if w.Probes.Post == nil {
		w.Log.Error(p.ID+" post", errors.New("notify is off; probe posts are skipped"))
		return false
	}
	w.Probes.send.Lock()
	defer w.Probes.send.Unlock()
	if err := w.Probes.Post(pf.Channel, markdown, probe.Thread(p.Path, p.Label)); err != nil {
		w.Log.Error(p.ID+" post", err)
		return false
	}
	w.Log.Clear(p.ID + " post")
	w.Log.Action("post", p.ID, pf.Channel)
	return true
}

// autoPost posts a confirmed state that differs from the last one posted and records it (R-PRB-037,
// R-PRB-051). The first confirmed GREEN is a silent baseline. A failed post is tried again at the
// next confirmed run, as the state is not recorded.
func (w *Worker) autoPost(p store.Probe, pf probe.Probe, run probe.Run) {
	if run.State == "" || run.State == p.NotifiedState {
		return
	}
	if p.NotifiedState != "" || run.State != probe.Green {
		text := fmt.Sprintf("probe %s: %s \u2192 %s\n\n```\n%s```\n",
			probe.Name(p.Path, p.Label), stateOrDash(p.NotifiedState), run.State, run.Evidence)
		if !w.post(p, pf, text) {
			return
		}
	}
	if err := w.Store.SetNotifiedState(p.ID, run.State); err != nil {
		w.Log.Error(p.ID+" notified", err)
	}
}
