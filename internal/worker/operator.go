package worker

import (
	"fmt"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// BlockedAfter is how long a pane sits on a permission prompt before its manager is told
// (R-WRK-081): long enough for an agent's own quick answer, short enough to matter.
const BlockedAfter = 2 * time.Minute

// blockedDue is how long after the block began the operator gets Trouble alert n (from 0): at
// 10 minutes, 1 hour and 4 hours, then every 4 hours (R-WRK-081).
func blockedDue(n int) time.Duration {
	switch n {
	case 0:
		return 10 * time.Minute
	case 1:
		return time.Hour
	}
	return time.Duration(n-1) * 4 * time.Hour
}

// noteBlocked tracks the pane's blocked status: after BlockedAfter it tells the manager once per
// blocked episode, and the operator on the blockedDue backoff (R-WRK-081, R-WRK-094); any other
// status ends the episode. herdr scrapes the screen, so it reads "blocked" on a pane that is
// compacting its conversation (field-2): once a step is due the screen is read, and a pane that
// shows the compaction is not told, the episode going on.
func (w *Worker) noteBlocked(sess store.Session, c Client, p *pane, status string, now time.Time) {
	if status != "blocked" {
		p.blockedSince, p.compactRead, p.blockedTold, p.blockedAlerts = time.Time{}, time.Time{}, false, 0
		return
	}
	if p.blockedSince.IsZero() {
		p.blockedSince = now
	}
	held := now.Sub(p.blockedSince)
	tell := !p.blockedTold && held >= BlockedAfter
	alert := w.Notify != nil && held >= blockedDue(p.blockedAlerts)
	if (tell || alert) && !w.compacting(sess, c, p, now) {
		w.blockedStep(sess, p, tell, alert, now)
	}
}

// blockedStep tells the manager, the operator or both of a blocked pane; a failed send or post is
// retried next pass.
func (w *Worker) blockedStep(sess store.Session, p *pane, tell, alert bool, now time.Time) {
	if tell {
		since := p.blockedSince.UTC().Format(time.RFC3339)
		p.blockedTold = w.tell(sess, fmt.Sprintf(
			"ness worker: session %s (%s) is blocked on a prompt in its tab since %s", sess.ID, sess.Scope, since),
			false, now)
	}
	if alert && w.deliver(sess, w.alertFor(sess, AlertTrouble, "is blocked on a prompt in its tab", "")) {
		for now.Sub(p.blockedSince) >= blockedDue(p.blockedAlerts) { // steps missed in an outage are not sent
			p.blockedAlerts++
		}
	}
}

// compactRecheck is how often a blocked pane's screen is read for the compaction line.
const compactRecheck = 30 * time.Second

// compacting reports whether the session's screen shows the client compacting its conversation,
// read at most every compactRecheck while the pane stays blocked. A screen that cannot be read says
// no: a pane that may need the operator is told about.
func (w *Worker) compacting(sess store.Session, c Client, p *pane, now time.Time) bool {
	if !p.compactRead.IsZero() && now.Sub(p.compactRead) < compactRecheck {
		return p.compactSeen
	}
	p.compactRead = now
	screen, err := w.Runtime.ReadANSI(sess.Pane)
	p.compactSeen = err == nil && c.Read(screen, now).Compacting
	return p.compactSeen
}
