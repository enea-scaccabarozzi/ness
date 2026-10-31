package worker

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// confirm is how many consecutive passes must read the same error, or the unavailable model,
// before the worker acts. It is a debounce against a screen caught mid-redraw, not the
// discriminator: on an idle pane the screen does not change, so two passes cannot tell a tool
// result that quotes a notice from the notice itself. The client atom's exact phrases anchored
// to column 0 (never a tool-result line) do that (H1, H2).
const confirm = 2

// modelUnavailable is the name under which an unavailable model is reported as an error.
const modelUnavailable = "model unavailable"

// seen counts consecutive readings of the same error and of the unavailable model.
func (p *pane) seen(sc session.Screen) {
	name := sc.Error
	if sc.Unavailable {
		name = modelUnavailable
	}
	switch name {
	case "":
		p.errName, p.errRuns = "", 0
	case p.errName:
		p.errRuns++
	default:
		p.errName, p.errRuns = name, 1
	}
	if sc.Unavailable {
		p.unavailRuns++
	} else {
		p.unavailRuns = 0
	}
}

// mustStop reports whether the model has been unavailable on enough passes and the operator is
// not in the pane: closing a tab loses the turn in flight and any draft, so one ambiguous
// screen, a focused pane or a draft never closes it (H2). Else the error report tells the manager.
func (p *pane) mustStop(focused bool) bool {
	return p.unavailRuns >= confirm && !focused && !p.draft && !p.stopped
}

// tell sends an fyi from the session to its manager (the operator for a root): the route is the
// session's own, the manager can read and answer it, and the body carries the episode so the
// idempotency key makes one item per episode (R-MSG-014). The notices are worded so that, drawn
// on the manager's screen, they match none of the client's patterns (H1).
func (w *Worker) tell(sess store.Session, body string, urgent bool, now time.Time) bool {
	if _, _, err := w.Store.Send(sess.ID, sess.Parent, store.IntentFYI, body, urgent, now); err != nil {
		w.Log.Error(sess.ID+" tell", err)
		return false
	}
	w.Log.Clear(sess.ID + " tell")
	return true
}

// reportStall sends one message per idle episode to the manager once the session has been idle for
// Stall with open obligations, or, for a child that has never handed off, with its brief owed
// (R-WRK-050); a new episode starts after the session worked again (R-WRK-052). With obligations
// the clock starts at the later of going idle and the oldest obligation's arrival: a session parked
// with nothing owed is not stalled the moment work reaches it. A pane whose footer shows a live
// background shell is working, not stalled (R-WRK-050). The phone hears of it only at the second
// miss (R-WRK-094).
// The episode lives in memory: a worker restart mid-episode sends one more (accepted for v1).
func (w *Worker) reportStall(sess store.Session, p *pane, now time.Time) {
	if p.stalled || p.background || now.Sub(w.clock(sess, p)) < w.Stall {
		return
	}
	what, ok := w.stallWhat(sess, now)
	if !ok {
		return
	}
	since := p.idleSince.UTC().Format(time.RFC3339)
	if w.tell(sess, fmt.Sprintf("ness worker: session %s (%s) has been idle since %s %s",
		sess.ID, sess.Scope, since, what), false, now) {
		p.stalled, p.stallText, p.stallTold = true, "has been idle since "+since+" "+what, now
		w.Log.Action("stall", sess.ID, fmt.Sprintf("idle since %s, %s", since, what))
		w.logTail(sess, p, "stall")
	}
}

// secondMiss reports whether the stall episode told to the manager is still open Stall after the
// report and nothing reached the session since: the manager did not act, so the phone is told
// (R-WRK-094).
func (w *Worker) secondMiss(sess store.Session, p *pane, now time.Time) bool {
	if !p.stalled || now.Sub(p.stallTold) < w.Stall {
		return false
	}
	at, err := w.Store.LastInbound(sess.ID)
	if err != nil {
		w.Log.Error(sess.ID+" stall", err)
		return false
	}
	return !at.After(p.stallTold)
}

// clock is when the idle spell counts from: going idle, or the newest item that reached the session,
// whichever is later. An answer disposed seconds ago has not been hinted yet, so the session has not
// had a chance to act on it (R-WRK-050).
func (w *Worker) clock(sess store.Session, p *pane) time.Time {
	at, err := w.Store.LastInbound(sess.ID)
	if err != nil {
		w.Log.Error(sess.ID+" stall", err)
		return p.idleSince
	}
	return latest(p.idleSince, at)
}

// latest is the later of a and b.
func latest(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// stallWhat is what a stall report says sess is stuck on, false when there is none to report yet:
// nothing while the session waits (R-WRK-099); else the obligations that are old enough, or the
// owed brief unless a descendant is busy. Only the session's own background shell excuses an
// obligation: a busy descendant may itself be waiting on this session (R-WRK-050).
func (w *Worker) stallWhat(sess store.Session, now time.Time) (what string, ok bool) {
	if w.waits(sess, now) {
		return "", false
	}
	obl, err := w.Store.Obligations(sess.ID)
	if err != nil {
		w.Log.Error(sess.ID+" stall", err)
		return "", false
	}
	if len(obl) == 0 {
		return "and has not handed off its brief", w.owesBrief(sess) && !w.treeBusy(sess.ID)
	}
	if now.Sub(obl[0].CreatedAt) < w.Stall { // oldest first
		return "", false
	}
	ids := make([]string, 0, len(obl))
	for _, it := range obl {
		ids = append(ids, it.ID)
	}
	return fmt.Sprintf("with %d open obligation(s): %s", len(obl), strings.Join(ids, ", ")), true
}

// owesBrief reports whether sess is a child that never handed off; a root owes nothing the worker
// can see (R-WRK-093 covers it).
func (w *Worker) owesBrief(sess store.Session) bool {
	if sess.Parent == store.Operator {
		return false
	}
	done, err := w.Store.HandedOff(sess.ID)
	if err != nil {
		w.Log.Error(sess.ID+" stall", err)
		return false
	}
	return !done
}

// waits reports whether sess waits on anything (R-WRK-099); a failed read says yes, so a store
// error never raises a report.
func (w *Worker) waits(sess store.Session, now time.Time) bool {
	ws, err := w.Store.Waits(sess.ID, now)
	if err != nil {
		w.Log.Error(sess.ID+" stall", err)
		return true
	}
	return len(ws) > 0
}

// alertStop sends the Trouble alert of a stop; visit retries it while the post fails, since the
// closed tab ends the pane's passes (R-WRK-083).
func (w *Worker) alertStop(sess store.Session, p *pane) {
	w.once(sess, p, "stop", AlertTrouble, "stopped: its tier's model id no longer resolves")
}

// reportError sends one message per episode once the same error pattern was read on confirm
// consecutive passes (R-WRK-051).
func (w *Worker) reportError(sess store.Session, p *pane, now time.Time) {
	if p.errRuns < confirm || p.errored || p.stopped {
		return
	}
	if w.tell(sess, fmt.Sprintf("ness worker: session %s (%s) shows %q on its screen since %s; "+
		"it may need a nudge or a restart", sess.ID, sess.Scope, p.errName,
		now.UTC().Format(time.RFC3339)), false, now) {
		p.errored = true
		w.Log.Action("stall", sess.ID, "screen shows "+p.errName)
	}
}

// stop tells the manager first, then closes the tab of a session whose tier's model is unavailable
// (R-SESS-009, R-WRK-062): a failed send leaves the tab open and the next pass retries; the body has
// no clock, so the idempotency key makes one item per run. An adopted root's tab is the operator's
// own and a session without a tab has none: the worker only tells (close.go's CloseTab guard). The
// row stays open. The worker never changes a model itself (R-MOD-012).
func (w *Worker) stop(sess store.Session, p *pane, now time.Time) {
	own := sess.Tab != "" && !sess.Adopted
	fix := fmt.Sprintf("Fix the id under tiers.%s in .ness/ness.toml", sess.Tier)
	body := fmt.Sprintf("ness worker stopped session %s (%s), run %d: its tier's model id no longer resolves. "+
		"%s, then run ness open --resume %s", sess.ID, sess.Scope, sess.Run, fix, sess.ID)
	if !own {
		body = fmt.Sprintf("ness worker: session %s (%s), run %d: its tier's model id no longer resolves; "+
			"this tab is not the worker's to close. %s, then restart the client in that pane",
			sess.ID, sess.Scope, sess.Run, fix)
	}
	if !w.tell(sess, body, true, now) {
		return
	}
	if own {
		if err := w.Runtime.CloseTab(sess.Tab); err != nil && !errors.Is(err, session.ErrGone) {
			w.Log.Error(sess.ID+" stop", err)
			return
		}
	}
	w.Log.Clear(sess.ID + " stop")
	p.stopped = true
	w.alertStop(sess, p)
	if own {
		w.Log.Action("stop", sess.ID, "model unavailable")
	} else {
		w.Log.Action("told", sess.ID, "model unavailable; the tab is not the worker's to close")
	}
}
