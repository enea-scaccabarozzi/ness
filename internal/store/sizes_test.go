package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// Rejects a migration that guesses a size for an existing row: the project's profiles decide it
// at the first resolve (FillSizes).
func TestMigrationTenLeavesEveryRowUnsized(t *testing.T) {
	// covers: R-SESS-154
	path := freshPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	db := raw(t, path)
	for _, q := range store.Migrations[:9] {
		if _, err := db.ExecContext(t.Context(), q); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range []string{
		`INSERT INTO sessions (id, scope, created_at) VALUES ('` + sA + `', 'root', '2026-10-08T00:00:00Z')`,
		`INSERT INTO sessions (id, parent, scope, created_at) VALUES ('` + sB + `', '` + sA + `', 'kid', '2026-10-08T00:00:00Z')`,
		`PRAGMA user_version = 9`,
	} {
		if _, err := db.ExecContext(t.Context(), q); err != nil {
			t.Fatal(err)
		}
	}
	s := mustOpen(t, path)
	for id, want := range map[string]string{sA: "", sB: ""} {
		if got, err := s.Session(id); err != nil || got.Size != want {
			t.Fatalf("size of %s = %q, %v; want %q", id, got.Size, err, want)
		}
	}
}

// Rejects a launch that drops the size or a resize that does not persist.
func TestLaunchRecordsSizeAndSetSizeChangesIt(t *testing.T) {
	// covers: R-SESS-155
	s := mustOpen(t, freshPath(t))
	if err := s.AddLaunch(store.Launch{ID: sA, Scope: "r", Size: "quick"}, now); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Session(sA); got.Size != "quick" {
		t.Fatalf("size = %q, want quick", got.Size)
	}
	if err := s.SetSize(sA, "full"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Session(sA); got.Size != "full" {
		t.Fatalf("size after SetSize = %q, want full", got.Size)
	}
	if err := s.SetSize("s-000000000000", "full"); err == nil {
		t.Fatal("SetSize of an unknown session succeeded")
	}
}

// Rejects a fill that sizes a child, overwrites a chosen size, or reports an unsized root wrongly.
func TestFillSizesGivesUnsizedRootsTheDefault(t *testing.T) {
	// covers: R-SESS-154
	s := mustOpen(t, freshPath(t))
	for _, l := range []store.Launch{
		{ID: sA, Scope: "a"}, {ID: sB, Scope: "b", Size: "quick"}, {ID: "s-cccccccccccc", Parent: sA, Scope: "c"},
	} {
		if err := s.AddLaunch(l, now); err != nil {
			t.Fatal(err)
		}
	}
	if has, err := s.HasUnsizedRoot(); err != nil || !has {
		t.Fatalf("HasUnsizedRoot = %v, %v; want true", has, err)
	}
	if err := s.FillSizes("program"); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{sA: "program", sB: "quick", "s-cccccccccccc": ""} {
		if got, _ := s.Session(id); got.Size != want {
			t.Errorf("size of %s = %q, want %q", id, got.Size, want)
		}
	}
	if has, _ := s.HasUnsizedRoot(); has {
		t.Fatal("HasUnsizedRoot after the fill")
	}
}
