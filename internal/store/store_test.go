package store_test

import (
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

var now = time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)

func mustOpen(t *testing.T, path string) *store.Store {
	t.Helper()
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}

func freshPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "ness", "state.sqlite3")
}

// raw opens the file with database/sql alone (the driver is registered by importing store).
func raw(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func userVersion(t *testing.T, path string) int {
	t.Helper()
	var v int
	if err := raw(t, path).QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestOpenCreatesDirectoryAndSchema(t *testing.T) {
	// covers: R-MSG-046, R-MSG-002
	path := freshPath(t)
	s := mustOpen(t, path)
	if got := userVersion(t, path); got != store.Version() {
		t.Fatalf("user_version = %d, want %d", got, store.Version())
	}
	if err := s.AddSession("s-aaaaaaaaaaaa", "root", store.Operator, now); err != nil {
		t.Fatalf("schema not usable: %v", err)
	}
	var mode string
	if err := raw(t, path).QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("journal_mode = %q, %v; want wal", mode, err)
	}
}

func TestOpenMigratesAnOlderStoreForward(t *testing.T) {
	// covers: R-MSG-046
	path := freshPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	// A version-0 store: the file exists with no schema at all.
	if _, err := raw(t, path).Exec("PRAGMA user_version = 0"); err != nil {
		t.Fatal(err)
	}
	s := mustOpen(t, path)
	if _, err := s.Session("s-aaaaaaaaaaaa"); !errors.Is(err, store.ErrNoSession) {
		t.Fatalf("after migration Session() = %v, want ErrNoSession (table exists, row absent)", err)
	}
	if got := userVersion(t, path); got != store.Version() {
		t.Fatalf("user_version = %d, want %d", got, store.Version())
	}
}

func TestOpenRefusesANewerStore(t *testing.T) {
	// covers: R-MSG-046
	path := freshPath(t)
	first, err := store.Open(path) // not mustOpen: closed here, once
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := raw(t, path).Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	_, err = store.Open(path)
	if !errors.Is(err, store.ErrNewer) {
		t.Fatalf("Open = %v, want ErrNewer", err)
	}
}

func TestConcurrentFirstOpen(t *testing.T) {
	// Review Focus 3: many processes create the store at once (fresh clone, twenty hooks).
	// covers: R-MSG-003
	path := freshPath(t)
	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := store.Open(path)
			if err != nil {
				errs <- err
				return
			}
			errs <- s.Close()
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
	if got := userVersion(t, path); got != store.Version() {
		t.Fatalf("user_version = %d, want %d", got, store.Version())
	}
}

// gitRepo makes a repository with one commit and one linked worktree, hermetically.
func gitRepo(t *testing.T) (repo, worktree string) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = dir
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("-c", "init.defaultBranch=main", "init", "-q", "repo")
	run("-C", "repo", "-c", "user.name=t", "-c", "user.email=t@example.invalid",
		"commit", "-q", "--allow-empty", "-m", "init")
	run("-C", "repo", "worktree", "add", "-q", filepath.Join(dir, "wt"))
	return filepath.Join(dir, "repo"), filepath.Join(dir, "wt")
}

func TestLocateIsSharedByWorktrees(t *testing.T) {
	// covers: R-MSG-001
	repo, wt := gitRepo(t)
	want := filepath.Join(repo, ".git", "ness", "state.sqlite3")
	for _, dir := range []string{repo, wt, filepath.Join(wt, ".")} {
		got, err := store.Locate(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("Locate(%s) = %s, want %s", dir, got, want)
		}
	}
	if _, err := store.Locate(t.TempDir()); err == nil {
		t.Fatal("Locate outside a repository succeeded")
	}
}

func TestMigratesV1ToV2(t *testing.T) {
	// covers: R-MSG-046
	path := freshPath(t)
	s := mustOpen(t, path)
	if err := s.AddSession("s-aaaaaaaaaaaa", "root", store.Operator, now); err != nil {
		t.Fatal(err)
	}
	v2 := append(append([]string{}, store.Migrations...), `ALTER TABLE sessions ADD COLUMN probe_x TEXT`)
	if err := s.MigrateWith(v2); err != nil {
		t.Fatal(err)
	}
	if got := userVersion(t, path); got != len(v2) {
		t.Fatalf("user_version = %d, want %d", got, len(v2))
	}
	var n int
	row := raw(t, path).QueryRowContext(t.Context(), `SELECT count(*) FROM sessions WHERE probe_x IS NULL`)
	if err := row.Scan(&n); err != nil || n != 1 {
		t.Fatalf("v2 column or the v1 row is missing: n=%d err=%v", n, err)
	}
	if err := s.MigrateWith(v2); err != nil { // current: no-op
		t.Fatal(err)
	}
	if err := s.MigrateWith(store.Migrations); !errors.Is(err, store.ErrNewer) {
		t.Fatalf("a v2 store under a v1 binary = %v, want ErrNewer", err)
	}
}

func TestOpeningACurrentStoreWritesNothing(t *testing.T) {
	// covers: R-CFG-036
	path := freshPath(t)
	first, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = first.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	holder := raw(t, path)
	holder.SetMaxOpenConns(1)
	conn, err := holder.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err = conn.ExecContext(t.Context(), "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	second, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open while another connection holds the write lock: %v", err)
	}
	if err = second.Close(); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("Open took %v with the write lock held; it must not wait on busy_timeout", took)
	}
	if _, err = conn.ExecContext(t.Context(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("opening a current store changed the file")
	}
}
