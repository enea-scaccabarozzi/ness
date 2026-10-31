package store

import "database/sql"

// Migrations and MigrateWith let tests run a migration the shipped list does not have yet.
var Migrations = migrations

func (s *Store) MigrateWith(migs []string) error { return s.migrate(migs) }

// RawDB is the connection, for the invariant checks of the concurrency test.
func (s *Store) RawDB() *sql.DB { return s.db }
