package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Phases the store names itself; the full ordered list lives in internal/session (R-SESS-044).
const (
	PhaseRegistered = "registered"
	PhaseReady      = "ready"
)

// ErrNoHeartbeat is returned by Heartbeat before the worker ever ran.
var ErrNoHeartbeat = errors.New("no worker heartbeat")

// Launch is what ness open records before its first side effect (R-SESS-010, R-SESS-031).
// Worktree and Branch are empty for an adopted root; Record is repo-relative.
type Launch struct {
	ID, Parent, Scope, Title, Tier, Effort, Client string
	Size                                           string // a root's work size; empty for a child
	Record, Base, Worktree, Branch                 string
	RouterReason                                   string
	Adopted                                        bool
}

// AddLaunch inserts the session in phase registered. An adopted root (open --root --here) is
// ready at once (R-SESS-038).
func (s *Store) AddLaunch(l Launch, now time.Time) error {
	phase, ready := PhaseRegistered, any(nil)
	if l.Adopted {
		phase, ready = PhaseReady, stamp(now)
	}
	_, err := s.db.ExecContext(context.Background(),
		`INSERT INTO sessions (id, parent, scope, created_at, title, tier, effort, client, phase, phase_at,
		 ready_at, first_ready_at, record, base, worktree, branch, router_reason, adopted, size)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		l.ID, nullable(l.Parent), l.Scope, stamp(now), l.Title, l.Tier, l.Effort, l.Client, phase, stamp(now),
		ready, ready, l.Record, l.Base, l.Worktree, l.Branch, l.RouterReason, l.Adopted, l.Size)
	return err
}

// SetPhase records the phase about to start (R-SESS-031).
func (s *Store) SetPhase(id, phase string, now time.Time) error {
	return s.touch(ErrNoSession, `UPDATE sessions SET phase = ?, phase_at = ? WHERE id = ?`, phase, stamp(now), id)
}

// BindTab binds the session to a new tab and pane for run n, dropping the previous agent
// (R-SESS-048). The client session id is kept: a relaunch resumes the stored transcript
// (R-SESS-047); only BindAgent rebinds it.
func (s *Store) BindTab(id, pane, tab string, run int) error {
	return s.touch(ErrNoSession,
		`UPDATE sessions SET pane = ?, tab = ?, run = ?, agent = '' WHERE id = ?`, pane, tab, run, id)
}

// BindAgent records the herdr agent name and the client's session id (R-SESS-027, R-SESS-035).
func (s *Store) BindAgent(id, agent, clientSession string) error {
	return s.touch(ErrNoSession,
		`UPDATE sessions SET agent = ?, client_session = ? WHERE id = ?`, agent, clientSession, id)
}

// NewRun prepares a relaunch of an exited session: the binding and the ready mark are cleared
// (the client session id stays, so the relaunch resumes it), the phase restarts at phase, and a
// non-empty tier replaces the stored one (R-SESS-047, R-SESS-051).
func (s *Store) NewRun(id, phase, tier, effort string, now time.Time) error {
	return s.touch(ErrNoSession,
		`UPDATE sessions SET pane = '', tab = '', agent = '', ready_at = NULL, phase = ?, phase_at = ?,
		 tier = CASE WHEN ? = '' THEN tier ELSE ? END, effort = CASE WHEN ? = '' THEN effort ELSE ? END
		 WHERE id = ?`, phase, stamp(now), tier, tier, effort, effort, id)
}

// ClearReady drops the ready mark, as a relaunch does; the stalled-launch view reads it.
func (s *Store) ClearReady(id string) error {
	return s.touch(ErrNoSession, `UPDATE sessions SET ready_at = NULL WHERE id = ?`, id)
}

// SetTier changes the tier and effort of a session whose launch continues (R-SESS-051).
func (s *Store) SetTier(id, tier, effort string) error {
	return s.touch(ErrNoSession, `UPDATE sessions SET tier = ?, effort = ? WHERE id = ?`, tier, effort, id)
}

// SetRouterReason records why a resume moved the session's rung (R-MOD-043).
func (s *Store) SetRouterReason(id, reason string) error {
	return s.touch(ErrNoSession, `UPDATE sessions SET router_reason = ? WHERE id = ?`, reason, id)
}

// SetSize changes a root's work size (ness size).
func (s *Store) SetSize(id, size string) error {
	return s.touch(ErrNoSession, `UPDATE sessions SET size = ? WHERE id = ?`, size, id)
}

// HasUnsizedRoot reports a root that predates sizes and has not been given one yet.
func (s *Store) HasUnsizedRoot() (bool, error) {
	var n int
	err := s.db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM sessions WHERE parent IS NULL AND size = ''`).Scan(&n)
	return n > 0, err
}

// FillSizes gives every unsized root the project's default size (Resolved.DefaultSize).
func (s *Store) FillSizes(size string) error {
	_, err := s.db.ExecContext(context.Background(),
		`UPDATE sessions SET size = ? WHERE parent IS NULL AND size = ''`, size)
	return err
}

// SetIntegration records the integration commit or PR url that close reads (R-SESS-099).
func (s *Store) SetIntegration(id, ref string, now time.Time) error {
	return s.touch(ErrNoSession,
		`UPDATE sessions SET integrated = ?, integrated_at = ? WHERE id = ?`, ref, stamp(now), id)
}

// RecordClose marks the session closed with its note and keep reason; the row stays for status
// --all and stats (R-SESS-065, R-SESS-067, R-SESS-068).
func (s *Store) RecordClose(id, note, keep string, now time.Time) error {
	return s.touch(ErrNoSession,
		`UPDATE sessions SET state = 'closed', closed_at = ?, close_note = ?, keep_reason = ? WHERE id = ?`,
		stamp(now), note, keep, id)
}

// Retire drops every open item addressed to recipient (deferred ones included) from inboxes and
// obligations, returning how many (R-SESS-069, R-WRK-020).
func (s *Store) Retire(recipient string, now time.Time) (int, error) {
	res, err := s.db.ExecContext(context.Background(),
		`UPDATE items SET retired_at = ? WHERE recipient IS ? AND retired_at IS NULL
		 AND (disposition IS NULL OR disposition = 'deferred')`,
		stamp(now), nullable(recipient))
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// SettleFrom disposes every open item sender sent to recipient (deferred ones included) with the
// disposition and answer, without a reply item: a manager owes nothing for a child it closed
// (R-SESS-123), and a child closed with --withdraw reads nothing more (R-SESS-130). It returns
// how many.
func (s *Store) SettleFrom(sender, recipient, disposition, answer string, now time.Time) (int, error) {
	res, err := s.db.ExecContext(context.Background(),
		`UPDATE items SET disposition = ?, answer = ?, disposed_at = ?
		 WHERE sender IS ? AND recipient IS ? AND retired_at IS NULL
		 AND (disposition IS NULL OR disposition = 'deferred')`,
		disposition, answer, stamp(now), nullable(sender), nullable(recipient))
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// Heartbeat is the worker's last pass (R-WRK-008).
type Heartbeat struct {
	At      time.Time
	Version string
}

// Heartbeat returns the worker's last heartbeat or ErrNoHeartbeat.
func (s *Store) Heartbeat() (Heartbeat, error) {
	var at, version string
	row := s.db.QueryRowContext(context.Background(), `SELECT at, version FROM heartbeat WHERE id = 1`)
	err := row.Scan(&at, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return Heartbeat{}, ErrNoHeartbeat
	}
	if err != nil {
		return Heartbeat{}, err
	}
	return Heartbeat{At: parseStamp(at), Version: version}, nil
}

// SetHeartbeat writes the single heartbeat row.
func (s *Store) SetHeartbeat(now time.Time, version string) error {
	_, err := s.db.ExecContext(context.Background(),
		`INSERT INTO heartbeat (id, at, version) VALUES (1, ?, ?)
		 ON CONFLICT (id) DO UPDATE SET at = excluded.at, version = excluded.version`, stamp(now), version)
	return err
}

// gitDiscovery are the variables that make git ignore -C and use another repository. Any ness
// process started from a git hook or an unusual shell would otherwise open the wrong store.
var gitDiscovery = []string{
	"GIT_DIR=", "GIT_WORK_TREE=", "GIT_COMMON_DIR=", "GIT_INDEX_FILE=", "GIT_OBJECT_DIRECTORY=",
	"GIT_ALTERNATE_OBJECT_DIRECTORIES=", "GIT_NAMESPACE=", "GIT_CEILING_DIRECTORIES=",
	"GIT_DISCOVERY_ACROSS_FILESYSTEM=",
}

// CleanGitEnv returns env without the git discovery variables; authorship and transport stay.
func CleanGitEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !hasAnyPrefix(kv, gitDiscovery) {
			out = append(out, kv)
		}
	}
	return out
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
