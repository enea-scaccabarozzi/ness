package worker

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/enea-scaccabarozzi/ness/internal/probe"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

const (
	// limitHold is how long a usage-limit error in the transcript keeps the pane silent. The notice
	// names no reset, so after this the pane gets one hint again: credits that came back are used,
	// and one that did not costs an instant API error (R-WRK-097).
	limitHold = 30 * time.Minute
	// tailLines and tailRunes bound the diagnostic copy of the screen that goes to the log.
	tailLines = 6
	tailRunes = 160
)

// noteActivity reads the session's conversation through its client (R-WRK-096) and folds it into
// the pane: pending background work, the usage-limit episode, and the end of a stall episode by a
// real assistant turn. A client without a reading, or a read that fails, leaves all of it to the
// screen: tracked is false and a working pane then ends its episode as it always did.
func (w *Worker) noteActivity(sess store.Session, c Client, p *pane, live string, now time.Time) {
	p.tracked, p.pending, p.limitText = false, false, ""
	if c.Activity == nil {
		return
	}
	if live != "" { // the agent's own conversation: after /clear or a forking resume the stored id is old
		sess.ClientSession = live
	}
	act, ok := c.Activity(sess, now)
	if !ok {
		return
	}
	p.tracked, p.pending, p.limitText, p.limitAt = true, act.Pending, act.Limit, act.LimitAt
	if act.Remote != "" { // a conversation without one (after /clear) keeps the session's last
		p.remote = act.Remote
	}
	if act.Turn.After(p.turn) { // the agent worked: a stall or Your move already told is over
		if !p.turn.IsZero() { // a first read after a restart may be the turn that parked
			w.endPark(sess, act.Turn)
		}
		p.turn = act.Turn
		p.newEpisode()
	}
	if p.limitText == "" {
		delete(p.alerted, "limit")
		p.limitLogged, p.limitTailed = false, false
		return
	}
	if !p.limitLogged {
		p.limitLogged = true
		w.Log.Action("limited", sess.ID, "usage limit in the transcript: "+p.limitText)
	}
	if now.Sub(p.limitAt) < limitHold { // an older error, read again after a restart, was told once
		w.once(sess, p, "limit", AlertTrouble, "is out of usage: "+p.limitText)
	}
}

// endPark ends an open-ended park at the session's first real turn after the compaction consumed
// its boundary: the session moved on (R-WRK-099). A park with an until-time ends only at that time.
func (w *Worker) endPark(sess store.Session, turn time.Time) {
	if sess.ParkedAt.IsZero() || !sess.ParkedUntil.IsZero() || sess.BoundaryNote != "" ||
		!turn.After(sess.ParkedAt) {
		return
	}
	if err := w.Store.Unpark(sess.ID); err != nil {
		w.Log.Error(sess.ID+" park", err)
	}
}

// newEpisode ends the stall and Your move episodes: the operator and the manager may be told again.
func (p *pane) newEpisode() {
	p.stalled = false
	delete(p.alerted, "stall")
	delete(p.alerted, "yourmove")
}

// keepTail remembers the last tailLines non-blank lines of the screen, the evidence a stall report
// or a suspected limit logs (R-WRK-050, R-WRK-097): the next field analysis starts from what the
// pane showed, not from a guess.
func (p *pane) keepTail(ansi string) {
	lines := strings.Split(strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(session.StripANSI(ansi)), "\n")
	p.tail = p.tail[:0]
	for i := len(lines) - 1; i >= 0 && len(p.tail) < tailLines; i-- {
		if t := strings.TrimSpace(lines[i]); t != "" {
			p.tail = append([]string{t}, p.tail...)
		}
	}
}

// logTail writes the remembered screen lines for why, secrets configured for the probes redacted and
// each line cut, so a log line never carries a page of the pane.
func (w *Worker) logTail(sess store.Session, p *pane, why string) {
	var secrets []string
	if w.Probes != nil {
		for _, name := range w.Probes.Redact {
			if v := w.Probes.Getenv(name); v != "" {
				secrets = append(secrets, v)
			}
		}
	}
	out := make([]string, 0, len(p.tail))
	for _, l := range p.tail {
		if utf8.RuneCountInString(l) > tailRunes {
			l = string([]rune(l)[:tailRunes]) + "…"
		}
		out = append(out, probe.Redact(l, secrets))
	}
	w.Log.Action("screen", sess.ID, why+", last lines: "+strings.Join(out, " ¦ "))
}

// tailLimit logs the screen once per limit episode, when the transcript is what says the pane is
// limited (R-WRK-097).
func (w *Worker) tailLimit(sess store.Session, p *pane) {
	if p.limitText != "" && !p.limitTailed {
		p.limitTailed = true
		w.logTail(sess, p, "usage limit suspected")
	}
}

// report sends the reports and alerts of an idle pane. act reaches it only when the pane is not
// limited, so a transcript limit older than limitHold no longer mutes them (R-WRK-097).
func (w *Worker) report(sess store.Session, p *pane, now time.Time) {
	w.reportError(sess, p, now)
	w.reportStall(sess, p, now)
	w.alertReports(sess, p, now)
}
