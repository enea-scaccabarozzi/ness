package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"errors"
	"regexp"
	"strings"
	"time"
)

// Probe states (R-PRB-071): paused is a probe without a live owner; stopped is final.
const (
	ProbeActive  = "active"
	ProbePaused  = "paused"
	ProbeStopped = "stopped"
)

// ErrNoProbe is returned for a probe the store does not know.
var ErrNoProbe = errors.New("no such probe")

// Probe is one row: identity among live rows is Path, Owner and Label (R-PRB-084). Label is
// free text the owner gives one start of a file, "" for none (R-PRB-085). NeedsEdit holds the
// file hash at a --probe-bug verdict, "" when none (R-PRB-065); FileError is the last reload
// failure; NotifiedState is the last confirmed state posted to the channel (R-PRB-037).
type Probe struct {
	ID, Path, Owner, Label, State   string
	LastState, Evidence, Hash       string
	Verdict, VerdictNote, NeedsEdit string
	Note, FileError, NotifiedState  string
	ConfirmedState                  string
	Streak, Runs                    int
	FinalWake                       bool
	StartedAt, NextRunAt, LastRunAt time.Time
	VerdictAt, StoppedAt            time.Time
}

// ProbeRun is what one scheduled run writes back.
type ProbeRun struct {
	State, Evidence, Hash, FileError string
	Confirmed                        string // the confirmed state after this run (R-PRB-032)
	Streak, Runs                     int
	At, Next                         time.Time
	// StartedAt and Due, when set, are the row the run was made for: RecordRun writes only an active
	// row still having both, so a stop, pause or restart while a floor ran drops its result.
	StartedAt, Due time.Time
}

// NewProbeID returns "p-" plus 8 base32 characters.
func NewProbeID() string {
	var b [5]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand cannot fail on supported platforms
	}
	return "p-" + strings.ToLower(base32.StdEncoding.EncodeToString(b[:]))
}

const selectProbe = `SELECT id, path, owner, label, state, started_at, next_run_at, last_run_at, last_state,
	streak, runs, evidence, hash, file_error, verdict, verdict_note, verdict_at, needs_edit, final_wake, note,
	stopped_at,
	notified_state, confirmed_state FROM probes`

// labelRe is what a label may be: short, one word, safe in a thread key and an env var.
var labelRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,31}$`)

// ValidLabel reports whether label is "" or matches [A-Za-z0-9][A-Za-z0-9._-]{0,31} (R-PRB-086).
func ValidLabel(label string) bool { return label == "" || labelRe.MatchString(label) }

// StartProbe registers path with label for owner with its first run at next: the owner's own live
// probe with that label restarts (its window too, unless keepWindow, R-PRB-062); a paused probe at the path is adopted
// (R-PRB-071), both only at the same label; otherwise a new probe (R-PRB-060, R-PRB-084, R-PRB-085).
// Stopped rows are history.
func (s *Store) StartProbe(path, owner, label string, keepWindow bool, next, now time.Time) (Probe, error) {
	var id string
	err := s.inTx(func(tx *sql.Tx) error {
		qerr := tx.QueryRowContext(context.Background(),
			`SELECT id FROM probes WHERE path = ? AND label = ?
			 AND ((owner = ? AND state = 'active') OR state = 'paused')
			 ORDER BY CASE WHEN owner = ? THEN 0 ELSE 1 END, started_at, id LIMIT 1`,
			path, label, owner, owner).Scan(&id)
		if errors.Is(qerr, sql.ErrNoRows) {
			id = NewProbeID()
			_, qerr = tx.ExecContext(context.Background(),
				`INSERT INTO probes (id, path, owner, label, state, started_at, next_run_at)
				 VALUES (?, ?, ?, ?, 'active', ?, ?)`,
				id, path, owner, label, stamp(now), stamp(next))
			return qerr
		}
		if qerr != nil {
			return qerr
		}
		_, qerr = tx.ExecContext(context.Background(),
			`UPDATE probes SET owner = ?, state = 'active', next_run_at = ?,
			 started_at = CASE WHEN ? THEN started_at ELSE ? END, final_wake = 0, note = '', file_error = ''
			 WHERE id = ?`, owner, stamp(next), keepWindow, stamp(now), id)
		if qerr != nil {
			return qerr
		}
		_, qerr = tx.ExecContext(context.Background(),
			`UPDATE probes SET state = 'stopped', stopped_at = ?, note = 'merged into ' || ?
			 WHERE path = ? AND label = ? AND state = 'paused' AND id <> ?`, stamp(now), id, path, label, id)
		return qerr
	})
	if err != nil {
		return Probe{}, err
	}
	return s.Probe(id)
}

// Probes lists active and paused probes by path, label then owner; all adds stopped ones.
func (s *Store) Probes(all bool) ([]Probe, error) {
	q := selectProbe + ` WHERE state <> 'stopped' ORDER BY path, label, owner`
	if all {
		q = selectProbe + ` ORDER BY path, label, owner, started_at`
	}
	return s.queryProbes(q)
}

// Probe returns one probe or ErrNoProbe.
func (s *Store) Probe(id string) (Probe, error) {
	return s.oneProbe(selectProbe+` WHERE id = ?`, id)
}

// ProbeOf returns the owner's active or paused probe at path with label (R-PRB-084), or ErrNoProbe.
func (s *Store) ProbeOf(path, owner, label string) (Probe, error) {
	return s.oneProbe(selectProbe+` WHERE path = ? AND owner = ? AND label = ? AND state <> 'stopped'`,
		path, owner, label)
}

// LiveLabels returns the labels of the owner's active or paused probes at path, sorted: what a
// stop without a label has to choose from (R-PRB-088).
func (s *Store) LiveLabels(path, owner string) ([]string, error) {
	rows, err := s.queryProbes(selectProbe+` WHERE path = ? AND owner = ? AND state <> 'stopped' ORDER BY label`,
		path, owner)
	labels := make([]string, 0, len(rows))
	for _, r := range rows {
		labels = append(labels, r.Label)
	}
	return labels, err
}

func (s *Store) oneProbe(query string, args ...any) (Probe, error) {
	out, err := s.queryProbes(query, args...)
	if err != nil {
		return Probe{}, err
	}
	if len(out) == 0 {
		return Probe{}, ErrNoProbe
	}
	return out[0], nil
}

func (s *Store) queryProbes(query string, args ...any) (out []Probe, err error) {
	rows, err := s.db.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		var (
			p                                         Probe
			started, next, last, verdictAt, stoppedAt sql.NullString
		)
		if serr := rows.Scan(&p.ID, &p.Path, &p.Owner, &p.Label, &p.State, &started, &next, &last,
			&p.LastState, &p.Streak, &p.Runs, &p.Evidence, &p.Hash, &p.FileError, &p.Verdict,
			&p.VerdictNote, &verdictAt, &p.NeedsEdit,
			&p.FinalWake, &p.Note, &stoppedAt, &p.NotifiedState,
			&p.ConfirmedState); serr != nil {
			return nil, serr
		}
		p.StartedAt, p.NextRunAt, p.LastRunAt = nullStamp(started), nullStamp(next), nullStamp(last)
		p.VerdictAt, p.StoppedAt = nullStamp(verdictAt), nullStamp(stoppedAt)
		out = append(out, p)
	}
	return out, rows.Err()
}

// RecordRun writes a run's outcome and the next run; a changed file clears needs-edit
// (R-PRB-065); a file error keeps the last state, evidence and run time. A run made for a row
// (StartedAt, Due) that is no longer that row is not written: ErrNoProbe.
func (s *Store) RecordRun(id string, r ProbeRun) error {
	if !r.StartedAt.IsZero() {
		return s.touch(ErrNoProbe, runUpdate+` AND state = 'active' AND started_at = ? AND next_run_at = ?`,
			append(runArgs(id, r), stamp(r.StartedAt), stamp(r.Due))...)
	}
	if r.FileError != "" {
		return s.touch(ErrNoProbe, `UPDATE probes SET file_error = ?, next_run_at = ? WHERE id = ?`,
			r.FileError, stamp(r.Next), id)
	}
	return s.touch(ErrNoProbe, runUpdate, runArgs(id, r)...)
}

const runUpdate = `UPDATE probes SET last_state = ?, evidence = ?, streak = ?, runs = ?, last_run_at = ?,
	 next_run_at = ?, needs_edit = CASE WHEN needs_edit <> ? THEN '' ELSE needs_edit END, hash = ?, file_error = '',
	 confirmed_state = ? WHERE id = ?`

func runArgs(id string, r ProbeRun) []any {
	return []any{r.State, r.Evidence, r.Streak, r.Runs, stamp(r.At), stamp(r.Next), r.Hash, r.Hash, r.Confirmed, id}
}

// ErrProbeActive is returned by SetProbeState for the active state: only StartProbe and
// TransferProbes activate, and they respect the one-live-row-per-owner-and-path index.
var ErrProbeActive = errors.New("a probe is activated by starting or transferring it")

// SetProbeState pauses or stops a probe with a note; stopped sets stopped_at.
func (s *Store) SetProbeState(id, state, note string, now time.Time) error {
	if state == ProbeActive {
		return ErrProbeActive
	}
	var stopped any
	if state == ProbeStopped {
		stopped = stamp(now)
	}
	return s.touch(ErrNoProbe,
		`UPDATE probes SET state = ?, note = ?, stopped_at = ? WHERE id = ?`, state, note, stopped, id)
}

// MarkFinalWake records that the window's final wake went out (R-PRB-042).
func (s *Store) MarkFinalWake(id string) error {
	return s.touch(ErrNoProbe, `UPDATE probes SET final_wake = 1 WHERE id = ?`, id)
}

// SetNotifiedState records the confirmed state last posted for the probe (R-PRB-037).
func (s *Store) SetNotifiedState(id, state string) error {
	return s.touch(ErrNoProbe, `UPDATE probes SET notified_state = ? WHERE id = ?`, state, id)
}

// TransferProbes moves from's live probes to the session to (R-PRB-070); to the operator they pause with their
// last owner kept (R-PRB-071). A move to a session activates the probe, so a pause that raced the
// close does not stick; one colliding with to's own live probe at the path stops instead.
func (s *Store) TransferProbes(from, to string, now time.Time) (n int, err error) {
	err = s.inTx(func(tx *sql.Tx) error {
		var xerr error
		if to == Operator {
			n, xerr = pauseProbes(tx, from)
		} else {
			n, xerr = moveProbes(tx, from, to, now)
		}
		return xerr
	})
	return n, err
}

func pauseProbes(tx *sql.Tx, from string) (int, error) {
	res, err := tx.ExecContext(context.Background(),
		`UPDATE probes SET state = 'paused', note = 'owner closed' WHERE owner = ? AND state <> 'stopped'`, from)
	return affected(res, err)
}

func moveProbes(tx *sql.Tx, from, to string, now time.Time) (int, error) {
	live := `SELECT id FROM probes t WHERE t.path = probes.path AND t.label = probes.label
		AND t.owner = ? AND t.state <> 'stopped'`
	res, err := tx.ExecContext(context.Background(),
		`UPDATE probes SET state = 'stopped', stopped_at = ?, note = 'merged into ' || (`+live+`)
		 WHERE owner = ? AND state <> 'stopped' AND EXISTS (`+live+`)`, stamp(now), to, from, to)
	merged, err := affected(res, err)
	if err != nil {
		return 0, err
	}
	res, err = tx.ExecContext(context.Background(),
		`UPDATE probes SET owner = ?, state = 'active', note = 'from ' || owner
		 WHERE owner = ? AND state <> 'stopped'`, to, from)
	moved, err := affected(res, err)
	return merged + moved, err
}

func affected(res sql.Result, err error) (int, error) {
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// ErrProbeNotActive is returned by SendWake for a probe that is paused, stopped or unknown.
var ErrProbeNotActive = errors.New("probe is not active")

// SendWake puts a request from the probe's current owner to itself in its inbox, tagged with
// the probe (R-PRB-038). The recipient is read from the row in the same statement, so a probe
// that moved or paused while its floor ran wakes nobody stale. The body carries the run number,
// so no two wakes share a key.
func (s *Store) SendWake(probeID, body string, now time.Time) (string, error) {
	id := NewItemID()
	key := itemKey(probeID, IntentRequest, body, stamp(now))
	res, err := s.db.ExecContext(context.Background(),
		`INSERT INTO items (id, key, sender, recipient, intent, body, urgent, created_at, probe)
		 SELECT ?, ?, owner, owner AS recipient, 'request', ?, 0, ?, id FROM probes WHERE id = ? AND state = 'active'`,
		id, key, body, stamp(now), probeID)
	n, err := affected(res, err)
	if err == nil && n == 0 {
		err = ErrProbeNotActive
	}
	return id, err
}

// Dispose answers an item with no reply item: a verdict answers a probe, not a session
// (R-PRB-064). A final disposition is ErrDisposed.
func (s *Store) Dispose(id, disposition, answer string, now time.Time) error {
	return s.inTx(func(tx *sql.Tx) error {
		it, err := scanItem(tx.QueryRowContext(context.Background(), selectItem+` WHERE id = ?`, id))
		if err != nil {
			return err
		}
		return disposeTx(tx, it, disposition, answer, now)
	})
}

// RecordVerdict stores the owner's verdict; needsEdit is the file hash for a --probe-bug verdict,
// "" otherwise (R-PRB-064, R-PRB-065).
func (s *Store) RecordVerdict(id, state, note, needsEdit string, now time.Time) error {
	return s.touch(ErrNoProbe,
		`UPDATE probes SET verdict = ?, verdict_note = ?, verdict_at = ?, needs_edit = ? WHERE id = ?`,
		state, note, stamp(now), needsEdit, id)
}
