package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Post is the progress of one multi-part post.
type Post struct {
	Key, Channel string
	Parts, Sent  int
}

// Thread returns the platform thread id recorded for key on channel, "" when none (R-NTF-040).
func (s *Store) Thread(channel, key string) (string, error) {
	var id string
	err := s.db.QueryRowContext(context.Background(),
		`SELECT id FROM threads WHERE channel = ? AND key = ?`, channel, key).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// SetThread records the thread id of key on channel; a first post starts the thread.
func (s *Store) SetThread(channel, key, id string) error {
	_, err := s.db.ExecContext(context.Background(),
		`INSERT INTO threads (channel, key, id) VALUES (?, ?, ?)
		 ON CONFLICT (channel, key) DO UPDATE SET id = excluded.id`, channel, key, id)
	return err
}

// ErrPostBusy is returned by BeginPost for a post another run is sending.
var ErrPostBusy = errors.New("is being sent by another run")

// ClaimTTL is how long a claim holds. A claim older than this is stale: the run that made it
// crashed or hung, and a retry may take over.
// ponytail: age only, no pid probe; a run that outlives the TTL can be doubled by a retry.
const ClaimTTL = 10 * time.Minute

// BeginPost records a post of n parts under key and claims it for owner, or returns how many
// parts an earlier attempt already sent (R-NTF-007). A post that was sent in full starts over at
// 0: a retry follows a failure, so a new invocation after success is a new post. A post claimed
// by another run less than ClaimTTL ago is refused with ErrPostBusy, so two runs of the same
// post never both send. Release the claim with ReleasePost.
func (s *Store) BeginPost(key, channel string, n int, now time.Time, owner string) (sent int, err error) {
	err = s.inTx(func(tx *sql.Tx) error {
		var had int
		var by, at sql.NullString
		qerr := tx.QueryRowContext(context.Background(),
			`SELECT sent, parts, claimed_by, claimed_at FROM posts WHERE key = ?`, key).Scan(&sent, &had, &by, &at)
		if errors.Is(qerr, sql.ErrNoRows) {
			_, qerr = tx.ExecContext(context.Background(),
				`INSERT INTO posts (key, channel, parts, created_at, updated_at, claimed_by, claimed_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?)`, key, channel, n, stamp(now), stamp(now), owner, stamp(now))
			return qerr
		}
		if qerr != nil {
			return qerr
		}
		if by.Valid && by.String != owner && now.Sub(nullStamp(at)) < ClaimTTL {
			return fmt.Errorf("post %s %w", key, ErrPostBusy)
		}
		if sent >= had {
			sent = 0
		}
		_, qerr = tx.ExecContext(context.Background(),
			`UPDATE posts SET sent = ?, updated_at = ?, claimed_by = ?, claimed_at = ? WHERE key = ?`,
			sent, stamp(now), owner, stamp(now), key)
		return qerr
	})
	return sent, err
}

// ReleasePost drops owner's claim on the post, on completion and on failure alike.
func (s *Store) ReleasePost(key, owner string) error {
	_, err := s.db.ExecContext(context.Background(),
		`UPDATE posts SET claimed_by = NULL, claimed_at = NULL WHERE key = ? AND claimed_by = ?`, key, owner)
	return err
}

// ThreadRef is a thread mapping to record with a sent mark; the zero value records nothing.
type ThreadRef struct{ Channel, Key, ID string }

// MarkSent records that the first sent parts of the post went out and, in the same transaction,
// the thread the first part started, so a crash cannot leave a sent part without its thread.
func (s *Store) MarkSent(key string, sent int, now time.Time, th ThreadRef) error {
	return s.inTx(func(tx *sql.Tx) error {
		res, err := tx.ExecContext(context.Background(),
			`UPDATE posts SET sent = ?, updated_at = ? WHERE key = ?`, sent, stamp(now), key)
		if err != nil {
			return err
		}
		if n, rerr := res.RowsAffected(); rerr != nil || n == 0 {
			return errors.Join(errors.New("no such post"), rerr)
		}
		if th.ID == "" {
			return nil
		}
		_, err = tx.ExecContext(context.Background(),
			`INSERT INTO threads (channel, key, id) VALUES (?, ?, ?)
			 ON CONFLICT (channel, key) DO UPDATE SET id = excluded.id`, th.Channel, th.Key, th.ID)
		return err
	})
}

// Posts lists every recorded post, oldest first.
func (s *Store) Posts() (out []Post, err error) {
	rows, err := s.db.QueryContext(context.Background(),
		`SELECT key, channel, parts, sent FROM posts ORDER BY created_at, rowid`)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		var p Post
		if serr := rows.Scan(&p.Key, &p.Channel, &p.Parts, &p.Sent); serr != nil {
			return nil, serr
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
