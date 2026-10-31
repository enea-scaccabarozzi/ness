package store_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// oldStore returns the path of a closed store at schema n.
func oldStore(t *testing.T, n int) string {
	t.Helper()
	path := freshPath(t)
	s, err := store.OpenWith(path, store.Migrations[:n])
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestBackupBeforeMigration(t *testing.T) {
	// covers: R-UPD-010, R-MSG-046
	n := store.Version() - 1
	path := oldStore(t, n)
	writeFile(t, store.BackupPath(path, n), "stale") // a stale file of the same name is replaced
	mustOpen(t, path)
	if v := userVersion(t, store.BackupPath(path, n)); v != n {
		t.Fatalf("backup schema = %d, want %d", v, n)
	}
	if v := userVersion(t, path); v != store.Version() {
		t.Fatalf("store schema = %d, want %d", v, store.Version())
	}
}

func TestNoBackupForFreshOrCurrent(t *testing.T) {
	// covers: R-UPD-010
	path := freshPath(t)
	for range 2 {
		s, err := store.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if m, err := filepath.Glob(path + ".bak-v*"); err != nil || len(m) != 0 {
		t.Fatalf("backups = %v, %v, want none", m, err)
	}
}

func TestKeepTwoBackups(t *testing.T) {
	// covers: R-UPD-011 — keeps the backup just written and the newest one below it
	n := store.Version() - 1
	path := oldStore(t, n)
	for _, k := range []int{n - 2, n - 1, n + 1} {
		writeFile(t, store.BackupPath(path, k), "old")
	}
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := filepath.Glob(path + ".bak-v*")
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	if want := []string{store.BackupPath(path, n-1), store.BackupPath(path, n)}; !slices.Equal(got, want) {
		t.Fatalf("backups = %v, want %v", got, want)
	}
}

func TestBackupConcurrentOpeners(t *testing.T) {
	// Review focus 1: no opener fails, and the backup holds the old schema, not a half-migrated copy.
	n := store.Version() - 1
	path := oldStore(t, n)
	errs := make(chan error, 4)
	for range 4 {
		go func() {
			s, err := store.Open(path)
			if err == nil {
				err = s.Close()
			}
			errs <- err
		}()
	}
	for range 4 {
		if err := <-errs; err != nil {
			t.Fatalf("Open: %v", err)
		}
	}
	if v := userVersion(t, store.BackupPath(path, n)); v != n {
		t.Fatalf("backup schema = %d, want %d", v, n)
	}
	if v := userVersion(t, path); v != store.Version() {
		t.Fatalf("store schema = %d, want %d", v, store.Version())
	}
	if m, _ := filepath.Glob(path + ".bak-v*.tmp*"); len(m) != 0 {
		t.Fatalf("temp files left: %v", m)
	}
}

func TestFailedBackupRefusesMigration(t *testing.T) {
	// covers: R-UPD-010 — no migration runs without its backup
	n := store.Version() - 1
	path := oldStore(t, n)
	// A non-empty directory at the backup path makes the rename into place fail.
	if err := os.MkdirAll(filepath.Join(store.BackupPath(path, n), "x"), 0o700); err != nil {
		t.Fatal(err)
	}
	if s, err := store.Open(path); err == nil {
		_ = s.Close()
		t.Fatal("Open migrated without a backup")
	}
	if v := userVersion(t, path); v != n {
		t.Fatalf("store schema = %d, want %d (untouched)", v, n)
	}
	if m, _ := filepath.Glob(path + ".bak-v*.tmp*"); len(m) != 0 {
		t.Fatalf("temp files left: %v", m)
	}
}
