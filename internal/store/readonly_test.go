package store_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestDeadRoutes(t *testing.T) {
	// covers: R-DOC-012
	s := tree(t)
	live, _, err := s.Send(sA, sC, store.IntentRequest, "to a live session", false, now)
	if err != nil {
		t.Fatal(err)
	}
	dead, _, err := s.Send(sA, sB, store.IntentRequest, "to be closed", false, now)
	if err != nil {
		t.Fatal(err)
	}
	deferred, _, err := s.Send(sA, sB, store.IntentRequest, "deferred then closed", false, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Reply(deferred, sB, "later", store.Deferred, now); err != nil {
		t.Fatal(err)
	}
	answered, _, err := s.Send(sA, sB, store.IntentRequest, "answered", false, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Reply(answered, sB, "done", store.Resolved, now); err != nil {
		t.Fatal(err)
	}
	if err = s.CloseSession(sB); err != nil {
		t.Fatal(err)
	}
	got, err := s.DeadRoutes()
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got) // same-instant items have no stable order
	want := []string{dead + " to " + sB + " (closed)", deferred + " to " + sB + " (closed)"}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("DeadRoutes = %v, want %v (live item %s and the answered one must not be listed)", got, want, live)
	}
	if _, err = s.Retire(sB, now); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.DeadRoutes(); len(got) != 0 {
		t.Fatalf("retired items are not dead routes, got %v", got)
	}
}

func TestOpenReadOnly(t *testing.T) {
	// covers: R-DOC-001
	path := freshPath(t)
	if _, err := store.OpenReadOnly(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing store: %v, want fs.ErrNotExist", err)
	}
	if _, err := os.Stat(filepath.Dir(path)); err == nil {
		t.Fatal("OpenReadOnly created the store directory")
	}
	w := mustOpen(t, path)
	if err := w.AddSession(sA, "scope", store.Operator, now); err != nil {
		t.Fatal(err)
	}
	ro, err := store.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ro.Close() }()
	if all, serr := ro.Sessions(true); serr != nil || len(all) != 1 {
		t.Fatalf("read: %v, %d sessions", serr, len(all))
	}
	if werr := ro.CloseSession(sA); werr == nil {
		t.Fatal("a read-only store accepted a write")
	}
}

func TestOpenReadOnlyRefusesOtherSchemas(t *testing.T) {
	// covers: R-DOC-001
	path := freshPath(t)
	w := mustOpen(t, path)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	db := raw(t, path)
	if _, err := db.Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.OpenReadOnly(path); !errors.Is(err, store.ErrNewer) {
		t.Fatalf("newer store: %v, want ErrNewer", err)
	}
	db = raw(t, path)
	if _, err := db.Exec("PRAGMA user_version = 1"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.OpenReadOnly(path); err == nil {
		t.Fatal("an older store opened read-only without a migration")
	}
}

func TestOpenReadOnlyCreatesNoWalFiles(t *testing.T) {
	// covers: R-DOC-001
	path := freshPath(t)
	w, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = w.Close(); err != nil { // the last close checkpoints and removes -wal and -shm
		t.Fatal(err)
	}
	// A clean close checkpoints the log; drop any leftover so the read-only open starts from none.
	for _, ext := range []string{"-wal", "-shm"} {
		if rerr := os.Remove(path + ext); rerr != nil && !os.IsNotExist(rerr) {
			t.Fatal(rerr)
		}
	}
	ro, err := store.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ro.Sessions(true); err != nil {
		t.Fatal(err)
	}
	if err = ro.Close(); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{"-wal", "-shm"} {
		if _, serr := os.Stat(path + ext); serr == nil {
			t.Fatalf("OpenReadOnly created %s", path+ext)
		}
	}
}
