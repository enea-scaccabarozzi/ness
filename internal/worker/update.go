package worker

import (
	"cmp"
	"fmt"
	"strings"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// Stale is a session's worktree lagging its base branch: the worktree's ness version From, the
// base's To, the base's name, the Running-sessions notes between them ("ness V: <notes>", oldest
// first) and the skills whose files changed (R-UPD-070).
type Stale struct {
	From, To, Base string
	Notes          []string
	Skills         []string
}

// Body is the update request: the versions, the action and the notes. It is built only from the
// pair, so the idempotent item key sends it once per version pair (R-MSG-014).
func (s Stale) Body() string {
	skills := cmp.Or(strings.Join(s.Skills, ", "), "none: skip")
	notes := cmp.Or(strings.Join(s.Notes, "\n"), "none")
	return fmt.Sprintf("ness update: this worktree's ness files are %s; %s has %s.\n"+
		"At your next stopping point:\n1. git merge %s\n2. ness sync --check (expect exit 0)\n"+
		"3. re-load these skills: %s\nRunning sessions notes:\n%s", s.From, s.Base, s.To, s.Base, skills, notes)
}

// deliverUpdate puts the update request in an idle stale session's own inbox and an fyi copy in
// its root's; a root gets none for itself (R-UPD-070, R-UPD-071). A failed check or send is logged
// once per text.
func (w *Worker) deliverUpdate(sess store.Session, now time.Time) {
	if w.Stale == nil {
		return
	}
	key := sess.ID + " update"
	st, stale, err := w.Stale(sess)
	if err != nil {
		w.Log.Error(key, err)
		return
	}
	if !stale {
		w.Log.Clear(key)
		return
	}
	id, created, err := w.Store.Send(sess.ID, sess.ID, store.IntentRequest, st.Body(), false, now)
	if err != nil {
		w.Log.Error(key, err)
		return
	}
	if created {
		w.Log.Action("update", sess.ID, id+" "+st.From+" -> "+st.To)
	}
	if root := w.root(sess); root.ID != sess.ID {
		body := "ness update copy for " + sess.ID + ":\n" + st.Body()
		if _, _, err = w.Store.Send(sess.ID, root.ID, store.IntentFYI, body, false, now); err != nil {
			w.Log.Error(key, err)
			return
		}
	}
	w.Log.Clear(key)
}
