BEGIN TRANSACTION;
CREATE TABLE heartbeat (id INTEGER PRIMARY KEY CHECK (id = 1), at TEXT NOT NULL, version TEXT NOT NULL);
CREATE TABLE "items" (
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
CREATE TABLE posts (
		key        TEXT PRIMARY KEY,
		channel    TEXT NOT NULL,
		parts      INTEGER NOT NULL,
		sent       INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		claimed_by TEXT,
		claimed_at TEXT
	);
CREATE TABLE probes (
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
	, label TEXT NOT NULL DEFAULT '');
CREATE TABLE sessions (
		id            TEXT PRIMARY KEY,
		parent        TEXT REFERENCES sessions(id),
		scope         TEXT NOT NULL,
		state         TEXT NOT NULL DEFAULT 'open' CHECK (state IN ('open', 'closed')),
		created_at    TEXT NOT NULL,
		ready_at      TEXT,
		boundary_note TEXT,
		boundary_at   TEXT
	, title TEXT NOT NULL DEFAULT '', tier TEXT NOT NULL DEFAULT '', effort TEXT NOT NULL DEFAULT '', client TEXT NOT NULL DEFAULT 'claude', phase TEXT NOT NULL DEFAULT 'ready', phase_at TEXT NOT NULL DEFAULT '', run INTEGER NOT NULL DEFAULT 0, pane TEXT NOT NULL DEFAULT '', tab TEXT NOT NULL DEFAULT '', agent TEXT NOT NULL DEFAULT '', client_session TEXT NOT NULL DEFAULT '', worktree TEXT NOT NULL DEFAULT '', branch TEXT NOT NULL DEFAULT '', record TEXT NOT NULL DEFAULT '', base TEXT NOT NULL DEFAULT '', adopted INTEGER NOT NULL DEFAULT 0, router_reason TEXT NOT NULL DEFAULT '', integrated TEXT NOT NULL DEFAULT '', integrated_at TEXT, closed_at TEXT, close_note TEXT NOT NULL DEFAULT '', keep_reason TEXT NOT NULL DEFAULT '', first_ready_at TEXT, size TEXT NOT NULL DEFAULT '', parked_at TEXT, parked_until TEXT, parked_note TEXT);
INSERT INTO "sessions" VALUES('s-fixture',NULL,'fixture','open','2026-10-31T00:00:00Z','2026-10-31T00:00:00Z',NULL,NULL,'fixture','','','claude','ready','2026-10-31T00:00:00Z',0,'','','','','','','','main',0,'','',NULL,NULL,'','','2026-10-31T00:00:00Z','program',NULL,NULL,NULL);
CREATE TABLE spawns (
		session     TEXT NOT NULL,
		agent_type  TEXT NOT NULL,
		kind        TEXT NOT NULL,
		requested   TEXT NOT NULL,
		recommended TEXT NOT NULL,
		verdict     TEXT NOT NULL,
		reason      TEXT NOT NULL,
		at          TEXT NOT NULL);
CREATE TABLE threads (
		channel TEXT NOT NULL,
		key     TEXT NOT NULL,
		id      TEXT NOT NULL,
		PRIMARY KEY (channel, key)
	);
CREATE INDEX items_recipient ON items (recipient, disposed_at);
CREATE UNIQUE INDEX probes_live ON probes (path, owner, label) WHERE state <> 'stopped';
CREATE INDEX items_sender ON items (sender, disposed_at);
CREATE INDEX spawns_session ON spawns (session, at);
COMMIT;
PRAGMA user_version = 12;
