package store

import (
	"strings"
	"time"
)

// Wait is one thing a session waits on (R-WRK-099). Kind is "ask" (an item to the operator),
// "item" (an item to another session), "probe" (an active probe it owns) or "parked" (its boundary
// record). Ref names the item or probe, "" for a park; Note says to whom, which path, or why it
// parked. Until is when the wait ends by itself, zero when open-ended.
type Wait struct {
	Kind, Ref, Note string
	Until           time.Time
}

// Waits lists what id waits on at now, derived from the store on every call: its unanswered asks,
// then its unanswered items to other sessions, then its active probes, then its park while it holds
// (R-WRK-099). An accepted item still waits: the answer is owed.
func (s *Store) Waits(id string, now time.Time) ([]Wait, error) {
	out, err := s.itemWaits(id)
	if err != nil {
		return nil, err
	}
	probes, err := s.queryProbes(selectProbe+` WHERE owner = ? AND state = 'active' ORDER BY started_at, id`, id)
	if err != nil {
		return nil, err
	}
	for _, p := range probes {
		out = append(out, Wait{Kind: "probe", Ref: p.ID, Note: strings.TrimSpace(p.Path + " " + p.Label)})
	}
	park, ok, err := s.parkWait(id, now)
	if ok {
		out = append(out, park)
	}
	return out, err
}

// itemWaits lists id's unanswered items, the asks to the operator first, each oldest first.
func (s *Store) itemWaits(id string) ([]Wait, error) {
	items, err := queryItems(s.db, selectItem+` WHERE sender = ? AND recipient IS NOT sender
		AND (disposition IS NULL OR disposition IN ('deferred', 'accepted'))
		AND (intent IN ('request', 'decision') OR (intent = 'checkpoint' AND ask = 1))
		AND retired_at IS NULL ORDER BY recipient IS NOT NULL, created_at, rowid`, id)
	out := make([]Wait, 0, len(items))
	for _, it := range items {
		w := Wait{Kind: "item", Ref: it.ID, Note: "to " + it.Recipient}
		if it.Recipient == Operator {
			w.Kind, w.Note = "ask", "to the operator"
		}
		out = append(out, w)
	}
	return out, err
}

// parkWait is id's park while it holds: until its until-time, or, open-ended, until something
// reaches the session after it parked.
func (s *Store) parkWait(id string, now time.Time) (Wait, bool, error) {
	sess, err := s.Session(id)
	if err != nil || sess.ParkedAt.IsZero() {
		return Wait{}, false, err
	}
	held := now.Before(sess.ParkedUntil)
	if sess.ParkedUntil.IsZero() {
		last, lerr := s.LastInbound(id)
		if lerr != nil {
			return Wait{}, false, lerr
		}
		held = !last.After(sess.ParkedAt)
	}
	return Wait{Kind: "parked", Note: sess.ParkedNote, Until: sess.ParkedUntil}, held, nil
}
