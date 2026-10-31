package store

import (
	"context"
	"errors"
	"time"
)

// Spawn is one decision of the pre-tool hook on a subagent call (R-HOOK-033). Requested and
// Recommended are rung names; Verdict is allowed, below, escalated, refused, fork or would-refuse.
type Spawn struct {
	Session, AgentType, Kind, Requested, Recommended, Verdict, Reason string
	At                                                                time.Time
}

// AddSpawn records one spawn decision.
func (s *Store) AddSpawn(sp Spawn) error {
	_, err := s.db.ExecContext(context.Background(), `INSERT INTO spawns
		(session, agent_type, kind, requested, recommended, verdict, reason, at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sp.Session, sp.AgentType, sp.Kind, sp.Requested, sp.Recommended, sp.Verdict, sp.Reason, stamp(sp.At))
	return err
}

// Spawns returns the spawn decisions of session, oldest first; session "" returns them all.
func (s *Store) Spawns(session string) (out []Spawn, err error) {
	rows, err := s.db.QueryContext(context.Background(), `SELECT session, agent_type, kind, requested,
		recommended, verdict, reason, at FROM spawns WHERE ? = '' OR session = ? ORDER BY at, rowid`, session, session)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		var sp Spawn
		var at string
		if err := rows.Scan(&sp.Session, &sp.AgentType, &sp.Kind, &sp.Requested, &sp.Recommended,
			&sp.Verdict, &sp.Reason, &at); err != nil {
			return nil, err
		}
		sp.At = parseStamp(at)
		out = append(out, sp)
	}
	return out, rows.Err()
}
