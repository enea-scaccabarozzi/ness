package worker

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// Interval is the pass interval (R-WRK-024).
const Interval = 5 * time.Second

// Client is what the worker needs from a client atom: how to read its pane (R-WRK-015,
// R-WRK-040, R-WRK-051) and the text that resumes it after a usage limit (R-WRK-043).
type Client struct {
	Read   func(ansi string, now time.Time) session.Screen
	Resume string
	// Rename is the text that renames a running conversation; nil when the client has none.
	Rename func(name string) string
	// Activity reads the session's conversation, where the screen is silent: background work still
	// running, the last real turn, a usage-limit error (R-WRK-096). Nil: the client has no reading.
	Activity func(sess store.Session, now time.Time) (session.Activity, bool)
}

// Worker is one project's worker: the store, the runtime, a Client per client name (a session of
// a client without one is skipped), the client's usage file (nil: none), the binary's version for
// the heartbeat, the clock, the log, and the stall and quiet windows from [worker].
type Worker struct {
	Store   *store.Store
	Runtime session.Runtime
	Clients map[string]Client
	Usage   func(now time.Time) (time.Time, bool)
	Version string
	Now     func() time.Time
	Log     *Log
	Stall   time.Duration
	Quiet   time.Duration
	Probes  *Probes // nil: the probes feature is off
	// Notify posts one alert to the operator's channel (CX-16); nil: no channel, nobody is told.
	Notify func(Alert) error
	// Project names sessions in their client: <Project>/<title> (R-WRK-071).
	Project string
	// Stale reports whether the session's worktree lock is behind its base branch's (R-UPD-070);
	// nil: the check is off.
	Stale func(sess store.Session) (Stale, bool, error)
	// Update runs the unattended update step after the sessions (R-UPD-050, R-UPD-080..082); its
	// error is logged once per text under key update. Nil: no update step.
	Update func(now time.Time) error

	panes map[string]*pane         // by session id; what the worker remembers between passes
	open  map[string]store.Session // the open sessions of this pass
}

// pane is what the worker remembers of a session's pane: the ids its last hint covered, the input
// box and when it last changed, the live limit notice, the stall episode.
type pane struct {
	run                  int
	at                   string
	hinted               []string
	lastDraft            time.Time // when the box's content last changed (R-WRK-018)
	box                  string    // the draft on the last read, "" for an empty box
	idleSince            time.Time
	busyStart            time.Time // when the latest run of busy passes began
	stalled, errored     bool
	alerted              map[string]bool // episodes the operator was told about: stall, error, stop, limit, yourmove
	stallText            string          // the sentence of the stall report, for its alert
	stallTold            time.Time       // when the manager was told of the stall episode (R-WRK-094)
	status               string          // herdr's agent status on the last pass
	blockedSince         time.Time       // when the pane's blocked status was first seen (R-WRK-081)
	blockedTold          bool            // the manager was told of the blocked episode
	blockedAlerts        int             // Trouble alerts sent in the blocked episode
	stopped              bool            // the stop of an unavailable model is done
	errName              string          // the error pattern read on the last pass
	errRuns, unavailRuns int             // consecutive passes that read it, and the unavailable model
	limit                *session.Limit  // the notice on screen
	nextRead             time.Time       // under a reset-less notice, the next screen read
	compactRead          time.Time       // when a blocked pane's screen was last read for a compaction
	compactSeen          bool            // what that read found
	draft                bool            // the box held a draft on the last read
	background           bool            // the screen showed a live background task, or the transcript one pending
	pending              bool            // the transcript shows background work launched and not finished
	tracked              bool            // the client's activity was read this pass
	turn                 time.Time       // the last real assistant turn read; a newer one ends an episode
	limitText            string          // the usage-limit error that ends the transcript, "" when none
	limitAt              time.Time       // when that error was recorded
	limitLogged          bool            // the limit episode is in the log
	limitTailed          bool            // the screen at the limit is in the log
	tail                 []string        // the last lines of the screen on the last read
	compacted            bool            // /compact was typed but the request could not be cleared
	resumed              bool            // the resume was typed for this limit
	renamed              bool            // the adopted root's rename was typed (R-WRK-071)
	stale                string          // the notice the resume answered, ignored while it stays on screen (H3)
	remote               string          // the last Remote Control URL read in the transcript (R-WRK-082)
}

// working notes a pane that is not idle. Work after a known reset is the operator (or the agent)
// resuming by hand: the notice still on screen is stale from then on (H3).
func (p *pane) working(now time.Time) {
	if p.limit != nil && !p.limit.Reset.IsZero() && !now.Before(p.limit.Reset) {
		p.stale = p.limit.Notice
	}
	if p.busyStart.IsZero() || !p.idleSince.IsZero() {
		p.busyStart = now // a busy pass after an idle one begins a new turn
	}
	p.idleSince, p.box = time.Time{}, "" // a submitted draft left the box: not an edit (R-WRK-018)
	p.errored, p.background = false, false
	delete(p.alerted, "error")
	if !p.tracked { // without the transcript any work ends the stall episode; with it only a real turn does
		p.newEpisode()
	}
	p.errName, p.errRuns, p.unavailRuns = "", 0, 0
}

// quiet reports whether the input box changed within q (R-WRK-018). Focus and scrolling are not
// the operator at the keyboard: herdr reports the selected tab as focused with nobody there (CX-15).
func (p *pane) quiet(now time.Time, q time.Duration) bool {
	return !p.lastDraft.IsZero() && now.Sub(p.lastDraft) < q
}

// Pass is one pass (design §5 "Machinery"): the heartbeat (R-WRK-008), zombie retirement
// (R-WRK-020), the due probes (R-WRK-060), every open session's pane, then the update step. The
// error is the store's; a pane's or the update step's trouble is logged and the pass goes on.
func (w *Worker) Pass() error {
	now := w.Now()
	if err := w.Store.SetHeartbeat(now, w.Version); err != nil {
		return err
	}
	retired, err := w.Store.RetireDead(now)
	if err != nil {
		return err
	}
	for _, r := range retired {
		w.Log.Action("retire", r.ID, "to "+r.Recipient+" (closed or unknown)")
	}
	fileLimit := w.fileLimit(now)
	sessions, err := w.Store.Sessions(false)
	if err != nil {
		return err
	}
	if w.panes == nil {
		w.panes = map[string]*pane{}
	}
	live := map[string]store.Session{}
	for _, sess := range sessions {
		live[sess.ID] = sess
	}
	if w.Probes != nil {
		w.probes(now, live)
	}
	w.open = live
	w.alertItems(now)
	for _, sess := range sessions {
		w.visit(sess, now, fileLimit)
	}
	w.forget(live)
	w.update(now)
	return nil
}

// update runs the update step, its error logged once per text under key update.
func (w *Worker) update(now time.Time) {
	if w.Update == nil {
		return
	}
	if err := w.Update(now); err != nil {
		w.Log.Error("update", err)
		return
	}
	w.Log.Clear("update")
}

// forget drops the memory of panes whose session is no longer open.
func (w *Worker) forget(live map[string]store.Session) {
	for id := range w.panes {
		if _, ok := live[id]; !ok {
			delete(w.panes, id)
		}
	}
}

// pane returns the session's memory; a new run or pane is a new process, so it starts blank (H5).
func (w *Worker) pane(sess store.Session) *pane {
	p, ok := w.panes[sess.ID]
	if !ok || p.run != sess.Run || p.at != sess.Pane {
		p = &pane{run: sess.Run, at: sess.Pane}
		w.panes[sess.ID] = p
	}
	return p
}

// visit decides what one session's pane gets this pass. A session still launching (no pane, no
// ready mark) gets nothing; a pane whose agent is gone, not the session's own, or not idle gets
// nothing and one log line (R-WRK-013); a non-empty box gets nothing, focus holds nothing
// (R-WRK-014, R-WRK-017); a limited pane is silent (R-WRK-042, R-WRK-047); otherwise
// one resume after a reset (R-WRK-043), a compaction at the boundary (R-WRK-030) or one hint
// (R-WRK-010, R-WRK-011, R-WRK-019). An idle stale session gets its update item first, which the
// hint then covers (R-UPD-070). A session whose model is reported unavailable is stopped and its
// manager told (R-SESS-009, R-WRK-062); a screen error or an idle spell with open obligations is
// reported to the manager once per episode (R-WRK-050, R-WRK-051, R-WRK-052).
func (w *Worker) visit(sess store.Session, now, fileLimit time.Time) {
	c, ok := w.Clients[sess.Client]
	if !ok {
		w.Log.Error(sess.ID, fmt.Errorf("client %s is not delivered to", sess.Client))
		return
	}
	if sess.Pane == "" || sess.ReadyAt.IsZero() || session.PromptPending(sess) {
		return
	}
	p := w.pane(sess)
	if p.stopped {
		w.alertStop(sess, p)
	}
	st, ok := w.agent(sess)
	p.status = st.Status
	if !ok {
		return
	}
	w.noteActivity(sess, c, p, st.ClientSession, now)
	w.noteBlocked(sess, c, p, st.Status, now)
	if !idle(st.Status) {
		p.working(now)
		return
	}
	w.deliverUpdate(sess, now)
	if w.observe(sess, c, p, now) {
		w.act(sess, c, p, st.Focused, now, now.Before(fileLimit))
	}
}

// idle classifies herdr's agent status (schema AgentStatus: idle, working, blocked, done, unknown).
// done is idle after unseen background work finished: the same prompt, in a tab the operator has not
// focused since, and it stays done until they do (herdr's lifecycle). blocked holds a permission
// prompt a keystroke would answer; working, unknown and any value a newer herdr adds are not idle.
func idle(status string) bool {
	switch status {
	case "idle", "done":
		return true
	case "working", "blocked", "unknown":
		return false
	}
	return false
}

// fileLimit is the end of the client's usage-file limit, zero when none is in force (R-WRK-047).
func (w *Worker) fileLimit(now time.Time) time.Time {
	if w.Usage == nil {
		return time.Time{}
	}
	if until, ok := w.Usage(now); ok {
		return until
	}
	return time.Time{}
}

// act is the one thing an observed, idle pane gets: nothing while limited or holding a draft;
// else the stop of a session whose model is unavailable, the reports, and the resume, the
// compaction or the hint, in that order.
func (w *Worker) act(sess store.Session, c Client, p *pane, focused bool, now time.Time, fileLimited bool) {
	if fileLimited || p.limited(now) {
		p.idleSince = now // the idle clock restarts when the limit ends
		w.tailLimit(sess, p)
		return
	}
	if p.mustStop(focused) {
		w.stop(sess, p, now)
		return
	}
	w.report(sess, p, now)
	switch {
	case p.draft:
	case w.resumeOrRename(sess, p, c, now):
	case sess.BoundaryNote != "":
		if !p.quiet(now, w.Quiet) { // the operator was just here (R-WRK-018): same wait as a hint
			w.compact(sess, p)
		}
	default:
		w.hint(sess, p, now)
	}
}

// observe notes the input box of an idle pane (a draft, when it changed) and the limit notice on
// screen; false when the screen was not read this pass or could not be.
func (w *Worker) observe(sess store.Session, c Client, p *pane, now time.Time) bool {
	if p.idleSince.IsZero() {
		p.idleSince = now
	}
	if !p.readable(now) {
		return false
	}
	screen, err := w.Runtime.ReadANSI(sess.Pane)
	if err != nil {
		w.Log.Error(sess.ID+" read", err)
		return false
	}
	w.Log.Clear(sess.ID + " read")
	p.nextRead = now.Add(limitRecheck)
	sc := c.Read(screen, now)
	w.noteScreen(sess.ID, sc)
	if sc.Draft != p.box {
		p.lastDraft = now
	}
	p.box, p.draft = sc.Draft, sc.Draft != ""
	p.seen(sc)
	p.keepTail(screen)
	p.background = sc.Background || p.pending
	w.noteLimit(sess, p, sc.Limit)
	return true
}

// agent is the pane's state when its live agent is the session's own (a new P4 rule: an adopted root
// has no stored name and accepts any). The agent is the session's own when its herdr name or its
// client conversation id matches: herdr can clear a name while the agent lives on (R-WRK-084).
// Otherwise one log line and false.
func (w *Worker) agent(sess store.Session) (session.PaneState, bool) {
	st, err := w.Runtime.State(sess.Pane)
	switch {
	case err != nil:
	case !st.Live:
		err = errors.New("no agent in pane " + sess.Pane)
	case sess.Agent != "" && st.Agent != sess.Agent &&
		(sess.ClientSession == "" || st.ClientSession != sess.ClientSession):
		err = fmt.Errorf("pane %s runs agent %s, not %s", sess.Pane, st.Agent, sess.Agent)
	}
	if err != nil {
		w.Log.Error(sess.ID, err)
		return st, false
	}
	w.Log.Clear(sess.ID)
	return st, true
}

// noteScreen logs, once per episode, a client that shows no prompt (R-WRK-014 fails open).
func (w *Worker) noteScreen(id string, sc session.Screen) {
	if sc.NoPrompt {
		w.Log.Error(id+" prompt", errors.New("no input box on screen; treated as empty"))
	} else {
		w.Log.Clear(id + " prompt")
	}
}

// fresh reports whether items has one the last hint did not cover, and whether such an item is urgent.
func (p *pane) fresh(items []store.Item) (fresh, urgent bool) {
	for _, it := range items {
		if !slices.Contains(p.hinted, it.ID) {
			fresh = true
			urgent = urgent || it.Urgent
		}
	}
	return fresh, urgent
}

// hint types one hint covering every unread item when the set has an item the last hint did not
// cover (R-WRK-011, R-WRK-012); a non-urgent set waits out the quiet window, an urgent item
// does not (R-MSG-042). A failed Run leaves the set uncovered, so the next pass retries.
func (w *Worker) hint(sess store.Session, p *pane, now time.Time) {
	items, err := w.Store.Unread(sess.ID)
	if err != nil {
		w.Log.Error(sess.ID+" unread", err)
		return
	}
	w.Log.Clear(sess.ID + " unread")
	if len(items) == 0 {
		p.hinted = nil
		return
	}
	fresh, urgent := p.fresh(items)
	if !fresh || (!urgent && p.quiet(now, w.Quiet)) || p.unavailRuns > 0 {
		return // an unavailable model awaiting its second reading: the pane is about to be closed
	}
	if err := w.Runtime.Run(sess.Pane, Hint(items)); err != nil {
		w.Log.Error(sess.ID+" hint", err)
		return
	}
	w.Log.Clear(sess.ID + " hint")
	p.hinted = make([]string, 0, len(items))
	for _, it := range items {
		p.hinted = append(p.hinted, it.ID)
	}
	w.Log.Action("hint", sess.ID, fmt.Sprintf("%d unread", len(items)))
}
