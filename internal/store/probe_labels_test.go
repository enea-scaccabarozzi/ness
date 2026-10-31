package store_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestTwoLabelsAtOnePathAreTwoLiveProbes(t *testing.T) {
	// The same file started twice by one owner with different labels is two probes; each label is
	// found by ProbeOf, and the unlabelled lookup finds neither (R-PRB-084, R-PRB-085).
	// covers: R-PRB-084, R-PRB-085
	s := tree(t)
	a, err := s.StartProbe("probes/x.md", sA, "a", false, now, now)
	if err != nil || a.Label != "a" {
		t.Fatalf("StartProbe a = %+v, %v", a, err)
	}
	b, err := s.StartProbe("probes/x.md", sA, "b", false, now, now)
	if err != nil || b.ID == a.ID || b.Label != "b" {
		t.Fatalf("StartProbe b = %+v, %v; want a second probe", b, err)
	}
	if list, lerr := s.Probes(false); lerr != nil || len(list) != 2 || list[0].Label != "a" || list[1].Label != "b" {
		t.Fatalf("Probes = %+v, %v", list, lerr)
	}
	if got, gerr := s.ProbeOf("probes/x.md", sA, "a"); gerr != nil || got.ID != a.ID {
		t.Fatalf("ProbeOf a = %+v, %v", got, gerr)
	}
	if _, err = s.ProbeOf("probes/x.md", sA, ""); !errors.Is(err, store.ErrNoProbe) {
		t.Fatalf("ProbeOf without a label: %v", err)
	}
	un, err := s.StartProbe("probes/x.md", sA, "", false, now, now)
	if err != nil || un.ID == a.ID || un.ID == b.ID {
		t.Fatalf("an unlabelled start is a third probe: %+v, %v", un, err)
	}
}

func TestRestartWithLabelKeepsTheOtherWindow(t *testing.T) {
	// Restarting label a resets a's window; b's window is untouched (R-PRB-062, R-PRB-085).
	// covers: R-PRB-062, R-PRB-085
	s := tree(t)
	a, _ := s.StartProbe("probes/x.md", sA, "a", false, now, now)
	b, _ := s.StartProbe("probes/x.md", sA, "b", false, now, now)
	later := now.Add(24 * time.Hour)
	again, err := s.StartProbe("probes/x.md", sA, "a", false, later, later)
	if err != nil || again.ID != a.ID || !again.StartedAt.Equal(later) {
		t.Fatalf("restart a = %+v, %v", again, err)
	}
	if got, _ := s.Probe(b.ID); !got.StartedAt.Equal(now) || !got.NextRunAt.Equal(now) {
		t.Fatalf("b was touched by a's restart: %+v", got)
	}
}

func TestAdoptAndMergeRespectLabels(t *testing.T) {
	// A paused probe is adopted only by a start with its label; closing a session merges a moved
	// probe only into the manager's probe with the same label (R-PRB-071, R-PRB-070, R-PRB-085).
	// covers: R-PRB-070, R-PRB-071, R-PRB-085
	s := tree(t) // sA root; sB, sC children of sA
	pa, _ := s.StartProbe("probes/x.md", sB, "a", false, now, now)
	if err := s.SetProbeState(pa.ID, store.ProbePaused, "owner closed", now); err != nil {
		t.Fatal(err)
	}
	other, err := s.StartProbe("probes/x.md", sC, "b", false, now, now)
	if err != nil || other.ID == pa.ID {
		t.Fatalf("a start with another label adopted the paused probe: %+v, %v", other, err)
	}
	if got, _ := s.Probe(pa.ID); got.State != store.ProbePaused {
		t.Fatalf("paused probe a = %+v; want paused", got)
	}
	adopted, err := s.StartProbe("probes/x.md", sC, "a", true, now, now)
	if err != nil || adopted.ID != pa.ID || adopted.Owner != sC {
		t.Fatalf("adopting a: %+v, %v", adopted, err)
	}

	mgr, _ := s.StartProbe("probes/y.md", sA, "r1", false, now, now)
	same, _ := s.StartProbe("probes/y.md", sB, "r1", false, now, now)
	diff, _ := s.StartProbe("probes/y.md", sB, "r2", false, now, now)
	if n, terr := s.TransferProbes(sB, sA, now); terr != nil || n != 2 {
		t.Fatalf("TransferProbes = %d, %v", n, terr)
	}
	if got, _ := s.Probe(same.ID); got.State != store.ProbeStopped || got.Note != "merged into "+mgr.ID {
		t.Fatalf("same label = %+v; want merged", got)
	}
	if got, _ := s.Probe(diff.ID); got.State != store.ProbeActive || got.Owner != sA {
		t.Fatalf("other label = %+v; want moved and active", got)
	}
}

func TestMigrationEightKeepsOldProbesUnlabelled(t *testing.T) {
	// A version-7 store with a probe opens at the current version; the row has the empty label and
	// the live index is now per label (R-PRB-085).
	// covers: R-PRB-085
	path := freshPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	db := raw(t, path)
	for _, q := range store.Migrations[:7] {
		if _, err := db.ExecContext(t.Context(), q); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range []string{
		`INSERT INTO sessions (id, scope, created_at) VALUES ('` + sA + `', 'root', '2026-10-08T00:00:00Z')`,
		`INSERT INTO probes (id, path, owner, state, started_at) VALUES
		 ('p-old', 'probes/x.md', '` + sA + `', 'active', '2026-10-08T00:00:00Z')`,
		`PRAGMA user_version = 7`,
	} {
		if _, err := db.ExecContext(t.Context(), q); err != nil {
			t.Fatal(err)
		}
	}
	s := mustOpen(t, path)
	if got, err := s.ProbeOf("probes/x.md", sA, ""); err != nil || got.ID != "p-old" || got.Label != "" {
		t.Fatalf("old probe = %+v, %v", got, err)
	}
	if again, err := s.StartProbe("probes/x.md", sA, "", false, now, now); err != nil || again.ID != "p-old" {
		t.Fatalf("an unlabelled restart must keep the old row: %+v, %v", again, err)
	}
	if lab, err := s.StartProbe("probes/x.md", sA, "v2", false, now, now); err != nil || lab.ID == "p-old" {
		t.Fatalf("a labelled start must be a second row: %+v, %v", lab, err)
	}
}

func TestValidLabel(t *testing.T) {
	// covers: R-PRB-086
	for label, ok := range map[string]bool{
		"": true, "a": true, "3fa9c2d1": true, "v1.2.3": true, "rel_1-b": true, "A9": true,
		"-x": false, ".x": false, "a b": false, "a/b": false, "a:b": false, "a\nb": false,
		"0123456789012345678901234567890123": false,
	} {
		if got := store.ValidLabel(label); got != ok {
			t.Errorf("ValidLabel(%q) = %v, want %v", label, got, ok)
		}
	}
}
