package store_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// Rejects a spawn log that drops or swaps a field, or loses the time.
func TestSpawnRoundTrip(t *testing.T) {
	// covers: R-HOOK-033
	s := tree(t)
	in := store.Spawn{Session: sA, AgentType: "general-purpose", Kind: "search", Requested: "lean",
		Recommended: "economy", Verdict: "escalated", Reason: "needs judgment", At: now}
	if err := s.AddSpawn(in); err != nil {
		t.Fatal(err)
	}
	got, err := s.Spawns(sA)
	if err != nil || len(got) != 1 {
		t.Fatalf("Spawns = %+v, %v", got, err)
	}
	if !got[0].At.Equal(now) {
		t.Fatalf("At = %v, want %v", got[0].At, now)
	}
	got[0].At = in.At
	if got[0] != in {
		t.Fatalf("round trip = %+v, want %+v", got[0], in)
	}
}

// Rejects an unordered read (insertion or rowid order with out-of-order times), and a session
// filter that leaks other sessions or a "" that means "none" instead of "all".
func TestSpawnsOrderAndFilter(t *testing.T) {
	// covers: R-HOOK-033
	s := tree(t)
	add := func(session, verdict string, at time.Time) {
		t.Helper()
		if err := s.AddSpawn(store.Spawn{Session: session, AgentType: "ness-sweep", Kind: "search",
			Requested: "economy", Recommended: "economy", Verdict: verdict, At: at}); err != nil {
			t.Fatal(err)
		}
	}
	add(sA, "allowed", now.Add(2*time.Minute))
	add(sB, "refused", now.Add(time.Minute))
	add(sA, "fork", now)
	add(sA, "would-refuse", now.Add(3*time.Minute))
	a, err := s.Spawns(sA)
	if err != nil || len(a) != 3 || a[0].Verdict != "fork" || a[1].Verdict != "allowed" || a[2].Verdict != "would-refuse" {
		t.Fatalf("Spawns(sA) = %+v, %v; want fork, allowed, would-refuse", a, err)
	}
	all, err := s.Spawns("")
	if err != nil || len(all) != 4 || all[1].Session != sB || all[1].Verdict != "refused" {
		t.Fatalf("Spawns(\"\") = %+v, %v; want four, oldest first", all, err)
	}
	if none, err := s.Spawns(sC); err != nil || len(none) != 0 {
		t.Fatalf("Spawns(sC) = %+v, %v; want none", none, err)
	}
}

// Rejects a migration that rebuilds or wipes existing tables instead of adding the spawn log.
func TestSpawnMigrationKeepsRows(t *testing.T) {
	// covers: R-HOOK-033
	path := freshPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	db := raw(t, path)
	for _, q := range store.Migrations[:10] {
		if _, err := db.ExecContext(t.Context(), q); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range []string{
		`INSERT INTO sessions (id, scope, created_at, size) VALUES ('` + sA + `', 'root', '2026-10-08T00:00:00Z', 'quick')`,
		`PRAGMA user_version = 10`,
	} {
		if _, err := db.ExecContext(t.Context(), q); err != nil {
			t.Fatal(err)
		}
	}
	s := mustOpen(t, path)
	if got, err := s.Session(sA); err != nil || got.Size != "quick" || got.Scope != "root" {
		t.Fatalf("session after migration = %+v, %v", got, err)
	}
	if got, err := s.Spawns(""); err != nil || len(got) != 0 {
		t.Fatalf("Spawns after migration = %+v, %v; want empty", got, err)
	}
	if err := s.AddSpawn(store.Spawn{Session: sA, Verdict: "allowed", At: now}); err != nil {
		t.Fatal(err)
	}
	if store.Version() < 11 || !strings.Contains(store.Migrations[10], "CREATE TABLE spawns") {
		t.Fatalf("Version = %d; the spawn log is migration 11", store.Version())
	}
}
