package store

import (
	"context"
	"database/sql"
	"fmt"
)

// migrations[i] brings the schema from version i to i+1. Append only; a shipped entry is never
// edited (R-MSG-046). P3 appended migration 2, P3b migration 3, P5a migration 4, P5b migration 5,
// CX-21 migration 6, B5 migration 7 (checkpoint, ask, alerted_at); later plans append after.
var migrations = []string{
	`CREATE TABLE sessions (
		id            TEXT PRIMARY KEY,
		parent        TEXT REFERENCES sessions(id),
		scope         TEXT NOT NULL,
		state         TEXT NOT NULL DEFAULT 'open' CHECK (state IN ('open', 'closed')),
		created_at    TEXT NOT NULL,
		ready_at      TEXT,
		boundary_note TEXT,
		boundary_at   TEXT
	);
	CREATE TABLE items (
		id          TEXT PRIMARY KEY,
		key         TEXT NOT NULL UNIQUE,
		sender      TEXT REFERENCES sessions(id),
		recipient   TEXT REFERENCES sessions(id),
		intent      TEXT NOT NULL CHECK (intent IN ('request', 'fyi', 'decision', 'handoff', 'reply')),
		body        TEXT NOT NULL,
		urgent      INTEGER NOT NULL DEFAULT 0,
		created_at  TEXT NOT NULL,
		pulled_at   TEXT,
		shown_at    TEXT,
		disposition TEXT CHECK (disposition IN ('resolved', 'declined', 'deferred', 'accepted')),
		answer      TEXT,
		disposed_at TEXT
	);
	CREATE INDEX items_recipient ON items (recipient, disposed_at);`,

	// Migration 2 (P3): launch state per session, retired items, the worker heartbeat.
	`ALTER TABLE sessions ADD COLUMN title TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN tier TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN effort TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN client TEXT NOT NULL DEFAULT 'claude';
	ALTER TABLE sessions ADD COLUMN phase TEXT NOT NULL DEFAULT 'ready';
	ALTER TABLE sessions ADD COLUMN phase_at TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN run INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE sessions ADD COLUMN pane TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN tab TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN agent TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN client_session TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN worktree TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN branch TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN record TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN base TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN adopted INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE sessions ADD COLUMN router_reason TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN integrated TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN integrated_at TEXT;
	ALTER TABLE sessions ADD COLUMN closed_at TEXT;
	ALTER TABLE sessions ADD COLUMN close_note TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN keep_reason TEXT NOT NULL DEFAULT '';
	ALTER TABLE items ADD COLUMN retired_at TEXT;
	CREATE TABLE heartbeat (id INTEGER PRIMARY KEY CHECK (id = 1), at TEXT NOT NULL, version TEXT NOT NULL);`,

	// Migration 3 (P3b): the first ready mark, never cleared, so a relaunch that stalls before its
	// run is bound does not make a session that ran look like one that never did (R-SESS-066).
	`ALTER TABLE sessions ADD COLUMN first_ready_at TEXT;
	UPDATE sessions SET first_ready_at = ready_at;`,

	// Migration 4 (P5a): the thread key → platform id mapping per channel (R-NTF-040) and the
	// progress of multi-part posts, so a retry resumes at the first unsent part (R-NTF-007).
	`CREATE TABLE threads (
		channel TEXT NOT NULL,
		key     TEXT NOT NULL,
		id      TEXT NOT NULL,
		PRIMARY KEY (channel, key)
	);
	CREATE TABLE posts (
		key        TEXT PRIMARY KEY,
		channel    TEXT NOT NULL,
		parts      INTEGER NOT NULL,
		sent       INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		claimed_by TEXT,
		claimed_at TEXT
	);`,

	// Migration 5 (P5b): probes and the probe a wake item belongs to (R-PRB-038, R-PRB-066). Only live
	// rows are unique per path and owner, so a stopped probe can be started again; notified_state is
	// the last state posted, which confirm-gated notifications compare against (R-PRB-037); confirmed_state
	// is the last state that reached its confirm streak, which wake: change compares against (R-PRB-032).
	`CREATE TABLE probes (
		id             TEXT PRIMARY KEY,
		path           TEXT NOT NULL,
		owner          TEXT NOT NULL REFERENCES sessions(id),
		state          TEXT NOT NULL CHECK (state IN ('active', 'paused', 'stopped')),
		started_at     TEXT NOT NULL,
		next_run_at    TEXT,
		last_run_at    TEXT,
		last_state     TEXT NOT NULL DEFAULT '',
		streak         INTEGER NOT NULL DEFAULT 0,
		runs           INTEGER NOT NULL DEFAULT 0,
		evidence       TEXT NOT NULL DEFAULT '',
		hash           TEXT NOT NULL DEFAULT '',
		file_error     TEXT NOT NULL DEFAULT '',
		verdict        TEXT NOT NULL DEFAULT '',
		verdict_note   TEXT NOT NULL DEFAULT '',
		verdict_at     TEXT,
		needs_edit     TEXT NOT NULL DEFAULT '',
		final_wake     INTEGER NOT NULL DEFAULT 0,
		note           TEXT NOT NULL DEFAULT '',
		stopped_at     TEXT,
		notified_state TEXT NOT NULL DEFAULT '',
		confirmed_state TEXT NOT NULL DEFAULT ''
	);
	CREATE UNIQUE INDEX probes_live ON probes (path, owner) WHERE state <> 'stopped';
	ALTER TABLE items ADD COLUMN probe TEXT;`,

	// Migration 6 (CX-21): the withdrawn and returned dispositions (R-MSG-060, R-MSG-064). SQLite
	// cannot alter a CHECK, so items is rebuilt with every column it has at version 5.
	`CREATE TABLE items_v6 (
		id          TEXT PRIMARY KEY,
		key         TEXT NOT NULL UNIQUE,
		sender      TEXT REFERENCES sessions(id),
		recipient   TEXT REFERENCES sessions(id),
		intent      TEXT NOT NULL CHECK (intent IN ('request', 'fyi', 'decision', 'handoff', 'reply')),
		body        TEXT NOT NULL,
		urgent      INTEGER NOT NULL DEFAULT 0,
		created_at  TEXT NOT NULL,
		pulled_at   TEXT,
		shown_at    TEXT,
		disposition TEXT CHECK (disposition IN
			('resolved', 'declined', 'deferred', 'accepted', 'returned', 'withdrawn')),
		answer      TEXT,
		disposed_at TEXT,
		retired_at  TEXT,
		probe       TEXT
	);
	INSERT INTO items_v6 (id, key, sender, recipient, intent, body, urgent, created_at, pulled_at,
		shown_at, disposition, answer, disposed_at, retired_at, probe)
	SELECT id, key, sender, recipient, intent, body, urgent, created_at, pulled_at,
		shown_at, disposition, answer, disposed_at, retired_at, probe FROM items;
	DROP TABLE items;
	ALTER TABLE items_v6 RENAME TO items;
	CREATE INDEX items_recipient ON items (recipient, disposed_at);`,

	// Migration 7 (B5): the checkpoint intent and the ask and alerted_at columns (R-MSG-067).
	// Existing rows count as alerted so a new channel replays no backlog.
	`CREATE TABLE items_v7 (
		id          TEXT PRIMARY KEY,
		key         TEXT NOT NULL UNIQUE,
		sender      TEXT REFERENCES sessions(id),
		recipient   TEXT REFERENCES sessions(id),
		intent      TEXT NOT NULL CHECK (intent IN
			('request', 'fyi', 'decision', 'handoff', 'reply', 'checkpoint')),
		body        TEXT NOT NULL,
		urgent      INTEGER NOT NULL DEFAULT 0,
		created_at  TEXT NOT NULL,
		pulled_at   TEXT,
		shown_at    TEXT,
		disposition TEXT CHECK (disposition IN
			('resolved', 'declined', 'deferred', 'accepted', 'returned', 'withdrawn')),
		answer      TEXT,
		disposed_at TEXT,
		retired_at  TEXT,
		probe       TEXT,
		ask         INTEGER NOT NULL DEFAULT 0,
		alerted_at  TEXT
	);
	INSERT INTO items_v7 (id, key, sender, recipient, intent, body, urgent, created_at, pulled_at,
		shown_at, disposition, answer, disposed_at, retired_at, probe, alerted_at)
	SELECT id, key, sender, recipient, intent, body, urgent, created_at, pulled_at,
		shown_at, disposition, answer, disposed_at, retired_at, probe, created_at FROM items;
	DROP TABLE items;
	ALTER TABLE items_v7 RENAME TO items;
	CREATE INDEX items_recipient ON items (recipient, disposed_at);`,

	// Migration 8 (field 2): a probe label, so one file can run once per release (R-PRB-084, R-PRB-085).
	// Existing rows keep the empty label; live rows are unique per path, owner and label.
	`ALTER TABLE probes ADD COLUMN label TEXT NOT NULL DEFAULT '';
	DROP INDEX probes_live;
	CREATE UNIQUE INDEX probes_live ON probes (path, owner, label) WHERE state <> 'stopped';`,

	// Migration 9 (field 2): the stall clock asks for the answers to a sender's items on every pass.
	`CREATE INDEX items_sender ON items (sender, disposed_at);`,

	// Migration 10 (field 2): the work size of a root. Existing rows stay unsized: the project's
	// profiles pick the size of its old roots at the first command (FillSizes); children have none.
	`ALTER TABLE sessions ADD COLUMN size TEXT NOT NULL DEFAULT ''`,

	// Migration 11 (model select): the pre-tool hook's decision on every subagent call (R-HOOK-033).
	`CREATE TABLE spawns (
		session     TEXT NOT NULL,
		agent_type  TEXT NOT NULL,
		kind        TEXT NOT NULL,
		requested   TEXT NOT NULL,
		recommended TEXT NOT NULL,
		verdict     TEXT NOT NULL,
		reason      TEXT NOT NULL,
		at          TEXT NOT NULL);
	CREATE INDEX spawns_session ON spawns (session, at);`,

	// Migration 12 (liveness): the parked record a boundary leaves, which the compaction does not
	// clear (R-MSG-034, R-WRK-099). Existing boundaries are not parks.
	`ALTER TABLE sessions ADD COLUMN parked_at TEXT;
	ALTER TABLE sessions ADD COLUMN parked_until TEXT;
	ALTER TABLE sessions ADD COLUMN parked_note TEXT;`,
}

// Version is the schema version this binary writes.
func Version() int { return len(migrations) }

// migrate brings the schema to len(migs). A current store is read and left alone: no write
// lock, so the hot path of every hook never contends. Otherwise the work runs in one immediate
// transaction that re-reads the version, so concurrent first openers serialize and the second
// one finds the work done. A newer store is refused.
func (s *Store) migrate(migs []string) error {
	v, err := s.version()
	if err != nil {
		return err
	}
	if v == len(migs) {
		return nil
	}
	return s.inTx(func(tx *sql.Tx) error {
		var cur int
		if qerr := tx.QueryRowContext(context.Background(), "PRAGMA user_version").Scan(&cur); qerr != nil {
			return qerr
		}
		if cur > len(migs) {
			return fmt.Errorf("%w: store schema %d, binary schema %d; upgrade ness", ErrNewer, cur, len(migs))
		}
		for i := cur; i < len(migs); i++ {
			if _, merr := tx.ExecContext(context.Background(), migs[i]); merr != nil {
				return fmt.Errorf("migration %d: %w", i+1, merr)
			}
		}
		_, uerr := tx.ExecContext(context.Background(), fmt.Sprintf("PRAGMA user_version = %d", len(migs)))
		return uerr
	})
}

func (s *Store) version() (int, error) {
	var v int
	err := s.db.QueryRowContext(context.Background(), "PRAGMA user_version").Scan(&v)
	return v, err
}
