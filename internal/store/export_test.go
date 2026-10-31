package store

import "database/sql"

// Migrations and MigrateWith let tests run a migration the shipped list does not have yet.
var Migrations = migrations

func (s *Store) MigrateWith(migs []string) error { return s.migrate(migs) }

// RawDB is the connection, for the invariant checks of the concurrency test.
func (s *Store) RawDB() *sql.DB { return s.db }

// OpenWith opens through the same path as Open but migrates only to len(migs), so a test builds a
// store at schema N with OpenWith(path, Migrations[:N]).
func OpenWith(path string, migs []string) (*Store, error) { return open(path, migs) }
