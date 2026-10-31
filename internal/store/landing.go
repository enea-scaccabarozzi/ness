package store

import (
	"context"
	"database/sql"
	"time"
)

// LastLanding returns the latest integration time of any descendant of root, the zero time when
// none has landed (R-MSG-071).
func (s *Store) LastLanding(root string) (time.Time, error) {
	return s.latest(`WITH RECURSIVE tree(id) AS (
			SELECT id FROM sessions WHERE parent = ?
			UNION SELECT s.id FROM sessions s JOIN tree t ON s.parent = t.id)
		SELECT MAX(integrated_at) FROM sessions WHERE id IN tree`, root)
}

// LastCheckpoint returns when root last sent the operator a checkpoint, the zero time when it
// never did (R-MSG-072).
func (s *Store) LastCheckpoint(root string) (time.Time, error) {
	return s.latest(`SELECT MAX(created_at) FROM items
		WHERE sender = ? AND recipient IS NULL AND intent = ?`, root, IntentCheckpoint)
}

// latest runs a query that yields one nullable timestamp.
func (s *Store) latest(query string, args ...any) (time.Time, error) {
	var at sql.NullString
	if err := s.db.QueryRowContext(context.Background(), query, args...).Scan(&at); err != nil {
		return time.Time{}, err
	}
	return parseStamp(at.String), nil
}
