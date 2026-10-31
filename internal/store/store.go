// Package store is ness's SQLite state: the sessions of a project and the messages between
// them, shared by the primary checkout and every worktree (R-MSG-001). It is the only store ness
// reads (R-MSG-004); the old harness stores are never opened.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver, registered as "sqlite" (R-MSG-002)
)

// Operator is the sender, recipient or parent value of the operator: no session (R-SESS-037).
const Operator = ""

// ErrNewer is returned by Open when the store was written by a newer ness (R-MSG-046).
var ErrNewer = errors.New("store schema is newer than this ness")

// ErrOlder is returned by OpenReadOnly when the store is behind this ness; any ness command migrates it.
var ErrOlder = errors.New("store schema is older than this ness")

// SchemaError is OpenReadOnly's refusal of a store of another schema than this binary's. It
// unwraps to ErrNewer or ErrOlder.
type SchemaError struct {
	Store, Binary int
}

func (e *SchemaError) Error() string {
	if e.Store > e.Binary {
		return fmt.Sprintf("%v: store schema %d, binary schema %d; upgrade ness", ErrNewer, e.Store, e.Binary)
	}
	return fmt.Sprintf("%v: store schema %d, binary schema %d; any ness command migrates it",
		ErrOlder, e.Store, e.Binary)
}

// Unwrap returns ErrNewer for a newer store and ErrOlder for an older one.
func (e *SchemaError) Unwrap() error {
	if e.Store > e.Binary {
		return ErrNewer
	}
	return ErrOlder
}

// Store is an open state database.
type Store struct {
	db   *sql.DB
	path string
}

// Locate returns the store path of the checkout containing dir: <git-common-dir>/ness/state.sqlite3,
// the same file from the primary checkout and from every worktree.
func Locate(dir string) (string, error) {
	args := []string{"-C", dir, "rev-parse", "--path-format=absolute", "--git-common-dir"}
	cmd := exec.CommandContext(context.Background(), "git", args...) //nolint:gosec // fixed git subcommand
	cmd.Env = CleanGitEnv(os.Environ())
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s is not inside a git repository", dir)
	}
	return filepath.Join(strings.TrimSpace(string(out)), "ness", "state.sqlite3"), nil
}

// Open opens or creates the store at path and brings its schema to Version, copying an older
// store to BackupPath first (R-UPD-010).
//
// Connection settings, one reason each: journal_mode=WAL, so readers (inbox, status, the
// worker) never block a writer and a crash cannot corrupt the file; busy_timeout=5000, so a
// second process waits up to 5 s for the write lock instead of failing with SQLITE_BUSY;
// _txlock=immediate, so every transaction takes the write lock at BEGIN and two processes can
// never deadlock upgrading read locks; foreign_keys=ON, so an item can only name sessions that
// exist. One connection per process keeps the pragmas and the transactions serial.
func Open(path string) (*Store, error) { return open(path, migrations) }

// open is Open with the migrations to bring the schema to, so a test can build an older store.
func open(path string, migs []string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	dsn := "file:" + path + "?_txlock=immediate&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, path: path}
	if err = s.walMode(); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	if err = s.migrate(migs); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	return s, nil
}

// OpenReadOnly opens an existing store without creating, migrating or writing anything, for
// ness doctor (R-DOC-001). A missing file is fs.ErrNotExist; a store of another schema version is
// refused with a *SchemaError: newer wraps ErrNewer, older asks for any ness command to migrate it.
func OpenReadOnly(path string) (*Store, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	// Without a -wal file no writer is live, and a plain read-only open of a WAL database would
	// create -wal and -shm; immutable=1 reads the file as it is and touches nothing.
	mode := "mode=ro&immutable=1"
	if _, werr := os.Stat(path + "-wal"); werr == nil {
		mode = "mode=ro"
	}
	db, err := sql.Open("sqlite", "file:"+path+"?"+mode+"&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	v, err := s.version()
	if err == nil && v != Version() {
		err = &SchemaError{Store: v, Binary: Version()}
	}
	if err != nil {
		return nil, errors.Join(err, db.Close())
	}
	return s, nil
}

// walMode switches the file to WAL once; the mode is persistent, so later opens only read it.
// SQLite answers a mode switch that races another connection with SQLITE_BUSY without calling
// the busy handler, so the first-open race retries here.
// ponytail: fixed 10 ms retry for up to 5 s, matching busy_timeout; only fresh stores pay it.
func (s *Store) walMode() error {
	ctx := context.Background()
	deadline := time.Now().Add(5 * time.Second) // a retry bound, not a stamp: Deps.Now does not apply
	for {
		var mode string
		err := s.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode)
		if err == nil && mode == "wal" {
			return nil
		}
		if err == nil {
			err = s.db.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&mode)
			if err == nil && mode == "wal" {
				return nil
			}
		}
		if (err != nil && !strings.Contains(err.Error(), "SQLITE_BUSY")) || time.Now().After(deadline) {
			return fmt.Errorf("store: set WAL mode: %w (mode %q)", err, mode)
		}
		<-time.After(10 * time.Millisecond)
	}
}

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

// inTx runs fn in one immediate transaction: committed when fn returns nil, rolled back otherwise.
func (s *Store) inTx(fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	if err = fn(tx); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	return tx.Commit()
}

// timeLayout parses stored stamps; stampLayout writes them at a fixed nine-digit fraction, because
// RFC3339Nano drops trailing zeros and "10:00:00Z" would sort after "10:00:00.5Z" as text.
const (
	timeLayout  = time.RFC3339Nano
	stampLayout = "2006-01-02T15:04:05.000000000Z07:00"
)

// stamp formats a time for storage: RFC 3339 in UTC, so text order is time order.
func stamp(t time.Time) string { return t.UTC().Format(stampLayout) }

func parseStamp(s string) time.Time {
	t, err := time.Parse(timeLayout, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func nullStamp(ns sql.NullString) time.Time {
	if !ns.Valid {
		return time.Time{}
	}
	return parseStamp(ns.String)
}

// nullable stores the operator (empty id) as NULL so foreign keys hold.
func nullable(id string) any {
	if id == Operator {
		return nil
	}
	return id
}
