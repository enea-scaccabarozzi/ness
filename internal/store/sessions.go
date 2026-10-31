package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrNoSession is returned for a session id the store does not know.
var ErrNoSession = errors.New("no such session")

// Session is one row of the session tree. Parent is Operator for a root, whose manager is the
// operator (R-MSG-013). Phase is the stored launch phase; the displayed phase is derived by
// internal/session.Shown from Phase, PhaseAt and ReadyAt.
type Session struct {
	ID, Parent, Scope, Title                  string
	Tier, Effort, Client, Phase               string
	Size                                      string // a root's work size; empty for a child
	Run                                       int
	Pane, Tab, Agent, ClientSession           string
	Worktree, Branch, Record, Base            string
	RouterReason, Integrated                  string
	Adopted, Closed                           bool
	CreatedAt, PhaseAt, ReadyAt, IntegratedAt time.Time
	FirstReadyAt                              time.Time
	ClosedAt, BoundaryAt                      time.Time
	BoundaryNote, CloseNote, KeepReason       string
	// ParkedAt, ParkedUntil and ParkedNote are the last boundary's parked record: the compaction
	// does not clear it, a new boundary replaces it (R-MSG-034). ParkedUntil is zero when open-ended.
	ParkedAt, ParkedUntil time.Time
	ParkedNote            string
}

// AddSession records an open session; parent Operator makes a root.
func (s *Store) AddSession(id, scope, parent string, now time.Time) error {
	_, err := s.db.ExecContext(context.Background(),
		`INSERT INTO sessions (id, parent, scope, created_at, phase_at, title) VALUES (?, ?, ?, ?, ?, ?)`,
		id, nullable(parent), scope, stamp(now), stamp(now), scope)
	return err
}

// CloseSession marks the session closed; the row stays for history (D37).
func (s *Store) CloseSession(id string) error {
	return s.touch(ErrNoSession, `UPDATE sessions SET state = 'closed' WHERE id = ?`, id)
}

// MarkReady writes the ready mark the launch waits for; the first mark is kept (R-HOOK-003,
// R-HOOK-009).
func (s *Store) MarkReady(id string, now time.Time) error {
	return s.touch(ErrNoSession, `UPDATE sessions SET ready_at = COALESCE(ready_at, ?),
		 first_ready_at = COALESCE(first_ready_at, ?) WHERE id = ?`, stamp(now), stamp(now), id)
}

// SetBoundary records that the session wrote its state down and asks to be compacted when idle
// (R-MSG-034); the worker consumes it (R-WRK-030). It also replaces the parked record: parked at
// now until until, or, with a zero until, until something reaches the session (R-WRK-099).
func (s *Store) SetBoundary(id, note string, until, now time.Time) error {
	var u any
	if !until.IsZero() {
		u = stamp(until)
	}
	return s.touch(ErrNoSession,
		`UPDATE sessions SET boundary_note = ?, boundary_at = ?, parked_note = ?, parked_at = ?, parked_until = ?
		WHERE id = ?`, note, stamp(now), note, stamp(now), u, id)
}

// ClearBoundary drops the boundary request once the worker compacted the session (R-WRK-032); the
// parked record stays.
func (s *Store) ClearBoundary(id string) error {
	return s.touch(ErrNoSession, `UPDATE sessions SET boundary_note = NULL, boundary_at = NULL WHERE id = ?`, id)
}

// Unpark drops the session's parked record: an open-ended park ends at the session's first real
// turn after its compaction (R-WRK-099).
func (s *Store) Unpark(id string) error {
	return s.touch(ErrNoSession,
		`UPDATE sessions SET parked_at = NULL, parked_until = NULL, parked_note = NULL WHERE id = ?`, id)
}

const selectSession = `SELECT id, parent, scope, state, created_at, ready_at, boundary_note, boundary_at,
	title, tier, effort, client, phase, phase_at, run, pane, tab, agent, client_session, worktree, branch,
	record, base, adopted, router_reason, integrated, integrated_at, closed_at, close_note, keep_reason,
	first_ready_at, size, parked_at, parked_until, parked_note
	FROM sessions`

// Session returns one session or ErrNoSession.
func (s *Store) Session(id string) (Session, error) {
	row := s.db.QueryRowContext(context.Background(), selectSession+` WHERE id = ?`, id)
	out, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNoSession
	}
	return out, err
}

// AdoptedBy returns the open session adopted with --here whose client session is clientSession, or
// ErrNoSession; an empty clientSession never matches (R-SESS-114).
func (s *Store) AdoptedBy(clientSession string) (Session, error) {
	if clientSession == "" {
		return Session{}, ErrNoSession
	}
	row := s.db.QueryRowContext(context.Background(),
		selectSession+` WHERE adopted = 1 AND state = 'open' AND client_session = ?`, clientSession)
	out, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNoSession
	}
	return out, err
}

// Sessions lists open sessions in creation order, or every session with all.
func (s *Store) Sessions(all bool) ([]Session, error) {
	q := selectSession
	if !all {
		q += ` WHERE state = 'open'`
	}
	return s.querySessions(q + ` ORDER BY created_at, rowid`)
}

// Children lists the open children of parent in creation order; Operator lists the roots.
func (s *Store) Children(parent string) ([]Session, error) {
	return s.querySessions(selectSession+` WHERE parent IS ? AND state = 'open' ORDER BY created_at, rowid`,
		nullable(parent))
}

func (s *Store) querySessions(query string, args ...any) (out []Session, err error) {
	rows, err := s.db.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		sess, serr := scanSession(rows)
		if serr != nil {
			return nil, serr
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

func scanSession(r scanner) (Session, error) {
	var (
		out                                           Session
		parent, readyAt, bNote, bAt, created, phaseAt sql.NullString
		integratedAt, closedAt, firstReady            sql.NullString
		parkedAt, parkedUntil, parkedNote             sql.NullString
		state                                         string
	)
	err := r.Scan(&out.ID, &parent, &out.Scope, &state, &created, &readyAt, &bNote, &bAt,
		&out.Title, &out.Tier, &out.Effort, &out.Client, &out.Phase, &phaseAt, &out.Run, &out.Pane, &out.Tab,
		&out.Agent, &out.ClientSession, &out.Worktree, &out.Branch, &out.Record, &out.Base, &out.Adopted,
		&out.RouterReason, &out.Integrated, &integratedAt, &closedAt, &out.CloseNote, &out.KeepReason,
		&firstReady, &out.Size, &parkedAt, &parkedUntil, &parkedNote)
	if err != nil {
		return Session{}, err
	}
	out.Parent, out.Closed, out.BoundaryNote = parent.String, state == "closed", bNote.String
	out.CreatedAt, out.ReadyAt, out.BoundaryAt = nullStamp(created), nullStamp(readyAt), nullStamp(bAt)
	out.PhaseAt, out.IntegratedAt, out.ClosedAt = nullStamp(phaseAt), nullStamp(integratedAt), nullStamp(closedAt)
	out.FirstReadyAt = nullStamp(firstReady)
	out.ParkedAt, out.ParkedUntil, out.ParkedNote = nullStamp(parkedAt), nullStamp(parkedUntil), parkedNote.String
	return out, nil
}

// touch runs an update that must hit exactly one row, or returns notFound.
func (s *Store) touch(notFound error, query string, args ...any) error {
	res, err := s.db.ExecContext(context.Background(), query, args...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return notFound
	}
	return nil
}
