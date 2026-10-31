package worker

import (
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

const (
	resumeGrace  = time.Minute      // the resume text is typed at reset + 60 s (R-WRK-043)
	limitRecheck = 15 * time.Minute // how often a pane under a reset-less notice is read (R-WRK-045)
)

// readable reports whether the pane is read this pass: a pane under a notice without a reset time
// is read only every limitRecheck (R-WRK-045); in between it is left alone.
func (p *pane) readable(now time.Time) bool {
	return p.limit == nil || !p.limit.Reset.IsZero() || !now.Before(p.nextRead)
}

// noteLimit records the notice on screen: a new notice, or a new reset, is logged once and arms
// one resume; no notice clears the state. The notice the resume answered stays on the visible
// screen and is re-resolved against every later clock, so it is remembered by its text and read
// as no limit until a screen without it is seen (H3).
func (w *Worker) noteLimit(sess store.Session, p *pane, lim *session.Limit) {
	if lim == nil || lim.Notice == p.stale {
		if lim == nil {
			p.stale = ""
		}
		p.limit, p.resumed = nil, false
		return
	}
	if p.limit == nil || !p.limit.Reset.Equal(lim.Reset) {
		p.resumed = false
		when := "no reset time"
		if !lim.Reset.IsZero() {
			when = "resets " + lim.Reset.UTC().Format(time.RFC3339)
		}
		w.Log.Action("limited", sess.ID, when)
	}
	p.limit = lim
}

// limited reports whether the pane stays silent now (R-WRK-042): a notice without a reset while
// it is on screen; a notice with one until reset + 60 s.
func (p *pane) limited(now time.Time) bool {
	if p.limitText != "" && now.Sub(p.limitAt) < limitHold { // the transcript's word (R-WRK-097)
		return true
	}
	switch {
	case p.limit == nil:
		return false
	case p.limit.Reset.IsZero():
		return true
	}
	return now.Before(p.limit.Reset.Add(resumeGrace))
}

// resume types the client's resume text once after a known reset passed (R-WRK-043) and reports
// whether this pass was spent on it. The idle clock restarts: the limit was not the agent's idling.
func (w *Worker) resume(sess store.Session, p *pane, c Client, now time.Time) bool {
	if p.limit == nil || p.limit.Reset.IsZero() || p.resumed {
		return false
	}
	if p.quiet(now, w.Quiet) {
		return true // due, but the operator was just here (R-WRK-018): this pass is spent waiting
	}
	if err := w.Runtime.Run(sess.Pane, c.Resume); err != nil {
		w.Log.Error(sess.ID+" resume", err)
		p.resumed = true // no retry for this limit: hints must not wait on a failing resume
		return true
	}
	w.Log.Clear(sess.ID + " resume")
	p.resumed, p.idleSince, p.stale = true, now, p.limit.Notice
	w.Log.Action("resume", sess.ID, "after the reset at "+p.limit.Reset.UTC().Format(time.RFC3339))
	return true
}

// compact types /compact with the boundary note and clears the request (R-WRK-030, R-WRK-032).
// The note is one line: the command is one line.
// When the request cannot be cleared the command is not typed again: the next pass only retries the clear.
func (w *Worker) compact(sess store.Session, p *pane) {
	if !p.compacted {
		if err := w.Runtime.Run(sess.Pane, "/compact "+store.Excerpt(sess.BoundaryNote, 500)); err != nil {
			w.Log.Error(sess.ID+" compact", err)
			return
		}
		p.compacted = true
		w.Log.Action("compact", sess.ID, sess.BoundaryNote)
	}
	if err := w.Store.ClearBoundary(sess.ID); err != nil {
		w.Log.Error(sess.ID+" compact", err)
		return
	}
	p.compacted = false
	w.Log.Clear(sess.ID + " compact")
}

// resumeOrRename types the resume after a reset, else an adopted root's rename; it reports
// whether this pass was spent.
func (w *Worker) resumeOrRename(sess store.Session, p *pane, c Client, now time.Time) bool {
	return w.resume(sess, p, c, now) || w.rename(sess, p, c, now)
}

// rename names an adopted root in its client once per worker run (R-WRK-071): ness launched every
// other session with its name (R-SESS-120), but an adopted conversation was already running. It
// reports whether this pass was spent on it. A failed rename is logged and not retried: the
// session works the same without its name.
func (w *Worker) rename(sess store.Session, p *pane, c Client, now time.Time) bool {
	if !sess.Adopted || c.Rename == nil || p.renamed || p.quiet(now, w.Quiet) {
		return false
	}
	p.renamed = true
	name := session.ClientName(w.Project, sess.Title)
	if err := w.Runtime.Run(sess.Pane, c.Rename(name)); err != nil {
		w.Log.Error(sess.ID+" rename", err)
		return true
	}
	w.Log.Action("rename", sess.ID, name)
	return true
}
